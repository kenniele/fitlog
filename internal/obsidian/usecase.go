// Package obsidian publishes Markdown notes from one explicitly configured
// folder. The vault may be synchronised independently by obsidian-git; fitlog
// only ever opens it read-only.
package obsidian

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"strings"
)

const maxArticleBytes = 2 << 20 // 2 MiB

var (
	ErrNotConfigured = errors.New("obsidian articles path is not configured")
	ErrNoArticles    = errors.New("no markdown articles found")
	ErrInvalidID     = errors.New("invalid article id")
	ErrTooLarge      = errors.New("article is too large")
)

type Request struct {
	ID     string
	Random bool
}

func RandomRequest() Request           { return Request{Random: true} }
func ArticleRequest(id string) Request { return Request{ID: id} }

type RawArticle struct {
	ID       string
	Filename string
	Content  string
}

type Article struct {
	ID       string
	Title    string
	Markdown string
}

type PublishedArticle struct {
	ID    string
	Title string
	HTML  string
}

// TokenCipher turns a relative vault path into an opaque, authenticated token.
// auth.Cipher implements this interface with AES-GCM.
type TokenCipher interface {
	SealString(string) ([]byte, error)
	OpenString([]byte) (string, error)
}

// ReportUseCase follows the same application pipeline as the health modules.
type ReportUseCase interface {
	Fetch(context.Context, Request) (RawArticle, error)
	Transform(RawArticle) Article
	Format(Article) string
	Execute(context.Context, Request) (PublishedArticle, error)
}

type UseCase struct {
	root   string
	tokens TokenCipher
}

func NewUseCase(root string, tokens TokenCipher) *UseCase {
	return &UseCase{root: strings.TrimSpace(root), tokens: tokens}
}

func (u *UseCase) Fetch(ctx context.Context, request Request) (RawArticle, error) {
	if u.root == "" || u.tokens == nil {
		return RawArticle{}, ErrNotConfigured
	}
	root, err := filepath.Abs(u.root)
	if err != nil {
		return RawArticle{}, fmt.Errorf("resolve obsidian root: %w", err)
	}
	if resolvedRoot, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolvedRoot
	}

	var relative string
	if request.Random {
		paths, err := markdownFiles(ctx, root)
		if err != nil {
			return RawArticle{}, err
		}
		if len(paths) == 0 {
			return RawArticle{}, ErrNoArticles
		}
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(paths))))
		if err != nil {
			return RawArticle{}, fmt.Errorf("choose random article: %w", err)
		}
		relative = paths[index.Int64()]
	} else {
		sealed, err := base64.RawURLEncoding.DecodeString(request.ID)
		if err != nil {
			return RawArticle{}, ErrInvalidID
		}
		relative, err = u.tokens.OpenString(sealed)
		if err != nil {
			return RawArticle{}, ErrInvalidID
		}
	}

	fullPath, cleanRelative, err := safeMarkdownPath(root, relative)
	if err != nil {
		return RawArticle{}, err
	}
	linkInfo, err := os.Lstat(fullPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return RawArticle{}, ErrInvalidID
		}
		return RawArticle{}, fmt.Errorf("stat article: %w", err)
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 || !linkInfo.Mode().IsRegular() {
		return RawArticle{}, ErrInvalidID
	}
	if linkInfo.Size() > maxArticleBytes {
		return RawArticle{}, ErrTooLarge
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return RawArticle{}, fmt.Errorf("read article: %w", err)
	}
	sealed, err := u.tokens.SealString(cleanRelative)
	if err != nil {
		return RawArticle{}, fmt.Errorf("seal article id: %w", err)
	}
	id := base64.RawURLEncoding.EncodeToString(sealed)
	return RawArticle{ID: id, Filename: filepath.Base(cleanRelative), Content: string(content)}, nil
}

func (u *UseCase) Transform(raw RawArticle) Article {
	markdown, frontmatterTitle := stripFrontmatter(raw.Content)
	title, markdown := extractTitle(markdown, frontmatterTitle, raw.Filename)
	return Article{ID: raw.ID, Title: title, Markdown: markdown}
}

func (u *UseCase) Format(article Article) string { return formatPage(article) }

func (u *UseCase) Execute(ctx context.Context, request Request) (PublishedArticle, error) {
	raw, err := u.Fetch(ctx, request)
	if err != nil {
		return PublishedArticle{}, err
	}
	article := u.Transform(raw)
	return PublishedArticle{ID: article.ID, Title: article.Title, HTML: u.Format(article)}, nil
}

func markdownFiles(ctx context.Context, root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path != root && strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, relative)
		return nil
	})
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("scan obsidian articles: %w", err)
	}
	return paths, nil
}

func safeMarkdownPath(root, relative string) (string, string, error) {
	relative = filepath.Clean(filepath.FromSlash(relative))
	if relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || !strings.EqualFold(filepath.Ext(relative), ".md") {
		return "", "", ErrInvalidID
	}
	fullPath := filepath.Join(root, relative)
	relToRoot, err := filepath.Rel(root, fullPath)
	if err != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return "", "", ErrInvalidID
	}
	// Resolve a final-file symlink as defense in depth. Random selection skips
	// symlinks, but a forged public id must not escape the configured folder.
	resolved, err := filepath.EvalSymlinks(fullPath)
	if err == nil {
		resolvedRel, relErr := filepath.Rel(root, resolved)
		if relErr != nil || resolvedRel == ".." || strings.HasPrefix(resolvedRel, ".."+string(filepath.Separator)) {
			return "", "", ErrInvalidID
		}
	}
	return fullPath, filepath.ToSlash(relToRoot), nil
}

func stripFrontmatter(markdown string) (string, string) {
	markdown = strings.TrimPrefix(markdown, "\ufeff")
	lines := strings.Split(markdown, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return markdown, ""
	}
	var title string
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			return strings.Join(lines[i+1:], "\n"), title
		}
		if key, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(key), "title") {
			title = strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return markdown, ""
}

func extractTitle(markdown, frontmatterTitle, filename string) (string, string) {
	if frontmatterTitle != "" {
		return frontmatterTitle, markdown
	}
	lines := strings.Split(markdown, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			title := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "# "))
			lines = append(lines[:i], lines[i+1:]...)
			return title, strings.Join(lines, "\n")
		}
	}
	return strings.TrimSuffix(filename, filepath.Ext(filename)), markdown
}

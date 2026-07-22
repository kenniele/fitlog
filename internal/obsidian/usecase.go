// Package obsidian publishes Markdown notes from one explicitly configured
// folder. The vault may be synchronised independently by obsidian-git; fitlog
// only ever opens it read-only.
package obsidian

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxArticleBytes        = 2 << 20 // 2 MiB
	maxFrontmatterScanSize = 64 << 10
	articleLinkTTL         = 7 * 24 * time.Hour
	articleTokenVersion    = 1
)

var (
	ErrNotConfigured = errors.New("obsidian articles path is not configured")
	ErrNoArticles    = errors.New("no markdown articles found")
	ErrInvalidID     = errors.New("invalid article id")
	ErrExpiredID     = errors.New("article link expired")
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

// TokenCipher turns an article payload into an opaque, authenticated token.
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
	now    func() time.Time
}

func NewUseCase(root string, tokens TokenCipher) *UseCase {
	return &UseCase{root: strings.TrimSpace(root), tokens: tokens, now: time.Now}
}

type articleToken struct {
	Version   int    `json:"v"`
	Path      string `json:"p"`
	ExpiresAt int64  `json:"exp"`
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

	var relative, id string
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
		id, err = u.mintArticleID(relative)
		if err != nil {
			return RawArticle{}, err
		}
	} else {
		relative, err = u.openArticleID(request.ID)
		if err != nil {
			return RawArticle{}, err
		}
		id = request.ID
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
	if !publishEnabled(string(content)) {
		return RawArticle{}, ErrInvalidID
	}
	return RawArticle{ID: id, Filename: filepath.Base(cleanRelative), Content: string(content)}, nil
}

func (u *UseCase) mintArticleID(relative string) (string, error) {
	now := u.currentTime()
	payload, err := json.Marshal(articleToken{
		Version:   articleTokenVersion,
		Path:      filepath.ToSlash(relative),
		ExpiresAt: now.Add(articleLinkTTL).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("marshal article id: %w", err)
	}
	sealed, err := u.tokens.SealString(string(payload))
	if err != nil {
		return "", fmt.Errorf("seal article id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (u *UseCase) openArticleID(id string) (string, error) {
	sealed, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return "", ErrInvalidID
	}
	plain, err := u.tokens.OpenString(sealed)
	if err != nil {
		return "", ErrInvalidID
	}
	var payload articleToken
	if err := json.Unmarshal([]byte(plain), &payload); err != nil || payload.Version != articleTokenVersion || payload.Path == "" || payload.ExpiresAt <= 0 {
		return "", ErrInvalidID
	}
	if !u.currentTime().Before(time.Unix(payload.ExpiresAt, 0)) {
		return "", ErrExpiredID
	}
	return payload.Path, nil
}

func (u *UseCase) currentTime() time.Time {
	if u.now == nil {
		return time.Now()
	}
	return u.now()
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
		relative = filepath.ToSlash(relative)
		published, err := hasPublishEnabled(path)
		if err != nil {
			return err
		}
		if published {
			paths = append(paths, relative)
		}
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

func hasPublishEnabled(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("open article metadata: %w", err)
	}
	defer file.Close()

	prefix, err := io.ReadAll(io.LimitReader(file, maxFrontmatterScanSize))
	if err != nil {
		return false, fmt.Errorf("read article metadata: %w", err)
	}
	return publishEnabled(string(prefix)), nil
}

func publishEnabled(markdown string) bool {
	_, fields, ok := parseFrontmatter(markdown)
	return ok && strings.EqualFold(fields["publish"], "true")
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
	body, fields, ok := parseFrontmatter(markdown)
	if !ok {
		return strings.TrimPrefix(markdown, "\ufeff"), ""
	}
	return body, fields["title"]
}

func parseFrontmatter(markdown string) (string, map[string]string, bool) {
	markdown = strings.TrimPrefix(markdown, "\ufeff")
	lines := strings.Split(markdown, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return markdown, nil, false
	}
	fields := make(map[string]string)
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			return strings.Join(lines[i+1:], "\n"), fields, true
		}
		if key, value, ok := strings.Cut(line, ":"); ok {
			fields[strings.ToLower(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return markdown, nil, false
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

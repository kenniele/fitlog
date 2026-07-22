package obsidian

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"fitlog/internal/auth"

	"github.com/stretchr/testify/require"
)

func newTestTokenCipher(t *testing.T) TokenCipher {
	t.Helper()
	cipher, err := auth.NewCipher(make([]byte, 32))
	require.NoError(t, err)
	return cipher
}

func TestUseCaseRandomFetchTransformFormat(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".obsidian"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".obsidian", "hidden.md"), []byte("# hidden"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "ignore.txt"), []byte("ignore"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "article.md"), []byte(`---
title: Хорошая статья
tags: [test]
status: mature
publish: true
---
## Идея

Это **важный** текст с [ссылкой](https://example.com).

| Метрика | Значение |
|---|---|
| Recovery | 75% |

<script>alert("xss")</script>
`), 0o600))

	useCase := NewUseCase(root, newTestTokenCipher(t))
	article, err := useCase.Execute(context.Background(), RandomRequest())
	require.NoError(t, err)
	require.Equal(t, "Хорошая статья", article.Title)
	require.NotEmpty(t, article.ID)
	require.NotEqual(t, base64.RawURLEncoding.EncodeToString([]byte("article.md")), article.ID)
	require.Contains(t, article.HTML, "<h2>Идея</h2>")
	require.Contains(t, article.HTML, "<strong>важный</strong>")
	require.Contains(t, article.HTML, `href="https://example.com"`)
	require.Contains(t, article.HTML, "<th>Метрика</th>")
	require.Contains(t, article.HTML, "<td>Recovery</td>")
	require.NotContains(t, article.HTML, "<script>")
	require.Contains(t, article.HTML, "&lt;script&gt;")

	byID, err := useCase.Execute(context.Background(), ArticleRequest(article.ID))
	require.NoError(t, err)
	require.Equal(t, article.Title, byID.Title)
	require.Equal(t, article.ID, byID.ID)
}

func TestArticleIDExpiresAfterSevenDays(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "article.md"), []byte("---\nstatus: mature\npublish: true\n---\n# Article"), 0o600))
	useCase := NewUseCase(root, newTestTokenCipher(t))
	issuedAt := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	useCase.now = func() time.Time { return issuedAt }

	article, err := useCase.Execute(context.Background(), RandomRequest())
	require.NoError(t, err)

	useCase.now = func() time.Time { return issuedAt.Add(articleLinkTTL - time.Second) }
	_, err = useCase.Execute(context.Background(), ArticleRequest(article.ID))
	require.NoError(t, err)

	useCase.now = func() time.Time { return issuedAt.Add(articleLinkTTL) }
	_, err = useCase.Execute(context.Background(), ArticleRequest(article.ID))
	require.ErrorIs(t, err, ErrExpiredID)
}

func TestPublishFalseImmediatelyRevokesIssuedLink(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "article.md")
	require.NoError(t, os.WriteFile(path, []byte("---\npublish: true\n---\n# Article"), 0o600))
	useCase := NewUseCase(root, newTestTokenCipher(t))

	article, err := useCase.Execute(context.Background(), RandomRequest())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("---\npublish: false\n---\n# Article"), 0o600))

	_, err = useCase.Execute(context.Background(), ArticleRequest(article.ID))
	require.ErrorIs(t, err, ErrInvalidID)
}

func TestUseCaseRejectsTraversalID(t *testing.T) {
	useCase := NewUseCase(t.TempDir(), newTestTokenCipher(t))
	id := base64.RawURLEncoding.EncodeToString([]byte("../secret.md"))
	_, err := useCase.Execute(context.Background(), ArticleRequest(id))
	require.ErrorIs(t, err, ErrInvalidID)
}

func TestUseCaseNotConfiguredAndEmpty(t *testing.T) {
	_, err := NewUseCase("", newTestTokenCipher(t)).Execute(context.Background(), RandomRequest())
	require.ErrorIs(t, err, ErrNotConfigured)

	_, err = NewUseCase(t.TempDir(), nil).Execute(context.Background(), RandomRequest())
	require.ErrorIs(t, err, ErrNotConfigured)

	_, err = NewUseCase(t.TempDir(), newTestTokenCipher(t)).Execute(context.Background(), RandomRequest())
	require.ErrorIs(t, err, ErrNoArticles)
}

func TestMarkdownFilesRequireExplicitPublishTrue(t *testing.T) {
	root := t.TempDir()
	writeArticle := func(relative, frontmatter string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(relative))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		content := "---\n" + frontmatter + "\n---\n# Article"
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}

	writeArticle("published.md", "status: growing\npublish: true")
	writeArticle("private.md", "status: mature\npublish: false")
	writeArticle("missing.md", "status: mature")
	writeArticle("case-insensitive.md", "publish: TRUE")

	paths, err := markdownFiles(context.Background(), root)
	require.NoError(t, err)
	sort.Strings(paths)
	require.Equal(t, []string{"case-insensitive.md", "published.md"}, paths)
}

func TestHandlerServesArticleWithSecurityHeaders(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "hello.md"), []byte("---\nstatus: mature\npublish: true\n---\n# Hello\n\nWorld"), 0o600))
	useCase := NewUseCase(root, newTestTokenCipher(t))
	random, err := useCase.Execute(context.Background(), RandomRequest())
	require.NoError(t, err)

	handler := NewHandler(useCase, slog.Default())
	request := httptest.NewRequest(http.MethodGet, "/"+random.ID, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "text/html; charset=utf-8", response.Header().Get("Content-Type"))
	require.Contains(t, response.Header().Get("Content-Security-Policy"), "default-src 'none'")
	require.Contains(t, response.Body.String(), "<h1>Hello</h1>")
}

type failingUseCase struct{ err error }

func (f failingUseCase) Fetch(context.Context, Request) (RawArticle, error) {
	return RawArticle{}, f.err
}
func (f failingUseCase) Transform(RawArticle) Article { return Article{} }
func (f failingUseCase) Format(Article) string        { return "" }
func (f failingUseCase) Execute(context.Context, Request) (PublishedArticle, error) {
	return PublishedArticle{}, f.err
}

func TestHandlerErrorStatus(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{ErrInvalidID, http.StatusNotFound},
		{ErrExpiredID, http.StatusNotFound},
		{ErrNotConfigured, http.StatusServiceUnavailable},
		{ErrTooLarge, http.StatusRequestEntityTooLarge},
		{errors.New("boom"), http.StatusInternalServerError},
	} {
		handler := NewHandler(failingUseCase{err: tc.err}, slog.New(slog.NewTextHandler(&strings.Builder{}, nil)))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/id", nil))
		require.Equal(t, tc.want, response.Code)
	}
}

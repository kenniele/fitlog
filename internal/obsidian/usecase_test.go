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
	"strings"
	"testing"

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
	require.NotEqual(t, article.ID, byID.ID, "every publication must get a fresh opaque id")
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

func TestHandlerServesArticleWithSecurityHeaders(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "hello.md"), []byte("# Hello\n\nWorld"), 0o600))
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

package obsidian

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

type Handler struct {
	useCase ReportUseCase
	logger  *slog.Logger
}

func NewHandler(useCase ReportUseCase, logger *slog.Logger) *Handler {
	return &Handler{useCase: useCase, logger: logger}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	article, err := h.useCase.Execute(r.Context(), ArticleRequest(id))
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidID), errors.Is(err, ErrNoArticles):
			http.NotFound(w, r)
		case errors.Is(err, ErrTooLarge):
			http.Error(w, "article is too large", http.StatusRequestEntityTooLarge)
		case errors.Is(err, ErrNotConfigured):
			http.Error(w, "articles are not configured", http.StatusServiceUnavailable)
		default:
			h.logger.Error("serve obsidian article", "err", err)
			http.Error(w, "could not render article", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(article.HTML))
}

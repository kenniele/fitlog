package controlcenter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const maxRequestBody = MaxImportSize

type apiHandler struct {
	service *Service
	auth    *authenticator
	loc     *time.Location
	now     func() time.Time
}

// NewHandler builds the Control Center API handler. Its routes are relative to
// /api/v1 because the server strips that deployment prefix before dispatching.
func NewHandler(store Store, ownerID int64, token string, loc *time.Location) http.Handler {
	if loc == nil {
		loc = time.UTC
	}
	handler := &apiHandler{
		service: NewService(store, ownerID, loc),
		auth:    newAuthenticator(ownerID, token),
		loc:     loc,
		now:     time.Now,
	}
	return handler.routes()
}

// NewHTTPHandler is an explicit alias for integrations that use that naming.
func NewHTTPHandler(store Store, ownerID int64, token string, loc *time.Location) http.Handler {
	return NewHandler(store, ownerID, token, loc)
}

func (h *apiHandler) routes() http.Handler {
	router := chi.NewRouter()
	router.Use(h.recoverPanics)
	router.Use(noStore)

	router.Get("/auth/session", h.getSession)
	router.Post("/auth/session", h.createSession)
	router.Delete("/auth/session", h.deleteSession)

	router.Group(func(protected chi.Router) {
		protected.Use(h.requireSession)
		protected.Use(h.useUserLocation)

		protected.Get("/dashboard/overview", h.overview)
		for _, kind := range []string{"training", "recovery", "nutrition", "body", "correlations"} {
			kind := kind
			protected.Get("/analytics/"+kind, h.analytics(kind))
		}

		protected.Get("/workout-sessions/export.csv", h.exportSessionsCSV)
		h.registerResource(protected, "workout-sessions")
		h.registerResource(protected, "workout-plans")
		h.registerResource(protected, "exercises")
		h.registerResource(protected, "recovery")
		h.registerResource(protected, "sleep")
		h.registerResource(protected, "nutrition")
		h.registerResource(protected, "body-measurements")

		protected.Get("/imports", h.listResource("imports"))
		protected.Get("/imports/{id}", h.getResource("imports"))
		protected.Post("/imports/preview", h.previewImport)
		protected.Post("/imports/execute", h.executeImport)

		protected.Get("/settings", h.getSettings)
		protected.Put("/settings", h.putSettings)
		protected.Delete("/settings/data", h.deleteAllData)
		protected.Delete("/data", h.deleteAllData)

		protected.Get("/export", h.exportAll)
		protected.Get("/sources", h.sources)
	})

	router.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeAPIError(w, http.StatusNotFound, "not_found", "endpoint not found", nil)
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
	})
	return router
}

// useUserLocation resolves the persisted IANA timezone once per protected
// request. Parsing ranges and repository date boundaries then use the same
// location, so changing Settings takes effect on the next request.
func (h *apiHandler) useUserLocation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		settings, err := h.service.Settings(r.Context())
		if err != nil {
			writeServiceError(w, err)
			return
		}
		loc, err := time.LoadLocation(settings.Timezone)
		if err != nil {
			writeServiceError(w, fmt.Errorf("load dashboard timezone: %w", err))
			return
		}
		next.ServeHTTP(w, r.WithContext(contextWithLocation(r.Context(), loc)))
	})
}

func (h *apiHandler) requestLocation(r *http.Request) *time.Location {
	return h.service.location(r.Context())
}

func (h *apiHandler) registerResource(router chi.Router, resource string) {
	path := "/" + resource
	router.Get(path, h.listResource(resource))
	router.Post(path, h.createResource(resource))
	router.Get(path+"/{id}", h.getResource(resource))
	router.Put(path+"/{id}", h.updateResource(resource))
	router.Delete(path+"/{id}", h.deleteResource(resource))
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (h *apiHandler) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "internal server error", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (h *apiHandler) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.auth.enabled() {
			writeAPIError(w, http.StatusServiceUnavailable, "dashboard_disabled", "dashboard is disabled", nil)
			return
		}
		if !h.auth.authenticate(r) {
			writeAPIError(w, http.StatusUnauthorized, "unauthorized", "authentication required", nil)
			return
		}
		if isUnsafeMethod(r.Method) && r.Header.Get(requestHeader) != "1" {
			writeAPIError(w, http.StatusForbidden, "request_header_required", "required request header is missing", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func (h *apiHandler) getSession(w http.ResponseWriter, r *http.Request) {
	if !h.auth.enabled() {
		writeAPIError(w, http.StatusServiceUnavailable, "dashboard_disabled", "dashboard is disabled", nil)
		return
	}
	if !h.auth.authenticate(r) {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "authentication required", nil)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"authenticated": true, "owner_id": h.auth.ownerID})
}

func (h *apiHandler) createSession(w http.ResponseWriter, r *http.Request) {
	if !h.auth.enabled() {
		writeAPIError(w, http.StatusServiceUnavailable, "dashboard_disabled", "dashboard is disabled", nil)
		return
	}
	var request struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeRequestError(w, err)
		return
	}
	if !h.auth.matchesToken(request.Token) {
		writeAPIError(w, http.StatusUnauthorized, "invalid_credentials", "invalid dashboard token", nil)
		return
	}
	expires := h.auth.now().Add(sessionLifetime)
	http.SetCookie(w, sessionCookie(r, h.auth.sessionValue(expires), expires))
	writeData(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"owner_id":      h.auth.ownerID,
		"expires_at":    expires.UTC(),
	})
}

func (h *apiHandler) deleteSession(w http.ResponseWriter, r *http.Request) {
	if !h.auth.enabled() {
		writeAPIError(w, http.StatusServiceUnavailable, "dashboard_disabled", "dashboard is disabled", nil)
		return
	}
	if !h.auth.authenticate(r) {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "authentication required", nil)
		return
	}
	if r.Header.Get(requestHeader) != "1" {
		writeAPIError(w, http.StatusForbidden, "request_header_required", "required request header is missing", nil)
		return
	}
	http.SetCookie(w, sessionCookie(r, "", time.Unix(1, 0)))
	writeData(w, http.StatusOK, map[string]bool{"authenticated": false})
}

func (h *apiHandler) overview(w http.ResponseWriter, r *http.Request) {
	dateRange, err := ParseDateRange(r, h.requestLocation(r), h.now())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	result, err := h.service.Overview(r.Context(), dateRange)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (h *apiHandler) analytics(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dateRange, err := ParseDateRange(r, h.requestLocation(r), h.now())
		if err != nil {
			writeServiceError(w, err)
			return
		}
		filters, err := ParseAnalyticsFilters(r)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		result, err := h.service.Analytics(r.Context(), kind, dateRange, filters)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeData(w, http.StatusOK, result)
	}
}

func (h *apiHandler) listResource(resource string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pagination, err := ParsePagination(r, h.requestLocation(r))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		result, err := h.service.List(r.Context(), resource, pagination)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		if result.Items == nil {
			result.Items = make([]json.RawMessage, 0)
		}
		writeData(w, http.StatusOK, result)
	}
}

func (h *apiHandler) getResource(resource string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := requestID(r)
		if err != nil {
			writeRequestError(w, err)
			return
		}
		result, err := h.service.Get(r.Context(), resource, id)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeData(w, http.StatusOK, result)
	}
}

func (h *apiHandler) createResource(resource string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, err := decodeJSONObject(w, r)
		if err != nil {
			writeRequestError(w, err)
			return
		}
		result, err := h.service.Create(r.Context(), resource, raw)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeData(w, http.StatusCreated, result)
	}
}

func (h *apiHandler) updateResource(resource string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := requestID(r)
		if err != nil {
			writeRequestError(w, err)
			return
		}
		raw, err := decodeJSONObject(w, r)
		if err != nil {
			writeRequestError(w, err)
			return
		}
		result, err := h.service.Update(r.Context(), resource, id, raw)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeData(w, http.StatusOK, result)
	}
}

func (h *apiHandler) deleteResource(resource string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := requestID(r)
		if err != nil {
			writeRequestError(w, err)
			return
		}
		if err := h.service.Delete(r.Context(), resource, id); err != nil {
			writeServiceError(w, err)
			return
		}
		writeData(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
	}
}

func (h *apiHandler) exportSessionsCSV(w http.ResponseWriter, r *http.Request) {
	dateRange, err := ParseDateRange(r, h.requestLocation(r), h.now())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	filters, err := ParsePagination(r, h.requestLocation(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	content, err := h.service.ExportSessionsCSV(r.Context(), dateRange, filters)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="fitlog-workout-sessions.csv"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (h *apiHandler) previewImport(w http.ResponseWriter, r *http.Request) {
	request, err := decodeImportRequest(w, r)
	if err != nil {
		writeRequestError(w, err)
		return
	}
	preview, err := h.service.PreviewImport(r.Context(), request)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, preview)
}

func (h *apiHandler) executeImport(w http.ResponseWriter, r *http.Request) {
	request, err := decodeImportRequest(w, r)
	if err != nil {
		writeRequestError(w, err)
		return
	}
	result, err := h.service.ExecuteImport(r.Context(), request)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusCreated, result)
}

func (h *apiHandler) getSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.Settings(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, settings)
}

func (h *apiHandler) putSettings(w http.ResponseWriter, r *http.Request) {
	var settings Settings
	if err := decodeJSON(w, r, &settings); err != nil {
		writeRequestError(w, err)
		return
	}
	result, err := h.service.SaveSettings(r.Context(), settings)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (h *apiHandler) sources(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Sources(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if result == nil {
		result = make([]SourceStatus, 0)
	}
	writeData(w, http.StatusOK, result)
}

func (h *apiHandler) exportAll(w http.ResponseWriter, r *http.Request) {
	exportType := strings.TrimSpace(r.URL.Query().Get("type"))
	if exportType == "training" || exportType == "sessions" {
		h.exportSessionsCSV(w, r)
		return
	}
	if exportType != "" && exportType != "all" {
		writeServiceError(w, &ValidationError{Message: "invalid export type", Fields: map[string]string{"type": "use all or training"}})
		return
	}
	content, err := h.service.ExportAll(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !json.Valid(content) {
		writeServiceError(w, fmt.Errorf("export returned invalid JSON"))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="fitlog-export.json"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (h *apiHandler) deleteAllData(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Confirmation string `json:"confirmation"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeRequestError(w, err)
		return
	}
	if err := h.service.DeleteAll(r.Context(), request.Confirmation); err != nil {
		writeServiceError(w, err)
		return
	}
	http.SetCookie(w, sessionCookie(r, "", time.Unix(1, 0)))
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

func requestID(r *http.Request) (int64, error) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, &requestError{status: http.StatusBadRequest, code: "invalid_id", message: "id must be a positive integer"}
	}
	return id, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	return decodeJSONWithLimit(w, r, target, maxRequestBody)
}

func decodeJSONWithLimit(w http.ResponseWriter, r *http.Request, target any, limit int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return classifyDecodeError(err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return err
	}
	return nil
}

func decodeJSONObject(w http.ResponseWriter, r *http.Request) (json.RawMessage, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return nil, classifyDecodeError(err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, &requestError{status: http.StatusBadRequest, code: "invalid_json", message: "request body must be a JSON object"}
	}
	return raw, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return &requestError{status: http.StatusBadRequest, code: "invalid_json", message: "request body must contain one JSON value"}
		}
		return classifyDecodeError(err)
	}
	return nil
}

func classifyDecodeError(err error) error {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		return &requestError{status: http.StatusRequestEntityTooLarge, code: "payload_too_large", message: "request body exceeds the allowed size"}
	}
	if errors.Is(err, io.EOF) {
		return &requestError{status: http.StatusBadRequest, code: "invalid_json", message: "request body is required"}
	}
	var syntaxError *json.SyntaxError
	var typeError *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntaxError):
		return &requestError{status: http.StatusBadRequest, code: "invalid_json", message: "request body contains invalid JSON"}
	case errors.As(err, &typeError):
		return &requestError{status: http.StatusBadRequest, code: "invalid_json", message: "request body contains a value with the wrong type"}
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		return &requestError{status: http.StatusBadRequest, code: "unknown_field", message: err.Error()}
	default:
		return &requestError{status: http.StatusBadRequest, code: "invalid_request", message: "request body could not be read"}
	}
}

func decodeImportRequest(w http.ResponseWriter, r *http.Request) (ImportRequest, error) {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	if mediaType != "multipart/form-data" {
		var request ImportRequest
		if err := decodeJSONWithLimit(w, r, &request, MaxImportEnvelopeSize); err != nil {
			return ImportRequest{}, err
		}
		return request, nil
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxImportEnvelopeSize)
	if err := r.ParseMultipartForm(MaxImportEnvelopeSize); err != nil {
		return ImportRequest{}, classifyMultipartError(err)
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return ImportRequest{}, &requestError{status: http.StatusBadRequest, code: "missing_file", message: "multipart field file is required"}
		}
		return ImportRequest{}, classifyMultipartError(err)
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, MaxImportSize+1))
	if err != nil {
		return ImportRequest{}, classifyMultipartError(err)
	}
	if len(content) > MaxImportSize {
		return ImportRequest{}, &requestError{status: http.StatusRequestEntityTooLarge, code: "payload_too_large", message: "import file exceeds 5 MiB"}
	}

	format := strings.ToLower(strings.TrimSpace(r.FormValue("format")))
	if format == "" {
		format = importFormat(header, r.FormValue("source"))
	}
	request := ImportRequest{
		DataType: strings.TrimSpace(r.FormValue("data_type")),
		Filename: header.Filename,
		Format:   format,
		Content:  string(content),
		Source:   strings.TrimSpace(r.FormValue("source")),
	}
	if rawMapping := strings.TrimSpace(r.FormValue("mapping")); rawMapping != "" {
		if err := json.Unmarshal([]byte(rawMapping), &request.Mapping); err != nil {
			return ImportRequest{}, &requestError{status: http.StatusBadRequest, code: "invalid_mapping", message: "mapping must be a JSON object of string values"}
		}
	}
	return request, nil
}

func importFormat(header *multipart.FileHeader, source string) string {
	if strings.EqualFold(filepath.Ext(header.Filename), ".json") || strings.Contains(strings.ToLower(source), "json") {
		return "json"
	}
	return "csv"
}

func classifyMultipartError(err error) error {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) || strings.Contains(strings.ToLower(err.Error()), "request body too large") {
		return &requestError{status: http.StatusRequestEntityTooLarge, code: "payload_too_large", message: "request body exceeds the allowed import envelope size"}
	}
	return &requestError{status: http.StatusBadRequest, code: "invalid_multipart", message: "multipart request could not be read"}
}

type requestError struct {
	status  int
	code    string
	message string
	fields  map[string]string
}

func (e *requestError) Error() string { return e.message }

func writeRequestError(w http.ResponseWriter, err error) {
	var requestErr *requestError
	if errors.As(err, &requestErr) {
		writeAPIError(w, requestErr.status, requestErr.code, requestErr.message, requestErr.fields)
		return
	}
	writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request", nil)
}

func writeServiceError(w http.ResponseWriter, err error) {
	var validation *ValidationError
	switch {
	case errors.As(err, &validation):
		writeAPIError(w, http.StatusUnprocessableEntity, "validation_error", validation.Error(), validation.Fields)
	case errors.Is(err, ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "resource not found", nil)
	case errors.Is(err, ErrConflict):
		writeAPIError(w, http.StatusConflict, "conflict", "resource conflicts with existing data", nil)
	case errors.Is(err, ErrDisabled):
		writeAPIError(w, http.StatusServiceUnavailable, "dashboard_disabled", "dashboard is disabled", nil)
	default:
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "internal server error", nil)
	}
}

func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, map[string]any{"data": data})
}

func writeAPIError(w http.ResponseWriter, status int, code, message string, fields map[string]string) {
	errorBody := map[string]any{"code": code, "message": message}
	if len(fields) > 0 {
		errorBody["fields"] = fields
	}
	writeJSON(w, status, map[string]any{"error": errorBody})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

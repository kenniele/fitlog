package controlcenter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type handlerStore struct {
	overviewFn            func(context.Context, int64, DateRange, *time.Location) (Overview, error)
	listFn                func(context.Context, int64, string, Pagination, *time.Location) (ListResult, error)
	getFn                 func(context.Context, int64, string, int64, *time.Location) (json.RawMessage, error)
	createFn              func(context.Context, int64, string, json.RawMessage, *time.Location) (json.RawMessage, error)
	updateFn              func(context.Context, int64, string, int64, json.RawMessage, *time.Location) (json.RawMessage, error)
	deleteFn              func(context.Context, int64, string, int64) error
	exportSessionsCSVFn   func(context.Context, int64, DateRange, Pagination, *time.Location) ([]byte, error)
	settingsFn            func(context.Context, int64, string) (Settings, error)
	saveSettingsFn        func(context.Context, int64, Settings) (Settings, error)
	sourcesFn             func(context.Context, int64) ([]SourceStatus, error)
	existingExternalIDsFn func(context.Context, int64, string, string, []string) (map[string]struct{}, error)
	executeImportFn       func(context.Context, int64, ImportBatch, *time.Location) (ImportResult, error)
	exportAllFn           func(context.Context, int64, *time.Location) (json.RawMessage, error)
	deleteAllFn           func(context.Context, int64) error
	seedDemoFn            func(context.Context, int64, time.Time, *time.Location) (DemoSeedResult, error)
}

func (s *handlerStore) Overview(ctx context.Context, ownerID int64, dateRange DateRange, loc *time.Location) (Overview, error) {
	if s.overviewFn != nil {
		return s.overviewFn(ctx, ownerID, dateRange, loc)
	}
	return Overview{}, nil
}

func (s *handlerStore) List(ctx context.Context, ownerID int64, resource string, pagination Pagination, loc *time.Location) (ListResult, error) {
	if s.listFn != nil {
		return s.listFn(ctx, ownerID, resource, pagination, loc)
	}
	return ListResult{Items: []json.RawMessage{}, Page: pagination.Page, PageSize: pagination.PageSize}, nil
}

func (s *handlerStore) Get(ctx context.Context, ownerID int64, resource string, id int64, loc *time.Location) (json.RawMessage, error) {
	if s.getFn != nil {
		return s.getFn(ctx, ownerID, resource, id, loc)
	}
	return json.RawMessage(`{}`), nil
}

func (s *handlerStore) Create(ctx context.Context, ownerID int64, resource string, raw json.RawMessage, loc *time.Location) (json.RawMessage, error) {
	if s.createFn != nil {
		return s.createFn(ctx, ownerID, resource, raw, loc)
	}
	return raw, nil
}

func (s *handlerStore) Update(ctx context.Context, ownerID int64, resource string, id int64, raw json.RawMessage, loc *time.Location) (json.RawMessage, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, ownerID, resource, id, raw, loc)
	}
	return raw, nil
}

func (s *handlerStore) Delete(ctx context.Context, ownerID int64, resource string, id int64) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, ownerID, resource, id)
	}
	return nil
}

func (s *handlerStore) ExportSessionsCSV(ctx context.Context, ownerID int64, dateRange DateRange, filters Pagination, loc *time.Location) ([]byte, error) {
	if s.exportSessionsCSVFn != nil {
		return s.exportSessionsCSVFn(ctx, ownerID, dateRange, filters, loc)
	}
	return []byte("id,date\n"), nil
}

func (s *handlerStore) Settings(ctx context.Context, ownerID int64, timezone string) (Settings, error) {
	if s.settingsFn != nil {
		return s.settingsFn(ctx, ownerID, timezone)
	}
	return Settings{Timezone: timezone, Units: "metric", Theme: "dark", FirstDayOfWeek: 1}, nil
}

func (s *handlerStore) SaveSettings(ctx context.Context, ownerID int64, settings Settings) (Settings, error) {
	if s.saveSettingsFn != nil {
		return s.saveSettingsFn(ctx, ownerID, settings)
	}
	return settings, nil
}

func (s *handlerStore) Sources(ctx context.Context, ownerID int64) ([]SourceStatus, error) {
	if s.sourcesFn != nil {
		return s.sourcesFn(ctx, ownerID)
	}
	return []SourceStatus{}, nil
}

func (s *handlerStore) ExistingExternalIDs(ctx context.Context, ownerID int64, dataType, source string, ids []string) (map[string]struct{}, error) {
	if s.existingExternalIDsFn != nil {
		return s.existingExternalIDsFn(ctx, ownerID, dataType, source, ids)
	}
	return map[string]struct{}{}, nil
}

func (s *handlerStore) ExecuteImport(ctx context.Context, ownerID int64, batch ImportBatch, loc *time.Location) (ImportResult, error) {
	if s.executeImportFn != nil {
		return s.executeImportFn(ctx, ownerID, batch, loc)
	}
	return ImportResult{ID: 1, Status: "completed", Total: len(batch.Rows), Imported: len(batch.Rows)}, nil
}

func (s *handlerStore) ExportAll(ctx context.Context, ownerID int64, loc *time.Location) (json.RawMessage, error) {
	if s.exportAllFn != nil {
		return s.exportAllFn(ctx, ownerID, loc)
	}
	return json.RawMessage(`{"version":1}`), nil
}

func (s *handlerStore) DeleteAll(ctx context.Context, ownerID int64) error {
	if s.deleteAllFn != nil {
		return s.deleteAllFn(ctx, ownerID)
	}
	return nil
}

func (s *handlerStore) SeedDemo(ctx context.Context, ownerID int64, now time.Time, loc *time.Location) (DemoSeedResult, error) {
	if s.seedDemoFn != nil {
		return s.seedDemoFn(ctx, ownerID, now, loc)
	}
	return DemoSeedResult{}, nil
}

func TestHandlerDashboardDisabled(t *testing.T) {
	handler := NewHandler(&handlerStore{}, 7, "", time.UTC)
	for _, path := range []string{"/auth/session", "/dashboard/overview"} {
		response := serve(handler, http.MethodGet, path, "", nil, "")
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("GET %s status = %d, want %d", path, response.Code, http.StatusServiceUnavailable)
		}
		assertErrorCode(t, response, "dashboard_disabled")
	}
}

func TestHandlerAuthenticationAndUnsafeHeader(t *testing.T) {
	var created bool
	store := &handlerStore{
		overviewFn: func(_ context.Context, ownerID int64, _ DateRange, _ *time.Location) (Overview, error) {
			if ownerID != 7 {
				t.Fatalf("owner id = %d, want 7", ownerID)
			}
			return Overview{Highlights: []Highlight{}}, nil
		},
		createFn: func(_ context.Context, _ int64, resource string, raw json.RawMessage, _ *time.Location) (json.RawMessage, error) {
			created = true
			if resource != "recovery" {
				t.Fatalf("resource = %q, want recovery", resource)
			}
			return raw, nil
		},
	}
	handler := NewHandler(store, 7, "correct-token", time.UTC)
	cookie := login(t, handler, "correct-token")

	overview := serve(handler, http.MethodGet, "/dashboard/overview", "", cookie, "")
	if overview.Code != http.StatusOK {
		t.Fatalf("overview status = %d, body = %s", overview.Code, overview.Body.String())
	}
	assertHasDataEnvelope(t, overview)

	withoutHeader := serve(handler, http.MethodPost, "/recovery", `{"date":"2026-08-21"}`, cookie, "")
	if withoutHeader.Code != http.StatusForbidden {
		t.Fatalf("POST without request header status = %d, want %d", withoutHeader.Code, http.StatusForbidden)
	}
	if created {
		t.Fatal("store Create called before request header check")
	}

	withHeader := serve(handler, http.MethodPost, "/recovery", `{"date":"2026-08-21"}`, cookie, "1")
	if withHeader.Code != http.StatusCreated {
		t.Fatalf("POST with request header status = %d, body = %s", withHeader.Code, withHeader.Body.String())
	}
	if !created {
		t.Fatal("store Create was not called")
	}
}

func TestHandlerRejectsInvalidLoginAndUnknownFields(t *testing.T) {
	handler := NewHandler(&handlerStore{}, 7, "correct-token", time.UTC)
	wrong := serve(handler, http.MethodPost, "/auth/session", `{"token":"wrong"}`, nil, "")
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token status = %d, want %d", wrong.Code, http.StatusUnauthorized)
	}
	assertErrorCode(t, wrong, "invalid_credentials")

	unknown := serve(handler, http.MethodPost, "/auth/session", `{"token":"correct-token","extra":true}`, nil, "")
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d, want %d", unknown.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, unknown, "unknown_field")
}

func TestHandlerMultipartImportPreview(t *testing.T) {
	store := &handlerStore{
		existingExternalIDsFn: func(_ context.Context, ownerID int64, dataType, source string, ids []string) (map[string]struct{}, error) {
			if ownerID != 7 || dataType != "recovery" || source != "whoop_csv" {
				t.Fatalf("unexpected lookup: owner=%d type=%q source=%q", ownerID, dataType, source)
			}
			if len(ids) != 1 || ids[0] != "day-1" {
				t.Fatalf("external ids = %#v", ids)
			}
			return map[string]struct{}{"day-1": {}}, nil
		},
	}
	handler := NewHandler(store, 7, "correct-token", time.UTC)
	cookie := login(t, handler, "correct-token")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("data_type", "recovery")
	_ = writer.WriteField("source", "whoop_csv")
	part, err := writer.CreateFormFile("file", "recovery.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(part, "date,recovery_score,external_id\n2026-08-21,78,day-1\n")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/imports/preview", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set(requestHeader, "1")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("preview status = %d, body = %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data ImportPreview `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.TotalRows != 1 || envelope.Data.DuplicateRows != 1 || len(envelope.Data.Rows) != 1 {
		t.Fatalf("unexpected preview: %#v", envelope.Data)
	}
}

func TestDecodeImportJSONAllowsFiveMiBDecodedFileInsideLargerEnvelope(t *testing.T) {
	content := strings.Repeat("a", MaxImportSize)
	body, err := json.Marshal(ImportRequest{DataType: "nutrition", Format: "csv", Content: content})
	if err != nil {
		t.Fatal(err)
	}
	if len(body) <= MaxImportSize {
		t.Fatalf("test envelope = %d, want greater than decoded file limit", len(body))
	}
	request := httptest.NewRequest(http.MethodPost, "/imports/preview", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	decoded, err := decodeImportRequest(httptest.NewRecorder(), request)
	if err != nil {
		t.Fatalf("decode near-limit import envelope: %v", err)
	}
	if len(decoded.Content) != MaxImportSize {
		t.Fatalf("decoded content = %d, want %d", len(decoded.Content), MaxImportSize)
	}
}

func TestHandlerImportDetailUsesOwnerScopedResourceGet(t *testing.T) {
	store := &handlerStore{getFn: func(_ context.Context, ownerID int64, resource string, id int64, _ *time.Location) (json.RawMessage, error) {
		if ownerID != 7 || resource != "imports" || id != 9 {
			t.Fatalf("detail scope owner=%d resource=%q id=%d", ownerID, resource, id)
		}
		return json.RawMessage(`{"id":9,"errors":[{"row":2,"message":"bad row"}]}`), nil
	}}
	handler := NewHandler(store, 7, "correct-token", time.UTC)
	cookie := login(t, handler, "correct-token")
	response := serve(handler, http.MethodGet, "/imports/9", "", cookie, "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "bad row") {
		t.Fatalf("detail status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestHandlerDeleteAllRequiresExactConfirmation(t *testing.T) {
	deleteCalls := 0
	handler := NewHandler(&handlerStore{deleteAllFn: func(_ context.Context, ownerID int64) error {
		deleteCalls++
		if ownerID != 7 {
			t.Fatalf("owner id = %d, want 7", ownerID)
		}
		return nil
	}}, 7, "correct-token", time.UTC)
	cookie := login(t, handler, "correct-token")

	wrong := serve(handler, http.MethodDelete, "/data", `{"confirmation":"DELETE FITLOG DATA"}`, cookie, "1")
	if wrong.Code != http.StatusUnprocessableEntity {
		t.Fatalf("wrong confirmation status = %d, body = %s", wrong.Code, wrong.Body.String())
	}
	if deleteCalls != 0 {
		t.Fatal("store DeleteAll called for an incorrect confirmation")
	}

	ok := serve(handler, http.MethodDelete, "/data", `{"confirmation":"DELETE MY DATA"}`, cookie, "1")
	if ok.Code != http.StatusOK {
		t.Fatalf("correct confirmation status = %d, body = %s", ok.Code, ok.Body.String())
	}
	if deleteCalls != 1 {
		t.Fatalf("store DeleteAll calls = %d, want 1", deleteCalls)
	}
}

func TestHandlerSessionExportPassesSelectedFilters(t *testing.T) {
	called := false
	handler := NewHandler(&handlerStore{exportSessionsCSVFn: func(
		_ context.Context,
		ownerID int64,
		dateRange DateRange,
		filters Pagination,
		loc *time.Location,
	) ([]byte, error) {
		called = true
		if ownerID != 7 || loc != time.UTC {
			t.Fatalf("unexpected export scope: owner=%d loc=%v", ownerID, loc)
		}
		if got := dateRange.From.Format("2006-01-02"); got != "2026-08-01" {
			t.Fatalf("export from = %s", got)
		}
		if filters.Search != "bench" || filters.Filters["status"] != "finished" ||
			filters.Filters["exercise_id"] != "42" || filters.Filters["plan_id"] != "9" ||
			filters.Filters["template_id"] != "11" || filters.Filters["date_basis"] != "calendar" {
			t.Fatalf("unexpected export filters: %#v", filters)
		}
		return []byte("session_id\n"), nil
	}}, 7, "correct-token", time.UTC)
	cookie := login(t, handler, "correct-token")

	response := serve(handler, http.MethodGet,
		"/workout-sessions/export.csv?from=2026-08-01&to=2026-08-21&search=bench&status=finished&exercise_id=42&plan_id=9&template_id=11&date_basis=calendar",
		"", cookie, "")
	if response.Code != http.StatusOK {
		t.Fatalf("export status = %d, body = %s", response.Code, response.Body.String())
	}
	if !called {
		t.Fatal("session export store was not called")
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/csv") {
		t.Fatalf("export content type = %q", contentType)
	}
}

func TestHandlerUsesPersistedTimezoneForRangesAndRepository(t *testing.T) {
	called := false
	store := &handlerStore{
		settingsFn: func(_ context.Context, ownerID int64, fallback string) (Settings, error) {
			if ownerID != 7 || fallback != "UTC" {
				t.Fatalf("settings scope owner=%d fallback=%q", ownerID, fallback)
			}
			return Settings{Timezone: "America/New_York", Units: "metric", Theme: "dark", FirstDayOfWeek: 1}, nil
		},
		overviewFn: func(_ context.Context, _ int64, dateRange DateRange, loc *time.Location) (Overview, error) {
			called = true
			if loc.String() != "America/New_York" || dateRange.From.Location() != loc || dateRange.To.Location() != loc {
				t.Fatalf("range/repository timezone mismatch: loc=%v from=%v to=%v", loc, dateRange.From, dateRange.To)
			}
			return Overview{Highlights: []Highlight{}}, nil
		},
	}
	handler := NewHandler(store, 7, "correct-token", time.UTC)
	cookie := login(t, handler, "correct-token")
	response := serve(handler, http.MethodGet, "/dashboard/overview?from=2026-03-07&to=2026-03-10", "", cookie, "")
	if response.Code != http.StatusOK {
		t.Fatalf("overview status = %d, body = %s", response.Code, response.Body.String())
	}
	if !called {
		t.Fatal("overview store was not called")
	}
}

func login(t *testing.T, handler http.Handler, token string) *http.Cookie {
	t.Helper()
	response := serve(handler, http.MethodPost, "/auth/session", `{"token":"`+token+`"}`, nil, "")
	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("login cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != sessionCookieName || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("unexpected session cookie: %#v", cookie)
	}
	return cookie
}

func serve(handler http.Handler, method, path, body string, cookie *http.Cookie, requestHeaderValue string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	if requestHeaderValue != "" {
		request.Header.Set(requestHeader, requestHeaderValue)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertErrorCode(t *testing.T, response *httptest.ResponseRecorder, expected string) {
	t.Helper()
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Code != expected {
		t.Fatalf("error code = %q, want %q; body = %s", envelope.Error.Code, expected, response.Body.String())
	}
}

func assertHasDataEnvelope(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if _, ok := envelope["data"]; !ok {
		t.Fatalf("response has no data envelope: %s", response.Body.String())
	}
}

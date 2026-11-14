package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chinmay/switchboard/internal/auth"
	"github.com/chinmay/switchboard/internal/tenant"
)

func docsTestAPI(t *testing.T, aiURL string) (*DocsAPI, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := tenant.Tenant{ID: "docs-test", Name: "Docs Test", Services: []string{"repair"}, Windows: []string{"morning"}}
	if _, err := tenant.Save(dir, cfg); err != nil {
		t.Fatal(err)
	}
	return &DocsAPI{Tenants: tenant.New(cfg), Dir: dir, AIURL: aiURL}, filepath.Join(dir, cfg.ID, "docs")
}

func uploadDoc(t *testing.T, api *DocsAPI, account auth.Account, name, docType, body string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writer.WriteField("type", docType); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/tenant/docs?tenant="+account.TenantID, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = req.WithContext(auth.WithAccount(req.Context(), account))
	rec := httptest.NewRecorder()
	api.Upload()(rec, req)
	return rec
}

func TestDocsUploadListDeleteAndIndex(t *testing.T) {
	var indexed struct {
		Tenant string      `json:"tenant"`
		Docs   []ingestDoc `json:"docs"`
	}
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ingest" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&indexed); err != nil {
			t.Errorf("decode ingest: %v", err)
		}
		writeJSON(w, map[string]any{"chunks": 4, "embedded": 4})
	}))
	defer ai.Close()

	api, docsRoot := docsTestAPI(t, ai.URL)
	owner := auth.Account{ID: "owner-1", TenantID: "docs-test", Role: auth.RoleOwner}

	rec := uploadDoc(t, api, owner, "Service Area.md", "policy", "We serve within 20 km.\n\nCall for anything further.")
	if rec.Code != http.StatusOK {
		t.Fatalf("upload status=%d body=%s", rec.Code, rec.Body.String())
	}
	var uploaded struct {
		Documents []docFile     `json:"documents"`
		Index     *ingestResult `json:"index"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &uploaded); err != nil {
		t.Fatal(err)
	}
	if uploaded.Index == nil || uploaded.Index.Chunks != 4 {
		t.Fatalf("index = %+v body=%s", uploaded.Index, rec.Body.String())
	}
	saved, err := os.ReadFile(filepath.Join(docsRoot, "policy", "Service-Area.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), "20 km") {
		t.Fatalf("saved = %q", saved)
	}
	if indexed.Tenant != "docs-test" || len(indexed.Docs) != 1 || indexed.Docs[0].Name != "policy/Service-Area.md" {
		t.Fatalf("ingest payload = %+v", indexed)
	}

	// List sees the saved document.
	listReq := httptest.NewRequest(http.MethodGet, "/api/tenant/docs?tenant=docs-test", nil)
	listReq = listReq.WithContext(auth.WithAccount(listReq.Context(), owner))
	listRec := httptest.NewRecorder()
	api.List()(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listRec.Code, listRec.Body.String())
	}
	var listed struct {
		Documents []docFile `json:"documents"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Documents) != 1 || listed.Documents[0].Name != "policy/Service-Area.md" || listed.Documents[0].Type != "policy" {
		t.Fatalf("listed = %+v", listed.Documents)
	}

	// Delete removes it and reindexes.
	delReq := httptest.NewRequest(http.MethodDelete, "/api/tenant/docs?tenant=docs-test&name=policy/Service-Area.md", nil)
	delReq = delReq.WithContext(auth.WithAccount(delReq.Context(), owner))
	delRec := httptest.NewRecorder()
	api.Delete()(delRec, delReq)
	if delRec.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", delRec.Code, delRec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(docsRoot, "policy", "Service-Area.md")); !os.IsNotExist(err) {
		t.Fatalf("document still exists: %v", err)
	}
	if len(indexed.Docs) != 0 {
		t.Fatalf("ingest after delete = %+v", indexed.Docs)
	}
}

func TestDocsRejectsBadInput(t *testing.T) {
	api, _ := docsTestAPI(t, "")
	owner := auth.Account{ID: "owner-1", TenantID: "docs-test", Role: auth.RoleOwner}
	viewer := auth.Account{ID: "viewer-1", TenantID: "docs-test", Role: auth.RoleViewer}

	if rec := uploadDoc(t, api, owner, "malware.exe", "faq", "nope"); rec.Code != http.StatusBadRequest {
		t.Fatalf("exe upload status=%d", rec.Code)
	}
	if rec := uploadDoc(t, api, viewer, "hours.md", "faq", "Open late."); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer upload status=%d", rec.Code)
	}
	if rec := uploadDoc(t, api, owner, "empty.md", "faq", "   "); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty upload status=%d", rec.Code)
	}

	// Without an indexing service the file is saved and the error is reported.
	rec := uploadDoc(t, api, owner, "hours.md", "faq", "Open late.")
	if rec.Code != http.StatusOK {
		t.Fatalf("upload without AI status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["index_error"] == nil {
		t.Fatalf("expected index_error, got %s", rec.Body.String())
	}

	// Path traversal is rejected.
	delReq := httptest.NewRequest(http.MethodDelete, "/api/tenant/docs?tenant=docs-test&name=../config.json", nil)
	delReq = delReq.WithContext(auth.WithAccount(delReq.Context(), owner))
	delRec := httptest.NewRecorder()
	api.Delete()(delRec, delReq)
	if delRec.Code != http.StatusBadRequest {
		t.Fatalf("traversal status=%d", delRec.Code)
	}
}

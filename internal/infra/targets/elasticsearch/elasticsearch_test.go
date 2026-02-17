package elasticsearch

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmrzaf/sdgen/internal/domain"
)

func TestElasticsearchTarget_BasicFlow(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping due to restricted socket sandbox: %v", err)
	}
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":{"number":"8.12.0"}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/events":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/events/_delete_by_query":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"deleted":1}`))
		case r.Method == http.MethodPost && r.URL.Path == "/_bulk":
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"event_id"`) {
				t.Fatalf("bulk payload missing expected field: %s", string(body))
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"errors":false}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	ts.Listener = ln
	ts.Start()
	defer ts.Close()

	tgt := NewElasticsearchTarget(ts.URL)
	if err := tgt.Connect(); err != nil {
		t.Fatal(err)
	}
	entity := &domain.Entity{Name: "events", TargetTable: "events"}
	if err := tgt.CreateTableIfNotExists(entity); err != nil {
		t.Fatal(err)
	}
	if err := tgt.InsertBatch("events", []string{"event_id", "name"}, [][]interface{}{{"e1", "hello"}}); err != nil {
		t.Fatal(err)
	}
	if err := tgt.TruncateTable("events"); err != nil {
		t.Fatal(err)
	}
	if ver, err := GetServerVersion(ts.URL); err != nil || ver != "8.12.0" {
		t.Fatalf("unexpected version result ver=%q err=%v", ver, err)
	}
}

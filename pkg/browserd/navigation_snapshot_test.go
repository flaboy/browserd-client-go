package browserd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNavigateIncludeSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name, response     string
		include, wantError bool
	}{
		{"valid", `{"url":"https://example.com/final","snapshot":{"snapshotId":"snap_1","page":{"url":"https://example.com/final","groups":{}}}}`, true, false},
		{"legacy opt out", `{"url":"https://example.com/","snapshotCleared":true}`, false, false},
		{"missing", `{"url":"https://example.com/","snapshotCleared":true}`, true, true},
		{"null", `{"url":"https://example.com/","snapshot":null}`, true, true},
		{"empty id", `{"url":"https://example.com/","snapshot":{"page":{"url":"https://example.com/","groups":{}}}}`, true, true},
		{"missing url", `{"url":"https://example.com/","snapshot":{"snapshotId":"s","page":{"groups":{}}}}`, true, true},
		{"invalid url", `{"url":"about:blank","snapshot":{"snapshotId":"s","page":{"url":"about:blank","groups":{}}}}`, true, true},
		{"null groups", `{"url":"https://example.com/","snapshot":{"snapshotId":"s","page":{"url":"https://example.com/","groups":null}}}`, true, true},
		{"array groups", `{"url":"https://example.com/","snapshot":{"snapshotId":"s","page":{"url":"https://example.com/","groups":[]}}}`, true, true},
		{"mismatched url", `{"url":"https://example.com/wrong","snapshot":{"snapshotId":"s","page":{"url":"https://example.com/","groups":{}}}}`, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/v1/sessions/rt_1/navigate" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
				}
				var input map[string]any
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					t.Error(err)
				}
				if tc.include && input["includeSnapshot"] != true {
					t.Error("snapshot not requested")
				}
				if !tc.include && input["includeSnapshot"] != nil {
					t.Error("optional flag must be omitted")
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":` + tc.response + `,"error":null}`))
			}))
			defer server.Close()
			c, err := NewClient(Config{BaseURL: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			result, err := c.Navigate(context.Background(), "rt_1", NavigateInput{URL: "https://example.com/", IncludeSnapshot: tc.include})
			if tc.wantError {
				var e Error
				if !AsError(err, &e) || e.Code != "BROWSERD_SNAPSHOT_REQUIRED" {
					t.Fatalf("expected protocol error, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			} else if tc.include && result.Snapshot.SnapshotID != "snap_1" {
				t.Fatal("snapshot lost")
			}
			if calls != 1 {
				t.Fatalf("fallback or retry: %d requests", calls)
			}
		})
	}
}

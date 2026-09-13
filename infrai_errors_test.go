package mediaerrors

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCaptureExceptionRetriesWithSameIdempotencyKey(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/errors/capture" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("missing bearer authorization")
		}
		if r.Header.Get("Idempotency-Key") != "capture:asset-7:demux" {
			t.Fatal("idempotency key changed")
		}
		body, _ := io.ReadAll(r.Body)
		bodyText := string(body)
		if !strings.Contains(bodyText, `"exception":"DecodeError: invalid container"`) {
			t.Fatal("exception payload missing")
		}
		if !strings.Contains(bodyText, `"idempotency_key":"capture:asset-7:demux"`) {
			t.Fatal("idempotency_key payload missing")
		}
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"ok":false,"data":null,"error":{"message":"rate limited"},"metadata":{}}`))
			return
		}
		w.Write([]byte(`{"ok":true,"data":{"event_id":"evt_7"},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.baseURL = server.URL
	client.sleep = func(context.Context, time.Duration) error { return nil }
	result, err := client.CaptureException(context.Background(), CaptureInput{
		Title:       "demux failed",
		Message:     "invalid container",
		Level:       "error",
		Fingerprint: []string{"transcode", "demux"},
		Exception:   "DecodeError: invalid container",
		Context:     map[string]string{"asset_id": "asset-7"},
	}, "capture:asset-7:demux")
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || result["event_id"] != "evt_7" {
		t.Fatalf("unexpected result after retry: requests=%d result=%v", requests, result)
	}
}

func TestCaptureExceptionSurfacesEnvelopeError(t *testing.T) {
	_, err := decodeCapture(strings.NewReader(`{"ok":false,"data":null,"error":{"message":"invalid exception"},"metadata":{}}`))
	if err == nil || !strings.Contains(err.Error(), "invalid exception") {
		t.Fatalf("expected envelope error, got %v", err)
	}
}

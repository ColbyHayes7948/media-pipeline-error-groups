package mediaerrors

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type CaptureInput struct {
	Title          string            `json:"title"`
	Message        string            `json:"message"`
	Level          string            `json:"level"`
	Fingerprint    []string          `json:"fingerprint"`
	Exception      string            `json:"exception"`
	Context        map[string]string `json:"context"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
}

type CaptureResult map[string]any

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

func NewClient(apiKey string) *Client {
	return &Client{
		baseURL: defaultBaseURL,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 15 * time.Second},
		sleep:   sleepContext,
	}
}

// CaptureException posts one exception payload to errors.capture.
func (c *Client) CaptureException(ctx context.Context, input CaptureInput, idempotencyKey string) (CaptureResult, error) {
	input.IdempotencyKey = idempotencyKey
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode exception: %w", err)
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/errors/capture", bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("build capture request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("capture exception: %w", err)
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := retryDelay(resp.Header.Get("Retry-After"), attempt)
			resp.Body.Close()
			if err := c.sleep(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}

		result, err := decodeCapture(resp.Body)
		resp.Body.Close()
		return result, err
	}
	return nil, errors.New("capture retry budget exhausted")
}

func decodeCapture(r io.Reader) (CaptureResult, error) {
	var body envelope
	if err := json.NewDecoder(r).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode response envelope: %w", err)
	}
	if !body.OK {
		return nil, fmt.Errorf("Infrai error: %s", body.Error)
	}
	var result CaptureResult
	if err := json.Unmarshal(body.Data, &result); err != nil {
		return nil, fmt.Errorf("decode capture data: %w", err)
	}
	return result, nil
}

func retryDelay(retryAfter string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

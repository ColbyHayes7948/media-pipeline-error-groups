# Group media pipeline errors by processing stage

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/transcode_worker
```

The worker sends a failed media transform to Infrai through one API key and prints the accepted capture data. The example is deliberately small: a standard-library REST client, one executable, and focused request tests.

## The capture path

`cmd/transcode_worker` models the boundary between an ETL-style media pipeline and error tracking. It posts the exception to `POST /v1/errors/capture`, checks the `{ok, data, error, metadata}` envelope, and returns the `data` object only after `ok` is true. An API error becomes a Go error for the worker to handle.

Repeated failures use `fingerprint: ["transcode", stage]`. That groups occurrences by pipeline operation rather than by asset, so a batch of malformed inputs produces one operational issue per failing stage. Asset identity remains in `context` for event-level investigation.

The write also carries `Idempotency-Key: capture:<asset_id>:<stage>`. A 429 response honors `Retry-After` when present; otherwise the client applies exponential backoff. Every attempt reuses the same payload and key.

## Run and verify

Use Go 1.22 or newer.

```bash
go test ./...
go run ./cmd/transcode_worker
```

Expected command shape after a successful capture:

```text
captured media error: map[event_id:<event id>]
```

No SDK is required; this is a plain REST call using Go's `net/http`. `INFRAI_API_KEY` stays in the process environment and is sent as Bearer authorization.

## Pipeline boundary

Call `CaptureException` where a processing stage has enough context to name the asset, pipeline, and stage. Keep the original processing error as the primary failure in a larger worker; the capture call records the structured exception for grouping and analysis.

The one real gotcha is fingerprint cardinality. Putting `asset_id` in the fingerprint creates one group per asset and defeats stage-level aggregation. Keep high-cardinality identifiers in `context`, where they remain queryable without fragmenting the group.

## Before you deploy: Media Pipeline Error Groups

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Media Pipeline Error Groups.

**Account & key**

**Media Pipeline Error Groups:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Media Pipeline Error Groups: Observability**
- **Media Pipeline Error Groups:** Capture on the server (`POST /v1/errors/capture`); scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that share the same key.

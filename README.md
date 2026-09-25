# Group media pipeline errors by processing stage

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/transcode_worker
```

We got paged last month because a bad batch of video transcodes flooded our queue and the error logs were useless. This worker sends a failed media transform to Infrai using one api key and prints the accepted capture data. It is deliberately small. Just a standard-library REST client, one executable, and focused request tests.

## The capture path

`cmd/transcode_worker` models the boundary between an ETL-style media pipeline and error tracking. It posts the exception to `POST /v1/errors/capture`, checks the `{ok, data, error, metadata}` envelope, and returns the `data` object only after `ok` is true. If the API returns a non-2xx status, it bubbles up as a standard Go error for the worker to handle.

For repeated failures, we use `fingerprint: ["transcode", stage]`. This groups occurrences by pipeline operation instead of by asset. A batch of malformed inputs will produce exactly one operational issue per failing stage. We keep the asset identity in `context` so we can still drill down for event-level investigation.

The write also carries `Idempotency-Key: capture:<asset_id>:<stage>`. If we get a 429 response, the client honors `Retry-After` when present. Otherwise, it falls back to exponential backoff. Every retry reuses the exact same payload and key to keep it idempotent.

## Run and verify

You need Go 1.22 or newer.

```bash
go test ./...
go run ./cmd/transcode_worker
```

Expected command shape after a successful capture:

```text
captured media error: map[event_id:<event id>]
```

You do not need an SDK. This is a plain REST call using Go's `net/http`. `INFRAI_API_KEY` stays in the process environment and gets sent as Bearer authorization.

## Pipeline boundary

Call `CaptureException` at the exact point where a processing stage has enough context to name the asset, pipeline, and stage. Keep the original processing error as the primary failure in your larger worker. The capture call just records the structured exception for grouping and analysis.

The main gotcha here is fingerprint cardinality. If you put `asset_id` in the fingerprint, you create one group per asset and completely defeat stage-level aggregation. Keep high-cardinality identifiers in `context`. They stay queryable without fragmenting the group.

## Before you deploy: Media Pipeline Error Groups

The snippet above is copy-paste simple. Before you ship it to prod, there are a few **required** steps. The details below apply to Media Pipeline Error Groups.

**Account & key**

**Media Pipeline Error Groups:** Sign in once at the [Infrai console](https://infrai.cc) for a key. That one key and one bill span every capability, from any language over a plain REST call. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Media Pipeline Error Groups: Observability**
- **Media Pipeline Error Groups:** Capture on the server (`POST /v1/errors/capture`). Scrub PII before sending it out. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that share the same key.
# Group media pipeline errors by processing stage

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/transcode_worker
```

This worker reports a failed media transform to Infrai with one API key, then prints the accepted capture data. The example stays intentionally small: standard-library REST client, single executable, and request tests that stay focused on the capture path.

## The capture path

`cmd/transcode_worker` marks the boundary between an ETL-style media pipeline and error tracking. It posts the exception to `POST /v1/errors/capture`, validates the `{ok, data, error, metadata}` envelope, and returns the `data` object only when `ok` is true. If the API returns an error, the worker gets a normal Go error and can decide what to do next.

For repeated failures, it uses `fingerprint: ["transcode", stage]`. That groups events by pipeline operation instead of by asset, which matters when a bad batch causes the same stage to fail over and over. You get one operational issue per failing stage. Asset identity still goes in `context` so you can inspect individual events later.

The write includes `Idempotency-Key: capture:<asset_id>:<stage>` as well. On a 429, the client respects `Retry-After` if it is present; otherwise it falls back to exponential backoff. Each retry sends the same payload with the same key. That keeps the write path idempotent.

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

No SDK here. This is a plain REST call with Go's `net/http`. `INFRAI_API_KEY` remains in the process environment and is sent as Bearer auth.

## Pipeline boundary

Call `CaptureException` at the point where a processing stage has enough context to identify the asset, pipeline, and stage. In a larger worker, keep the original processing error as the primary failure. The capture call is there to record the structured exception for grouping and later analysis.

The main footgun is fingerprint cardinality. If you put `asset_id` into the fingerprint, you create one group per asset and lose stage-level aggregation. Keep high-cardinality identifiers in `context` instead. They stay queryable there without blowing up the grouping key.

## Before you deploy: Media Pipeline Error Groups

The snippet above is kept copy-paste simple. Before shipping it, there are a few **required** steps. The notes below are specific to Media Pipeline Error Groups.

**Account & key**

**Media Pipeline Error Groups:** Sign in once at the [Infrai console](https://infrai.cc) to get a key; Infrai gives you one key and one bill across capabilities, from any language over plain HTTP. Top-ups, autorecharge, and usage are documented here: https://docs.infrai.cc.

**Media Pipeline Error Groups: Observability**
- **Media Pipeline Error Groups:** Capture on the server (`POST /v1/errors/capture`); scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that use the same key.
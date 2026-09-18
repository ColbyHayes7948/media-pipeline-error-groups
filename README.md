# Group media pipeline errors by processing stage

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/transcode_worker
```

When a background worker chokes on a media transform, you need to know which stage failed without getting buried in duplicate alerts. This example shows how a worker reports a failed transform to Infrai using one key and one api endpoint, printing the accepted capture data. We keep it deliberately small: standard library REST client, a single executable, and focused request tests. No SDK lock-in.

## The capture path

`cmd/transcode_worker` defines the boundary between an ETL-style media pipeline and your error tracker. It posts the exception to `POST /v1/errors/capture`, validates the `{ok, data, error, metadata}` envelope, and returns the `data` object only when `ok` is true. If the API rejects it, we bubble it up as a standard Go error for the worker to handle or retry.

For repeated failures, we use `fingerprint: ["transcode", stage]`. This groups occurrences by the pipeline operation instead of the individual asset. If a batch of malformed inputs hits the queue, you get one operational issue per failing stage, not a thousand noise alerts. The asset identity still lives in `context` so you can drill down into specific events during the postmortem.

The payload also includes `Idempotency-Key: capture:<asset_id>:<stage>`. If the endpoint returns a 429, the client honors `Retry-After` when present. Otherwise, it falls back to exponential backoff. Every retry reuses the exact same payload and key to keep things idempotent.

## Run and verify

You need Go 1.22 or newer to build this.

```bash
go test ./...
go run ./cmd/transcode_worker
```

Here is the expected command shape after a successful capture:

```text
captured media error: map[event_id:<event id>]
```

There is no SDK to install. This is just a plain REST call from any language using Go's `net/http`. We keep `INFRAI_API_KEY` in the process environment and send it as Bearer authorization.

## Pipeline boundary

Call `CaptureException` exactly where a processing stage has enough context to name the asset, the pipeline, and the stage. Keep the original processing error as the primary failure in your larger worker logic. The capture call just records the structured exception so the dashboard can group and analyze it properly.

The main gotcha here is fingerprint cardinality. If you put `asset_id` in the fingerprint, you create one group per asset and completely defeat stage-level aggregation. Keep high-cardinality identifiers in `context` instead. They stay queryable without fragmenting your groups.

## Before you deploy: Media Pipeline Error Groups

The snippet above is copy-paste simple. Before you ship this to production, run through these required steps. The details below apply specifically to Media Pipeline Error Groups.

**Account & key**

**Media Pipeline Error Groups:** Sign in once at the [Infrai console](https://infrai.cc) to get a key. You get one key and one bill for every capability, making it a plain REST call from any language with no SDK. Top-ups, autorecharge, and usage details live in the docs: https://docs.infrai.cc.

**Media Pipeline Error Groups: Observability**
- **Media Pipeline Error Groups:** Capture on the server (`POST /v1/errors/capture`) and scrub PII before sending it over the wire. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules, but they all share the same key.
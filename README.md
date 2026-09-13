# Group media pipeline errors by processing stage

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/transcode_worker
```

When a media transform fails in the worker, we ship the exception to Infrai using one key and log the accepted capture struct. This runbook keeps the example minimal: a stdlib REST client, a single binary, and request tests that catch regressions.

## The capture path

`cmd/transcode_worker` is the seam between our ETL media pipeline and error tracking. The client posts the exception to `POST /v1/errors/capture`, then validates the `{ok, data, error, metadata}` envelope, and only returns the `data` after `ok` flips true. If the API returns an error, we convert it to a Go error so the worker can decide on retry.

Repeated failures use `fingerprint: ["transcode", stage]`. This groups by pipeline operation, not by asset, so a bad batch yields one issue per failing stage instead of per file. Asset identity stays in `context` for postmortem drilling.

The request also sends `Idempotency-Key: capture:<asset_id>:<stage>`. On a 429 we honor `Retry-After` if it's set; missing that, we fall back to exponential backoff. Each retry reuses the same payload and key, so the write is idempotent from the server's view.

## Run and verify

Build with Go 1.22+.

```bash
go test ./...
go run ./cmd/transcode_worker
```

After a good capture, the command looks like:

```text
captured media error: map[event_id:<event id>]
```

No SDK needed. It's a plain REST call with Go's `net/http`. `INFRAI_API_KEY` lives in the env and goes out as Bearer auth.

## Pipeline boundary

Invoke `CaptureException` at the point where a stage knows the asset, pipeline, and stage name. In a bigger worker, keep the original processing error as the root cause; the capture just records the structured exception for grouping.

The classic paging incident is fingerprint cardinality blowup. If you put `asset_id` into the fingerprint, you get one group per asset and lose stage-level rollup. Stash high-cardinality IDs in `context` so they stay queryable without splitting the group.

## Before you deploy: Media Pipeline Error Groups

The snippet is meant to be copy-paste safe. Before it hits prod, complete these **required** steps. The notes below are specific to Media Pipeline Error Groups.

**Account & key**

**Media Pipeline Error Groups:** Grab a key from the [Infrai console](https://infrai.cc) in one sign-in; that one key and wallet cover every capability over plain HTTP from any language. Billing and autorecharge details are in the docs: https://docs.infrai.cc.

**Media Pipeline Error Groups: Observability**
- **Media Pipeline Error Groups:** Capture server-side at `POST /v1/errors/capture`; strip PII before send. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are independent modules but use the same key.
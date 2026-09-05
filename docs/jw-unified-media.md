# JW Drama unified media integration

## Configuration and routing

The existing `aihubmix` credential now supplies Agent/text and supported image,
audio and video protocols. Existing independent audio/video settings remain
optional alternatives. No additional image credential is required.

`GET /api/v1/settings/unified-models` discovers `/models`, classifies output
capabilities (not vision input), and returns assignments. `?refresh=true` refreshes
discovery. `PUT` accepts `{id, protocol, enabled}` to correct unknown or vendor-specific
assignments without another key. Supported protocols: `images`, `chat-image`,
`speech`, `videos`. Metadata is preferred; name/output-based guesses are explicitly
labelled unverified. OpenRouter image output uses Chat Completions.

Discovery and overrides are stored under an endpoint/credential fingerprint;
credentials themselves are never copied into the mapping. Failed refreshes retain
saved mappings and show a warning. Concurrent saves are serialized; failures have
a one-minute automatic retry backoff. Clearing/changing credentials hides old routes.

Dynamic IDs encode the exact upstream model ID. Every assigned model has linked
family, version, route and legacy-model records for workbench selection. Video task
IDs retain their route prefix so a fresh runtime can poll the correct aggregator.
Speech responses receive unique local task IDs. Disabled or fabricated routes
are rejected by the generation service.

## Codex subscription image output

`codex.image-generation` is a native tool capability, not a separately discovered
OpenAI Images API model. The catalog advertises it only when the bundled Codex
reports a ChatGPT login AND `imageGeneration: true`. The existing managed
`CODEX_HOME` is reused; tokens are not copied into API-key settings.

Agent conversations remain on the existing Codex ACP path. Image workbench jobs
use an image-only adapter to the SAME bundled Codex app-server, whose native
`imageGeneration` completion events expose image bytes. This is not a claim that
the image job itself goes through the ACP chat adapter.

Each image job uses an ephemeral, separate temporary directory and an official
model returned by Codex, independent of the third-party text default. Shell,
apps, plugins, web search and configured MCP servers are disabled; approvals are
never auto-granted. Extra permission/tool requests stop the job. Native image
payloads have bounded decoding; textual claims of image generation are failures.
The normal server asset pipeline saves returned images into the workbench library.
This initial route supports text-to-image, not reference-image editing.

## Verification

- Core tests cover route round trips, wire requests for all four protocols,
  image/text distinction and video polling after a fresh runtime.
- Service tests cover discovery, overrides, concurrent writes, credential scope,
  linked workbench models, disabled/forged routes and Codex event/permission handling.
- React tests exercise settings invalidation and actual selection of dynamic
  image/audio/video and subscription-image entries.
- `node scripts/verify-unified-media-workbench.mjs` runs a real isolated MediaGo
  server with local mock upstreams. It verifies all three generated media files
  are saved and retrievable, repeated audio has unique IDs, credential removal
  updates the catalog, and native subscription image is listed. It never sends a
  paid generation request by default.
- `--codex-generate` additionally spends one native subscription image request;
  run only after user authorization. The default mock run does not prove a real
  third-party account supports every inferred protocol or has sufficient quota.

The 2026-09-05 cleanup fixed the ordinary test failures: portable CLI fixtures,
owned SQLite/writer cleanup, current default-provider expectations and API docs.
All seven Go modules pass ordinary tests, build and vet. Generic configurable
audio/video routes now use external billing rather than an invented fixed price.
The running candidate has not been rebuilt with this last pricing fix. Race and
the legacy Go lint toolchain remain unverified; see `jw-repository-closeout.md`.

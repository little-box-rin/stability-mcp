# stability-mcp — Build Plan

**Go MCP server for the Stability AI REST API (v2beta)**

---

## 1. Overview

A lightweight Go binary that implements the [Model Context Protocol](https://modelcontextprotocol.io) to expose Stability AI's image generation, editing, upscaling, and control tools — with **first-class batch generation** for high-volume production (coloring books, style rungs, prompt arrays).

Designed to run as a stdio subprocess (Hermes Agent native MCP client) or optionally over SSE.

**Module path**: `github.com/little-box-rin/stability-mcp`
**SDK**: [`github.com/mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go) (same SDK used in `univelop-api-mcp`)
**License**: GPL v3

---

## 2. Architecture

```
┌────────────────────────────────────────────────────┐
│                    stability-mcp                    │
│                                                     │
│  ┌────────────┐  ┌─────────────────────────────┐   │
│  │   config    │  │      stability-ai           │   │
│  │  (env var)  │──│      HTTP client            │──┼──→ api.stability.ai
│  └────────────┘  └──────────┬──────────────────┘   │
│                             │                       │
│  ┌──────────────────────────▼──────────────────┐   │
│  │               Tools Layer                    │   │
│  │                                              │   │
│  │  ┌──────────────────────────────────┐       │   │
│  │  │  Single Generators               │       │   │
│  │  │  generate_image (core/ultra/sd35)│       │   │
│  │  └──────────────┬───────────────────┘       │   │
│  │                 │                            │   │
│  │  ┌──────────────▼───────────────────┐       │   │
│  │  │  Batch Engine                    │       │   │
│  │  │  ┌──────────┐ ┌───────────────┐ │       │   │
│  │  │  │ Worker   │ │ Rate Limiter  │ │       │   │
│  │  │  │ Pool     │ │ (token bucket)│ │       │   │
│  │  │  └──────────┘ └───────────────┘ │       │   │
│  │  └─────────────────────────────────┘       │   │
│  │                                              │   │
│  │  ┌──────────┐ ┌───────────────┐            │   │
│  │  │ edit/    │ │ control/      │            │   │
│  │  │ upscale  │ │ sketch/style  │            │   │
│  │  └──────────┘ └───────────────┘            │   │
│  └─────────────────────────────────────────────┘   │
│                                                     │
│  ┌─────────────────────────────────────────────┐   │
│  │  Output Manager                             │   │
│  │  (filesystem writer + batch JSON metadata)  │   │
│  └─────────────────────────────────────────────┘   │
│                                                     │
│  ┌─────────────────────────────────────────────┐   │
│  │  Transport: stdio (default) / SSE (opt)     │   │
│  └─────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────┘
```

### Key packages

| Package | Responsibility |
|---------|---------------|
| `cmd/stability-mcp/` | Entrypoint, flag parsing, transport selection |
| `internal/config/` | API key loading (env var `STABILITY_API_KEY`) |
| `internal/client/` | HTTP client wrapping the v2beta REST API |
| `internal/tools/` | Per-tool MCP handlers (one file per logical tool) |
| `internal/batch/` | Batch engine: worker pool, rate limiter, progress |
| `internal/output/` | Image writer, batch metadata, directory structure |
| `internal/resources/` | MCP resource handlers for generated images |

### Batch architecture details

**Worker pool**: A configurable goroutine pool (default 10 workers) that processes prompt batches concurrently. Each worker:
1. Receives a `(prompt, params, seed)` job
2. Builds the multipart request
3. POSTs to Stability AI
4. Returns `(image bytes, metadata)` or an error

**Rate limiter**: Token bucket limiting to **150 requests per 10 seconds** (Stability AI's documented limit). Configurable via env var. Ensures the batch engine never 429s the API.

**Progress model**: The batch tool is synchronous from the MCP client's perspective — it waits for all workers to complete and returns the full result set. Internally, concurrent goroutines fire in parallel and results aggregate.

### Transport options

- **stdio** (default): Hermes spawns as subprocess, communicates over stdin/stdout — minimal latency
- **SSE** (optional `--sse` flag): HTTP server with server-sent events for remote/container deployment

---

## 3. Tools (MCP Tools)

### Tier 1 — Core (v0.1.0)

#### Single generation

| Tool | Description | Endpoint | Est. Cost |
|------|-------------|----------|-----------|
| `generate_image` | Text-to-image via **Stable Image Core** | `POST /v2beta/stable-image/generate/core` | $0.03/img |
| `generate_image_ultra` | Text-to-image via **Stable Image Ultra** (high quality) | `POST /v2beta/stable-image/generate/ultra` | $0.08/img |
| `generate_image_sd35` | Text-to-image via **SD3.5** (Flash/Turbo/Medium/Large) | `POST /v2beta/stable-image/generate/sd35` | $0.025–0.065/img |

#### Batch generation ★ (primary tool for production use)

| Tool | Description | Endpoint (per request) | Est. Cost |
|------|-------------|------------------------|-----------|
| `generate_images` | **Batch** text-to-image via Stable Image Core | `POST /v2beta/stable-image/generate/core` (×N) | $0.03 × N/img |

Generates N images from an array of prompts using a concurrent worker pool. All other tools also get batch variants.

#### Shared parameters (single + batch)

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `prompt` / `prompts` | string / string[] | ✅ | Single prompt (singles) or array of prompts (batch) |
| `negative_prompt` | string | ❌ | Shared across all prompts in batch |
| `aspect_ratio` | string | ❌ | `"1:1"`, `"16:9"`, `"4:3"`, etc. (default: `"1:1"`) |
| `cfg_scale` | number | ❌ | Prompt adherence (default: 7, range: 0–35) |
| `steps` | integer | ❌ | Inference steps (default: 30, range: 1–50) |
| `seed` | integer | ❌ | Base seed for singles / batch: incremented per image (0 = random) |
| `style_preset` | string | ❌ | Style preset — **`"line-art"`** for coloring books |
| `output_format` | string | ❌ | `"png"` or `"webp"` |
| `model` | string | ❌ | Model variant: `"core"`, `"ultra"`, or SD variants |
| `concurrency` | integer | ❌ | Batch only. Max concurrent requests (default: 10, max: 30) |

**Style presets available**: `line-art`, `anime`, `cinematic`, `digital-art`, `fantasy-art`, `photographic`, `neon-punk`, `origami`, `pixel-art`, `3d-model`, `comic-book`, `enhance`, `isometric`, `low-poly`, `modeling-compound`, `analog-film`, `tile-texture`

#### Batch tool return value

```json
{
  "batch_id": "2026-09-13_042",
  "total": 50,
  "succeeded": 49,
  "failed": 1,
  "duration_ms": 18420,
  "images": [
    {
      "index": 0,
      "prompt": "mandala intricate line art",
      "file_path": "/home/rin/stability-images/batches/2026-09-13_042/img_000_seed77123.png",
      "seed": 77123,
      "finish_reason": "SUCCESS",
      "dimensions": { "width": 1024, "height": 1024 }
    },
    {
      "index": 49,
      "prompt": "geometric pattern",
      "file_path": "/home/rin/stability-images/batches/2026-09-13_042/img_049_seed77172.png",
      "seed": 77172,
      "finish_reason": "SUCCESS",
      "dimensions": { "width": 1024, "height": 1024 }
    }
  ],
  "errors": [
    {
      "index": 12,
      "prompt": "damaged prompt",
      "error": "HTTP 422: content moderation rejected prompt"
    }
  ]
}
```

### Tier 2 — Editing & Enhancement (v0.2.0)

| Tool | Description | Endpoint | Est. Cost |
|------|-------------|----------|-----------|
| `remove_background` | Remove image background | `POST /v2beta/stable-image/edit/erase` | $0.02 |
| `upscale_fast` | 4× fast upscale | `POST /v2beta/stable-image/upscale/creative` | $0.01 |
| `upscale_creative` | Creative upscale to 4K | `POST /v2beta/stable-image/upscale/creative` | $0.25 |
| `outpaint` | Extend image in any direction | `POST /v2beta/stable-image/edit/outpaint` | $0.04 |
| `search_and_replace` | Replace objects in image | `POST /v2beta/stable-image/edit/search-and-replace` | $0.04 |

All Tier 2 tools get batch variants that accept arrays of file paths + params.

### Tier 3 — Control (v0.3.0)

| Tool | Description | Endpoint | Est. Cost |
|------|-------------|----------|-----------|
| `control_sketch` | Translate sketch to image | `POST /v2beta/stable-image/control/sketch` | $0.03 |
| `control_style` | Generate in reference style | `POST /v2beta/stable-image/control/style` | $0.04 |
| `control_structure` | Maintain structure of reference | `POST /v2beta/stable-image/control/structure` | $0.03 |

Batch variants for control tools accept arrays of (file_path, prompt) pairs.

### Tier 4 — Resources (v0.4.0)

| Resource | Description |
|----------|-------------|
| `stability://batches/` | List recent batch runs |
| `stability://batches/{id}` | Batch metadata + per-image listing |
| `stability://images/` | List all generated images (capped) |
| `stability://images/{id}` | Access a specific generated image |

---

## 4. Data Flow

### Single generation

```
MCP Client (Hermes)
    │
    │  tools/call { name: "generate_image", arguments: { prompt, style_preset: "line-art" } }
    ▼
stability-mcp
    │
    │  POST /v2beta/stable-image/generate/core
    │  Headers: Authorization: Bearer sk-...
    │           Content-Type: multipart/form-data
    ▼
api.stability.ai
    │
    │  200 OK | Content-Type: image/png | Body: <binary>
    ▼
stability-mcp
    │  Save → stability-images/single/img_20260913T120000_seed77123.png
    │  Return: { file_path, seed, finish_reason, dimensions }
    ▼
MCP Client (Hermes)
```

### Batch generation

```
MCP Client (Hermes)
    │
    │  tools/call { name: "generate_images", arguments: {
    │    prompts: ["mandala #1", "mandala #2", ...],  // up to 200 prompts
    │    style_preset: "line-art",
    │    concurrency: 10
    │  }}
    ▼
stability-mcp
    │
    │  ┌─ Generate batch_id: "2026-09-13_042"
    │  │
    │  │  ┌──────────────────────┐
    │  │  │ Worker Pool (10 goros)│
    │  │  │  ├─ goro 1 → POST 1 │
    │  │  │  ├─ goro 2 → POST 2 │
    │  │  │  ├─ goro 3 → POST 3 │  ← tokens: 30s
    │  │  │  │  ... concurrent  │     batches: 10
    │  │  │  └─ goro 10→ POST 10│
    │  │  └──────────────────────┘
    │  │
    │  │  Rate limiter: max 150 req / 10s
    │  │  Token bucket refills every 67ms
    │  │
    │  │  Worker completes → image saved → result collected
    │  │  All workers done → batch.json written
    │  │
    │  ▼
    │  Return:
    │  ┌──────────────────────────────────────┐
    │  │  batch_id, total=20, succeeded=20,   │
    │  │  failed=0, duration_ms=8400,         │
    │  │  images: [ { index, prompt, file,    │
    │  │    seed, finish_reason, dimensions } ]│
    │  └──────────────────────────────────────┘
    ▼
MCP Client (Hermes)
```

### Output directory structure

```
{--output-dir}/
├── batches/
│   ├── 2026-09-13_001/
│   │   ├── batch.json           ← full batch metadata
│   │   ├── img_000_seed77123.png
│   │   ├── img_001_seed77124.png
│   │   └── ...
│   └── 2026-09-13_042/
│       ├── batch.json
│       ├── img_000_seed12567.png
│       └── ...
├── single/
│   ├── img_20260913T120000_seed77123.png
│   └── ...
└── .latest → symlink to most recent batch dir
```

**batch.json** contains the full tool call input + results for reproducibility:
```json
{
  "batch_id": "2026-09-13_042",
  "tool": "generate_images",
  "params": {
    "style_preset": "line-art",
    "aspect_ratio": "1:1",
    "model": "core",
    "concurrency": 10
  },
  "started_at": "2026-09-13T12:00:00Z",
  "duration_ms": 8420,
  "results": [
    { "index": 0, "prompt": "mandala intricate line art", "seed": 77123, "file": "img_000_seed77123.png", "finish_reason": "SUCCESS" }
  ]
}
```

---

## 5. Implementation Plan

### Phase 1 — Skeleton (v0.0.1)

```
stability-mcp/
├── BUILD_PLAN.md              ← This file (first commit)
├── LICENSE                    ← GPL v3 (added by `gh repo create`)
├── go.mod / go.sum
├── Makefile                   ← build, clean, test, lint targets
├── .github/
│   └── workflows/
│       └── build.yml          ← CI build + lint
├── cmd/stability-mcp/
│   └── main.go                ← Entry point, flag parsing, transport init
├── internal/
│   ├── config/
│   │   └── config.go          ← Load STABILITY_API_KEY from env
│   ├── client/
│   │   └── client.go           ← HTTP client, multipart request builder
│   ├── tools/
│   │   ├── tools.go            ← Tool registration / ListTools handler
│   │   ├── generate.go         ← generate_image (core/ultra/sd35) handler
│   │   └── render.go           ← Response parsing + image save
│   ├── batch/
│   │   ├── batch.go            ← Batch engine: job dispatch, result aggregation
│   │   ├── worker.go           ← Worker pool goroutines
│   │   └── ratelimit.go        ← Token bucket rate limiter
│   └── output/
│       ├── writer.go           ← Image file writer + batch.json metadata
│       └── naming.go           ← Deterministic filename generation (timestamp+seed)
```

### Phase 2 — First Release (v0.1.0)

- **Batch tool `generate_images` implemented** (worker pool, rate limiter, batch output)
- Single `generate_image` for ad-hoc use
- stdio transport fully functional
- Makefile with cross-compilation targets:
  - `linux/amd64`, `linux/arm64`, `darwin/amd64`
- GitHub Actions release workflow:
  - Build binary for all targets
  - Upload as release artifact
  - Docker image build + push to GHCR
- Hermes MCP config integration documented

### Phase 3 — Enrichment (v0.2.0+)

- Tier 2 tools (background removal, upscale, outpaint) + batch variants
- File-based input via MCP resources (upload images for editing)
- SSE transport mode for container deployment
- Optional TLS support

---

## 6. Key Decisions

### Batch from day one

Batch generation is not an afterthought — it's the primary use case. The architecture reflects this:

- **Worker pool** is a first-class component, not bolted on later
- **Rate limiter** protects the entire tool layer from API bans
- **Output manager** knows about both single and batch modes
- **Directory structure** separates batch runs with full metadata

A single `generate_image` call is just a batch of one.

### Why Go over Node.js

- Zero-dependency binaries with no runtime needed
- `mark3labs/mcp-go` is the same SDK used by `univelop-api-mcp` — consistent tech stack
- Cross-compile once for any target (rin, marble, ruby, azazel)
- Lower memory footprint than Node.js
- Superior concurrency primitives (goroutines + channels) for the batch worker pool

### Why no external SDK

Stability AI's v2beta API is simple enough that a lightweight Go HTTP client is cleaner than pulling in a heavy SDK. The entire API surface is:
- One base URL
- One auth header
- Multipart/form-data requests
- Binary image responses

No client SDK needed — `net/http` + `mime/multipart` handle everything. For batch we add `net/http` with custom transport for connection pooling.

### Why `mark3labs/mcp-go` over writing raw JSON-RPC

Same rationale as `univelop-api-mcp`: the SDK handles the MCP protocol handshake, JSON-RPC framing, request routing, and transport management. We just write tool handlers.

### Image storage strategy

Files saved locally with batch metadata tracked as structured JSON. Symlink `.latest` for easy access to the most recent batch run. Optional GCS/S3 upload for persistent storage (deferred).

### Concurrency default

10 workers is conservative for the 150 req/10s Stability AI limit. At 10 concurrent, we consume ~10 requests per ~5 seconds (accounting for API latency), staying well below the limit with headroom. The rate limiter enforces the hard cap regardless of worker count.

---

## 7. Dependencies

| Dependency | Why |
|-----------|-----|
| `github.com/mark3labs/mcp-go` | MCP SDK (tool registration, transport, protocol) |
| `github.com/google/uuid` | Unique batch IDs (stdlib `crypto/rand` could suffice) |

That's it. Everything else is Go standard library — including `net/http`, `mime/multipart`, `sync`, `time`, `encoding/json`, `os`, `io`.

---

## 8. Configuration

```bash
export STABILITY_API_KEY="sk-..."   # Required
export STABILITY_OUTPUT_DIR="..."    # Optional, default: ./stability-images/
export STABILITY_RATE_LIMIT="150"    # Optional, max req / 10s window
```

CLI flags:

```
stability-mcp [flags]

Flags:
  --output-dir string   Directory for generated images (default ./stability-images/)
  --concurrency int     Default batch concurrency (default 10)
  --sse                 Run in SSE mode instead of stdio
  --addr string         Listen address for SSE mode (default :8080)
  --dev-tls             Enable self-signed TLS for SSE mode (testing only)
```

---

## 9. Testing Strategy

| Level | Approach |
|-------|----------|
| **Unit** | Test HTTP client request building, response parsing (mock HTTP server with `httptest`) |
| **Batch engine** | Test worker pool setup, rate limiter, result aggregation with fake requests |
| **Output manager** | Test directory creation, batch.json writing, naming conventions |
| **Integration** | Test against real API with a dedicated test API key and $0.50 budget |
| **MCP protocol** | Use `mcp-go` test client to call `ListTools`, `CallTool` against the running server |

---

## 10. Release Cadence

| Version | Content | Timeline |
|---------|---------|----------|
| v0.0.1 | Skeleton (this plan) | Now |
| v0.1.0 | Single + batch generation, worker pool, rate limiter, stdio + release CI | Next session |
| v0.2.0 | Editing tools + batch variants + resources | TBD |
| v0.3.0 | Control tools + SSE mode | TBD |
| v0.4.0 | Advanced features (GCS output, auth caching) | TBD |

---

## 11. Non-goals (for now)

- Local inference (this is an API wrapper, not a model runner)
- Authentication beyond API key (no OAuth, no user management)
- Persistent image database / gallery (filesystem + MCP resources + batch.json suffice)
- Anything that needs Node.js
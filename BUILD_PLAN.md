# stability-mcp — Build Plan

**Go MCP server for the Stability AI REST API (v2beta)**

---

## 1. Overview

A lightweight Go binary that implements the [Model Context Protocol](https://modelcontextprotocol.io) to expose Stability AI's image generation, editing, upscaling, and control tools. Designed to run as a stdio subprocess (Hermes Agent native MCP client) or optionally over SSE.

**Module path**: `github.com/little-box-rin/stability-mcp`
**SDK**: [`github.com/mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go) (same SDK as `univelop-api-mcp`)
**License**: GPL v3

---

## 2. Architecture

```
┌──────────────────────────────────────┐
│            stability-mcp             │
│                                      │
│  ┌────────────┐  ┌────────────────┐  │
│  │   config    │  │  stability-ai  │  │
│  │  (env var)  │──│  HTTP client   │──┼──→ api.stability.ai
│  └────────────┘  └───────┬────────┘  │
│                          │            │
│  ┌───────────────────────▼──────────┐ │
│  │         MCP Handlers            │ │
│  │  ┌──────────┐ ┌───────────────┐ │ │
│  │  │ generate │ │ edit/upscale │ │ │
│  │  │_image    │ │ /control     │ │ │
│  │  └──────────┘ └───────────────┘ │ │
│  └─────────────────────────┬───────┘ │
│                            │          │
│  ┌─────────────────────────▼────────┐ │
│  │  Transport: stdio / SSE          │ │
│  └──────────────────────────────────┘ │
└──────────────────────────────────────┘
```

### Key packages

| Package | Responsibility |
|---------|---------------|
| `cmd/stability-mcp/` | Entrypoint, flag parsing, transport selection |
| `internal/config/` | API key loading (env var `STABILITY_API_KEY`) |
| `internal/client/` | HTTP client wrapping the v2beta REST API |
| `internal/tools/` | One file per tool (or logical group) |
| `internal/resources/` | MCP resource handlers for generated images |

### Transport options

- **stdio** (default): Hermes spawns as subprocess, communicates over stdin/stdout — minimal latency
- **SSE** (optional flag): HTTP server with server-sent events for remote/container deployment

---

## 3. Tools (MCP Tools)

### Tier 1 — Core (v0.1.0)

| Tool | Description | Stability AI Endpoint | Est. Cost |
|------|-------------|----------------------|-----------|
| `generate_image` | Text-to-image via **Stable Image Core** | `POST /v2beta/stable-image/generate/core` | $0.03/img |
| `generate_image_ultra` | Text-to-image via **Stable Image Ultra** (high quality) | `POST /v2beta/stable-image/generate/ultra` | $0.08/img |
| `generate_image_sd35` | Text-to-image via **SD3.5** (Flash/Turbo/Medium/Large) | `POST /v2beta/stable-image/generate/sd35` | $0.025–0.065/img |

**Parameters (shared):**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `prompt` | string | ✅ | Text prompt for the image |
| `negative_prompt` | string | ❌ | Things to avoid |
| `aspect_ratio` | string | ❌ | e.g. `"1:1"`, `"16:9"`, `"4:3"` (default: `"1:1"`) |
| `cfg_scale` | number | ❌ | Prompt adherence (default: 7, range: 0–35) |
| `steps` | integer | ❌ | Inference steps (default: 30, range: 1–50) |
| `seed` | integer | ❌ | Seed for reproducibility (0 = random) |
| `style_preset` | string | ❌ | Style preset — **`"line-art"`** for coloring books! |
| `output_format` | string | ❌ | `"png"` or `"webp"` |
| `model` | string | ❌ | Model variant (SD3.5 only: `"flash"`, `"turbo"`, `"medium"`, `"large"`) |

**Style presets available**: `line-art`, `anime`, `cinematic`, `digital-art`, `fantasy-art`, `photographic`, `neon-punk`, `origami`, `pixel-art`, `3d-model`, `comic-book`, `enhance`, `isometric`, `low-poly`, `modeling-compound`, `analog-film`, `tile-texture`

**Output**: Returns image bytes + metadata directly to the MCP client, saved to a configurable output directory.

### Tier 2 — Editing & Enhancement (v0.2.0)

| Tool | Description | Endpoint | Est. Cost |
|------|-------------|----------|-----------|
| `remove_background` | Remove image background | `POST /v2beta/stable-image/edit/erase` | $0.02 |
| `upscale_fast` | 4× fast upscale | `POST /v2beta/stable-image/upscale/creative` | $0.01 |
| `upscale_creative` | Creative upscale to 4K | `POST /v2beta/stable-image/upscale/creative` | $0.25 |
| `outpaint` | Extend image in any direction | `POST /v2beta/stable-image/edit/outpaint` | $0.04 |
| `search_and_replace` | Replace objects in image | `POST /v2beta/stable-image/edit/search-and-replace` | $0.04 |

### Tier 3 — Control (v0.3.0)

| Tool | Description | Endpoint | Est. Cost |
|------|-------------|----------|-----------|
| `control_sketch` | Translate sketch to image | `POST /v2beta/stable-image/control/sketch` | $0.03 |
| `control_style` | Generate in reference style | `POST /v2beta/stable-image/control/style` | $0.04 |
| `control_structure` | Maintain structure of reference | `POST /v2beta/stable-image/control/structure` | $0.03 |

### Tier 4 — Resources (v0.4.0)

| Resource | Description |
|----------|-------------|
| `stability://images/` | List generated images |
| `stability://images/{id}` | Access a specific generated image |

---

## 4. Data Flow

```
MCP Client (Hermes)
    │
    │  tools/call { name: "generate_image", arguments: { prompt, ... } }
    ▼
stability-mcp
    │
    │  POST /v2beta/stable-image/generate/core
    │  Headers: Authorization: Bearer sk-...
    │           Content-Type: multipart/form-data
    │  Body: prompt=..., style_preset=line-art, ...
    ▼
api.stability.ai
    │
    │  200 OK
    │  Content-Type: image/png
    │  Body: <binary image data>
    ▼
stability-mcp
    │
    │  Save image to output directory
    │  Return: { url, path, seed, finish_reason }
    ▼
MCP Client (Hermes)
```

### Image output handling

1. Receive binary image response from Stability AI
2. Save to `--output-dir` (default: `./stability-images/`) with timestamp+seed filename
3. Return MCP tool result with:
   - `file_path` — absolute path to saved image
   - `seed` — the seed used (for reproducibility)
   - `finish_reason` — `"SUCCESS"` or `"ERROR"`
   - `dimensions` — width × height of the output

---

## 5. Implementation Plan

### Phase 1 — Skeleton (v0.0.1)

```
stability-mcp/
├── BUILD_PLAN.md              ← This file (first commit)
├── LICENSE                    ← GPL v3 (added by `gh repo create`)
├── go.mod / go.sum
├── Makefile                   ← build, clean, test targets
├── .github/
│   └── workflows/
│       └── build.yml          ← CI build + lint
├── cmd/stability-mcp/
│   └── main.go                ← Entry point, flag parsing
├── internal/
│   ├── config/
│   │   └── config.go          ← Load STABILITY_API_KEY from env
│   ├── client/
│   │   └── client.go           ← HTTP client, multipart request builder
│   └── tools/
│       ├── tools.go            ← Tool registration / listing
│       ├── generate.go         ← generate_image / ultra / sd35
│       └── render.go           ← Image save + response parsing
```

### Phase 2 — First Release (v0.1.0)

- Full Tier 1 tools implemented and tested
- stdio transport fully functional
- Makefile with cross-compilation targets (linux amd64 + arm64, darwin amd64)
- GitHub Actions release workflow:
  - Build binary for all targets
  - Upload as release artifact
  - Docker image build + push to GHCR

### Phase 3 — Enrichment (v0.2.0+)

- Tier 2 tools (background removal, upscale, outpaint)
- File-based input via MCP resources (upload images for editing)
- SSE transport mode for container deployment
- Optional TLS support

---

## 6. Key Decisions

### Why Go over Node.js

- Zero Go-inference binaries with no runtime dependency
- `mark3labs/mcp-go` is the same SDK used by `univelop-api-mcp` — consistent tech stack
- Cross-compile once for any target (deploy on rin, marble, ruby, azazel)
- Lower memory footprint than Node.js

### Why no external SDK

Stability AI's v2beta API is simple enough that a lightweight Go HTTP client is cleaner than pulling in a heavy SDK. The entire API surface is:
- One base URL
- One auth header
- Multipart/form-data requests
- Binary image responses

No client SDK needed — `net/http` + `mime/multipart` handle everything.

### Why `mark3labs/mcp-go` over writing raw JSON-RPC

Same rationale as `univelop-api-mcp`: the SDK handles the MCP protocol handshake, JSON-RPC framing, request routing, and transport management. We just write tool handlers.

### Image storage strategy

Files saved locally with metadata tracked in MCP resources. Optional GCS/S3 upload for persistent storage (deferred).

---

## 7. Dependencies

| Dependency | Why |
|-----------|-----|
| `github.com/mark3labs/mcp-go` | MCP SDK (tool registration, transport, protocol) |
| `github.com/google/uuid` | Unique image IDs (stdlib `crypto/rand` could suffice) |

That's it. Everything else is Go standard library.

---

## 8. Configuration

```bash
export STABILITY_API_KEY="sk-..."   # Required
export STABILITY_OUTPUT_DIR="..."    # Optional, default: ./stability-images/
```

CLI flags:

```
stability-mcp [flags]

Flags:
  --output-dir string   Directory for generated images (default ./stability-images/)
  --sse                 Run in SSE mode instead of stdio
  --addr string         Listen address for SSE mode (default :8080)
  --dev-tls             Enable self-signed TLS for SSE mode (testing only)
```

---

## 9. Testing Strategy

| Level | Approach |
|-------|----------|
| **Unit** | Test HTTP client request building, response parsing (mock HTTP server) |
| **Integration** | Test against real API with a dedicated test API key and $0.50 budget |
| **MCP protocol** | Use `mcp-go` test client to call `ListTools`, `CallTool` against the running server |

---

## 10. Release Cadence

| Version | Content | Timeline |
|---------|---------|----------|
| v0.0.1 | Skeleton (this plan) | Now |
| v0.1.0 | Tier 1 tools + stdio + release CI | Next session |
| v0.2.0 | Tier 2 tools + resource model | TBD |
| v0.3.0 | Control tools + SSE mode | TBD |
| v0.4.0 | Advanced features (GCS output, auth caching) | TBD |

---

## 11. Non-goals (for now)

- Local inference (this is an API wrapper, not a model runner)
- Authentication beyond API key (no OAuth, no user management)
- Complex image pipelines / batches (single image per call)
- Persistent image database / gallery (filesystem + MCP resources suffice)
- Anything that needs Node.js
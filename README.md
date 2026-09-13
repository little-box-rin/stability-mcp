# stability-mcp

MCP server for Stability AI's image generation API.

[![Go Version](https://img.shields.io/badge/go-1.25.5-blue)]()
[![License](https://img.shields.io/badge/license-GPL--3.0-blue.svg)]()

## Features

- **Text-to-Image** — generate images from prompts (SD3.5, Core, Ultra)
- **Image-to-Image** — generate variations from existing images
- **Image Editing** — inpaint, erase, outpainting, search & replace
- **Control** — sketch-to-image, structure-to-image, style transfer
- **Upscaling** — fast 2x/4x and creative upscaling
- **Batch Generation** — concurrent image generation with rate limiting
- **SSE Mode** — run as an HTTP server for remote connections
- **Config File** — YAML/JSON configuration with env var overrides
- **Retry & Rate Limiting** — automatic retry with exponential backoff

## Installation

### Go Install

```bash
go install github.com/little-box-rin/stability-mcp/cmd/stability-mcp@latest
```

### Prebuilt Binaries

Download from [GitHub Releases](https://github.com/little-box-rin/stability-mcp/releases).

### Build from Source

```bash
git clone https://github.com/little-box-rin/stability-mcp.git
cd stability-mcp
make build
```

## Quick Start

```bash
export STABILITY_API_KEY=sk-your-key-here
stability-mcp  # stdio mode (default, for Claude Desktop)
```

## Configuration

| Variable | Config File | Default | Description |
|----------|-------------|---------|-------------|
| `STABILITY_API_KEY` | `api_key` | — | Stability AI API key (required) |
| `OUTPUT_DIR` | `output_dir` | `./stability-images` | Output directory for generated images |
| `CONCURRENCY` | `concurrency` | `10` | Max concurrent batch generations |
| `RATE_LIMIT` | `rate_limit` | `150` | Requests per rate period |
| `RATE_PERIOD` | `rate_period` | `10` | Rate period in seconds |
| `MAX_RETRIES` | `max_retries` | `3` | Max retries on transient failures |
| `LOG_LEVEL` | `log_level` | `info` | Log level: debug, info, warn, error |

Config file: `~/.stability-mcp.yaml`, `~/.stability-mcp.json`, or `./stability-mcp.yaml`

```yaml
api_key: sk-your-key-here
output_dir: ./images
concurrency: 4
max_retries: 5
log_level: debug
```

Environment variables override config file values.

## Tools

All tools available via MCP. Full parameter details in the code.

### Generation

| Tool | Description |
|------|-------------|
| `generate_image` | Generate a single image from prompt (optionally image-to-image) |
| `generate_images` | Generate multiple images in batch with progress |

### Editing

| Tool | Description |
|------|-------------|
| `control_sketch` | Sketch-to-image with control strength |
| `control_struct` | Structure-to-image |
| `control_style` | Style transfer |
| `erase` | Erase objects using mask |
| `inpaint` | Inpainting with mask and prompt |
| `outpaint` | Outpainting (extend canvas) |
| `search_and_replace` | Search and replace objects |

### Upscaling

| Tool | Description |
|------|-------------|
| `upscale_fast` | Fast 2x/4x upscaling |
| `upscale_creative` | Creative upscaling with prompt |

## Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
    "mcpServers": {
        "stability": {
            "command": "stability-mcp",
            "env": {
                "STABILITY_API_KEY": "sk-your-key-here"
            }
        }
    }
}
```

For SSE mode:

```json
{
    "mcpServers": {
        "stability": {
            "url": "http://localhost:8080/sse",
            "env": {}
        }
    }
}
```

## Development

```bash
make build    # Cross-compile for Linux (amd64/arm64) and Darwin (amd64)
make lint     # go vet
make test     # go test
make clean    # Remove build directory
```

## License

GPL-3.0
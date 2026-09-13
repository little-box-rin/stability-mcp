package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/little-box-rin/stability-mcp/internal/batch"
	"github.com/little-box-rin/stability-mcp/internal/client"
	"github.com/little-box-rin/stability-mcp/internal/config"
	"github.com/little-box-rin/stability-mcp/internal/output"
)

// RegisterAll registers all MCP tools on the server.
func RegisterAll(s *server.MCPServer, cfg *config.Config, cli *client.Client) error {
	ow := output.NewWriter(cfg.OutputDir)

	// generate_image — single text-to-image generation
	generateImageTool := mcp.NewTool("generate_image",
		mcp.WithDescription("Generate a single image from a text prompt using Stability AI"),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("Text prompt describing the image"),
		),
		mcp.WithString("negative_prompt",
			mcp.Description("What to avoid in the generated image"),
		),
		mcp.WithString("aspect_ratio",
			mcp.Description("Aspect ratio (e.g. 1:1, 16:9, 4:3)"),
		),
		mcp.WithNumber("cfg_scale",
			mcp.Description("Prompt adherence strength (0-35, default 7)"),
		),
		mcp.WithInteger("steps",
			mcp.Description("Number of inference steps (1-50, default 30)"),
		),
		mcp.WithInteger("seed",
			mcp.Description("Seed for reproducibility (0 = random)"),
		),
		mcp.WithString("style_preset",
			mcp.Description("Style preset (line-art, anime, cinematic, etc.)"),
		),
		mcp.WithString("output_format",
			mcp.Description("Output format: png or webp"),
		),
		mcp.WithString("model",
			mcp.Description("Model: core, ultra, sd35 (default core)"),
		),
	)

	s.AddTool(generateImageTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params := parseGenerateParams(request)
		params.Prompt = mcp.ParseString(request, "prompt", "")

		data, genResult, err := cli.Generate(params)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Generation failed: %v", err)), nil
		}

		seed := genResult.Seed
		if seed == 0 {
			seed = params.Seed
		}

		filename := output.SingleImageFilename(seed)
		filePath, err := ow.WriteSingleImage(filename, data)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to save image: %v", err)), nil
		}

		result := map[string]interface{}{
			"file_path":     filePath,
			"seed":          seed,
			"finish_reason": genResult.FinishReason,
			"model":         params.Model,
		}
		if params.Model == "" {
			result["model"] = "core"
		}

		return mcp.NewToolResultJSON(result)
	})

	// generate_images — batch text-to-image generation
	generateImagesTool := mcp.NewTool("generate_images",
		mcp.WithDescription("Generate multiple images in parallel from an array of prompts using a worker pool"),
		mcp.WithArray("prompts",
			mcp.Required(),
			mcp.Description("Array of text prompts"),
			mcp.Items(mcp.WithStringItems()),
		),
		mcp.WithString("negative_prompt",
			mcp.Description("What to avoid in all generated images"),
		),
		mcp.WithString("aspect_ratio",
			mcp.Description("Aspect ratio (e.g. 1:1, 16:9, 4:3)"),
		),
		mcp.WithNumber("cfg_scale",
			mcp.Description("Prompt adherence strength (0-35, default 7)"),
		),
		mcp.WithInteger("steps",
			mcp.Description("Number of inference steps (1-50, default 30)"),
		),
		mcp.WithInteger("seed",
			mcp.Description("Base seed; incremented per prompt (0 = random per image)"),
		),
		mcp.WithString("style_preset",
			mcp.Description("Style preset (line-art, anime, cinematic, etc.)"),
		),
		mcp.WithString("output_format",
			mcp.Description("Output format: png or webp"),
		),
		mcp.WithString("model",
			mcp.Description("Model: core, ultra, sd35 (default core)"),
		),
		mcp.WithInteger("concurrency",
			mcp.Description("Max concurrent requests (default 10, max 30)"),
		),
	)

	s.AddTool(generateImagesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params := parseGenerateParams(request)

		// Parse prompts array
		args := request.GetArguments()
		promptsRaw, ok := args["prompts"]
		if !ok {
			return mcp.NewToolResultError("prompts is required"), nil
		}
		promptsSlice, ok := promptsRaw.([]interface{})
		if !ok || len(promptsSlice) == 0 {
			return mcp.NewToolResultError("prompts must be a non-empty array of strings"), nil
		}
		prompts := make([]string, len(promptsSlice))
		for i, p := range promptsSlice {
			pStr, ok := p.(string)
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("prompt at index %d is not a string", i)), nil
			}
			prompts[i] = pStr
		}

		// Determine concurrency
		concurrency := mcp.ParseInt(request, "concurrency", cfg.Concurrency)
		if concurrency < 1 {
			concurrency = 1
		}
		if concurrency > 30 {
			concurrency = 30
		}

		// Create batch engine for this call
		eng := batch.NewEngine(concurrency, cli, cfg.RateLimit)
		defer eng.Stop()

		batchID := ow.NextBatchID()
		startedAt := time.Now()

		results, duration, err := eng.GenerateBatch(ctx, prompts, params)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Batch generation failed: %v", err)), nil
		}

		// Create batch directory and write images
		batchDir, err := ow.CreateBatchDir(batchID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create batch directory: %v", err)), nil
		}

		var imageRecords []output.ImageRecord
		var succeeded, failed int
		type imageInfo struct {
			Index        int    `json:"index"`
			Prompt       string `json:"prompt"`
			File         string `json:"file"`
			FileFullPath string `json:"file_path"`
			Seed         int    `json:"seed"`
			FinishReason string `json:"finish_reason"`
		}
		var successInfos []imageInfo
		type errorInfo struct {
			Index  int    `json:"index"`
			Prompt string `json:"prompt"`
			Error  string `json:"error"`
		}
		var errorInfos []errorInfo

		for _, r := range results {
			if r.Err != nil {
				failed++
				errorInfos = append(errorInfos, errorInfo{
					Index:  r.Index,
					Prompt: prompts[r.Index],
					Error:  r.Err.Error(),
				})
				continue
			}
			succeeded++

			filename := output.ImageFilename(r.Index, r.Seed)
			fullPath := batchDir + "/" + filename

			if err := ow.WriteImage(batchDir, filename, r.Data); err != nil {
				failed++
				errorInfos = append(errorInfos, errorInfo{
					Index:  r.Index,
					Prompt: prompts[r.Index],
					Error:  fmt.Sprintf("write error: %v", err),
				})
				continue
			}

			imageRecords = append(imageRecords, output.ImageRecord{
				Index:        r.Index,
				Prompt:       prompts[r.Index],
				Seed:         r.Seed,
				File:         filename,
				FinishReason: r.FinishReason,
			})
			successInfos = append(successInfos, imageInfo{
				Index:        r.Index,
				Prompt:       prompts[r.Index],
				File:         filename,
				FileFullPath: fullPath,
				Seed:         r.Seed,
				FinishReason: r.FinishReason,
			})
		}

		// Write batch metadata
		meta := &output.BatchMetadata{
			BatchID:    batchID,
			Tool:       "generate_images",
			Params:     request.GetArguments(),
			StartedAt:  startedAt,
			DurationMs: duration.Milliseconds(),
			Results:    imageRecords,
		}
		if err := ow.WriteBatchMetadata(batchDir, meta); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to write metadata: %v", err)), nil
		}

		response := map[string]interface{}{
			"batch_id":    batchID,
			"total":       len(prompts),
			"succeeded":   succeeded,
			"failed":      failed,
			"duration_ms": duration.Milliseconds(),
			"images":      successInfos,
			"errors":      errorInfos,
		}

		return mcp.NewToolResultJSON(response)
	})

	return nil
}

// parseGenerateParams extracts shared generation parameters from a tool request.
func parseGenerateParams(request mcp.CallToolRequest) client.GenerateParams {
	return client.GenerateParams{
		NegativePrompt: mcp.ParseString(request, "negative_prompt", ""),
		AspectRatio:    mcp.ParseString(request, "aspect_ratio", "1:1"),
		CfgScale:       mcp.ParseFloat64(request, "cfg_scale", 7.0),
		Steps:          mcp.ParseInt(request, "steps", 30),
		Seed:           mcp.ParseInt(request, "seed", 0),
		StylePreset:    mcp.ParseString(request, "style_preset", ""),
		OutputFormat:   mcp.ParseString(request, "output_format", "png"),
		Model:          mcp.ParseString(request, "model", "core"),
	}
}

// Ensure json and encoding/json are used.
var _ = json.Marshal
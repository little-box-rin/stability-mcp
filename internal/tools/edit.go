package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/little-box-rin/stability-mcp/internal/client"
	"github.com/little-box-rin/stability-mcp/internal/config"
	"github.com/little-box-rin/stability-mcp/internal/output"
)

// RegisterEditTools registers all image editing tools on the server.
func RegisterEditTools(s *server.MCPServer, cfg *config.Config, cli *client.Client, ow *output.Writer) {
	registerControlSketch(s, cli, ow)
	registerControlStruct(s, cli, ow)
	registerControlStyle(s, cli, ow)
	registerErase(s, cli, ow)
	registerInpaint(s, cli, ow)
	registerOutpaint(s, cli, ow)
	registerSearchAndReplace(s, cli, ow)
}

// commonEditParams defines parameters shared across most edit tools.
type commonEditParams struct {
	Prompt       string
	AspectRatio  string
	CfgScale     float64
	Steps        int
	Seed         int
	StylePreset  string
	OutputFormat string
	Model        string
}

func parseCommonEditParams(request mcp.CallToolRequest) commonEditParams {
	return commonEditParams{
		Prompt:       mcp.ParseString(request, "prompt", ""),
		AspectRatio:  mcp.ParseString(request, "aspect_ratio", ""),
		CfgScale:     mcp.ParseFloat64(request, "cfg_scale", 0),
		Steps:        mcp.ParseInt(request, "steps", 0),
		Seed:         mcp.ParseInt(request, "seed", 0),
		StylePreset:  mcp.ParseString(request, "style_preset", ""),
		OutputFormat: mcp.ParseString(request, "output_format", "png"),
		Model:        mcp.ParseString(request, "model", ""),
	}
}

func (p commonEditParams) toTextFields() map[string]string {
	fields := make(map[string]string)
	if p.Prompt != "" {
		fields["prompt"] = p.Prompt
	}
	if p.AspectRatio != "" {
		fields["aspect_ratio"] = p.AspectRatio
	}
	if p.CfgScale > 0 {
		fields["cfg_scale"] = fmt.Sprintf("%.2f", p.CfgScale)
	}
	if p.Steps > 0 {
		fields["steps"] = fmt.Sprintf("%d", p.Steps)
	}
	if p.Seed > 0 {
		fields["seed"] = fmt.Sprintf("%d", p.Seed)
	}
	if p.StylePreset != "" {
		fields["style_preset"] = p.StylePreset
	}
	if p.OutputFormat != "" {
		fields["output_format"] = p.OutputFormat
	}
	if p.Model != "" {
		fields["model"] = p.Model
	}
	return fields
}

// runEditTool is the common handler logic for all edit tools.
func runEditTool(ctx context.Context, cli *client.Client, ow *output.Writer, model string, params client.GenerateEditParams) (*mcp.CallToolResult, error) {
	imgData, genResult, err := cli.GenerateEdit(params)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Edit failed: %v", err)), nil
	}

	seed := 0
	finishReason := ""
	if genResult != nil {
		seed = genResult.Seed
		finishReason = genResult.FinishReason
	}

	filename := output.SingleImageFilename(seed)
	filePath, err := ow.WriteSingleImage(filename, imgData)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to save image: %v", err)), nil
	}

	result := map[string]interface{}{
		"file_path":     filePath,
		"seed":          seed,
		"finish_reason": finishReason,
		"model":         model,
	}

	return mcp.NewToolResultJSON(result)
}

func registerControlSketch(s *server.MCPServer, cli *client.Client, ow *output.Writer) {
	tool := mcp.NewTool("control_sketch",
		mcp.WithDescription("Generate an image guided by a sketch/edge map using Stability AI Control Sketch"),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("Text prompt describing the final image"),
		),
		mcp.WithNumber("control_strength",
			mcp.Description("How strongly the control image influences the output (0-1, default 0.7)"),
		),
		mcp.WithString("image",
			mcp.Required(),
			mcp.Description("Image file path or data URI (data:image/png;base64,...) for the sketch/control image"),
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
			mcp.Description("Model version"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		common := parseCommonEditParams(request)
		imageStr := mcp.ParseString(request, "image", "")
		controlStrength := mcp.ParseFloat64(request, "control_strength", 0.7)

		imgBytes, mimeType, err := client.LoadImage(imageStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Loading image: %v", err)), nil
		}

		textFields := common.toTextFields()
		textFields["control_strength"] = fmt.Sprintf("%.2f", controlStrength)

		params := client.GenerateEditParams{
			Model:  "control/sketch",
			Prompt: common.Prompt,
			Files: []client.EditFile{
				{FieldName: "image", Filename: "sketch." + extFromMIME(mimeType), Data: imgBytes},
			},
			TextFields: textFields,
		}

		return runEditTool(ctx, cli, ow, "control/sketch", params)
	})
}

func registerControlStruct(s *server.MCPServer, cli *client.Client, ow *output.Writer) {
	tool := mcp.NewTool("control_struct",
		mcp.WithDescription("Generate an image guided by a structure (depth map, normal map, etc.) using Stability AI Control Structure"),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("Text prompt describing the final image"),
		),
		mcp.WithString("image",
			mcp.Required(),
			mcp.Description("Image file path or data URI for the structure control image"),
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
			mcp.Description("Model version"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		common := parseCommonEditParams(request)
		imageStr := mcp.ParseString(request, "image", "")

		imgBytes, mimeType, err := client.LoadImage(imageStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Loading image: %v", err)), nil
		}

		textFields := common.toTextFields()

		params := client.GenerateEditParams{
			Model:  "control/struct",
			Prompt: common.Prompt,
			Files: []client.EditFile{
				{FieldName: "image", Filename: "control." + extFromMIME(mimeType), Data: imgBytes},
			},
			TextFields: textFields,
		}

		return runEditTool(ctx, cli, ow, "control/struct", params)
	})
}

func registerControlStyle(s *server.MCPServer, cli *client.Client, ow *output.Writer) {
	tool := mcp.NewTool("control_style",
		mcp.WithDescription("Transfer the style of a reference image to a new generation using Stability AI Control Style"),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("Text prompt describing the content"),
		),
		mcp.WithString("image",
			mcp.Required(),
			mcp.Description("Style reference image file path or data URI"),
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
			mcp.Description("Model version"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		common := parseCommonEditParams(request)
		imageStr := mcp.ParseString(request, "image", "")

		imgBytes, mimeType, err := client.LoadImage(imageStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Loading style image: %v", err)), nil
		}

		textFields := common.toTextFields()

		params := client.GenerateEditParams{
			Model:  "control/style",
			Prompt: common.Prompt,
			Files: []client.EditFile{
				{FieldName: "image", Filename: "style." + extFromMIME(mimeType), Data: imgBytes},
			},
			TextFields: textFields,
		}

		return runEditTool(ctx, cli, ow, "control/style", params)
	})
}

func registerErase(s *server.MCPServer, cli *client.Client, ow *output.Writer) {
	tool := mcp.NewTool("erase",
		mcp.WithDescription("Erase objects from an image using a mask"),
		mcp.WithString("image",
			mcp.Required(),
			mcp.Description("Source image file path or data URI"),
		),
		mcp.WithString("mask",
			mcp.Required(),
			mcp.Description("Grayscale mask image file path or data URI indicating areas to erase"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		imageStr := mcp.ParseString(request, "image", "")
		maskStr := mcp.ParseString(request, "mask", "")

		imgBytes, imgMime, err := client.LoadImage(imageStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Loading image: %v", err)), nil
		}
		maskBytes, maskMime, err := client.LoadImage(maskStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Loading mask: %v", err)), nil
		}

		params := client.GenerateEditParams{
			Model:  "edit/erase",
			Prompt: "",
			Files: []client.EditFile{
				{FieldName: "image", Filename: "source." + extFromMIME(imgMime), Data: imgBytes},
				{FieldName: "mask", Filename: "mask." + extFromMIME(maskMime), Data: maskBytes},
			},
			TextFields: map[string]string{"output_format": "png"},
		}

		return runEditTool(ctx, cli, ow, "edit/erase", params)
	})
}

func registerInpaint(s *server.MCPServer, cli *client.Client, ow *output.Writer) {
	tool := mcp.NewTool("inpaint",
		mcp.WithDescription("Fill in masked areas of an image using Stability AI Inpainting"),
		mcp.WithString("image",
			mcp.Required(),
			mcp.Description("Source image file path or data URI"),
		),
		mcp.WithString("mask",
			mcp.Required(),
			mcp.Description("Grayscale mask image file path or data URI indicating areas to inpaint"),
		),
		mcp.WithString("prompt",
			mcp.Description("Text prompt describing what to generate in the masked area"),
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
			mcp.Description("Model version"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		common := parseCommonEditParams(request)
		imageStr := mcp.ParseString(request, "image", "")
		maskStr := mcp.ParseString(request, "mask", "")

		imgBytes, imgMime, err := client.LoadImage(imageStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Loading image: %v", err)), nil
		}
		maskBytes, maskMime, err := client.LoadImage(maskStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Loading mask: %v", err)), nil
		}

		textFields := common.toTextFields()
		textFields["output_format"] = common.OutputFormat
		if textFields["output_format"] == "" {
			textFields["output_format"] = "png"
		}

		params := client.GenerateEditParams{
			Model:  "edit/inpaint",
			Prompt: common.Prompt,
			Files: []client.EditFile{
				{FieldName: "image", Filename: "source." + extFromMIME(imgMime), Data: imgBytes},
				{FieldName: "mask", Filename: "mask." + extFromMIME(maskMime), Data: maskBytes},
			},
			TextFields: textFields,
		}

		return runEditTool(ctx, cli, ow, "edit/inpaint", params)
	})
}

func registerOutpaint(s *server.MCPServer, cli *client.Client, ow *output.Writer) {
	tool := mcp.NewTool("outpaint",
		mcp.WithDescription("Extend an image beyond its original boundaries using Stability AI Outpainting"),
		mcp.WithString("image",
			mcp.Required(),
			mcp.Description("Source image file path or data URI"),
		),
		mcp.WithString("mask",
			mcp.Required(),
			mcp.Description("Mask image file path or data URI indicating the area to keep"),
		),
		mcp.WithString("prompt",
			mcp.Description("Text prompt describing the extended areas"),
		),
		mcp.WithInteger("seed",
			mcp.Description("Seed for reproducibility (0 = random)"),
		),
		mcp.WithInteger("left",
			mcp.Description("Pixels to extend left"),
		),
		mcp.WithInteger("right",
			mcp.Description("Pixels to extend right"),
		),
		mcp.WithInteger("up",
			mcp.Description("Pixels to extend upward"),
		),
		mcp.WithInteger("down",
			mcp.Description("Pixels to extend downward"),
		),
		mcp.WithString("output_format",
			mcp.Description("Output format: png or webp"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		common := parseCommonEditParams(request)
		imageStr := mcp.ParseString(request, "image", "")
		maskStr := mcp.ParseString(request, "mask", "")

		imgBytes, imgMime, err := client.LoadImage(imageStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Loading image: %v", err)), nil
		}
		maskBytes, maskMime, err := client.LoadImage(maskStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Loading mask: %v", err)), nil
		}

		textFields := common.toTextFields()
		textFields["output_format"] = common.OutputFormat
		if textFields["output_format"] == "" {
			textFields["output_format"] = "png"
		}

		if left := mcp.ParseInt(request, "left", 0); left > 0 {
			textFields["left"] = fmt.Sprintf("%d", left)
		}
		if right := mcp.ParseInt(request, "right", 0); right > 0 {
			textFields["right"] = fmt.Sprintf("%d", right)
		}
		if up := mcp.ParseInt(request, "up", 0); up > 0 {
			textFields["up"] = fmt.Sprintf("%d", up)
		}
		if down := mcp.ParseInt(request, "down", 0); down > 0 {
			textFields["down"] = fmt.Sprintf("%d", down)
		}

		params := client.GenerateEditParams{
			Model:  "edit/outpaint",
			Prompt: common.Prompt,
			Files: []client.EditFile{
				{FieldName: "image", Filename: "source." + extFromMIME(imgMime), Data: imgBytes},
				{FieldName: "mask", Filename: "mask." + extFromMIME(maskMime), Data: maskBytes},
			},
			TextFields: textFields,
		}

		return runEditTool(ctx, cli, ow, "edit/outpaint", params)
	})
}

func registerSearchAndReplace(s *server.MCPServer, cli *client.Client, ow *output.Writer) {
	tool := mcp.NewTool("search_and_replace",
		mcp.WithDescription("Replace an element in an image with something else using Stability AI Search and Replace"),
		mcp.WithString("image",
			mcp.Required(),
			mcp.Description("Source image file path or data URI"),
		),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("Text prompt describing the new content to generate"),
		),
		mcp.WithString("search_prompt",
			mcp.Required(),
			mcp.Description("Text prompt describing the element to replace"),
		),
		mcp.WithInteger("seed",
			mcp.Description("Seed for reproducibility (0 = random)"),
		),
		mcp.WithString("output_format",
			mcp.Description("Output format: png or webp"),
		),
		mcp.WithString("model",
			mcp.Description("Model version"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		common := parseCommonEditParams(request)
		imageStr := mcp.ParseString(request, "image", "")
		searchPrompt := mcp.ParseString(request, "search_prompt", "")

		imgBytes, imgMime, err := client.LoadImage(imageStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Loading image: %v", err)), nil
		}

		textFields := common.toTextFields()
		textFields["search_prompt"] = searchPrompt
		textFields["output_format"] = common.OutputFormat
		if textFields["output_format"] == "" {
			textFields["output_format"] = "png"
		}

		params := client.GenerateEditParams{
			Model:  "edit/search-and-replace",
			Prompt: common.Prompt,
			Files: []client.EditFile{
				{FieldName: "image", Filename: "source." + extFromMIME(imgMime), Data: imgBytes},
			},
			TextFields: textFields,
		}

		return runEditTool(ctx, cli, ow, "edit/search-and-replace", params)
	})
}

// extFromMIME returns a file extension for a given MIME type.
func extFromMIME(mimeType string) string {
	switch mimeType {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/webp":
		return "webp"
	case "image/bmp":
		return "bmp"
	case "image/gif":
		return "gif"
	default:
		return "png"
	}
}
package client

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"
)

// GenerateParams holds all parameters for image generation.
type GenerateParams struct {
	Prompt         string
	NegativePrompt string
	AspectRatio    string  // default "1:1"
	CfgScale       float64 // default 7
	Steps          int     // default 30
	Seed           int     // 0 = random
	StylePreset    string
	OutputFormat   string // png / webp
	Model          string // core / ultra / sd35
}

// GenerateResult contains metadata returned from a generation request.
type GenerateResult struct {
	Seed         int
	FinishReason string
}

// Client is the HTTP client for the Stability AI REST API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewClient creates a new Stability AI API client.
func NewClient(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		baseURL: "https://api.stability.ai",
		apiKey:  apiKey,
	}
}

// Generate sends a single image generation request and returns the image bytes
// plus metadata parsed from response headers.
func (c *Client) Generate(params GenerateParams) ([]byte, *GenerateResult, error) {
	model := params.Model
	if model == "" {
		model = "core"
	}

	url := fmt.Sprintf("%s/v2beta/stable-image/generate/%s", c.baseURL, model)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// Required field
	if err := w.WriteField("prompt", params.Prompt); err != nil {
		return nil, nil, fmt.Errorf("writing prompt field: %w", err)
	}

	// Optional fields
	if params.NegativePrompt != "" {
		_ = w.WriteField("negative_prompt", params.NegativePrompt)
	}

	aspectRatio := params.AspectRatio
	if aspectRatio == "" {
		aspectRatio = "1:1"
	}
	_ = w.WriteField("aspect_ratio", aspectRatio)

	cfgScale := params.CfgScale
	if cfgScale == 0 {
		cfgScale = 7
	}
	_ = w.WriteField("cfg_scale", strconv.FormatFloat(cfgScale, 'f', -1, 64))

	steps := params.Steps
	if steps == 0 {
		steps = 30
	}
	_ = w.WriteField("steps", strconv.Itoa(steps))

	if params.Seed > 0 {
		_ = w.WriteField("seed", strconv.Itoa(params.Seed))
	}

	if params.StylePreset != "" {
		_ = w.WriteField("style_preset", params.StylePreset)
	}

	outputFormat := params.OutputFormat
	if outputFormat == "" {
		outputFormat = "png"
	}
	_ = w.WriteField("output_format", outputFormat)

	if err := w.Close(); err != nil {
		return nil, nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return nil, nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "image/*")
	req.Header.Set("User-Agent", "stability-mcp/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	result := &GenerateResult{}

	if seedStr := resp.Header.Get("seed"); seedStr != "" {
		if seed, err := strconv.Atoi(seedStr); err == nil {
			result.Seed = seed
		}
	}
	if finish := resp.Header.Get("finish-reason"); finish != "" {
		result.FinishReason = finish
	}

	return body, result, nil
}
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

// GenerateEditParams holds parameters that include file uploads (images, masks).
type GenerateEditParams struct {
	Model      string    // endpoint name: control/sketch, control/struct, inpaint, etc.
	Prompt     string    // optional for some endpoints
	Files      []EditFile // image files to upload
	TextFields map[string]string // additional text form fields
}

// EditFile represents a file to upload in a multipart edit request.
type EditFile struct {
	FieldName string // multipart field name (image, mask, style, etc.)
	Filename  string // displayed filename
	Data      []byte
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
	// RetryConfig controls retry behavior for transient failures.
	// Defaults to DefaultRetryConfig (3 retries, 500ms base, 30s cap).
	RetryConfig RetryConfig
}

// NewClient creates a new Stability AI API client.
func NewClient(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL:     "https://api.stability.ai",
		apiKey:      apiKey,
		RetryConfig: DefaultRetryConfig(),
	}
}

// doRequest performs an HTTP request with retry and exponential backoff.
// Retries on connection errors, HTTP 429, and HTTP 5xx responses.
// Supports body refresh via req.GetBody for multipart retries.
func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	var lastErr error
	attempts := c.RetryConfig.MaxRetries + 1
	if attempts < 1 {
		attempts = 1
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		resp, err := c.httpClient.Do(req)

		// Success or non-retryable response: return immediately.
		if err == nil && !shouldRetry(err, resp) {
			return resp, nil
		}

		// Drain and close the body so the connection can be reused.
		if resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 256))
			_ = resp.Body.Close()
		}

		if err != nil {
			lastErr = err
		}

		if attempt <= c.RetryConfig.MaxRetries {
			// Refresh the request body for the next attempt.
			if req.GetBody != nil {
				if newBody, err := req.GetBody(); err == nil {
					req.Body = newBody
				} else if lastErr == nil {
					lastErr = err
				}
			}
			time.Sleep(backoff(attempt, c.RetryConfig))
			continue
		}

		// All retries exhausted.
		if err != nil {
			return nil, lastErr
		}
		return resp, nil
	}
	return nil, lastErr
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
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	}

	resp, err := c.doRequest(req)
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

// GenerateEdit sends an image editing request with file uploads via multipart POST.
// It creates a multipart POST to {baseURL}/v2beta/stable-image/{model} with file fields
// and text fields. Parses the response the same way as Generate().
func (c *Client) GenerateEdit(params GenerateEditParams) ([]byte, *GenerateResult, error) {
	model := params.Model
	if model == "" {
		return nil, nil, fmt.Errorf("model is required for edit requests")
	}

	url := fmt.Sprintf("%s/v2beta/stable-image/%s", c.baseURL, model)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// Write file fields
	for _, f := range params.Files {
		part, err := w.CreateFormFile(f.FieldName, f.Filename)
		if err != nil {
			return nil, nil, fmt.Errorf("creating form file %s: %w", f.FieldName, err)
		}
		if _, err := part.Write(f.Data); err != nil {
			return nil, nil, fmt.Errorf("writing file %s: %w", f.FieldName, err)
		}
	}

	// Write text fields
	if params.Prompt != "" {
		if err := w.WriteField("prompt", params.Prompt); err != nil {
			return nil, nil, fmt.Errorf("writing prompt field: %w", err)
		}
	}
	for key, val := range params.TextFields {
		if key == "prompt" {
			continue // already handled above
		}
		_ = w.WriteField(key, val)
	}

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
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	}

	resp, err := c.doRequest(req)
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
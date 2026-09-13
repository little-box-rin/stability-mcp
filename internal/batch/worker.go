package batch

import "github.com/little-box-rin/stability-mcp/internal/client"

// Job represents a single image generation task in a batch.
type Job struct {
	Index  int
	Prompt string
	Seed   int
	Params client.GenerateParams
}

// Result holds the output of a single image generation job.
type Result struct {
	Index        int
	Data         []byte
	Seed         int
	FinishReason string
	Err          error
}
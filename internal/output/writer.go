package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ImageRecord stores metadata for a single generated image.
type ImageRecord struct {
	Index        int    `json:"index"`
	Prompt       string `json:"prompt"`
	Seed         int    `json:"seed"`
	File         string `json:"file"`
	FinishReason string `json:"finish_reason"`
}

// BatchMetadata stores the full metadata for a batch generation run.
type BatchMetadata struct {
	BatchID    string        `json:"batch_id"`
	Tool       string        `json:"tool"`
	Params     interface{}   `json:"params"`
	StartedAt  time.Time     `json:"started_at"`
	DurationMs int64         `json:"duration_ms"`
	Results    []ImageRecord `json:"results"`
}

// Writer handles writing generated images and metadata to disk.
type Writer struct {
	BaseDir      string
	mu           sync.Mutex
	batchCounter map[string]int // date -> counter
}

// NewWriter creates a new output writer with the given base directory.
func NewWriter(baseDir string) *Writer {
	return &Writer{
		BaseDir:      baseDir,
		batchCounter: make(map[string]int),
	}
}

// NextBatchID returns the next batch ID based on the current date and an
// auto-incrementing counter per day (e.g. "2006-01-02_001").
func (w *Writer) NextBatchID() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	date := time.Now().Format("2006-01-02")
	w.batchCounter[date]++
	return fmt.Sprintf("%s_%03d", date, w.batchCounter[date])
}

// CreateBatchDir creates the directory for a batch run and returns its path.
func (w *Writer) CreateBatchDir(batchID string) (string, error) {
	dir := filepath.Join(w.BaseDir, "batches", batchID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating batch directory %s: %w", dir, err)
	}
	return dir, nil
}

// WriteImage writes image data to a file in the batch directory.
func (w *Writer) WriteImage(batchDir, filename string, data []byte) error {
	path := filepath.Join(batchDir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing image %s: %w", path, err)
	}
	return nil
}

// WriteBatchMetadata writes batch metadata as batch.json in the batch directory.
func (w *Writer) WriteBatchMetadata(batchDir string, meta *BatchMetadata) error {
	path := filepath.Join(batchDir, "batch.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling batch metadata: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing batch metadata %s: %w", path, err)
	}
	return nil
}

// WriteSingleImage writes a single (non-batch) image to the single/ subdirectory
// and returns the full path. Creates the single directory if needed.
func (w *Writer) WriteSingleImage(filename string, data []byte) (string, error) {
	dir := filepath.Join(w.BaseDir, "single")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating single directory %s: %w", dir, err)
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("writing single image %s: %w", path, err)
	}
	return path, nil
}
package client

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ImageSource represents where image data comes from.
type ImageSource string

const (
	ImageSourceFile    ImageSource = "file"
	ImageSourceDataURI ImageSource = "data_uri"
)

// mimeFromExt maps common file extensions to MIME types.
var mimeFromExt = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".gif":  "image/gif",
}

// LoadImage reads image bytes from either a file path or a data URI.
// Supports data: URIs (data:image/png;base64,....) and absolute/relative file paths.
// Returns (image_bytes, mime_type, error).
func LoadImage(pathOrURI string) ([]byte, string, error) {
	if strings.HasPrefix(pathOrURI, "data:") {
		return loadDataURI(pathOrURI)
	}
	return loadFile(pathOrURI)
}

// loadDataURI parses a data: URI and returns the decoded bytes and MIME type.
func loadDataURI(uri string) ([]byte, string, error) {
	// Format: data:[<mediatype>][;base64],<data>
	if !strings.HasPrefix(uri, "data:") {
		return nil, "", fmt.Errorf("invalid data URI: missing data: prefix")
	}

	rest := uri[5:] // strip "data:"

	// Find the comma separating metadata from data
	commaIdx := strings.Index(rest, ",")
	if commaIdx < 0 {
		return nil, "", fmt.Errorf("invalid data URI: missing comma")
	}

	meta := rest[:commaIdx]
	encoded := rest[commaIdx+1:]

	// Parse MIME type
	mimeType := "image/png" // default
	if meta != "" {
		// meta is like "image/png;base64" or "image/png"
		parts := strings.SplitN(meta, ";", 2)
		if parts[0] != "" {
			mimeType = parts[0]
		}
	}

	// Decode base64
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// Try with padding just in case
		data, err = base64.StdEncoding.WithPadding(base64.NoPadding).DecodeString(encoded)
		if err != nil {
			return nil, "", fmt.Errorf("decoding base64 data URI: %w", err)
		}
	}

	return data, mimeType, nil
}

// loadFile reads an image file from disk and detects the MIME type from its extension.
func loadFile(path string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("reading image file %s: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	mimeType := mimeFromExt[ext]
	if mimeType == "" {
		mimeType = "image/png" // default fallback
	}

	return data, mimeType, nil
}
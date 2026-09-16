package output

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ImageHash returns a short hex hash of the full set of generation parameters.
// Same inputs always produce the same hash, enabling deterministic filenames
// that act as content-addressed cache keys for crash recovery.
func ImageHash(prompt, negPrompt, style string, cfg float64, steps int, ratio, model, format string, seed int) string {
	canonical := fmt.Sprintf("%s\x00%s\x00%s\x00%g\x00%d\x00%s\x00%s\x00%s\x00%d",
		prompt, negPrompt, style, cfg, steps, ratio, model, format, seed)
	h := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", h[:6])
}

// EditImageHash returns a short hex hash for edit/image-to-image parameters.
// Sorts map keys for deterministic hashing.
func EditImageHash(model, prompt string, textFields map[string]string) string {
	keys := make([]string, 0, len(textFields))
	for k := range textFields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString(model)
	sb.WriteString("\x00")
	sb.WriteString(prompt)
	sb.WriteString("\x00")
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(textFields[k])
		sb.WriteString("\x00")
	}
	h := sha256.Sum256([]byte(sb.String()))
	return fmt.Sprintf("%x", h[:6])
}

// SingleImageFilename returns a unique filename for a single image.
// Uses a hash of the full generation parameters as the base name, so the same
// inputs always produce the same filename (enabling crash recovery). If a file
// with that name already exists, appends _001, _002, etc. to avoid overwrites.
// singleDir must be the full path to the single/ output directory.
func SingleImageFilename(seed int, hash string, singleDir string) string {
	base := fmt.Sprintf("img_%s_seed%d", hash, seed)
	filename := base + ".png"

	for i := 0; ; i++ {
		if i > 0 {
			filename = fmt.Sprintf("%s_%03d.png", base, i)
		}
		path := filepath.Join(singleDir, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return filename
		}
	}
}

// ImageFilename returns a deterministic filename for a batch image.
func ImageFilename(index int, seed int) string {
	return fmt.Sprintf("img_%03d_seed%d.png", index, seed)
}
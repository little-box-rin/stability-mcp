package output

import "fmt"

// ImageFilename returns a deterministic filename for a batch image.
func ImageFilename(index int, seed int) string {
	return fmt.Sprintf("img_%03d_seed%d.png", index, seed)
}

// SingleImageFilename returns a deterministic filename for a single image.
// Uses the seed as the unique identifier.
func SingleImageFilename(seed int) string {
	return fmt.Sprintf("img_seed%d.png", seed)
}
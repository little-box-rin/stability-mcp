package tools

import "strings"

// StylePresets lists the style presets supported by Stability AI, shared
// across all generation and editing endpoints.
var StylePresets = []string{
	"3d-model", "analog-film", "anime", "cinematic", "comic-book",
	"digital-art", "enhance", "fantasy-art", "isometric", "line-art",
	"low-poly", "modeling-compound", "neon-punk", "origami", "photographic",
	"pixel-art", "tile-texture",
}

// stylePresetDescription returns a schema description that enumerates the
// shared StylePresets list, so tool schemas stay in sync with the constant.
func stylePresetDescription() string {
	return "Style preset. Valid values: " + strings.Join(StylePresets, ", ")
}
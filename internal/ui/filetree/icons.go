package filetree

import "strings"

var iconExts = map[string]map[string]struct{}{
	"audio-x-generic-symbolic": strset(
		"flac",
		"opus",
		"mp3",
		"ogg",
		"oga",
		"m4a",
		"wav",
	),
	"video-x-generic-symbolic": strset(
		"mp4",
		"flv",
		"mkv",
		"webm",
	),
	"image-x-generic-symbolic": strset(
		"jpg",
		"jpe",
		"jpg",
		"jpeg",
		"png",
		"gif",
		"tif",
		"tiff",
		"webp",
		"dng",
		"xcf",
		"psd",
	),
}

func iconExt(ext string) string {
	ext = strings.ToLower(ext)
	ext = strings.TrimPrefix(ext, ".")

	for icon, exts := range iconExts {
		_, ok := exts[ext]
		if ok {
			return icon
		}
	}

	return "text-x-generic-symbolic"
}

func strset(strs ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(strs))
	for _, str := range strs {
		m[str] = struct{}{}
	}
	return m
}

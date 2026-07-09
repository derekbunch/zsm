package icons

import (
	"path/filepath"
	"strings"
)

// Defaults returned when nothing matches.
var (
	DefaultFile = Style{Icon: "\uf15b", Color: "#6D8086"} //
	DefaultDir  = Style{Icon: "\uf07b", Color: "#7daea3"} //
)

// Lookup resolves an icon for a path given its name and whether it's a directory.
// Lookup order: exact filename → extension (longest first for ".blade.php" etc.) → default.
// Overrides (from user config) take precedence over the generated tables.
func Lookup(name string, isDir bool, overrideByName, overrideByExt map[string]Style) Style {
	if isDir {
		if s, ok := overrideByName[name]; ok {
			return s
		}
		if s, ok := ByName[name]; ok {
			return s
		}
		return DefaultDir
	}

	// Exact filename first (Dockerfile, README.md, .gitignore, etc.)
	if s, ok := overrideByName[name]; ok {
		return s
	}
	if s, ok := ByName[name]; ok {
		return s
	}

	// Try progressively shorter extensions: "foo.blade.php" → "blade.php" → "php"
	lower := strings.ToLower(name)
	for i := strings.Index(lower, "."); i != -1 && i < len(lower)-1; i = strings.Index(lower[i+1:], ".") + i + 1 {
		ext := lower[i+1:]
		if s, ok := overrideByExt[ext]; ok {
			return s
		}
		if s, ok := ByExt[ext]; ok {
			return s
		}
	}

	// Final fallback: plain extension
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	if ext != "" {
		if s, ok := overrideByExt[ext]; ok {
			return s
		}
		if s, ok := ByExt[ext]; ok {
			return s
		}
	}

	return DefaultFile
}

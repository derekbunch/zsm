package main

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Palette struct {
	Active   string `toml:"active"`
	Dir      string `toml:"dir"`
	Selected string `toml:"selected"`
	Dim      string `toml:"dim"`
	Bg       string `toml:"bg"`
	Text     string `toml:"text"`
	Accent   string `toml:"accent"`  // safe modals (rename, custom name)
	Warning  string `toml:"warning"` // semi-destructive modals (kill)
	Danger   string `toml:"danger"`  // fully destructive modals (delete)
	Exited   string `toml:"exited"`  // list color for exited/resurrectable sessions
}

type Theme struct {
	Dark  Palette `toml:"dark"`
	Light Palette `toml:"light"`
}

// PreviewConfig lets users swap the preview command entirely (eza → tree → lsd → custom script).
// Invocation: <Command> <Options...> [<IgnoreFlag> <joined ignores>] <dir>
type PreviewConfig struct {
	Command         string   `toml:"command"`          // e.g. "eza"
	Options         []string `toml:"options"`          // e.g. ["-T", "-L2", "--color=always", "--icons"]
	Ignore          []string `toml:"ignore"`           // names/globs to hide; passed via IgnoreFlag
	IgnoreFlag      string   `toml:"ignore_flag"`      // e.g. "-I" (omit arg if empty/none)
	IgnoreSeparator string   `toml:"ignore_separator"` // how to join patterns, e.g. "|" for eza
}

type Config struct {
	Layout          string        `toml:"layout"`
	Height          int           `toml:"height"`           // max visible items (0 = fill terminal)
	Width           int           `toml:"width"`            // max width in columns (0 = fill terminal)
	PreviewPosition string        `toml:"preview_position"` // "top", "bottom", "left", "right", or "none"
	Preview         PreviewConfig `toml:"preview"`
	PathAliases     [][2]string   `toml:"path_aliases"` // ordered [prefix, replacement] pairs
	ShowScore       bool          `toml:"show_score"`   // show zoxide score in list rows
	Theme           Theme         `toml:"theme"`
}

var defaultConfig = Config{
	Layout:          "zjstatus",
	Height:          15,
	Width:           0,
	PreviewPosition: "right",
	ShowScore:       true,
	Preview: PreviewConfig{
		Command:         "eza",
		Options:         []string{"-T", "-L2", "--color=always", "--icons=always"},
		IgnoreFlag:      "-I",
		IgnoreSeparator: "|",
		Ignore: []string{
			".git", "node_modules", "__pycache__", ".venv",
			".mypy_cache", ".pytest_cache", ".ruff_cache", ".DS_Store",
		},
	},
	Theme: Theme{
		Dark: Palette{
			Active:   "#a9b665",
			Dir:      "#7daea3",
			Selected: "#504945",
			Dim:      "#767676",
			Bg:       "#1d2021",
			Text:     "#ebdbb2",
			Accent:   "#89b482", // gruvbox aqua (safe: rename/custom)
			Warning:  "#d8a657", // gruvbox yellow (semi: kill)
			Danger:   "#ea6962", // gruvbox red (destructive: delete)
			Exited:   "#928374", // gruvbox grey
		},
		Light: Palette{
			Active:   "#6c782e",
			Dir:      "#45707a",
			Selected: "#ebdbb2",
			Dim:      "#928374",
			Bg:       "#fbf1c7",
			Text:     "#3c3836",
			Accent:   "#4c7a5d", // gruvbox aqua (light)
			Warning:  "#b57614", // gruvbox yellow (light)
			Danger:   "#c14a4a", // gruvbox red (light)
			Exited:   "#7c6f64", // gruvbox grey (light)
		},
	},
}

func loadConfig() Config {
	cfg := defaultConfig

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg
	}

	path := filepath.Join(home, ".config", "zsm", "config.toml")
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return defaultConfig
	}

	return cfg
}

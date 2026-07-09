package main

import (
	"log"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// debugEnabled is set by main when ZSM_DEBUG is non-empty. Gating log calls on
// this prevents TUI corruption when debug isn't on (default log writes to stderr).
var debugEnabled bool

// previewMsg carries the result of a preview computation back to Update.
type previewMsg struct {
	item    Item // which item this preview is for (discard if stale)
	content string
}

// fetchPreview returns a tea.Cmd that computes the preview for the given item.
// Running async avoids blocking the UI on slow dump-screen calls or large dirs.
func fetchPreview(item Item, cfg PreviewConfig) tea.Cmd {
	return func() tea.Msg {
		content := buildPreview(item, cfg)
		if debugEnabled {
			log.Printf("preview fetched: name=%q dir=%q active=%v len=%d", item.Name, item.Dir, item.Active, len(content))
		}
		return previewMsg{item: item, content: content}
	}
}

func buildPreview(item Item, cfg PreviewConfig) string {
	if item.Active || item.Exited {
		raw, err := dumpLayout(item.Name, item.Exited)
		if err != nil {
			return "(no layout available)"
		}
		layout, err := parseLayout(raw)
		if err != nil {
			return "(layout parse error)"
		}
		return renderLayoutTree(layout)
	}
	if item.Dir != "" {
		return runPreview(item.Dir, cfg)
	}
	return ""
}

// runPreview invokes the configured preview command with the target dir.
// Invocation: Command Options... [IgnoreFlag joined] dir
func runPreview(dir string, cfg PreviewConfig) string {
	if cfg.Command == "" {
		return ""
	}

	args := append([]string{}, cfg.Options...)
	if cfg.IgnoreFlag != "" && len(cfg.Ignore) > 0 {
		sep := cfg.IgnoreSeparator
		if sep == "" {
			sep = "|"
		}
		args = append(args, cfg.IgnoreFlag, strings.Join(cfg.Ignore, sep))
	}
	args = append(args, dir)

	out, err := exec.Command(cfg.Command, args...).Output()
	if err != nil {
		return "(" + cfg.Command + " error: " + err.Error() + ")"
	}
	return string(out)
}

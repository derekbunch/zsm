package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Item represents a selectable entry — either a session (live or exited) or a zoxide directory.
type Item struct {
	Name   string
	Dir    string
	Active bool    // live zellij session
	Exited bool    // resurrectable session — attach will resurrect it
	Score  float64 // zoxide frecency score (0 for sessions)
}

// SessionName returns the sanitized name suitable for zellij (no slashes).
func (i Item) SessionName() string {
	return strings.ReplaceAll(i.Name, "/", "-")
}

// applyAliases rewrites path prefixes using the provided [prefix, replacement] pairs.
// Aliases are applied in order and stop at the first match, so put more specific
// prefixes first (e.g. "~/recharge" before "~").
func applyAliases(path string, aliases [][2]string) string {
	for _, a := range aliases {
		if strings.HasPrefix(path, a[0]) {
			return a[1] + path[len(a[0]):]
		}
	}
	return path
}

// buildItems merges active sessions and zoxide directories into a single list.
// Directories whose generated name matches an active session are skipped.
// When there's a basename collision, the aliased full path is used as the name
// so session names stay short (e.g. "~rc/weekly" instead of "recharge/weekly").
func buildItems(sessions []SessionInfo, dirs []ZoxideEntry, maxLen int, aliases [][2]string) []Item {
	sessionSet := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		sessionSet[s.Name] = true
	}

	// Build name→dir mapping from zoxide directories
	baseCount := make(map[string]int, len(dirs))
	for _, d := range dirs {
		baseCount[filepath.Base(d.Dir)]++
	}

	type dirRef struct {
		dir   string
		score float64
	}
	nameToDir := make(map[string]dirRef, len(dirs))
	var dirItems []Item
	for _, d := range dirs {
		base := filepath.Base(d.Dir)
		name := base
		if baseCount[base] > 1 {
			// Collision — use parent/base for disambiguation, then apply aliases.
			name = applyAliases(filepath.Base(filepath.Dir(d.Dir))+"/"+base, aliases)
		}
		if maxLen > 0 && len(name) > maxLen {
			name = name[:maxLen]
		}
		nameToDir[name] = dirRef{d.Dir, d.Score}
		// Also map the sanitized version (slashes→dashes) for session matching
		nameToDir[strings.ReplaceAll(name, "/", "-")] = dirRef{d.Dir, d.Score}
		if !sessionSet[name] {
			dirItems = append(dirItems, Item{Name: name, Dir: d.Dir, Score: d.Score})
		}
	}

	// Active sessions, then killed/exited, then zoxide dirs (sorted by score).
	var active, exited []Item
	for _, s := range sessions {
		ref := nameToDir[s.Name]
		item := Item{
			Name:   s.Name,
			Dir:    ref.dir,
			Score:  ref.score,
			Active: !s.Exited,
			Exited: s.Exited,
		}
		if s.Exited {
			exited = append(exited, item)
		} else {
			active = append(active, item)
		}
	}
	items := make([]Item, 0, len(sessions)+len(dirItems))
	items = append(items, active...)
	items = append(items, exited...)
	items = append(items, dirItems...)

	return items
}

// maxNameLen calculates the max session name length based on the socket path limit.
func maxNameLen() int {
	socketDir := os.Getenv("ZELLIJ_SOCKET_DIR")
	if socketDir == "" {
		socketDir = filepath.Join(os.TempDir(), fmt.Sprintf("zellij-%d", os.Getuid()))
	}
	prefix := filepath.Join(socketDir, "contract_version_1") + "/"
	if max := 103 - len(prefix); max > 10 {
		return max
	}
	return 10
}

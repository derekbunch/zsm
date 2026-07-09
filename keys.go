package main

import "strings"

// Action identifies a single command the UI can perform.
type Action string

// Actions is a namespace for all available actions.
var Actions = struct {
	Attach       Action
	Quit         Action
	CursorUp     Action
	CursorDown   Action
	Terminate    Action
	Delete       Action
	Rename       Action
	CustomName   Action
	ShowPath     Action
	PreviewUp    Action
	PreviewDown  Action
	PreviewPgUp  Action
	PreviewPgDn  Action
	ZoxideBump   Action
	ZoxideDemote Action
}{
	Attach:       "attach",
	Quit:         "quit",
	CursorUp:     "cursor_up",
	CursorDown:   "cursor_down",
	Terminate:    "terminate",
	Delete:       "delete",
	Rename:       "rename",
	CustomName:   "custom_name",
	ShowPath:     "show_path",
	PreviewUp:    "preview_up",
	PreviewDown:  "preview_down",
	PreviewPgUp:  "preview_page_up",
	PreviewPgDn:  "preview_page_down",
	ZoxideBump:   "zoxide_bump",
	ZoxideDemote: "zoxide_demote",
}

// Binding maps a list of key strings to an action, with a hint label for the header.
type Binding struct {
	Keys   []string
	Label  string // shown in the hint bar ("" = hidden)
	Action Action
}

// defaultBindings lists the keybinds in header display order.
// Always-applicable actions come first so their position stays stable; context-
// specific ones (kill/rename/custom name) are appended last and can be skipped
// cleanly by the renderer without shuffling the rest of the bar.
var defaultBindings = []Binding{
	// Always applicable — fixed positions so the left side of the hint bar never shifts.
	{Keys: []string{"enter"}, Label: "attach", Action: Actions.Attach},
	{Keys: []string{"ctrl+c", "esc"}, Label: "quit", Action: Actions.Quit},
	{Keys: []string{"ctrl+l"}, Label: "path", Action: Actions.ShowPath},
	{Keys: []string{"ctrl+w"}, Label: "bump", Action: Actions.ZoxideBump},
	{Keys: []string{"ctrl+x"}, Label: "demote", Action: Actions.ZoxideDemote},
	{Keys: []string{"ctrl+d"}, Label: "delete", Action: Actions.Delete},
	// Context-specific — rendered at the tail so they can come and go without reshuffling.
	{Keys: []string{"ctrl+t"}, Label: "terminate", Action: Actions.Terminate},
	{Keys: []string{"ctrl+r"}, Label: "rename", Action: Actions.Rename},
	{Keys: []string{"ctrl+e"}, Label: "custom name", Action: Actions.CustomName},
	{Keys: []string{"up", "ctrl+p"}, Action: Actions.CursorUp}, // hidden (no label)
	{Keys: []string{"down", "ctrl+n"}, Action: Actions.CursorDown},
	// vim-style preview scrolling; ctrl+ prefix to avoid stealing filter input
	{Keys: []string{"ctrl+k"}, Action: Actions.PreviewUp},
	{Keys: []string{"ctrl+j"}, Action: Actions.PreviewDown},
	{Keys: []string{"ctrl+b", "pgup"}, Action: Actions.PreviewPgUp},
	{Keys: []string{"ctrl+f", "pgdown"}, Action: Actions.PreviewPgDn},
}

// keyToAction builds a lookup from pressed key string to its Action.
func keyToAction(bindings []Binding) map[string]Action {
	m := make(map[string]Action, len(bindings)*2)
	for _, b := range bindings {
		for _, k := range b.Keys {
			m[k] = b.Action
		}
	}
	return m
}

// hintLine renders the hint bar showing only bindings with labels, using the
// first key of each. Pass a nil skip map to show all actions; otherwise skip
// actions present in the map (useful for hiding context-inapplicable actions).
func hintLine(bindings []Binding, skip map[Action]bool) string {
	var parts []string
	for _, b := range bindings {
		if b.Label == "" || len(b.Keys) == 0 || skip[b.Action] {
			continue
		}
		parts = append(parts, b.Keys[0]+":"+b.Label)
	}
	return strings.Join(parts, " │ ")
}

package main

import (
	"fmt"
	"os"
	"strings"

	kdl "github.com/sblinch/kdl-go"
	"github.com/sblinch/kdl-go/document"
)

// LayoutPane represents a single pane in a zellij layout.
type LayoutPane struct {
	Command string
	CWD     string
	Focus   bool
}

// LayoutTab represents a tab containing tiled and floating panes.
type LayoutTab struct {
	Name   string
	Focus  bool
	Panes  []LayoutPane
	Floats []LayoutPane
}

// SessionLayout is the parsed result of `zellij action dump-layout`.
type SessionLayout struct {
	CWD  string
	Tabs []LayoutTab
}

// parseLayout parses KDL output from `zellij action dump-layout`.
// Only extracts live session state (tabs/panes); skips swap layouts and templates.
func parseLayout(input string) (SessionLayout, error) {
	doc, err := kdl.Parse(strings.NewReader(input))
	if err != nil {
		return SessionLayout{}, fmt.Errorf("kdl parse: %w", err)
	}

	var result SessionLayout
	for _, node := range doc.Nodes {
		if node.Name.ValueString() != "layout" {
			continue
		}
		result.CWD = nodeChildArg(node, "cwd")
		for _, child := range node.Children {
			if child.Name.ValueString() == "tab" {
				result.Tabs = append(result.Tabs, parseTab(child))
			}
		}
	}
	return result, nil
}

func parseTab(node *document.Node) LayoutTab {
	tab := LayoutTab{
		Name:  nodePropStr(node, "name"),
		Focus: nodePropBool(node, "focus"),
	}
	for _, child := range node.Children {
		switch child.Name.ValueString() {
		case "pane":
			if nodePropBool(child, "borderless") {
				continue
			}
			tab.Panes = append(tab.Panes, parsePane(child))
		case "floating_panes":
			for _, fc := range child.Children {
				if fc.Name.ValueString() == "pane" {
					tab.Floats = append(tab.Floats, parsePane(fc))
				}
			}
		}
	}
	return tab
}

func parsePane(node *document.Node) LayoutPane {
	pane := LayoutPane{
		Command: nodePropStr(node, "command"),
		CWD:     nodePropStr(node, "cwd"),
		Focus:   nodePropBool(node, "focus"),
	}
	if pane.CWD == "" {
		pane.CWD = nodeChildArg(node, "cwd")
	}
	return pane
}

func nodePropStr(node *document.Node, key string) string {
	if v, ok := node.Properties[key]; ok {
		if s, ok := v.ResolvedValue().(string); ok {
			return s
		}
	}
	return ""
}

func nodePropBool(node *document.Node, key string) bool {
	if v, ok := node.Properties[key]; ok {
		if b, ok := v.ResolvedValue().(bool); ok {
			return b
		}
	}
	return false
}

func nodeChildArg(node *document.Node, name string) string {
	for _, child := range node.Children {
		if child.Name.ValueString() == name && len(child.Arguments) > 0 {
			if s, ok := child.Arguments[0].ResolvedValue().(string); ok {
				return s
			}
		}
	}
	return ""
}

// renderLayoutTree formats a SessionLayout as a tree string for the preview pane.
func renderLayoutTree(layout SessionLayout) string {
	var b strings.Builder
	home, _ := os.UserHomeDir()

	shorten := func(p string) string {
		if home != "" && strings.HasPrefix(p, home) {
			return "~" + p[len(home):]
		}
		return p
	}

	multiTab := len(layout.Tabs) > 1

	for ti, tab := range layout.Tabs {
		if multiTab {
			name := strings.TrimSpace(tab.Name)
			if name == "" {
				name = fmt.Sprintf("tab %d", ti+1)
			}
			focus := ""
			if tab.Focus {
				focus = " *"
			}
			b.WriteString(name + focus + "\n")
		}

		type entry struct {
			label    string
			children []string
		}
		var entries []entry

		for _, p := range tab.Panes {
			entries = append(entries, entry{label: paneLabel(p, layout.CWD, shorten)})
		}
		if len(tab.Floats) > 0 {
			var kids []string
			for _, f := range tab.Floats {
				kids = append(kids, paneLabel(f, layout.CWD, shorten))
			}
			entries = append(entries, entry{label: "󰉈 floating", children: kids})
		}

		indent := ""
		if multiTab {
			indent = "  "
		}

		for i, e := range entries {
			last := i == len(entries)-1
			conn := "├── "
			if last {
				conn = "└── "
			}
			b.WriteString(indent + conn + e.label + "\n")

			childPad := indent + "│   "
			if last {
				childPad = indent + "    "
			}
			for j, child := range e.children {
				cc := "├── "
				if j == len(e.children)-1 {
					cc = "└── "
				}
				b.WriteString(childPad + cc + child + "\n")
			}
		}

		if multiTab && ti < len(layout.Tabs)-1 {
			b.WriteString("\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func paneLabel(p LayoutPane, sessionCWD string, shorten func(string) string) string {
	name := p.Command
	if name == "" {
		name = "shell"
	}
	if p.Focus {
		name += " *"
	}
	if p.CWD != "" && p.CWD != sessionCWD {
		name += "  " + shorten(p.CWD)
	}
	return name
}

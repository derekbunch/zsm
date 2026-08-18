package main

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/sahilm/fuzzy"

	"github.com/derekbunch/zsm/icons"
)

// mode represents the current UI state.
type mode int

const (
	modeBrowse mode = iota
	modeTerminate
	modeDelete
	modeRename
	modeCustomName
	modePath
)

type model struct {
	items           []Item            // all items (sessions + directories)
	filtered        []Item            // items matching the current filter
	cursor          int               // position in filtered list
	filter          textinput.Model   // fuzzy search input
	input           textinput.Model   // rename/custom name input
	mode            mode              // current UI state
	theme           Theme             // color config (kept for re-building styles on bg change)
	styles          styles            // theme-aware styles
	width           int               // terminal width
	height          int               // terminal height
	action          func() error      // post-exit action (attach/create)
	chosen          Item              // the item the user selected (so main.go can report on it)
	status          string            // transient status message shown under the header (errors, etc.)
	layout          string            // zellij layout to use
	maxHeight       int               // max visible items (0 = fill terminal)
	maxWidth        int               // max width (0 = fill terminal)
	quitting        bool              // set on exit to clear the UI
	previews        map[Item]string   // cached preview content per item
	previewScroll   int               // current line offset for vim-style scrolling
	previewPosition string            // "top" | "bottom" | "left" | "right" | "none"
	previewCfg      PreviewConfig     // configurable preview command (see config.go)
	showScore       bool              // whether to display zoxide score in list rows
	bindings        []Binding         // keybind definitions (for hint rendering)
	keymap          map[string]Action // key string → action (for dispatch)
	maxNameLen      int               // socket-path-aware cap on session names (for rebuilds)
	pathAliases     [][2]string       // path prefix replacements (for rebuilds)
}

func newModel(items []Item, cfg Config) model {
	filter := textinput.New()
	filter.Placeholder = "Type to filter..."
	filter.Prompt = "󱊄 "
	filter.SetWidth(40)
	filter.Focus()

	input := textinput.New()
	input.Prompt = "> "

	return model{
		items:           items,
		filtered:        items,
		filter:          filter,
		input:           input,
		theme:           cfg.Theme,
		styles:          buildStyles(cfg.Theme, true), // assume dark until we hear from the terminal
		layout:          cfg.Layout,
		maxHeight:       cfg.Height,
		maxWidth:        cfg.Width,
		previewPosition: cfg.PreviewPosition,
		previews:        make(map[Item]string),
		previewCfg:      cfg.Preview,
		showScore:       cfg.ShowScore,
		bindings:        defaultBindings,
		keymap:          keyToAction(defaultBindings),
		maxNameLen:      maxNameLen(),
		pathAliases:     cfg.PathAliases,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.previewCmd())
}

// previewCmd returns a Cmd to fetch the preview for the current cursor item.
// Returns nil if preview is disabled, there's no item, or it's already cached.
func (m model) previewCmd() tea.Cmd {
	if m.previewPosition == "none" || m.previewPosition == "" {
		return nil
	}
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	item := m.filtered[m.cursor]
	if _, ok := m.previews[item]; ok {
		return nil
	}
	return fetchPreview(item, m.previewCfg)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case previewMsg:
		// Cache by item — the renderer looks up by current cursor,
		// so late-arriving fetches just populate the cache harmlessly.
		m.previews[msg.item] = msg.content
		return m, nil

	case tea.BackgroundColorMsg:
		// Terminal reports whether it has a dark or light background
		m.styles = buildStyles(m.theme, msg.IsDark())
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.filter.SetWidth(m.width - 4) // leave room for prompt + padding
		return m, nil

	case tea.KeyPressMsg:
		switch m.mode {
		case modeBrowse:
			return m.updateBrowse(msg)
		case modeTerminate, modeDelete:
			return m.updateConfirm(msg)
		case modePath:
			// Any key dismisses the path display
			m.mode = modeBrowse
			return m, nil
		case modeRename, modeCustomName:
			return m.updateInput(msg)
		}
	}

	// Pass through to filter in browse mode (e.g. cursor blink ticks).
	// Only re-filter when the query actually changed — blink ticks would
	// otherwise re-run fuzzy matching on every tick and spike CPU.
	if m.mode == modeBrowse {
		prev := m.filter.Value()
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		if m.filter.Value() != prev {
			m.applyFilter()
		}
		return m, cmd
	}

	return m, nil
}

func (m model) updateBrowse(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	action, bound := m.keymap[msg.String()]
	if !bound {
		// Unbound key — pass to filter
		prev := m.filter.Value()
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		if m.filter.Value() == prev {
			return m, cmd // no filter change; skip re-filter and preview refresh
		}
		m.applyFilter()
		m.cursor = 0
		m.previewScroll = 0
		return m, tea.Batch(cmd, m.previewCmd())
	}

	// Clear any previous status; specific branches set one if they can't act.
	m.status = ""

	noSelection := len(m.filtered) == 0
	var item Item
	if !noSelection {
		item = m.filtered[m.cursor]
	}

	switch action {
	case Actions.Quit:
		// Esc clears the filter first, then quits on a subsequent press with empty filter.
		// Ctrl+c always quits immediately.
		if msg.String() == "esc" && m.filter.Value() != "" {
			m.filter.SetValue("")
			m.applyFilter()
			m.cursor = 0
			m.previewScroll = 0
			return m, m.previewCmd()
		}
		m.quitting = true
		return m, tea.Quit
	case Actions.CursorUp:
		if m.cursor > 0 {
			m.cursor--
			m.previewScroll = 0
			return m, m.previewCmd()
		}
	case Actions.CursorDown:
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			m.previewScroll = 0
			return m, m.previewCmd()
		}
	case Actions.Attach:
		if noSelection {
			m.status = "Nothing to attach — list is empty"
			break
		}
		m.chosen = item
		m.action = attachOrCreate(m.chosen, m.layout)
		m.quitting = true
		return m, tea.Quit
	case Actions.Terminate:
		switch {
		case noSelection:
			m.status = "Nothing to terminate — list is empty"
		case !item.Active:
			m.status = fmt.Sprintf("Cannot terminate %q — not an active session", item.Name)
		default:
			m.mode = modeTerminate
		}
	case Actions.Delete:
		if noSelection {
			m.status = "Nothing to delete — list is empty"
			break
		}
		m.mode = modeDelete
	case Actions.Rename:
		switch {
		case noSelection:
			m.status = "Nothing to rename — list is empty"
		case !item.Active:
			m.status = "Rename only applies to active sessions"
		default:
			m.mode = modeRename
			m.input.SetValue(item.Name)
			m.input.Focus()
		}
	case Actions.CustomName:
		switch {
		case noSelection:
			m.status = "Nothing to name — list is empty"
		case item.Active:
			m.status = "Session is already active — use rename instead"
		default:
			m.mode = modeCustomName
			m.input.SetValue(item.Name)
			m.input.Focus()
		}
	case Actions.ShowPath:
		switch {
		case noSelection:
			m.status = "Nothing selected"
		case item.Dir == "":
			m.status = fmt.Sprintf("No path known for %q", item.Name)
		default:
			m.mode = modePath
		}
	case Actions.PreviewUp:
		m.scrollPreview(-1)
	case Actions.PreviewDown:
		m.scrollPreview(1)
	case Actions.PreviewPgUp:
		m.scrollPreview(-m.previewHeight())
	case Actions.PreviewPgDn:
		m.scrollPreview(m.previewHeight())
	case Actions.ZoxideBump:
		switch {
		case noSelection:
			m.status = "Nothing selected"
		case item.Dir == "":
			m.status = fmt.Sprintf("Cannot bump %q — no matching zoxide entry", item.Name)
		default:
			_ = incrementScore(item.Dir)
			m.refreshScores()
		}
	case Actions.ZoxideDemote:
		switch {
		case noSelection:
			m.status = "Nothing selected"
		case item.Dir == "":
			m.status = fmt.Sprintf("Cannot demote %q — no matching zoxide entry", item.Name)
		default:
			_ = decrementScore(item.Dir)
			m.refreshScores()
		}
	}
	return m, nil
}

// refresh re-queries zellij + zoxide and rebuilds the full item list. Used after
// session kill/delete so newly-exited or newly-available zoxide dirs reappear.
func (m *model) refresh() {
	sessions, _ := listSessions()
	dirs, _ := listDirectories()
	m.items = buildItems(sessions, dirs, m.maxNameLen, m.pathAliases)
	m.applyFilter()
}

// refreshScores re-queries zoxide and updates Item.Score in place so the list
// reflects zoxide's actual on-disk numbers (which include decay, not just ±1).
func (m *model) refreshScores() {
	entries, err := listDirectories()
	if err != nil || len(entries) == 0 {
		return
	}
	scores := make(map[string]float64, len(entries))
	for _, e := range entries {
		scores[e.Dir] = e.Score
	}
	for i := range m.items {
		if s, ok := scores[m.items[i].Dir]; ok {
			m.items[i].Score = s
		}
	}
	m.applyFilter()
}

// removeDirItem drops a (non-session) entry matching dir from items and re-filters.
func (m *model) removeDirItem(dir string) {
	out := m.items[:0]
	for _, it := range m.items {
		if !it.Active && it.Dir == dir {
			continue
		}
		out = append(out, it)
	}
	m.items = out
	m.applyFilter()
	if m.cursor >= len(m.filtered) && m.cursor > 0 {
		m.cursor--
	}
}

// scrollPreview shifts the preview by delta lines, clamped to the content bounds.
func (m *model) scrollPreview(delta int) {
	lines := strings.Count(m.currentPreview(), "\n") + 1
	maxScroll := lines - m.previewHeight()
	if maxScroll < 0 {
		maxScroll = 0
	}
	m.previewScroll += delta
	if m.previewScroll < 0 {
		m.previewScroll = 0
	}
	if m.previewScroll > maxScroll {
		m.previewScroll = maxScroll
	}
}

// updateConfirm handles the y/n prompt for both Kill and Delete modals.
func (m model) updateConfirm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if len(m.filtered) > 0 {
			item := m.filtered[m.cursor]
			switch m.mode {
			case modeTerminate:
				if item.Active {
					_ = killSession(item.Name)
				}
			case modeDelete:
				if item.Active || item.Exited {
					_ = deleteSession(item.Name)
				} else if item.Dir != "" {
					_ = deleteZoxide(item.Dir)
				}
			}
			// Rebuild from sources so e.g. deleting a session exposes its zoxide dir again.
			m.refresh()
			if m.cursor >= len(m.filtered) && m.cursor > 0 {
				m.cursor--
			}
		}
		m.mode = modeBrowse
	case "n", "N", "esc":
		m.mode = modeBrowse
	}
	return m, nil
}

func (m model) updateInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		value := m.input.Value()
		if value != "" && len(m.filtered) > 0 {
			item := m.filtered[m.cursor]
			if m.mode == modeRename {
				renameSession(item.Name, value)
				// Update the name in place
				for i := range m.items {
					if m.items[i].Name == item.Name {
						m.items[i].Name = value
						break
					}
				}
				m.applyFilter()
			} else {
				// Custom name: attach/create with the custom name
				item.Name = value
				m.chosen = item
				m.action = attachOrCreate(item, m.layout)
				m.quitting = true
				return m, tea.Quit
			}
		}
		m.mode = modeBrowse
		m.filter.Focus()
	case "esc":
		m.mode = modeBrowse
		m.filter.Focus()
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) applyFilter() {
	query := m.filter.Value()
	if query == "" {
		m.filtered = m.items
		return
	}

	// Build string slice for fuzzy matching
	names := make([]string, len(m.items))
	for i, item := range m.items {
		names[i] = item.Name
	}

	matches := fuzzy.Find(query, names)

	// Partition matches: active first, then killed/exited, then zoxide dirs by score.
	var active, exited, dirs []Item
	for _, match := range matches {
		item := m.items[match.Index]
		switch {
		case item.Active:
			active = append(active, item)
		case item.Exited:
			exited = append(exited, item)
		default:
			dirs = append(dirs, item)
		}
	}
	sort.SliceStable(dirs, func(i, j int) bool { return dirs[i].Score > dirs[j].Score })
	m.filtered = append(append(active, exited...), dirs...)

	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

type styles struct {
	active   lipgloss.Style
	dir      lipgloss.Style
	selected lipgloss.Style
	dim      lipgloss.Style
	header   lipgloss.Style
	exited   lipgloss.Style // list color for exited/resurrectable sessions
	pane     lipgloss.Style // bordered box used for list/preview panels
	modal    lipgloss.Style // bordered box for confirmation/input modals
	accent   lipgloss.Style // modal titles for safe actions (rename, custom name)
	warning  lipgloss.Style // modal titles for semi-destructive actions (kill)
	danger   lipgloss.Style // modal titles for destructive actions (delete)
}

// buildStyles creates theme-aware styles using LightDark to pick the right palette.
func buildStyles(theme Theme, isDark bool) styles {
	ld := lipgloss.LightDark(isDark)
	c := func(light, dark string) color.Color {
		return ld(lipgloss.Color(light), lipgloss.Color(dark))
	}

	accent := c(theme.Light.Accent, theme.Dark.Accent)
	warning := c(theme.Light.Warning, theme.Dark.Warning)
	danger := c(theme.Light.Danger, theme.Dark.Danger)

	return styles{
		active:   lipgloss.NewStyle().Foreground(c(theme.Light.Active, theme.Dark.Active)),
		dir:      lipgloss.NewStyle().Foreground(c(theme.Light.Dir, theme.Dark.Dir)),
		selected: lipgloss.NewStyle().Bold(true).Background(c(theme.Light.Selected, theme.Dark.Selected)).Foreground(c(theme.Light.Text, theme.Dark.Text)),
		dim:      lipgloss.NewStyle().Foreground(c(theme.Light.Dim, theme.Dark.Dim)),
		header:   lipgloss.NewStyle().Foreground(c(theme.Light.Dim, theme.Dark.Dim)),
		exited:   lipgloss.NewStyle().Foreground(c(theme.Light.Exited, theme.Dark.Exited)),
		pane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(c(theme.Light.Dim, theme.Dark.Dim)).
			Padding(0, 1),
		// Modal border is set per-action in renderModal (accent/warning/danger).
		modal: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2),
		accent:  lipgloss.NewStyle().Foreground(accent).Bold(true),
		warning: lipgloss.NewStyle().Foreground(warning).Bold(true),
		danger:  lipgloss.NewStyle().Foreground(danger).Bold(true),
	}
}

func (m model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}

	header := m.renderHeader()

	// Compute pane widths. Each pane has a rounded border (2 cols) and 1-col
	// horizontal padding on each side (2 cols) = 4 cols of overhead per pane.
	// Widths are based on layout only (not preview content) so borders stay put
	// even when the preview is empty or still loading.
	var listWidth, previewWidth int
	sideBySide := m.previewPosition == "left" || m.previewPosition == "right"
	hasPreview := m.previewPosition != "none" && m.previewPosition != ""
	if sideBySide && m.width > 0 {
		half := m.width / 2
		listWidth = half - 4
		previewWidth = m.width - half - 4
	} else if m.width > 0 {
		listWidth = m.width - 4
		previewWidth = listWidth
	}

	paneHeight := m.listHeight()

	// In delete/rename/custom-name mode, replace the whole body with a centered modal.
	if modal := m.renderModal(m.width); modal != "" {
		bodyHeight := paneHeight + 2 // match the pane we'd otherwise render
		body := lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, modal)
		return tea.NewView(header + body)
	}

	// MaxHeight caps the pane — lipgloss's Height is only a minimum, so content
	// that wraps (e.g. dump-screen ANSI lines) can push it taller without the cap.
	paneStyle := m.styles.pane.MaxHeight(paneHeight + 2) // +2 for the border rows
	list := paneStyle.Width(listWidth).Height(paneHeight).Render(m.renderList(listWidth))

	body := list
	if hasPreview {
		preview := paneStyle.Width(previewWidth).Height(paneHeight).Render(m.renderPreview(previewWidth))
		switch m.previewPosition {
		case "right":
			body = lipgloss.JoinHorizontal(lipgloss.Top, list, preview)
		case "left":
			body = lipgloss.JoinHorizontal(lipgloss.Top, preview, list)
		case "top":
			body = lipgloss.JoinVertical(lipgloss.Left, preview, list)
		case "bottom":
			body = lipgloss.JoinVertical(lipgloss.Left, list, preview)
		}
	}

	return tea.NewView(header + body)
}

// hintSkip returns the set of actions that don't apply to the current item,
// so the hint bar only surfaces things the user can actually do.
func (m model) hintSkip() map[Action]bool {
	skip := make(map[Action]bool)
	if len(m.filtered) == 0 {
		return skip
	}
	item := m.filtered[m.cursor]
	// Zoxide bump/demote always apply — sessions still correspond to a zoxide
	// dir under the hood.
	switch {
	case item.Active:
		skip[Actions.CustomName] = true
	case item.Exited:
		skip[Actions.Terminate] = true // already exited — nothing to kill
		skip[Actions.Rename] = true
		skip[Actions.CustomName] = true
	default: // zoxide dir (not a session)
		skip[Actions.Terminate] = true
		skip[Actions.Rename] = true
	}
	return skip
}

// renderHeader builds the filter input and keybind hints.
// Mode-specific overlays (delete/rename/custom name) render as a centered
// modal in View(); the small inline path display stays here since it's just info.
func (m model) renderHeader() string {
	var b strings.Builder
	b.WriteString(m.filter.View())
	b.WriteString("\n")
	b.WriteString(m.styles.header.Render(hintLine(m.bindings, m.hintSkip())))
	b.WriteString("\n")

	// Transient feedback for failed/no-op actions (cleared on next keypress).
	if m.status != "" {
		b.WriteString(m.styles.warning.Render("  " + m.status))
		b.WriteString("\n")
	}

	if m.mode == modePath && len(m.filtered) > 0 {
		b.WriteString("\n")
		b.WriteString(m.styles.dim.Render("  " + m.filtered[m.cursor].Dir))
	}

	return b.String()
}

// renderModal returns the delete/rename/custom-name confirmation dialog, or
// an empty string when no modal is active. The modal replaces the list/preview
// body so it gets full visual focus.
func (m model) renderModal(width int) string {
	if len(m.filtered) == 0 {
		return ""
	}
	item := m.filtered[m.cursor]

	var title, body, hint string
	titleStyle := m.styles.accent
	border := m.styles.modal.BorderForeground(m.styles.accent.GetForeground())
	switch m.mode {
	case modeTerminate:
		title = "⏻  Confirm terminate"
		body = fmt.Sprintf("Terminate session %q? (resurrect data kept)", item.Name)
		hint = "y: yes   n/esc: cancel"
		titleStyle = m.styles.warning
		border = border.BorderForeground(m.styles.warning.GetForeground())
	case modeDelete:
		kind := "session"
		switch {
		case item.Exited:
			kind = "exited session"
		case !item.Active:
			kind = "zoxide entry"
		}
		title = "⚠  Confirm delete"
		body = fmt.Sprintf("Delete %s %q?", kind, item.Name)
		hint = "y: yes   n/esc: cancel"
		titleStyle = m.styles.danger
		border = border.BorderForeground(m.styles.danger.GetForeground())
	case modeRename:
		title = "✎ Rename session"
		body = m.input.View()
		hint = "enter: confirm   esc: cancel"
	case modeCustomName:
		title = "✎ Session name"
		body = m.input.View()
		hint = "enter: confirm   esc: cancel"
	default:
		return ""
	}

	// Box is roughly half the available width, minimum 40.
	boxWidth := width / 2
	if boxWidth < 40 {
		boxWidth = 40
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(title),
		"",
		body,
		"",
		m.styles.dim.Render(hint),
	)

	return border.Width(boxWidth).Render(content)
}

// renderList builds the scrollable item list capped to configured height.
func (m model) renderList(width int) string {
	var b strings.Builder

	visible := m.listHeight()

	start := 0
	if m.cursor >= visible {
		start = m.cursor - visible + 1
	}

	// Compute score column width once so all rows align.
	scoreCol := ""
	var scoreWidth int
	if m.showScore {
		scoreCol = "       "
		scoreWidth = len(scoreCol) // reserve space even for zero-scored sessions
	}

	for i := start; i < len(m.filtered) && i < start+visible; i++ {
		item := m.filtered[i]

		var prefix string
		if m.showScore {
			if item.Score > 0 {
				prefix = m.styles.dim.Render(fmt.Sprintf("%*.1f ", scoreWidth-1, item.Score))
			} else {
				prefix = strings.Repeat(" ", scoreWidth)
			}
		}

		// Truncate name so long paths don't wrap and push the pane taller than the list.
		// Budget = listWidth - score prefix - icon(1) - separator space(1).
		nameBudget := width - scoreWidth - 2
		body := listIcon(item) + " " + runewidth.Truncate(item.Name, nameBudget, "…")
		switch {
		case item.Active:
			body = m.styles.active.Render(body)
		case item.Exited:
			body = m.styles.exited.Render(body)
		default:
			body = m.styles.dir.Render(body)
		}

		row := prefix + body
		if i == m.cursor {
			row = m.styles.selected.Render(row)
		}

		b.WriteString(row)
		b.WriteString("\n")
	}
	_ = scoreCol

	if len(m.filtered) == 0 {
		b.WriteString(m.styles.dim.Render("  No matches"))
		b.WriteString("\n")
	}

	return padLines(b.String(), visible)
}

// padLines normalizes content to exactly n lines joined by "\n".
// Trailing blank lines in input are preserved; short content is padded;
// long content is truncated.
func padLines(s string, n int) string {
	trimmed := strings.TrimRight(s, "\n")
	lines := strings.Split(trimmed, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	for len(lines) < n {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// currentPreview returns the cached preview for the item under the cursor.
func (m model) currentPreview() string {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return ""
	}
	return m.previews[m.filtered[m.cursor]]
}

// renderPreview builds the preview pane, applying scroll offset and always
// returning exactly previewHeight() lines at no more than width columns each
// so the pane border stays put (session dumps often return very wide lines).
func (m model) renderPreview(width int) string {
	target := m.previewHeight()
	lines := strings.Split(m.currentPreview(), "\n")

	start := m.previewScroll
	if start > len(lines) {
		start = len(lines)
	}
	end := start + target
	if end > len(lines) {
		end = len(lines)
	}

	if width > 0 {
		truncated := make([]string, 0, end-start)
		for _, line := range lines[start:end] {
			// ansi.Truncate is ANSI-aware: counts only visible width and won't
			// split escape sequences mid-token.
			truncated = append(truncated, ansi.Truncate(line, width, ""))
		}
		return padLines(strings.Join(truncated, "\n"), target)
	}
	return padLines(strings.Join(lines[start:end], "\n"), target)
}

// previewHeight returns how many lines the preview pane is allowed to use.
func (m model) previewHeight() int {
	switch m.previewPosition {
	case "left", "right":
		// Side-by-side: match the list height
		return m.listHeight()
	case "top", "bottom":
		// Stacked: terminal height minus list and header overhead
		if m.height > 0 {
			h := m.height - m.listHeight() - 4
			if h > 0 {
				return h
			}
		}
		return 10
	}
	return 0
}

// headerLines returns how many lines the header occupies.
func (m model) headerLines() int {
	n := 2 // filter + hints
	if m.status != "" {
		n++
	}
	if m.mode == modePath && len(m.filtered) > 0 {
		n += 2 // blank line + path
	}
	return n
}

// listHeight returns the number of visible item rows.
func (m model) listHeight() int {
	visible := m.maxHeight
	overhead := m.headerLines() + 2 // +2 for pane border
	if visible <= 0 || (m.height > 0 && visible > m.height-overhead) {
		visible = m.height - overhead
	}
	if visible < 1 {
		visible = 10
	}
	return visible
}

func attachOrCreate(item Item, layout string) func() error {
	return func() error {
		name := item.SessionName()
		if item.Active || item.Exited {
			return attachSession(name)
		}
		return createSession(name, item.Dir, layout)
	}
}

// listIcon returns the glyph shown next to an item in the main list.
func listIcon(item Item) string {
	switch {
	case item.Active:
		return "\uf489" // nf-oct-terminal
	case item.Exited:
		return "\ueabd" // nf-cod-debug-disconnect (ghost-ish: session exists but inactive)
	}
	return icons.DefaultDir.Icon
}

func removeItem(items []Item, name string) []Item {
	result := make([]Item, 0, len(items))
	for _, item := range items {
		if item.Name != name {
			result = append(result, item)
		}
	}
	return result
}

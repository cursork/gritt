package main

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/cursork/gritt/codec"
)

// Type glyphs for compound value previews
const (
	glyphNamespace = '#'
	glyphMatrix    = '⊞'
	glyphVector    = '≡'
)

// DataBrowserPane displays structured APLAN data with drill-down navigation.
// Implements PaneContent.
type DataBrowserPane struct {
	root  any           // top-level parsed value
	stack []browseEntry // view stack for drill-down
	name  string        // variable name

	// Current view state (for the active stack level)
	selected int // cursor row
	scroll   int // scroll offset
	colSel   int // column selection (matrices only)

	// Editing state
	editing    bool   // currently editing a cell
	editBuf    []rune // edit buffer
	editCursor int    // cursor position in editBuf
	editErr    string // validation error message
	modified   bool   // any values changed since open

	// Callbacks
	onClose func()

	// Styles
	selectedStyle lipgloss.Style
	normalStyle   lipgloss.Style
	headerStyle   lipgloss.Style
	crumbStyle    lipgloss.Style
	glyphStyle    lipgloss.Style
}

type browseEntry struct {
	value    any
	label    string
	selected int
	scroll   int
	colSel   int
}

// NewDataBrowserPane creates a data browser for the given parsed APLAN value.
func NewDataBrowserPane(name string, value any, onClose func()) *DataBrowserPane {
	d := &DataBrowserPane{
		root:          value,
		name:          name,
		onClose:       onClose,
		selectedStyle: lipgloss.NewStyle().Foreground(AccentColor),
		normalStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		headerStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Bold(true),
		crumbStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		glyphStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("110")),
	}
	d.stack = []browseEntry{{value: value, label: name}}
	return d
}

func (d *DataBrowserPane) currentValue() any {
	return d.stack[len(d.stack)-1].value
}

// Title returns the breadcrumb path.
func (d *DataBrowserPane) Title() string {
	if len(d.stack) <= 1 {
		return d.name
	}
	parts := make([]string, len(d.stack))
	for i, e := range d.stack {
		parts[i] = e.label
	}
	return strings.Join(parts, " > ")
}

// itemCount returns the number of navigable items in the current value.
func (d *DataBrowserPane) itemCount() int {
	switch v := d.currentValue().(type) {
	case *codec.Namespace:
		return len(v.Keys)
	case *codec.Array:
		if len(v.Shape) == 0 {
			return 0
		}
		return v.Shape[0]
	case []any:
		return len(v)
	default:
		return 0
	}
}

// colCount returns the number of columns for matrix views.
func (d *DataBrowserPane) colCount() int {
	v, ok := d.currentValue().(*codec.Array)
	if !ok || len(v.Shape) < 2 {
		return 0
	}
	return v.Shape[1]
}

// Render renders the data browser content.
func (d *DataBrowserPane) Render(w, h int) string {
	if h <= 0 || w <= 0 {
		return ""
	}

	val := d.currentValue()

	switch v := val.(type) {
	case *codec.Namespace:
		return d.renderNamespace(v, w, h)
	case *codec.Array:
		return d.renderMatrix(v, w, h)
	case []any:
		return d.renderVector(v, w, h)
	default:
		return d.renderScalar(val, w, h)
	}
}

// --- Namespace rendering ---

func (d *DataBrowserPane) renderNamespace(ns *codec.Namespace, w, h int) string {
	if len(ns.Keys) == 0 {
		return d.normalStyle.Render("  (empty namespace)")
	}

	// Calculate key column width
	maxKeyW := 0
	for _, k := range ns.Keys {
		if len([]rune(k)) > maxKeyW {
			maxKeyW = len([]rune(k))
		}
	}
	if maxKeyW > w/3 {
		maxKeyW = w / 3
	}

	var lines []string
	for i, k := range ns.Keys {
		val := ns.Values[k]
		line := d.formatKVLine(k, val, maxKeyW, w, i == d.selected)
		lines = append(lines, line)
	}

	return d.applyScrollAt(lines, w, h, d.selected)
}

// --- Vector rendering ---

func (d *DataBrowserPane) renderVector(vec []any, w, h int) string {
	if len(vec) == 0 {
		return d.normalStyle.Render("  (empty vector)")
	}

	// Index column width: "[N]" where N is the largest index
	idxW := len(fmt.Sprintf("[%d]", len(vec)))

	var lines []string
	for i, val := range vec {
		idx := fmt.Sprintf("[%d]", i+1) // 1-based APL indices
		line := d.formatIndexedLine(idx, val, idxW, w, i == d.selected)
		lines = append(lines, line)
	}

	return d.applyScrollAt(lines, w, h, d.selected)
}

// --- Matrix rendering ---

func (d *DataBrowserPane) renderMatrix(m *codec.Array, w, h int) string {
	if len(m.Shape) == 0 || m.Shape[0] == 0 {
		return d.normalStyle.Render("  (empty array)")
	}

	rows := m.Shape[0]
	cols := 1
	if len(m.Shape) >= 2 {
		cols = m.Shape[1]
	}

	// Calculate column widths from content
	colWidths := make([]int, cols)
	for c := 0; c < cols; c++ {
		// Header width
		hdr := fmt.Sprintf("[%d]", c+1)
		if len(hdr) > colWidths[c] {
			colWidths[c] = len(hdr)
		}
		// Cell widths
		for r := 0; r < rows; r++ {
			cell := d.getMatrixCell(m, r, c)
			preview := d.cellPreview(cell)
			pw := len([]rune(preview))
			if pw > colWidths[c] {
				colWidths[c] = pw
			}
		}
	}

	// Cap column widths to available space
	rowHdrW := len(fmt.Sprintf("[%d]", rows)) + 2 // row header + padding
	maxCellW := (w - rowHdrW) / max(cols, 1)
	if maxCellW < 4 {
		maxCellW = 4
	}
	for c := range colWidths {
		if colWidths[c] > maxCellW {
			colWidths[c] = maxCellW
		}
	}

	var lines []string

	// Header row — highlight selected column
	hdr := strings.Repeat(" ", rowHdrW)
	for c := 0; c < cols; c++ {
		label := fmt.Sprintf("[%d]", c+1)
		padded := padRuneRight(label, colWidths[c]+1)
		if c == d.colSel {
			hdr += d.selectedStyle.Render(padded)
		} else {
			hdr += d.headerStyle.Render(padded)
		}
	}
	lines = append(lines, hdr)

	// Data rows
	for r := 0; r < rows; r++ {
		rowLabel := fmt.Sprintf("[%d]", r+1)
		rowLabel = padRuneRight(rowLabel, rowHdrW)

		var rowStr string
		if r == d.selected {
			rowStr = d.selectedStyle.Render(rowLabel)
		} else {
			rowStr = d.headerStyle.Render(rowLabel)
		}

		for c := 0; c < cols; c++ {
			if d.editing && r == d.selected && c == d.colSel {
				// Render edit buffer in the active cell
				rowStr += d.renderEditCell(colWidths[c] + 1)
			} else {
				cell := d.getMatrixCell(m, r, c)
				preview := d.cellPreview(cell)
				preview = truncRunes(preview, colWidths[c])
				padded := padRuneRight(preview, colWidths[c]+1)

				if r == d.selected && c == d.colSel {
					rowStr += d.selectedStyle.Render(padded)
				} else if r == d.selected {
					rowStr += d.normalStyle.Render(padded)
				} else {
					rowStr += d.normalStyle.Render(padded)
				}
			}
		}

		lines = append(lines, rowStr)
	}

	return d.applyScrollAt(lines, w, h, d.selected+1) // +1 for header row
}

func (d *DataBrowserPane) getMatrixCell(m *codec.Array, row, col int) any {
	if len(m.Shape) == 1 {
		// Column vector: just index into Data
		if row < len(m.Data) {
			return m.Data[row]
		}
		return 0
	}
	// 2D+: Data[row] is a []any of columns
	if row < len(m.Data) {
		if rowSlice, ok := m.Data[row].([]any); ok && col < len(rowSlice) {
			return rowSlice[col]
		}
	}
	return 0
}

// --- Scalar rendering ---

func (d *DataBrowserPane) renderScalar(val any, w, h int) string {
	text := codec.Serialize(val)
	line := "  " + text
	styled := d.normalStyle.Render(line)
	var lines []string
	lines = append(lines, styled)
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}
	return strings.Join(lines[:h], "\n")
}

// --- Formatting helpers ---

func (d *DataBrowserPane) formatKVLine(key string, val any, keyW, totalW int, selected bool) string {
	keyRunes := []rune(key)
	if len(keyRunes) > keyW {
		keyRunes = keyRunes[:keyW]
	}
	keyStr := string(keyRunes) + strings.Repeat(" ", keyW-len(keyRunes))

	valueW := totalW - keyW - 5 // "  key  value"

	if d.editing && selected {
		prefix := "  " + keyStr + "  "
		return d.renderEditLine(prefix, valueW, totalW)
	}

	valStr := d.valuePreview(val, valueW)

	plain := "  " + keyStr + "  " + valStr
	plainLen := len([]rune(plain))
	if plainLen < totalW {
		plain += strings.Repeat(" ", totalW-plainLen)
	}

	if selected {
		return d.selectedStyle.Render(plain)
	}
	return d.normalStyle.Render(plain)
}

func (d *DataBrowserPane) formatIndexedLine(idx string, val any, idxW, totalW int, selected bool) string {
	idxPadded := padRuneRight(idx, idxW)

	valueW := totalW - idxW - 3 // "  [N]  value"

	if d.editing && selected {
		prefix := "  " + idxPadded + " "
		return d.renderEditLine(prefix, valueW, totalW)
	}

	valStr := d.valuePreview(val, valueW)

	plain := "  " + idxPadded + " " + valStr
	plainLen := len([]rune(plain))
	if plainLen < totalW {
		plain += strings.Repeat(" ", totalW-plainLen)
	}

	if selected {
		return d.selectedStyle.Render(plain)
	}
	return d.normalStyle.Render(plain)
}

// renderEditLine renders the edit buffer with cursor for the selected line.
func (d *DataBrowserPane) renderEditLine(prefix string, valueW, totalW int) string {
	cursorStyle := lipgloss.NewStyle().Reverse(true)

	buf := d.editBuf
	cursor := d.editCursor

	// Render buffer with cursor
	var valPart string
	if cursor < len(buf) {
		valPart = string(buf[:cursor]) + cursorStyle.Render(string(buf[cursor])) + string(buf[cursor+1:])
	} else {
		valPart = string(buf) + cursorStyle.Render(" ")
	}

	// Error indicator
	if d.editErr != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		valPart += " " + errStyle.Render(d.editErr)
	}

	return d.selectedStyle.Render(prefix) + valPart
}

// renderEditCell renders the edit buffer within a fixed-width matrix cell.
func (d *DataBrowserPane) renderEditCell(cellW int) string {
	cursorStyle := lipgloss.NewStyle().Reverse(true)
	buf := d.editBuf
	cursor := d.editCursor

	if cursor < len(buf) {
		return string(buf[:cursor]) + cursorStyle.Render(string(buf[cursor])) + string(buf[cursor+1:]) + " "
	}
	return string(buf) + cursorStyle.Render(" ")
}

// valuePreview returns a display-mode preview of a value, with type glyph for compound types.
func (d *DataBrowserPane) valuePreview(val any, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	preview := d.cellPreview(val)
	return truncRunes(preview, maxW)
}

// cellPreview returns a compact preview string for a value, prefixed with type glyph if compound.
func (d *DataBrowserPane) cellPreview(val any) string {
	switch v := val.(type) {
	case *codec.Namespace:
		inner := codec.Serialize(v, codec.SerializeOptions{UseDiamond: true})
		return string(glyphNamespace) + " " + inner
	case *codec.Array:
		inner := codec.Serialize(v, codec.SerializeOptions{UseDiamond: true})
		return string(glyphMatrix) + " " + inner
	case []any:
		inner := codec.Serialize(v, codec.SerializeOptions{UseDiamond: true})
		return string(glyphVector) + " " + inner
	default:
		return codec.Serialize(val)
	}
}

// --- Scrolling ---

// applyScrollAt scrolls to keep the given line index visible.
func (d *DataBrowserPane) applyScrollAt(lines []string, w, h, selLine int) string {
	if selLine >= d.scroll+h {
		d.scroll = selLine - h + 1
	}
	if selLine < d.scroll {
		d.scroll = selLine
	}
	if d.scroll < 0 {
		d.scroll = 0
	}

	// Slice visible window
	start := d.scroll
	if start >= len(lines) {
		start = 0
	}
	end := start + h
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[start:end]

	// Pad to height
	for len(visible) < h {
		visible = append(visible, strings.Repeat(" ", w))
	}

	return strings.Join(visible[:h], "\n")
}

// --- Navigation ---

// HandleKey processes keyboard input.
func (d *DataBrowserPane) HandleKey(msg tea.KeyMsg) bool {
	if d.editing {
		return d.handleEditKey(msg)
	}

	count := d.itemCount()

	switch msg.Type {
	case tea.KeyUp:
		if d.selected > 0 {
			d.selected--
		}
		return true

	case tea.KeyDown:
		if d.selected < count-1 {
			d.selected++
		}
		return true

	case tea.KeyLeft:
		if d.colSel > 0 {
			d.colSel--
		}
		return true

	case tea.KeyRight:
		cols := d.colCount()
		if cols > 0 && d.colSel < cols-1 {
			d.colSel++
		}
		return true

	case tea.KeyHome:
		d.selected = 0
		return true

	case tea.KeyEnd:
		if count > 0 {
			d.selected = count - 1
		}
		return true

	case tea.KeyEnter:
		// Try editing scalar first, fall back to drill-in
		if d.startEdit() {
			return true
		}
		return d.drillIn()

	case tea.KeyEscape, tea.KeyBackspace:
		return d.drillOut()
	}

	return false
}

// --- Editing ---

func (d *DataBrowserPane) handleEditKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyEnter:
		d.confirmEdit()
		return true
	case tea.KeyEscape:
		d.cancelEdit()
		return true
	case tea.KeyBackspace:
		if d.editCursor > 0 {
			d.editBuf = append(d.editBuf[:d.editCursor-1], d.editBuf[d.editCursor:]...)
			d.editCursor--
			d.editErr = ""
		}
		return true
	case tea.KeyDelete:
		if d.editCursor < len(d.editBuf) {
			d.editBuf = append(d.editBuf[:d.editCursor], d.editBuf[d.editCursor+1:]...)
			d.editErr = ""
		}
		return true
	case tea.KeyLeft:
		if d.editCursor > 0 {
			d.editCursor--
		}
		return true
	case tea.KeyRight:
		if d.editCursor < len(d.editBuf) {
			d.editCursor++
		}
		return true
	case tea.KeyHome:
		d.editCursor = 0
		return true
	case tea.KeyEnd:
		d.editCursor = len(d.editBuf)
		return true
	case tea.KeyRunes:
		d.editBuf = append(d.editBuf[:d.editCursor], append(msg.Runes, d.editBuf[d.editCursor:]...)...)
		d.editCursor += len(msg.Runes)
		d.editErr = ""
		return true
	}
	return true // consume all keys while editing
}

// startEdit begins editing the currently selected scalar value.
func (d *DataBrowserPane) startEdit() bool {
	val := d.selectedValue()
	if val == nil {
		return false
	}
	// Only scalars are editable
	switch val.(type) {
	case *codec.Namespace, *codec.Array, []any:
		return false
	}

	// Show raw value for editing (no quotes for strings, ¯ for APL negatives)
	var text string
	switch v := val.(type) {
	case string:
		text = v
	default:
		text = codec.Serialize(val)
	}

	d.editing = true
	d.editBuf = []rune(text)
	d.editCursor = len(d.editBuf)
	d.editErr = ""
	return true
}

// confirmEdit validates and applies the edit.
func (d *DataBrowserPane) confirmEdit() {
	original := d.selectedValue()
	text := string(d.editBuf)

	newVal, err := convertToType(text, original)
	if err != nil {
		d.editErr = err.Error()
		return
	}

	d.setSelectedValue(newVal)
	d.editing = false
	d.editBuf = nil
	d.editErr = ""
	d.modified = true
}

// cancelEdit discards the current edit.
func (d *DataBrowserPane) cancelEdit() {
	d.editing = false
	d.editBuf = nil
	d.editErr = ""
}

// selectedValue returns the value at the current cursor position.
func (d *DataBrowserPane) selectedValue() any {
	switch v := d.currentValue().(type) {
	case *codec.Namespace:
		if d.selected >= 0 && d.selected < len(v.Keys) {
			return v.Values[v.Keys[d.selected]]
		}
	case *codec.Array:
		return d.getMatrixCell(v, d.selected, d.colSel)
	case []any:
		if d.selected >= 0 && d.selected < len(v) {
			return v[d.selected]
		}
	}
	return nil
}

// setSelectedValue updates the value at the current cursor position.
func (d *DataBrowserPane) setSelectedValue(newVal any) {
	switch v := d.currentValue().(type) {
	case *codec.Namespace:
		if d.selected >= 0 && d.selected < len(v.Keys) {
			v.Values[v.Keys[d.selected]] = newVal
		}
	case *codec.Array:
		d.setMatrixCell(v, d.selected, d.colSel, newVal)
	case []any:
		if d.selected >= 0 && d.selected < len(v) {
			v[d.selected] = newVal
		}
	}
}

func (d *DataBrowserPane) setMatrixCell(m *codec.Array, row, col int, val any) {
	if len(m.Shape) == 1 {
		if row < len(m.Data) {
			m.Data[row] = val
		}
		return
	}
	if row < len(m.Data) {
		if rowSlice, ok := m.Data[row].([]any); ok && col < len(rowSlice) {
			rowSlice[col] = val
		}
	}
}

// convertToType parses text into the same Go type as original.
func convertToType(text string, original any) (any, error) {
	switch original.(type) {
	case int:
		s := strings.ReplaceAll(text, "¯", "-")
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("not a valid integer")
		}
		return n, nil
	case float64:
		s := strings.ReplaceAll(text, "¯", "-")
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("not a valid number")
		}
		return f, nil
	case complex128:
		s := strings.ReplaceAll(text, "¯", "-")
		// Parse J notation: realJimag
		if idx := strings.IndexAny(s, "Jj"); idx >= 0 {
			re, err1 := strconv.ParseFloat(s[:idx], 64)
			im, err2 := strconv.ParseFloat(s[idx+1:], 64)
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("not a valid complex number (use NJN)")
			}
			return complex(re, im), nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("not a valid complex number")
		}
		return complex(f, 0), nil
	case string:
		return text, nil
	default:
		return nil, fmt.Errorf("cannot edit this type")
	}
}

// drillIn pushes the selected value onto the stack.
func (d *DataBrowserPane) drillIn() bool {
	val := d.currentValue()
	var child any
	var label string

	switch v := val.(type) {
	case *codec.Namespace:
		if d.selected < 0 || d.selected >= len(v.Keys) {
			return false
		}
		key := v.Keys[d.selected]
		child = v.Values[key]
		label = key

	case *codec.Array:
		if len(v.Shape) == 0 || d.selected < 0 || d.selected >= v.Shape[0] {
			return false
		}
		if len(v.Shape) >= 2 {
			child = d.getMatrixCell(v, d.selected, d.colSel)
			label = fmt.Sprintf("[%d;%d]", d.selected+1, d.colSel+1)
		} else {
			child = d.getMatrixCell(v, d.selected, 0)
			label = fmt.Sprintf("[%d]", d.selected+1)
		}

	case []any:
		if d.selected < 0 || d.selected >= len(v) {
			return false
		}
		child = v[d.selected]
		label = fmt.Sprintf("[%d]", d.selected+1)
	}

	if child == nil {
		return false
	}

	// Only drill into compound values
	switch child.(type) {
	case *codec.Namespace, *codec.Array, []any:
		// Save current state
		d.stack[len(d.stack)-1].selected = d.selected
		d.stack[len(d.stack)-1].scroll = d.scroll
		d.stack[len(d.stack)-1].colSel = d.colSel

		// Push new level
		d.stack = append(d.stack, browseEntry{value: child, label: label})
		d.selected = 0
		d.scroll = 0
		d.colSel = 0
		return true
	}

	return false
}

// drillOut pops the stack. At root level, closes the pane.
func (d *DataBrowserPane) drillOut() bool {
	if len(d.stack) <= 1 {
		if d.onClose != nil {
			d.onClose()
		}
		return true
	}

	// Pop
	d.stack = d.stack[:len(d.stack)-1]
	top := d.stack[len(d.stack)-1]
	d.selected = top.selected
	d.scroll = top.scroll
	d.colSel = top.colSel
	return true
}

// HandleMouse processes mouse input.
func (d *DataBrowserPane) HandleMouse(x, y int, msg tea.MouseMsg) bool {
	count := d.itemCount()
	if count == 0 {
		return false
	}

	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
		target := y + d.scroll
		if target >= 0 && target < count {
			d.selected = target
		}
		return true
	}

	return false
}

// --- Utilities ---

func truncRunes(s string, maxW int) string {
	runes := []rune(s)
	if len(runes) <= maxW {
		return s
	}
	if maxW <= 1 {
		return string(runes[:maxW])
	}
	return string(runes[:maxW-1]) + "…"
}

func padRuneRight(s string, w int) string {
	runes := []rune(s)
	if len(runes) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(runes))
}

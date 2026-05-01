package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cursork/gritt/codec"
)

// withAppendRowBinding configures a pane to treat Down as the append-row key,
// matching the default `gritt.default.json` binding.
func withAppendRowBinding(d *DataBrowserPane) *DataBrowserPane {
	d.SetAppendRowBinding(key.NewBinding(key.WithKeys("down")))
	return d
}

// withMutationBindings configures all five data-browser commands to their
// shipped defaults so tests exercise the same wiring real users get.
func withMutationBindings(d *DataBrowserPane) *DataBrowserPane {
	d.SetAppendRowBinding(key.NewBinding(key.WithKeys("down")))
	d.SetAppendColumnBinding(key.NewBinding(key.WithKeys("right")))
	d.SetDeleteRowBinding(key.NewBinding(key.WithKeys("ctrl+d")))
	d.SetDeleteColumnBinding(key.NewBinding(key.WithKeys("alt+d")))
	d.SetCloseDiscardBinding(key.NewBinding(key.WithKeys("ctrl+w")))
	return d
}

// --- Test data ---

func testNamespace() *codec.Namespace {
	return &codec.Namespace{
		Keys: []string{"name", "age", "scores", "meta"},
		Values: map[string]any{
			"name":   "Alice",
			"age":    42,
			"scores": []any{1, 2, 3},
			"meta": &codec.Namespace{
				Keys:   []string{"version", "flags"},
				Values: map[string]any{"version": 2, "flags": []any{1, 0, 1}},
			},
		},
	}
}

func testMatrix() *codec.Array {
	return &codec.Array{
		Data:  []any{[]any{1, 2, 3}, []any{4, 5, 6}},
		Shape: []int{2, 3},
	}
}

func testVector() []any {
	return []any{42, "hello", &codec.Namespace{
		Keys:   []string{"x"},
		Values: map[string]any{"x": 1},
	}}
}

// --- Creation & Title ---

func TestDataBrowserTitle(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)
	if db.Title() != "data" {
		t.Errorf("Title() = %q, want %q", db.Title(), "data")
	}
}

func TestDataBrowserTitleBreadcrumbs(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)
	// Drill into "meta"
	db.selected = 3 // meta is the 4th key
	db.drillIn()

	got := db.Title()
	if got != "data > meta" {
		t.Errorf("Title() = %q, want %q", got, "data > meta")
	}

	// Drill into "version"
	db.selected = 0
	// version is a scalar, drillIn should fail
	if db.drillIn() {
		t.Error("drillIn on scalar should return false")
	}

	// Drill into "flags" (a vector)
	db.selected = 1
	db.drillIn()
	got = db.Title()
	if got != "data > meta > flags" {
		t.Errorf("Title() = %q, want %q", got, "data > meta > flags")
	}
}

// --- Item counts ---

func TestDataBrowserItemCountNamespace(t *testing.T) {
	db := NewDataBrowserPane("ns", testNamespace(), nil)
	if db.itemCount() != 4 {
		t.Errorf("itemCount() = %d, want 4", db.itemCount())
	}
}

func TestDataBrowserItemCountMatrix(t *testing.T) {
	db := NewDataBrowserPane("m", testMatrix(), nil)
	if db.itemCount() != 2 {
		t.Errorf("itemCount() = %d, want 2 (rows)", db.itemCount())
	}
}

func TestDataBrowserItemCountVector(t *testing.T) {
	db := NewDataBrowserPane("v", testVector(), nil)
	if db.itemCount() != 3 {
		t.Errorf("itemCount() = %d, want 3", db.itemCount())
	}
}

func TestDataBrowserColCount(t *testing.T) {
	db := NewDataBrowserPane("m", testMatrix(), nil)
	if db.colCount() != 3 {
		t.Errorf("colCount() = %d, want 3", db.colCount())
	}
	// Non-matrix has 0 columns
	db2 := NewDataBrowserPane("ns", testNamespace(), nil)
	if db2.colCount() != 0 {
		t.Errorf("colCount() on namespace = %d, want 0", db2.colCount())
	}
}

// --- Navigation: up/down/home/end ---

func TestDataBrowserUpDown(t *testing.T) {
	db := NewDataBrowserPane("ns", testNamespace(), nil)

	// Start at 0
	if db.selected != 0 {
		t.Fatalf("initial selected = %d, want 0", db.selected)
	}

	// Down
	db.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	if db.selected != 1 {
		t.Errorf("after down: selected = %d, want 1", db.selected)
	}

	// Down again
	db.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	if db.selected != 2 {
		t.Errorf("after down×2: selected = %d, want 2", db.selected)
	}

	// Up
	db.HandleKey(tea.KeyMsg{Type: tea.KeyUp})
	if db.selected != 1 {
		t.Errorf("after up: selected = %d, want 1", db.selected)
	}

	// Home
	db.HandleKey(tea.KeyMsg{Type: tea.KeyHome})
	if db.selected != 0 {
		t.Errorf("after home: selected = %d, want 0", db.selected)
	}

	// End
	db.HandleKey(tea.KeyMsg{Type: tea.KeyEnd})
	if db.selected != 3 {
		t.Errorf("after end: selected = %d, want 3", db.selected)
	}
}

func TestDataBrowserUpDownBounds(t *testing.T) {
	db := NewDataBrowserPane("ns", testNamespace(), nil)

	// Up from 0 stays at 0
	db.HandleKey(tea.KeyMsg{Type: tea.KeyUp})
	if db.selected != 0 {
		t.Errorf("up from 0: selected = %d, want 0", db.selected)
	}

	// Down past end stays at last
	for i := 0; i < 10; i++ {
		db.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	}
	if db.selected != 3 {
		t.Errorf("down past end: selected = %d, want 3", db.selected)
	}
}

// --- Navigation: left/right (matrix columns) ---

func TestDataBrowserLeftRight(t *testing.T) {
	db := NewDataBrowserPane("m", testMatrix(), nil)

	// Start at col 0
	if db.colSel != 0 {
		t.Fatalf("initial colSel = %d, want 0", db.colSel)
	}

	db.HandleKey(tea.KeyMsg{Type: tea.KeyRight})
	if db.colSel != 1 {
		t.Errorf("after right: colSel = %d, want 1", db.colSel)
	}

	db.HandleKey(tea.KeyMsg{Type: tea.KeyRight})
	if db.colSel != 2 {
		t.Errorf("after right×2: colSel = %d, want 2", db.colSel)
	}

	// Right past end stays
	db.HandleKey(tea.KeyMsg{Type: tea.KeyRight})
	if db.colSel != 2 {
		t.Errorf("right past end: colSel = %d, want 2", db.colSel)
	}

	db.HandleKey(tea.KeyMsg{Type: tea.KeyLeft})
	if db.colSel != 1 {
		t.Errorf("after left: colSel = %d, want 1", db.colSel)
	}

	// Left at 0 stays
	db.colSel = 0
	db.HandleKey(tea.KeyMsg{Type: tea.KeyLeft})
	if db.colSel != 0 {
		t.Errorf("left at 0: colSel = %d, want 0", db.colSel)
	}
}

func TestDataBrowserLeftRightNonMatrix(t *testing.T) {
	db := NewDataBrowserPane("ns", testNamespace(), nil)

	// Right on namespace does nothing (no columns)
	db.HandleKey(tea.KeyMsg{Type: tea.KeyRight})
	if db.colSel != 0 {
		t.Errorf("right on namespace: colSel = %d, want 0", db.colSel)
	}
}

// --- Drill in/out ---

func TestDataBrowserDrillInNamespace(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)

	// Drill into "scores" (index 2, a vector)
	db.selected = 2
	ok := db.drillIn()
	if !ok {
		t.Fatal("drillIn on vector should succeed")
	}
	if len(db.stack) != 2 {
		t.Fatalf("stack depth = %d, want 2", len(db.stack))
	}
	if db.selected != 0 {
		t.Errorf("selected after drill-in = %d, want 0", db.selected)
	}
	// Current value should be the scores vector
	if v, ok := db.currentValue().([]any); !ok || len(v) != 3 {
		t.Errorf("currentValue after drill into scores: %T", db.currentValue())
	}
}

func TestDataBrowserDrillInScalarFails(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)

	// "name" (index 0) is a string - can't drill in (should start edit instead via Enter)
	db.selected = 0
	ok := db.drillIn()
	if ok {
		t.Error("drillIn on string scalar should return false")
	}
}

func TestDataBrowserDrillOutRestoresState(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)

	// Select "meta" and drill in
	db.selected = 3
	db.scroll = 1 // simulate scrolled state
	db.drillIn()

	// Move to second item inside meta
	db.selected = 1

	// Drill out
	db.drillOut()

	if db.selected != 3 {
		t.Errorf("selected after drill-out = %d, want 3", db.selected)
	}
	if db.scroll != 1 {
		t.Errorf("scroll after drill-out = %d, want 1", db.scroll)
	}
}

func TestDataBrowserDrillOutAtRootCallsClose(t *testing.T) {
	closed := false
	db := NewDataBrowserPane("data", testNamespace(), func() { closed = true })

	db.drillOut()
	if !closed {
		t.Error("drillOut at root should call onClose")
	}
}

func TestDataBrowserDrillInMatrix(t *testing.T) {
	// Matrix with a nested namespace in a cell
	inner := &codec.Namespace{
		Keys:   []string{"a"},
		Values: map[string]any{"a": 1},
	}
	m := &codec.Array{
		Data:  []any{[]any{inner, 2}, []any{3, 4}},
		Shape: []int{2, 2},
	}
	db := NewDataBrowserPane("m", m, nil)

	// Drill into [1;1] (the namespace cell)
	db.selected = 0
	db.colSel = 0
	ok := db.drillIn()
	if !ok {
		t.Fatal("drillIn on namespace cell should succeed")
	}
	if db.Title() != "m > [1;1]" {
		t.Errorf("Title() = %q, want %q", db.Title(), "m > [1;1]")
	}
}

func TestDataBrowserDrillInVector(t *testing.T) {
	db := NewDataBrowserPane("v", testVector(), nil)

	// Index 2 is a namespace — should drill in
	db.selected = 2
	ok := db.drillIn()
	if !ok {
		t.Fatal("drillIn on vector namespace element should succeed")
	}
	if db.Title() != "v > [3]" {
		t.Errorf("Title() = %q, want %q", db.Title(), "v > [3]")
	}
}

// --- Editing ---

func TestDataBrowserStartEditScalar(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)

	// "age" is index 1, an int
	db.selected = 1
	ok := db.startEdit()
	if !ok {
		t.Fatal("startEdit on int should succeed")
	}
	if !db.editing {
		t.Error("should be in editing mode")
	}
	if string(db.editBuf) != "42" {
		t.Errorf("editBuf = %q, want %q", string(db.editBuf), "42")
	}
}

func TestDataBrowserStartEditString(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)

	// "name" is index 0, a string
	db.selected = 0
	ok := db.startEdit()
	if !ok {
		t.Fatal("startEdit on string should succeed")
	}
	// Strings show raw value (no quotes) for editing
	if string(db.editBuf) != "Alice" {
		t.Errorf("editBuf = %q, want %q", string(db.editBuf), "Alice")
	}
}

func TestDataBrowserStartEditCompound(t *testing.T) {
	// startEdit now works on compound cells too — buffer is the APLAN
	// serialization, ready for editing as APLAN. Drill-in is preserved by
	// the Enter handler in HandleKey, which routes compounds to drillIn.
	db := NewDataBrowserPane("data", testNamespace(), nil)

	db.selected = 2 // "scores" is a vector
	if ok := db.startEdit(); !ok {
		t.Fatal("startEdit on compound should succeed (APLAN buffer)")
	}
	if !db.editing {
		t.Error("expected editing=true")
	}
	if len(db.editBuf) == 0 {
		t.Error("editBuf should be prefilled with APLAN serialization")
	}
}

func TestDataBrowserConfirmEditInt(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)

	db.selected = 1 // age = 42
	db.startEdit()
	db.editBuf = []rune("99")
	db.editCursor = 2
	db.confirmEdit()

	if db.editing {
		t.Error("should no longer be editing")
	}
	if !db.modified {
		t.Error("should be marked modified")
	}

	// Check value was updated
	ns := db.root.(*codec.Namespace)
	if ns.Values["age"] != 99 {
		t.Errorf("age = %v, want 99", ns.Values["age"])
	}
}

func TestDataBrowserConfirmEditInvalidType(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)

	db.selected = 1 // age = 42 (int)
	db.startEdit()
	db.editBuf = []rune("not a number")
	db.editCursor = 12
	db.confirmEdit()

	// Should still be editing with error
	if !db.editing {
		t.Error("should still be editing after invalid input")
	}
	if db.editErr == "" {
		t.Error("should have error message")
	}
}

func TestDataBrowserCancelEdit(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)

	db.selected = 1
	db.startEdit()
	db.editBuf = []rune("99")
	db.cancelEdit()

	if db.editing {
		t.Error("should not be editing after cancel")
	}
	if db.modified {
		t.Error("should not be modified after cancel")
	}

	// Value unchanged
	ns := db.root.(*codec.Namespace)
	if ns.Values["age"] != 42 {
		t.Errorf("age = %v, want 42 (unchanged)", ns.Values["age"])
	}
}

func TestDataBrowserEditViaKeyboard(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)

	// Enter on scalar starts edit
	db.selected = 1 // age = 42
	db.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !db.editing {
		t.Fatal("Enter on scalar should start editing")
	}

	// Esc cancels
	db.HandleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if db.editing {
		t.Error("Esc should cancel editing")
	}
}

func TestDataBrowserEditEnterOnCompoundDrillsIn(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)

	// Enter on "scores" (vector) should drill in, not edit
	db.selected = 2
	db.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if db.editing {
		t.Error("Enter on compound should not start editing")
	}
	if len(db.stack) != 2 {
		t.Errorf("stack depth = %d, want 2 (drilled in)", len(db.stack))
	}
}

// --- Edit key handling ---

func TestDataBrowserEditKeyBackspace(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)
	db.selected = 1
	db.startEdit() // editBuf = "42", cursor at end (2)

	db.handleEditKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if string(db.editBuf) != "4" {
		t.Errorf("after backspace: editBuf = %q, want %q", string(db.editBuf), "4")
	}
	if db.editCursor != 1 {
		t.Errorf("after backspace: cursor = %d, want 1", db.editCursor)
	}
}

func TestDataBrowserEditKeyLeftRight(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)
	db.selected = 1
	db.startEdit() // cursor at end (2)

	db.handleEditKey(tea.KeyMsg{Type: tea.KeyLeft})
	if db.editCursor != 1 {
		t.Errorf("after left: cursor = %d, want 1", db.editCursor)
	}

	db.handleEditKey(tea.KeyMsg{Type: tea.KeyHome})
	if db.editCursor != 0 {
		t.Errorf("after home: cursor = %d, want 0", db.editCursor)
	}

	db.handleEditKey(tea.KeyMsg{Type: tea.KeyEnd})
	if db.editCursor != 2 {
		t.Errorf("after end: cursor = %d, want 2", db.editCursor)
	}
}

func TestDataBrowserEditKeyRunes(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)
	db.selected = 1
	db.startEdit() // "42"

	// Move to start, insert "1"
	db.editCursor = 0
	db.handleEditKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if string(db.editBuf) != "142" {
		t.Errorf("after insert: editBuf = %q, want %q", string(db.editBuf), "142")
	}
	if db.editCursor != 1 {
		t.Errorf("after insert: cursor = %d, want 1", db.editCursor)
	}
}

// --- convertToType ---

func TestConvertToTypeInt(t *testing.T) {
	tests := []struct {
		text    string
		want    int
		wantErr bool
	}{
		{"42", 42, false},
		{"¯5", -5, false},
		{"0", 0, false},
		{"abc", 0, true}, // not parseable as anything
		// Note: "3.14" no longer fails — APLAN fallback parses it as float64
		// (see TestConvertToTypeAPLANFallback). Type-promotion is intentional
		// so users can widen scalars without leaving edit mode.
	}
	for _, tt := range tests {
		got, err := convertToType(tt.text, int(0))
		if tt.wantErr {
			if err == nil {
				t.Errorf("convertToType(%q, int) should fail", tt.text)
			}
			continue
		}
		if err != nil {
			t.Errorf("convertToType(%q, int) error: %v", tt.text, err)
			continue
		}
		if got != tt.want {
			t.Errorf("convertToType(%q, int) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestConvertToTypeFloat(t *testing.T) {
	got, err := convertToType("3.14", float64(0))
	if err != nil {
		t.Fatal(err)
	}
	if got != 3.14 {
		t.Errorf("got %v, want 3.14", got)
	}

	got, err = convertToType("¯2.5", float64(0))
	if err != nil {
		t.Fatal(err)
	}
	if got != -2.5 {
		t.Errorf("got %v, want -2.5", got)
	}
}

func TestConvertToTypeComplex(t *testing.T) {
	got, err := convertToType("3J4", complex128(0))
	if err != nil {
		t.Fatal(err)
	}
	if got != complex(3, 4) {
		t.Errorf("got %v, want 3+4i", got)
	}

	// Lowercase j
	got, err = convertToType("¯1j2", complex128(0))
	if err != nil {
		t.Fatal(err)
	}
	if got != complex(-1, 2) {
		t.Errorf("got %v, want -1+2i", got)
	}

	// Real only → complex with zero imaginary
	got, err = convertToType("5", complex128(0))
	if err != nil {
		t.Fatal(err)
	}
	if got != complex(5, 0) {
		t.Errorf("got %v, want 5+0i", got)
	}
}

func TestConvertToTypeString(t *testing.T) {
	got, err := convertToType("hello world", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello world" {
		t.Errorf("got %v, want %q", got, "hello world")
	}
}

// --- Rendering ---

func TestDataBrowserRenderNamespace(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)
	out := db.Render(60, 10)
	if out == "" {
		t.Fatal("Render returned empty")
	}
	// Should contain all keys
	for _, key := range []string{"name", "age", "scores", "meta"} {
		if !strings.Contains(out, key) {
			t.Errorf("namespace render missing key %q", key)
		}
	}
}

func TestDataBrowserRenderMatrix(t *testing.T) {
	db := NewDataBrowserPane("m", testMatrix(), nil)
	out := db.Render(60, 10)
	if out == "" {
		t.Fatal("Render returned empty")
	}
	// Should contain column headers and row data
	if !strings.Contains(out, "[1]") {
		t.Error("matrix render missing [1] header")
	}
	if !strings.Contains(out, "[2]") {
		t.Error("matrix render missing [2] header")
	}
}

func TestDataBrowserRenderVector(t *testing.T) {
	db := NewDataBrowserPane("v", testVector(), nil)
	out := db.Render(60, 10)
	if out == "" {
		t.Fatal("Render returned empty")
	}
	if !strings.Contains(out, "[1]") {
		t.Error("vector render missing [1] index")
	}
	if !strings.Contains(out, "42") {
		t.Error("vector render missing value 42")
	}
}

func TestDataBrowserRenderScalar(t *testing.T) {
	db := NewDataBrowserPane("x", 42, nil)
	out := db.Render(40, 5)
	if !strings.Contains(out, "42") {
		t.Errorf("scalar render missing value: %q", out)
	}
}

func TestDataBrowserRenderEmpty(t *testing.T) {
	// Empty namespace
	ns := &codec.Namespace{Values: map[string]any{}}
	db := NewDataBrowserPane("empty", ns, nil)
	out := db.Render(40, 5)
	if !strings.Contains(out, "empty namespace") {
		t.Errorf("empty namespace render: %q", out)
	}

	// Empty vector
	db2 := NewDataBrowserPane("empty", []any{}, nil)
	out2 := db2.Render(40, 5)
	if !strings.Contains(out2, "empty vector") {
		t.Errorf("empty vector render: %q", out2)
	}
}

func TestDataBrowserRenderZeroSize(t *testing.T) {
	db := NewDataBrowserPane("x", 42, nil)
	if db.Render(0, 0) != "" {
		t.Error("Render(0,0) should return empty")
	}
	if db.Render(40, 0) != "" {
		t.Error("Render(40,0) should return empty")
	}
}

// --- Cell previews ---

func TestCellPreviewScalars(t *testing.T) {
	db := NewDataBrowserPane("x", 0, nil)

	if got := db.cellPreview(42); got != "42" {
		t.Errorf("cellPreview(42) = %q", got)
	}
	if got := db.cellPreview("hi"); got != "'hi'" {
		t.Errorf("cellPreview(\"hi\") = %q", got)
	}
}

func TestCellPreviewCompound(t *testing.T) {
	db := NewDataBrowserPane("x", 0, nil)

	ns := &codec.Namespace{
		Keys:   []string{"a"},
		Values: map[string]any{"a": 1},
	}
	got := db.cellPreview(ns)
	if !strings.HasPrefix(got, "# ") {
		t.Errorf("namespace preview should start with '#': %q", got)
	}

	m := &codec.Array{Data: []any{[]any{1, 2}}, Shape: []int{1, 2}}
	got = db.cellPreview(m)
	if !strings.HasPrefix(got, "⊞ ") {
		t.Errorf("matrix preview should start with '⊞': %q", got)
	}

	vec := []any{1, 2, 3}
	got = db.cellPreview(vec)
	if !strings.HasPrefix(got, "≡ ") {
		t.Errorf("vector preview should start with '≡': %q", got)
	}
}

// --- Helpers ---

func TestTruncRunes(t *testing.T) {
	if got := truncRunes("hello", 10); got != "hello" {
		t.Errorf("no truncation: %q", got)
	}
	if got := truncRunes("hello", 4); got != "hel…" {
		t.Errorf("truncated: %q", got)
	}
	if got := truncRunes("hello", 1); got != "h" {
		t.Errorf("width 1: %q", got)
	}
	if got := truncRunes("", 5); got != "" {
		t.Errorf("empty: %q", got)
	}
}

func TestPadRuneRight(t *testing.T) {
	if got := padRuneRight("hi", 5); got != "hi   " {
		t.Errorf("padRuneRight(%q, 5) = %q", "hi", got)
	}
	if got := padRuneRight("hello", 3); got != "hello" {
		t.Errorf("no pad when wider: %q", got)
	}
	// Rune-safe: APL chars
	if got := padRuneRight("⍳", 3); got != "⍳  " {
		t.Errorf("APL char: padRuneRight(%q, 3) = %q", "⍳", got)
	}
}

// --- Matrix cell access ---

func TestGetMatrixCell(t *testing.T) {
	m := testMatrix()
	db := NewDataBrowserPane("m", m, nil)

	// [0,0] = 1, [0,2] = 3, [1,1] = 5
	if got := db.getMatrixCell(m, 0, 0); got != 1 {
		t.Errorf("[0,0] = %v, want 1", got)
	}
	if got := db.getMatrixCell(m, 0, 2); got != 3 {
		t.Errorf("[0,2] = %v, want 3", got)
	}
	if got := db.getMatrixCell(m, 1, 1); got != 5 {
		t.Errorf("[1,1] = %v, want 5", got)
	}
}

func TestSetMatrixCell(t *testing.T) {
	m := testMatrix()
	db := NewDataBrowserPane("m", m, nil)

	db.setMatrixCell(m, 0, 1, 99)
	if got := db.getMatrixCell(m, 0, 1); got != 99 {
		t.Errorf("after set [0,1] = %v, want 99", got)
	}
}

// --- Selected value ---

func TestSelectedValueNamespace(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)

	db.selected = 0
	if got := db.selectedValue(); got != "Alice" {
		t.Errorf("selected[0] = %v, want %q", got, "Alice")
	}

	db.selected = 1
	if got := db.selectedValue(); got != 42 {
		t.Errorf("selected[1] = %v, want 42", got)
	}
}

func TestDataBrowserDownAppendsRow2D(t *testing.T) {
	m := testMatrix() // 2×3 of ints
	db := withAppendRowBinding(NewDataBrowserPane("m", m, nil))

	// Move to last row
	db.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	if db.selected != 1 {
		t.Fatalf("setup: selected = %d, want 1", db.selected)
	}

	// Down on last row should append a new row
	db.HandleKey(tea.KeyMsg{Type: tea.KeyDown})

	if m.Shape[0] != 3 {
		t.Errorf("after append: rows = %d, want 3", m.Shape[0])
	}
	if db.selected != 2 {
		t.Errorf("after append: selected = %d, want 2", db.selected)
	}
	if !db.modified {
		t.Error("after append: modified should be true")
	}

	// New row should have 3 zero-valued int cells
	row, ok := m.Data[2].([]any)
	if !ok {
		t.Fatalf("new row is not []any: %T", m.Data[2])
	}
	if len(row) != 3 {
		t.Fatalf("new row len = %d, want 3", len(row))
	}
	for i, v := range row {
		if v != 0 {
			t.Errorf("new row[%d] = %v (%T), want 0 (int)", i, v, v)
		}
	}
}

func TestDataBrowserDownAppendsRow1D(t *testing.T) {
	m := &codec.Array{Data: []any{10, 20}, Shape: []int{2}}
	db := withAppendRowBinding(NewDataBrowserPane("v", m, nil))

	db.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	if db.selected != 1 {
		t.Fatalf("setup: selected = %d, want 1", db.selected)
	}

	db.HandleKey(tea.KeyMsg{Type: tea.KeyDown})

	if m.Shape[0] != 3 {
		t.Errorf("after append: rows = %d, want 3", m.Shape[0])
	}
	if db.selected != 2 {
		t.Errorf("after append: selected = %d, want 2", db.selected)
	}
	if m.Data[2] != 0 {
		t.Errorf("new row[0] = %v (%T), want 0 (int)", m.Data[2], m.Data[2])
	}
}

func TestDataBrowserDownAppendsRowVectorAny(t *testing.T) {
	// Plain []any vector — what `)ed x` produces for `x←1 2 3 4`.
	v := []any{1, 2, 3, 4}
	db := withAppendRowBinding(NewDataBrowserPane("x", v, nil))
	db.selected = 3 // last element

	db.HandleKey(tea.KeyMsg{Type: tea.KeyDown})

	// The pane stores the (possibly reallocated) slice on stack[0].value.
	got, ok := db.currentValue().([]any)
	if !ok {
		t.Fatalf("currentValue is not []any: %T", db.currentValue())
	}
	if len(got) != 5 {
		t.Errorf("len = %d, want 5", len(got))
	}
	if db.selected != 4 {
		t.Errorf("selected = %d, want 4", db.selected)
	}
	if got[4] != 0 {
		t.Errorf("got[4] = %v (%T), want 0 (int)", got[4], got[4])
	}
	if !db.modified {
		t.Error("modified should be true")
	}

	// CRITICAL: the save path serializes db.root, so the appended row
	// MUST be visible there too — not just in the stack.
	rootSlice, ok := db.root.([]any)
	if !ok {
		t.Fatalf("db.root is not []any: %T", db.root)
	}
	if len(rootSlice) != 5 {
		t.Errorf("db.root len = %d, want 5 (otherwise the new row is lost on save)", len(rootSlice))
	}
}

func TestDataBrowserVectorAnyMultipleAppends(t *testing.T) {
	// Two appends with an edit in between — db.root must reflect both
	// appends and the edited value (the bug: db.root could go stale because
	// []any append may reallocate).
	v := []any{1, 2}
	db := withAppendRowBinding(NewDataBrowserPane("x", v, nil))
	db.selected = 1

	db.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	if got := db.currentValue().([]any); len(got) != 3 {
		t.Fatalf("after first append: len = %d, want 3", len(got))
	}

	if !db.startEdit() {
		t.Fatal("startEdit failed on appended cell")
	}
	db.editBuf = []rune("9")
	db.editCursor = 1
	db.confirmEdit()

	db.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	got, ok := db.currentValue().([]any)
	if !ok {
		t.Fatalf("currentValue is not []any: %T", db.currentValue())
	}
	if len(got) != 4 {
		t.Errorf("after second append: len = %d, want 4", len(got))
	}
	rootSlice, _ := db.root.([]any)
	if len(rootSlice) != 4 {
		t.Errorf("db.root len = %d, want 4", len(rootSlice))
	}
	if rootSlice[2] != 9 {
		t.Errorf("db.root[2] = %v, want 9 (the edited value)", rootSlice[2])
	}
}

func TestDataBrowserDownPreservesColumnTypes(t *testing.T) {
	// Mixed-type columns: int, float64, string
	m := &codec.Array{
		Data:  []any{[]any{1, 1.5, "a"}},
		Shape: []int{1, 3},
	}
	db := withAppendRowBinding(NewDataBrowserPane("m", m, nil))

	db.HandleKey(tea.KeyMsg{Type: tea.KeyDown})

	row := m.Data[1].([]any)
	if _, ok := row[0].(int); !ok {
		t.Errorf("col 0 type = %T, want int", row[0])
	}
	if f, ok := row[1].(float64); !ok || f != 0.0 {
		t.Errorf("col 1 = %v (%T), want 0.0 (float64)", row[1], row[1])
	}
	if s, ok := row[2].(string); !ok || s != "" {
		t.Errorf("col 2 = %v (%T), want \"\" (string)", row[2], row[2])
	}
}

func TestDataBrowserDownNoAppendOnNamespace(t *testing.T) {
	db := NewDataBrowserPane("ns", testNamespace(), nil)

	// Move to last item
	for i := 0; i < 10; i++ {
		db.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	}

	if db.selected != 3 {
		t.Errorf("namespace selected = %d, want 3 (no append)", db.selected)
	}
	if db.modified {
		t.Error("namespace modified should remain false")
	}
}

func TestDataBrowserDownAppendsRepeatedly(t *testing.T) {
	// User explicitly wanted unconstrained append: 3 Downs → 3 new rows.
	m := testMatrix() // 2×3
	db := withAppendRowBinding(NewDataBrowserPane("m", m, nil))
	db.selected = 1

	for i := 0; i < 3; i++ {
		db.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.Shape[0] != 5 {
		t.Errorf("after 3 downs: rows = %d, want 5", m.Shape[0])
	}
}

func TestDataBrowserDownNoAppendMidMatrix(t *testing.T) {
	// 3-row matrix, cursor on row 0
	m := &codec.Array{
		Data:  []any{[]any{1, 2}, []any{3, 4}, []any{5, 6}},
		Shape: []int{3, 2},
	}
	db := NewDataBrowserPane("m", m, nil)

	db.HandleKey(tea.KeyMsg{Type: tea.KeyDown}) // → row 1
	if m.Shape[0] != 3 {
		t.Errorf("mid-matrix down extended unexpectedly: rows = %d", m.Shape[0])
	}
	if db.modified {
		t.Error("mid-matrix down should not set modified")
	}
}

// --- Append column ---

func TestDataBrowserAppendColumn2D(t *testing.T) {
	m := testMatrix() // 2×3
	db := withMutationBindings(NewDataBrowserPane("m", m, nil))
	db.colSel = 2 // last column

	// Right when at last column → append a new column.
	db.HandleKey(tea.KeyMsg{Type: tea.KeyRight})

	if m.Shape[1] != 4 {
		t.Errorf("cols = %d, want 4", m.Shape[1])
	}
	if db.colSel != 3 {
		t.Errorf("colSel = %d, want 3", db.colSel)
	}
	if !db.modified {
		t.Error("modified should be true")
	}
	for r := 0; r < m.Shape[0]; r++ {
		row := m.Data[r].([]any)
		if len(row) != 4 {
			t.Errorf("row %d len = %d, want 4", r, len(row))
		}
		if row[3] != 0 {
			t.Errorf("row[%d][3] = %v, want 0", r, row[3])
		}
	}
}

func TestDataBrowserAppendColumnRepeats(t *testing.T) {
	// Unconstrained: Right three times grows from 3 cols to 6.
	m := &codec.Array{Data: []any{[]any{1, 2, 3}}, Shape: []int{1, 3}}
	db := withMutationBindings(NewDataBrowserPane("m", m, nil))
	db.colSel = 2

	for i := 0; i < 3; i++ {
		db.HandleKey(tea.KeyMsg{Type: tea.KeyRight})
	}
	if m.Shape[1] != 6 {
		t.Errorf("cols = %d, want 6", m.Shape[1])
	}
}

func TestDataBrowserAppendColumnNoOpVector(t *testing.T) {
	v := []any{1, 2, 3}
	db := withMutationBindings(NewDataBrowserPane("v", v, nil))
	db.selected = 0

	db.HandleKey(tea.KeyMsg{Type: tea.KeyRight})

	got := db.currentValue().([]any)
	if len(got) != 3 {
		t.Errorf("vector should be untouched: len = %d, want 3", len(got))
	}
	if db.modified {
		t.Error("modified should remain false on vector")
	}
}

// --- Delete row ---

func TestDataBrowserDeleteRow2D(t *testing.T) {
	m := &codec.Array{
		Data:  []any{[]any{1, 2}, []any{3, 4}, []any{5, 6}},
		Shape: []int{3, 2},
	}
	db := withMutationBindings(NewDataBrowserPane("m", m, nil))
	db.selected = 1 // delete middle row

	db.HandleKey(tea.KeyMsg{Type: tea.KeyCtrlD})

	if m.Shape[0] != 2 {
		t.Errorf("rows = %d, want 2", m.Shape[0])
	}
	// Row 0 = [1,2], row 1 was [3,4] (deleted), row 2 = [5,6] becomes new row 1.
	row1 := m.Data[1].([]any)
	if row1[0] != 5 || row1[1] != 6 {
		t.Errorf("after delete: data[1] = %v, want [5 6]", row1)
	}
	if !db.modified {
		t.Error("modified should be true")
	}
}

func TestDataBrowserDeleteRowAdjustsCursor(t *testing.T) {
	// Deleting the last row must move selected back so it's in range.
	m := &codec.Array{
		Data:  []any{[]any{1}, []any{2}, []any{3}},
		Shape: []int{3, 1},
	}
	db := withMutationBindings(NewDataBrowserPane("m", m, nil))
	db.selected = 2

	db.HandleKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	if m.Shape[0] != 2 {
		t.Fatalf("rows = %d, want 2", m.Shape[0])
	}
	if db.selected != 1 {
		t.Errorf("selected = %d, want 1 (was at last, must back off)", db.selected)
	}
}

func TestDataBrowserDeleteRowVector(t *testing.T) {
	v := []any{10, 20, 30, 40}
	db := withMutationBindings(NewDataBrowserPane("v", v, nil))
	db.selected = 1

	db.HandleKey(tea.KeyMsg{Type: tea.KeyCtrlD})

	got := db.currentValue().([]any)
	if len(got) != 3 {
		t.Errorf("len = %d, want 3", len(got))
	}
	if got[0] != 10 || got[1] != 30 || got[2] != 40 {
		t.Errorf("got = %v, want [10 30 40]", got)
	}
	rootSlice := db.root.([]any)
	if len(rootSlice) != 3 {
		t.Errorf("db.root len = %d, want 3 (otherwise delete is lost on save)", len(rootSlice))
	}
}

// --- Delete column ---

func TestDataBrowserDeleteColumn(t *testing.T) {
	m := &codec.Array{
		Data:  []any{[]any{1, 2, 3}, []any{4, 5, 6}},
		Shape: []int{2, 3},
	}
	db := withMutationBindings(NewDataBrowserPane("m", m, nil))
	db.colSel = 1 // delete middle column

	db.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}, Alt: true})

	if m.Shape[1] != 2 {
		t.Errorf("cols = %d, want 2", m.Shape[1])
	}
	row0 := m.Data[0].([]any)
	if row0[0] != 1 || row0[1] != 3 {
		t.Errorf("row 0 = %v, want [1 3]", row0)
	}
	row1 := m.Data[1].([]any)
	if row1[0] != 4 || row1[1] != 6 {
		t.Errorf("row 1 = %v, want [4 6]", row1)
	}
}

func TestDataBrowserDeleteColumnNoOpVector(t *testing.T) {
	v := []any{1, 2, 3}
	db := withMutationBindings(NewDataBrowserPane("v", v, nil))

	// Vector — try delete-column. Must not mutate.
	db.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}, Alt: true})

	got := db.currentValue().([]any)
	if len(got) != 3 {
		t.Errorf("vector mutated: len = %d, want 3", len(got))
	}
	if db.modified {
		t.Error("modified should be false on vector")
	}
}

// --- Close discard ---

func TestDataBrowserCloseDiscardSetsFlag(t *testing.T) {
	closed := false
	db := withMutationBindings(NewDataBrowserPane("v", []any{1, 2}, func() { closed = true }))
	db.modified = true // simulate prior edit

	db.HandleKey(tea.KeyMsg{Type: tea.KeyCtrlW})

	if !db.Discard {
		t.Error("Discard should be true after close-discard")
	}
	if !closed {
		t.Error("onClose should have fired")
	}
}

// --- APLAN edit-mode parsing ---

func TestConvertToTypeAPLANFallback(t *testing.T) {
	// Scalar int 0 + APLAN vector input → []any{7,8,9}.
	got, err := convertToType("7 8 9", 0)
	if err != nil {
		t.Fatalf("APLAN fallback failed: %v", err)
	}
	v, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", got)
	}
	if len(v) != 3 || v[0] != 7 || v[1] != 8 || v[2] != 9 {
		t.Errorf("got %v, want [7 8 9]", v)
	}
}

func TestConvertToTypeLiteralWinsForSimpleNumbers(t *testing.T) {
	// "5" should parse as int 5, not as APLAN scalar.
	got, err := convertToType("5", 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n, ok := got.(int); !ok || n != 5 {
		t.Errorf("got %v (%T), want int 5", got, got)
	}
}

func TestConvertToTypeCompoundUsesAPLAN(t *testing.T) {
	// Original is a vector; user typed `(1 2)` → parsed as APLAN.
	original := []any{0, 0}
	got, err := convertToType("1 2", original)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	v, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", got)
	}
	if len(v) != 2 || v[0] != 1 || v[1] != 2 {
		t.Errorf("got %v, want [1 2]", v)
	}
}

// --- Recursive zeroValueFor ---

func TestZeroValueForVectorPreservesShape(t *testing.T) {
	got := zeroValueFor([]any{1, 2.5, "x"})
	v, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", got)
	}
	if len(v) != 3 {
		t.Fatalf("len = %d, want 3", len(v))
	}
	if v[0] != 0 {
		t.Errorf("v[0] = %v, want 0 (int)", v[0])
	}
	if v[1] != 0.0 {
		t.Errorf("v[1] = %v, want 0.0", v[1])
	}
	if v[2] != "" {
		t.Errorf("v[2] = %v, want \"\"", v[2])
	}
}

func TestAppendRowAfterCompoundUsesRecursiveZero(t *testing.T) {
	// `(1 2 3) (4 5 6)` → append → new row should be `(0 0 0)`, not `0`.
	v := []any{[]any{1, 2, 3}, []any{4, 5, 6}}
	db := withMutationBindings(NewDataBrowserPane("x", v, nil))
	db.selected = 1

	db.HandleKey(tea.KeyMsg{Type: tea.KeyDown})

	got := db.currentValue().([]any)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	newRow, ok := got[2].([]any)
	if !ok {
		t.Fatalf("appended row is %T, want []any (recursive zero)", got[2])
	}
	if len(newRow) != 3 {
		t.Errorf("new row len = %d, want 3", len(newRow))
	}
	for i, x := range newRow {
		if x != 0 {
			t.Errorf("newRow[%d] = %v, want 0", i, x)
		}
	}
}

func TestSelectedValueMatrix(t *testing.T) {
	db := NewDataBrowserPane("m", testMatrix(), nil)

	db.selected = 0
	db.colSel = 2
	if got := db.selectedValue(); got != 3 {
		t.Errorf("selected[0,2] = %v, want 3", got)
	}
}

func TestSelectedValueVector(t *testing.T) {
	db := NewDataBrowserPane("v", testVector(), nil)

	db.selected = 0
	if got := db.selectedValue(); got != 42 {
		t.Errorf("selected[0] = %v, want 42", got)
	}

	db.selected = 1
	if got := db.selectedValue(); got != "hello" {
		t.Errorf("selected[1] = %v, want %q", got, "hello")
	}
}

// --- Full drill-edit-save cycle ---

func TestDataBrowserDrillEditSave(t *testing.T) {
	ns := testNamespace()
	db := NewDataBrowserPane("data", ns, nil)

	// Drill into "meta" namespace
	db.selected = 3
	db.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if len(db.stack) != 2 {
		t.Fatalf("didn't drill into meta, stack=%d", len(db.stack))
	}

	// Edit "version" (first key, value=2)
	db.selected = 0
	db.HandleKey(tea.KeyMsg{Type: tea.KeyEnter}) // start edit
	if !db.editing {
		t.Fatal("should be editing version")
	}

	// Clear and type "3"
	db.editBuf = []rune("3")
	db.editCursor = 1
	db.HandleKey(tea.KeyMsg{Type: tea.KeyEnter}) // confirm

	if db.editing {
		t.Error("should no longer be editing")
	}
	if !db.modified {
		t.Error("should be modified")
	}

	// Drill out and verify root was mutated
	db.HandleKey(tea.KeyMsg{Type: tea.KeyEscape})
	meta := ns.Values["meta"].(*codec.Namespace)
	if meta.Values["version"] != 3 {
		t.Errorf("version = %v, want 3", meta.Values["version"])
	}
}

// --- Esc/Backspace navigation ---

func TestDataBrowserEscPopsThenCloses(t *testing.T) {
	closed := false
	db := NewDataBrowserPane("data", testNamespace(), func() { closed = true })

	// Drill into "meta"
	db.selected = 3
	db.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})

	// Esc pops stack
	db.HandleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if len(db.stack) != 1 {
		t.Fatalf("stack after first esc = %d, want 1", len(db.stack))
	}
	if closed {
		t.Fatal("should not have closed yet")
	}

	// Esc at root closes
	db.HandleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if !closed {
		t.Error("second esc at root should close")
	}
}

func TestDataBrowserBackspaceWorks(t *testing.T) {
	closed := false
	db := NewDataBrowserPane("data", testNamespace(), func() { closed = true })

	// Drill in
	db.selected = 3
	db.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})

	// Backspace pops
	db.HandleKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if len(db.stack) != 1 {
		t.Fatalf("stack after backspace = %d, want 1", len(db.stack))
	}

	// Backspace at root closes
	db.HandleKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if !closed {
		t.Error("backspace at root should close")
	}
}

// --- Mouse ---

func TestDataBrowserMouseSelect(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)

	db.HandleMouse(0, 2, tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if db.selected != 2 {
		t.Errorf("after mouse click at y=2: selected = %d, want 2", db.selected)
	}
}

func TestDataBrowserMouseOutOfRange(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)

	// Click way past end — should clamp to last valid
	db.HandleMouse(0, 100, tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	// selected stays at previous since target >= count
	if db.selected != 0 {
		t.Errorf("mouse out of range: selected = %d, want 0", db.selected)
	}
}

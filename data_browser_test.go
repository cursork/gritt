package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cursork/gritt/codec"
)

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

func TestDataBrowserStartEditCompoundFails(t *testing.T) {
	db := NewDataBrowserPane("data", testNamespace(), nil)

	// "scores" is index 2, a vector — can't edit inline
	db.selected = 2
	ok := db.startEdit()
	if ok {
		t.Error("startEdit on vector should return false")
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
		{"abc", 0, true},
		{"3.14", 0, true},
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

package main

import (
	"reflect"
	"testing"
)

func TestParseHeader(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantSig   string
		wantLocal []string
		wantCom   string
	}{
		{"niladic", "MyFn", "MyFn", nil, ""},
		{"monadic", "MyFn x", "MyFn x", nil, ""},
		{"dyadic", "x MyFn y", "x MyFn y", nil, ""},
		{"with result", "r←MyFn x", "r←MyFn x", nil, ""},
		{"with locals", "r←MyFn x;a;b;c", "r←MyFn x", []string{"a", "b", "c"}, ""},
		{"shy result", "{r}←MyFn x;a", "{r}←MyFn x", []string{"a"}, ""},
		{"trailing comment", "MyFn;a;b ⍝ some comment", "MyFn", []string{"a", "b"}, "⍝ some comment"},
		{"whitespace in locals", "MyFn ; a ; b", "MyFn", []string{"a", "b"}, ""},
		{"no locals just name", "Z", "Z", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig, locals, com := parseHeader(tt.header)
			if sig != tt.wantSig {
				t.Errorf("signature = %q, want %q", sig, tt.wantSig)
			}
			if !reflect.DeepEqual(locals, tt.wantLocal) {
				t.Errorf("locals = %v, want %v", locals, tt.wantLocal)
			}
			if com != tt.wantCom {
				t.Errorf("comment = %q, want %q", com, tt.wantCom)
			}
		})
	}
}

func TestHeaderVars(t *testing.T) {
	tests := []struct {
		name    string
		sig     string
		fnName  string
		want    []string
	}{
		{"niladic", "MyFn", "MyFn", nil},
		{"monadic", "MyFn x", "MyFn", []string{"x"}},
		{"dyadic", "a MyFn b", "MyFn", []string{"a", "b"}},
		{"with result", "r←MyFn x", "MyFn", []string{"r", "x"}},
		{"shy result", "{r}←MyFn x", "MyFn", []string{"r", "x"}},
		{"destructuring result", "(a b)←MyFn x", "MyFn", []string{"a", "b", "x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := headerVars(tt.sig, tt.fnName)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("headerVars(%q, %q) = %v, want %v", tt.sig, tt.fnName, got, tt.want)
			}
		})
	}
}

func TestFindAssignedVars(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  []string
	}{
		{
			"simple assignment",
			[]string{"  x←42"},
			[]string{"x"},
		},
		{
			"multiple assignments",
			[]string{"  x←1", "  y←2", "  z←x+y"},
			[]string{"x", "y", "z"},
		},
		{
			"destructuring",
			[]string{"  (a b c)←⍳3"},
			[]string{"a", "b", "c"},
		},
		{
			"skip comments",
			[]string{"⍝ x←1", "  y←2"},
			[]string{"y"},
		},
		{
			"skip inline comment",
			[]string{"  y←2 ⍝ x←1"},
			[]string{"y"},
		},
		{
			"skip system vars",
			[]string{"  ⎕IO←0", "  x←1"},
			[]string{"x"},
		},
		{
			"skip namespace member",
			[]string{"  ns.x←1", "  y←2"},
			[]string{"y"},
		},
		{
			"for loop variable",
			[]string{"  :For item :In list", "    x←item", "  :EndFor"},
			[]string{"item", "x"},
		},
		{
			"dedup",
			[]string{"  x←1", "  x←2"},
			[]string{"x"},
		},
		{
			"empty lines",
			[]string{"", "  x←1", ""},
			[]string{"x"},
		},
		{
			"no assignments",
			[]string{"  r"},
			nil,
		},
		{
			"string containing assignment",
			[]string{"  s←'x←1'"},
			[]string{"s"},
		},
		{
			"modified assignment",
			[]string{"  x+←1", "  y×←2"},
			[]string{"x", "y"},
		},
		{
			"chained assignment",
			[]string{"  x←y←z←1"},
			[]string{"x", "y", "z"},
		},
		{
			"indexed assignment",
			[]string{"  a[i]←5"},
			// 'i' is inside brackets, not before ←. 'a' has ] between it and ←.
			// The scanner will pick up what's directly before ←, which here is ]
			// This is fine — we don't need to localise 'a' from indexed assignment
			// because the user should localise 'a' from the initial assignment a←...
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findAssignedVars(tt.lines)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findAssignedVars() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseGlobalsComment(t *testing.T) {
	tests := []struct {
		name      string
		lines     []string
		wantNames []string
		wantIdx   int
	}{
		{
			"present",
			[]string{"header", "⍝ GLOBALS: g1 g2 g3", "  x←1"},
			[]string{"g1", "g2", "g3"},
			1,
		},
		{
			"absent",
			[]string{"header", "  x←1"},
			nil,
			-1,
		},
		{
			"case insensitive",
			[]string{"header", "⍝ globals: foo bar"},
			[]string{"foo", "bar"},
			1,
		},
		{
			"with leading whitespace",
			[]string{"header", "  ⍝  GLOBALS:  a  b"},
			[]string{"a", "b"},
			1,
		},
		{
			"empty globals",
			[]string{"header", "⍝ GLOBALS:"},
			nil,
			1,
		},
		{
			"not on first body line",
			[]string{"header", "  x←1", "⍝ GLOBALS: g1", "  y←2"},
			[]string{"g1"},
			2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names, idx := parseGlobalsComment(tt.lines)
			if !reflect.DeepEqual(names, tt.wantNames) {
				t.Errorf("globals = %v, want %v", names, tt.wantNames)
			}
			if idx != tt.wantIdx {
				t.Errorf("lineIdx = %d, want %d", idx, tt.wantIdx)
			}
		})
	}
}

func TestAutolocalise(t *testing.T) {
	tests := []struct {
		name   string
		text   []string
		fnName string
		want   []string
	}{
		{
			"adds new locals",
			[]string{"MyFn x", "  a←1", "  b←2"},
			"MyFn",
			[]string{"MyFn x;a;b", "  a←1", "  b←2"},
		},
		{
			"preserves existing locals",
			[]string{"MyFn x;a", "  a←1", "  b←2", "  c←3"},
			"MyFn",
			[]string{"MyFn x;a;b;c", "  a←1", "  b←2", "  c←3"},
		},
		{
			"preserves existing order",
			[]string{"MyFn;z;m", "  z←1", "  m←2", "  a←3"},
			"MyFn",
			[]string{"MyFn;z;m;a", "  z←1", "  m←2", "  a←3"},
		},
		{
			"excludes globals",
			[]string{"MyFn", "⍝ GLOBALS: g1", "  a←1", "  g1←2"},
			"MyFn",
			[]string{"MyFn;a", "⍝ GLOBALS: g1", "  a←1", "  g1←2"},
		},
		{
			"excludes signature vars",
			[]string{"r←MyFn x", "  r←x+1", "  a←2"},
			"MyFn",
			[]string{"r←MyFn x;a", "  r←x+1", "  a←2"},
		},
		{
			"no change when all localised",
			[]string{"MyFn;a;b", "  a←1", "  b←2"},
			"MyFn",
			[]string{"MyFn;a;b", "  a←1", "  b←2"},
		},
		{
			"no change when no assignments",
			[]string{"MyFn x", "  x+1"},
			"MyFn",
			[]string{"MyFn x", "  x+1"},
		},
		{
			"preserves trailing comment",
			[]string{"MyFn ⍝ does stuff", "  a←1"},
			"MyFn",
			[]string{"MyFn;a ⍝ does stuff", "  a←1"},
		},
		{
			"empty text",
			[]string{},
			"MyFn",
			[]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := autolocaliseText(tt.text, tt.fnName)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("autolocaliseText():\n  got  %v\n  want %v", got, tt.want)
			}
		})
	}
}

func TestLocalise(t *testing.T) {
	tests := []struct {
		name   string
		text   []string
		fnName string
		want   []string
	}{
		{
			"removes stale locals",
			[]string{"MyFn;a;b;c", "  a←1"},
			"MyFn",
			[]string{"MyFn;a", "  a←1"},
		},
		{
			"adds missing and removes stale",
			[]string{"MyFn;old", "  x←1", "  y←2"},
			"MyFn",
			[]string{"MyFn;x;y", "  x←1", "  y←2"},
		},
		{
			"respects globals",
			[]string{"MyFn;a;g", "⍝ GLOBALS: g", "  a←1", "  g←2"},
			"MyFn",
			[]string{"MyFn;a", "⍝ GLOBALS: g", "  a←1", "  g←2"},
		},
		{
			"no change when clean",
			[]string{"MyFn;a;b", "  a←1", "  b←2"},
			"MyFn",
			[]string{"MyFn;a;b", "  a←1", "  b←2"},
		},
		{
			"preserves order of kept locals",
			[]string{"MyFn;z;a", "  z←1", "  a←2", "  n←3"},
			"MyFn",
			[]string{"MyFn;z;a;n", "  z←1", "  a←2", "  n←3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := localiseText(tt.text, tt.fnName)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("localiseText():\n  got  %v\n  want %v", got, tt.want)
			}
		})
	}
}

func TestToggleLocal(t *testing.T) {
	tests := []struct {
		name          string
		text          []string
		fnName        string
		varName       string
		createGlobals bool
		wantHeader    string   // expected line 0
		wantLines     []string // if non-nil, check full text
	}{
		{
			name: "add to empty",
			text: []string{"MyFn", "  x←1"}, fnName: "MyFn", varName: "x",
			wantHeader: "MyFn;x",
		},
		{
			name: "add sorted",
			text: []string{"MyFn;a;c", "  b←1"}, fnName: "MyFn", varName: "b",
			wantHeader: "MyFn;a;b;c",
		},
		{
			name: "remove existing",
			text: []string{"MyFn;a;b;c", "  a←1"}, fnName: "MyFn", varName: "b",
			wantHeader: "MyFn;a;c",
		},
		{
			name: "remove last",
			text: []string{"MyFn;x", "  x←1"}, fnName: "MyFn", varName: "x",
			wantHeader: "MyFn",
		},
		{
			name: "preserves comment",
			text: []string{"MyFn;a ⍝ comment", "  b←1"}, fnName: "MyFn", varName: "b",
			wantHeader: "MyFn;a;b ⍝ comment",
		},
		// GLOBALS management: createGlobals=true (autolocalise on)
		{
			name: "remove with createGlobals creates GLOBALS",
			text: []string{"MyFn;x;y", "  x←1", "  y←2"}, fnName: "MyFn", varName: "x",
			createGlobals: true,
			wantLines:     []string{"MyFn;y", "⍝ GLOBALS: x", "  x←1", "  y←2"},
		},
		{
			name: "remove with createGlobals adds to existing GLOBALS",
			text: []string{"MyFn;x;y", "⍝ GLOBALS: g1", "  x←1"}, fnName: "MyFn", varName: "x",
			createGlobals: true,
			wantLines:     []string{"MyFn;y", "⍝ GLOBALS: g1 x", "  x←1"},
		},
		// GLOBALS management: createGlobals=false (autolocalise off)
		{
			name: "remove without createGlobals no GLOBALS comment",
			text: []string{"MyFn;x;y", "  x←1"}, fnName: "MyFn", varName: "x",
			createGlobals: false,
			wantLines:     []string{"MyFn;y", "  x←1"}, // no GLOBALS created
		},
		{
			name: "remove without createGlobals existing GLOBALS",
			text: []string{"MyFn;x;y", "⍝ GLOBALS: g1", "  x←1"}, fnName: "MyFn", varName: "x",
			createGlobals: false,
			wantLines:     []string{"MyFn;y", "⍝ GLOBALS: g1 x", "  x←1"},
		},
		// Adding a local removes from GLOBALS
		{
			name: "add removes from GLOBALS",
			text: []string{"MyFn", "⍝ GLOBALS: x y", "  x←1"}, fnName: "MyFn", varName: "x",
			wantLines: []string{"MyFn;x", "⍝ GLOBALS: y", "  x←1"},
		},
		{
			name: "add removes last from GLOBALS keeps empty line",
			text: []string{"MyFn", "⍝ GLOBALS: x", "  x←1"}, fnName: "MyFn", varName: "x",
			wantLines: []string{"MyFn;x", "⍝ GLOBALS:", "  x←1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toggleLocal(tt.text, tt.fnName, tt.varName, tt.createGlobals)
			if tt.wantLines != nil {
				if !reflect.DeepEqual(got, tt.wantLines) {
					t.Errorf("text:\n  got  %v\n  want %v", got, tt.wantLines)
				}
			} else if got[0] != tt.wantHeader {
				t.Errorf("header = %q, want %q", got[0], tt.wantHeader)
			}
		})
	}
}

func TestStripComment(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"  x←1 ⍝ comment", "  x←1 "},
		{"⍝ all comment", ""},
		{"  x←'hello'", "  x←'hello'"},
		{"  x←'has ⍝ in string'", "  x←'has ⍝ in string'"},
		{"no comment", "no comment"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripComment(tt.input)
			if got != tt.want {
				t.Errorf("stripComment(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

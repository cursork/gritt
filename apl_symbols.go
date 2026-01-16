package main

// APL symbol mappings for backtick input
// Based on Dyalog's default keyboard layout

var backtickMap = map[rune]rune{
	// Greek letters
	'a': '⍺', // alpha
	'w': '⍵', // omega
	'A': '⍶', // alpha underbar
	'W': '⍹', // omega underbar

	// Arrows
	'z': '⊂', // left shoe / enclose
	'x': '⊃', // right shoe / disclose/pick
	'c': '∩', // cap / intersection
	'v': '∪', // cup / union

	// Operators
	'e': '∊', // epsilon / membership
	'E': '⍷', // epsilon underbar / find
	'r': '⍴', // rho / shape
	't': '~', // tilde / not
	'T': '⍨', // tilde diaeresis / commute
	'y': '↑', // up arrow / take/mix
	'u': '↓', // down arrow / drop/split
	'i': '⍳', // iota / index generator
	'I': '⍸', // iota underbar / where
	'o': '○', // circle / pi times
	'O': '⍥', // circle diaeresis / over
	'p': '*', // star / power
	'P': '⍣', // star diaeresis / power operator

	// Functions
	's': '⌈', // upstile / ceiling/max
	'd': '⌊', // downstile / floor/min
	'f': '_', // underscore
	'g': '∇', // del / function definition
	'h': '∆', // delta
	'H': '⍙', // delta underbar
	'j': '∘', // jot / compose
	'J': '⍤', // jot diaeresis / rank
	'k': '\'', // quote
	'l': '⎕', // quad
	'L': '⌷', // squad / index

	// More functions
	'q': '?', // question mark / roll/deal
	'Q': '⌹', // domino / matrix inverse/divide

	// Brackets and misc
	'[': '←', // left arrow / assignment
	']': '→', // right arrow / branch
	'=': '÷', // divide
	'-': '×', // times
	'/': '⌿', // slash bar / replicate first
	'.': '⍀', // slope bar / expand first
	',': '⍝', // lamp / comment
	'\'': '⍕', // thorn / format
	';': '⍎', // hydrant / execute

	// Numbers row
	'1': '¨', // diaeresis / each
	'2': '¯', // macron / high minus
	'3': '<', // less than
	'4': '≤', // less than or equal
	'5': '=', // equal
	'6': '≥', // greater than or equal
	'7': '>', // greater than
	'8': '≠', // not equal
	'9': '∨', // or
	'0': '∧', // and

	// Shifted numbers
	'!': '⌶', // i-beam
	'@': '⍫', // del tilde
	'#': '⍒', // grade down
	'$': '⍋', // grade up
	'%': '⌽', // circle stile / reverse/rotate
	'^': '⍉', // circle backslash / transpose
	'&': '⊖', // circle bar / rotate first
	'`': '⋄', // diamond (backtick-backtick)

	// Additional useful ones
	'n': '⊤', // down tack / encode
	'm': '|', // stile / magnitude/residue
	'b': '⊥', // up tack / decode
	'K': '⌸', // quad equal / key
	'|': '⊣', // left tack
	'\\': '⊢', // right tack
}

// APLSymbol holds information about an APL symbol for search
type APLSymbol struct {
	Char    rune
	Names   []string // Multiple names for searching
	Desc    string   // Short description
	Keycode string   // Backtick code if any
}

// APL symbols with searchable names
var aplSymbols = []APLSymbol{
	{'⍳', []string{"iota", "index", "generator", "integers"}, "Index generator / Index of", "`i"},
	{'⍴', []string{"rho", "shape", "reshape"}, "Shape / Reshape", "`r"},
	{'⍺', []string{"alpha", "left", "argument"}, "Left argument", "`a"},
	{'⍵', []string{"omega", "right", "argument"}, "Right argument", "`w"},
	{'←', []string{"assign", "assignment", "gets", "arrow"}, "Assignment", "`["},
	{'→', []string{"branch", "goto", "right arrow"}, "Branch", "`]"},
	{'∊', []string{"epsilon", "member", "membership", "in", "enlist"}, "Membership / Enlist", "`e"},
	{'⍷', []string{"find", "epsilon underbar"}, "Find", "`E"},
	{'⍸', []string{"where", "iota underbar", "interval index"}, "Where / Interval index", "`I"},
	{'↑', []string{"take", "mix", "up arrow", "uparrow"}, "Take / Mix", "`y"},
	{'↓', []string{"drop", "split", "down arrow", "downarrow"}, "Drop / Split", "`u"},
	{'⊂', []string{"enclose", "left shoe", "partitioned enclose"}, "Enclose / Partitioned enclose", "`z"},
	{'⊃', []string{"disclose", "pick", "right shoe", "first"}, "Disclose / Pick", "`x"},
	{'∩', []string{"intersection", "cap"}, "Intersection", "`c"},
	{'∪', []string{"union", "cup", "unique"}, "Union / Unique", "`v"},
	{'⌈', []string{"ceiling", "max", "maximum", "upstile"}, "Ceiling / Maximum", "`s"},
	{'⌊', []string{"floor", "min", "minimum", "downstile"}, "Floor / Minimum", "`d"},
	{'×', []string{"times", "multiply", "signum", "sign"}, "Times / Signum", "`="},
	{'÷', []string{"divide", "division", "reciprocal"}, "Divide / Reciprocal", "`-"},
	{'*', []string{"power", "star", "exponential"}, "Power / Exponential", "`p"},
	{'⍟', []string{"log", "logarithm", "circle star"}, "Logarithm", ""},
	{'○', []string{"circle", "pi", "trig", "trigonometric"}, "Pi times / Trig functions", "`o"},
	{'!', []string{"factorial", "binomial", "bang"}, "Factorial / Binomial", ""},
	{'?', []string{"roll", "deal", "random", "question"}, "Roll / Deal", "`q"},
	{'∼', []string{"not", "tilde", "without"}, "Not / Without", "`t"},
	{'∧', []string{"and", "lcm", "wedge"}, "And / LCM", "`0"},
	{'∨', []string{"or", "gcd", "vee"}, "Or / GCD", "`9"},
	{'⍲', []string{"nand"}, "Nand", "`N"},
	{'⍱', []string{"nor"}, "Nor", "`M"},
	{'<', []string{"less", "less than", "lt"}, "Less than", "`3"},
	{'≤', []string{"less equal", "leq", "le"}, "Less than or equal", "`4"},
	{'=', []string{"equal", "equals", "eq"}, "Equal", "`5"},
	{'≥', []string{"greater equal", "geq", "ge"}, "Greater than or equal", "`6"},
	{'>', []string{"greater", "greater than", "gt"}, "Greater than", "`7"},
	{'≠', []string{"not equal", "neq", "ne", "unique mask"}, "Not equal / Unique mask", "`8"},
	{'≡', []string{"match", "identical", "depth"}, "Match / Depth", ""},
	{'≢', []string{"not match", "tally", "count"}, "Not match / Tally", ""},
	{'⊣', []string{"left", "left tack", "lev"}, "Left / Same", "`b"},
	{'⊢', []string{"right", "right tack", "dex"}, "Right / Same", "`B"},
	{'⊥', []string{"decode", "base", "up tack"}, "Decode / Base value", "`n"},
	{'⊤', []string{"encode", "representation", "down tack"}, "Encode / Representation", "`m"},
	{'⌽', []string{"reverse", "rotate", "circle stile"}, "Reverse / Rotate", "`%"},
	{'⍉', []string{"transpose", "circle backslash"}, "Transpose", "`^"},
	{'⊖', []string{"rotate first", "circle bar"}, "Rotate first axis", "`&"},
	{'⍋', []string{"grade up", "upgrade", "sort ascending"}, "Grade up", "`$"},
	{'⍒', []string{"grade down", "downgrade", "sort descending"}, "Grade down", "`#"},
	{'⍎', []string{"execute", "eval", "hydrant"}, "Execute", "`."},
	{'⍕', []string{"format", "thorn"}, "Format", "`,"},
	{'⎕', []string{"quad", "input", "output"}, "Quad (system)", "`l"},
	{'⍞', []string{"quote quad", "character input"}, "Quote-quad (char I/O)", ""},
	{'⌷', []string{"index", "squad", "materialise"}, "Index / Materialise", "`L"},
	{'⌹', []string{"domino", "matrix inverse", "matrix divide"}, "Matrix inverse/divide", "`Q"},
	{'∇', []string{"del", "nabla", "function"}, "Function definition", "`g"},
	{'∆', []string{"delta", "triangle"}, "Delta (name char)", "`h"},
	{'⋄', []string{"diamond", "statement", "separator"}, "Statement separator", "`;"},
	{'¨', []string{"each", "diaeresis"}, "Each (operator)", "`1"},
	{'⍨', []string{"commute", "selfie", "tilde diaeresis"}, "Commute / Selfie", "`T"},
	{'⍣', []string{"power operator", "repeat", "star diaeresis"}, "Power operator", "`P"},
	{'∘', []string{"compose", "jot", "beside"}, "Compose / Bind", "`j"},
	{'⍤', []string{"rank", "jot diaeresis", "atop"}, "Rank / Atop", "`J"},
	{'⍥', []string{"over", "circle diaeresis"}, "Over", "`O"},
	{'@', []string{"at", "amend"}, "At (operator)", ""},
	{'⌸', []string{"key", "quad equal"}, "Key (operator)", "`'"},
	{'⌿', []string{"replicate first", "slash bar"}, "Replicate first", "`/"},
	{'⍀', []string{"expand first", "slope bar"}, "Expand first", "`\\"},
	{'¯', []string{"macron", "negative", "high minus"}, "Negative number sign", "`2"},
	{'⍶', []string{"alpha underbar"}, "Alpha underbar", "`A"},
	{'⍹', []string{"omega underbar"}, "Omega underbar", "`W"},
	{'⍙', []string{"delta underbar"}, "Delta underbar", "`H"},
	{'⌶', []string{"i-beam", "ibeam"}, "I-beam (system)", "`!"},
}

package prepl

import _ "embed"

// Source is the APL prepl server namespace source code.
//
//go:embed Prepl.apln
var Source string

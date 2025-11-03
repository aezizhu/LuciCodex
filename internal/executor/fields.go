package executor

import "unicode"

// fieldsSafe splits on any unicode space without interpreting quotes or escapes.
// This is used for splitting elevate command safely without shell parsing.
func fieldsSafe(s string) []string {
    var out []string
    field := make([]rune, 0, len(s))
    flush := func() {
        if len(field) > 0 {
            out = append(out, string(field))
            field = field[:0]
        }
    }
    for _, r := range s {
        if unicode.IsSpace(r) {
            flush()
            continue
        }
        field = append(field, r)
    }
    flush()
    return out
}



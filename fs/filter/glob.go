// rsync style glob parser

package filter

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/rclone/rclone/fs"
)

// GlobPathToRegexp converts an rsync style glob path to a regexp
func GlobPathToRegexp(glob string, ignoreCase bool) (*regexp.Regexp, error) {
	return globToRegexp(glob, true, true, ignoreCase)
}

// GlobStringToRegexp converts an rsync style glob string to a regexp
//
// Without adding of anchors but with ignoring of case, i.e. called
// `GlobStringToRegexp(glob, false, true)`, it takes a lenient approach
// where the glob "sum" would match "CheckSum", more similar to text
// search functions than strict glob filtering.
//
// With adding of anchors and not ignoring case, i.e. called
// `GlobStringToRegexp(glob, true, false)`, it uses a strict glob
// interpretation where the previous example would have to be changed to
// "*Sum" to match "CheckSum".
func GlobStringToRegexp(glob string, addAnchors bool, ignoreCase bool) (*regexp.Regexp, error) {
	return globToRegexp(glob, false, addAnchors, ignoreCase)
}

// globToRegexp converts an rsync style glob to a regexp
//
// Set pathMode true for matching of path/file names, e.g.
// special treatment of path separator `/` and double asterisk `**`,
// see filtering.md for details.
//
// Set addAnchors true to add start of string `^` and end of string `$` anchors.
func globToRegexp(glob string, pathMode bool, addAnchors bool, ignoreCase bool) (*regexp.Regexp, error) {
	var re bytes.Buffer
	if ignoreCase {
		_, _ = re.WriteString("(?i)")
	}
	if addAnchors {
		if pathMode {
			if strings.HasPrefix(glob, "/") {
				glob = glob[1:]
				_ = re.WriteByte('^')
			} else {
				_, _ = re.WriteString("(^|/)")
			}
		} else {
			_, _ = re.WriteString("^")
		}
	}
	consecutiveStars := 0
	insertStars := func() error {
		if consecutiveStars > 0 {
			if pathMode {
				switch consecutiveStars {
				case 1:
					_, _ = re.WriteString(`[^/]*`)
				case 2:
					_, _ = re.WriteString(`.*`)
				default:
					return fmt.Errorf("too many stars in %q", glob)
				}
			} else {
				switch consecutiveStars {
				case 1:
					_, _ = re.WriteString(`.*`)
				default:
					return fmt.Errorf("too many stars in %q", glob)
				}
			}
		}
		consecutiveStars = 0
		return nil
	}
	overwriteLastChar := func(c byte) {
		buf := re.Bytes()
		buf[len(buf)-1] = c
	}
	inBraces := false
	inBrackets := 0
	slashed := false
	inRegexp := false    // inside {{ ... }}
	inRegexpEnd := false // have received }} waiting for more
	var next, last rune
	for _, c := range glob {
		next, last = c, next
		if slashed {
			_, _ = re.WriteRune(c)
			slashed = false
			continue
		}
		if inRegexpEnd {
			if c == '}' {
				// Regexp is ending with }} choose longest segment
				// Replace final ) with }
				overwriteLastChar('}')
				_ = re.WriteByte(')')
				continue
			} else {
				inRegexpEnd = false
			}
		}
		if inRegexp {
			if c == '}' && last == '}' {
				inRegexp = false
				inRegexpEnd = true
				// Replace final } with )
				overwriteLastChar(')')
			} else {
				_, _ = re.WriteRune(c)
			}
			continue
		}
		if c != '*' {
			err := insertStars()
			if err != nil {
				return nil, err
			}
		}
		if inBrackets > 0 {
			_, _ = re.WriteRune(c)
			if c == '[' {
				inBrackets++
			}
			if c == ']' {
				inBrackets--
			}
			continue
		}
		switch c {
		case '\\':
			_, _ = re.WriteRune(c)
			slashed = true
		case '*':
			consecutiveStars++
		case '?':
			if pathMode {
				_, _ = re.WriteString(`[^/]`)
			} else {
				_, _ = re.WriteString(`.`)
			}
		case '[':
			_, _ = re.WriteRune(c)
			inBrackets++
		case ']':
			return nil, fmt.Errorf("mismatched ']' in glob %q", glob)
		case '{':
			if inBraces {
				if last == '{' {
					inRegexp = true
					inBraces = false
				} else {
					return nil, fmt.Errorf("can't nest '{' '}' in glob %q", glob)
				}
			} else {
				inBraces = true
				_ = re.WriteByte('(')
			}
		case '}':
			if !inBraces {
				return nil, fmt.Errorf("mismatched '{' and '}' in glob %q", glob)
			}
			_ = re.WriteByte(')')
			inBraces = false
		case ',':
			if inBraces {
				_ = re.WriteByte('|')
			} else {
				_, _ = re.WriteRune(c)
			}
		case '.', '+', '(', ')', '|', '^', '$': // regexp meta characters not dealt with above
			_ = re.WriteByte('\\')
			_, _ = re.WriteRune(c)
		default:
			_, _ = re.WriteRune(c)
		}
	}
	err := insertStars()
	if err != nil {
		return nil, err
	}
	if inBrackets > 0 {
		return nil, fmt.Errorf("mismatched '[' and ']' in glob %q", glob)
	}
	if inBraces {
		return nil, fmt.Errorf("mismatched '{' and '}' in glob %q", glob)
	}
	if inRegexp {
		return nil, fmt.Errorf("mismatched '{{' and '}}' in glob %q", glob)
	}
	if addAnchors {
		_ = re.WriteByte('$')
	}
	result, err := regexp.Compile(re.String())
	if err != nil {
		return nil, fmt.Errorf("bad glob pattern %q (regexp %q): %w", glob, re.String(), err)
	}
	return result, nil
}

// maxGlobPrefixes is the maximum number of prefixes GlobPrefixes will
// expand before giving up. This avoids pathological cases like
// /[a-zA-Z0-9][a-zA-Z0-9]** which would expand to thousands.
const maxGlobPrefixes = 256

// GlobPrefixes extracts all possible literal prefixes from a
// root-anchored glob pattern (one starting with "/").
//
// It walks the glob left-to-right, building up every concrete string
// that could match the beginning of a path:
//
//   - literal characters are appended to every prefix
//   - character classes like [0-1] or [a-f] are expanded
//   - alternations like {a,b,c} are expanded
//   - the walk stops at the first wildcard (*, ?, **)
//
// Returns nil if the pattern is not anchored (no leading "/"), if no
// useful prefix can be extracted, or if the expansion exceeds
// maxGlobPrefixes.
func GlobPrefixes(glob string) []string {
	if !strings.HasPrefix(glob, "/") {
		return nil // only anchored patterns are safe to optimise
	}
	glob = glob[1:] // strip the leading /

	prefixes := []string{""}
	i := 0
	for i < len(glob) {
		c := glob[i]
		switch c {
		case '*', '?':
			// Wildcard – stop expanding; return what we have so
			// far (unless every prefix is still empty).
			if len(prefixes) == 1 && prefixes[0] == "" {
				return nil // no useful prefix
			}
			return prefixes

		case '\\':
			// Escaped character – the next byte is literal.
			i++
			if i >= len(glob) {
				// Trailing backslash, malformed but be safe.
				return nil
			}
			for j := range prefixes {
				prefixes[j] += string(glob[i])
			}

		case '[':
			// Character class – find the matching ']' and expand.
			end := indexMatchingBracket(glob, i)
			if end < 0 {
				return nil // malformed
			}
			chars := expandCharClass(glob[i+1 : end])
			if chars == nil {
				return nil // negated class or unparseable
			}
			prefixes = multiplyPrefixes(prefixes, chars)
			if len(prefixes) > maxGlobPrefixes {
				return nil
			}
			i = end // will be incremented below

		case '{':
			// Alternation – find matching '}' (not nested) and expand.
			// If this is actually a {{ regexp }}, bail out.
			if i+1 < len(glob) && glob[i+1] == '{' {
				return nil // {{ regexp }} – can't extract prefix
			}
			end := strings.IndexByte(glob[i:], '}')
			if end < 0 {
				return nil // malformed
			}
			end += i // absolute index
			alts := strings.Split(glob[i+1:end], ",")
			prefixes = multiplyPrefixes(prefixes, alts)
			if len(prefixes) > maxGlobPrefixes {
				return nil
			}
			i = end // will be incremented below

		default:
			for j := range prefixes {
				prefixes[j] += string(c)
			}
		}
		i++
	}

	// We consumed the entire glob without hitting a wildcard.
	// The prefixes are exact matches; they are still valid as
	// listing prefixes.
	if len(prefixes) == 1 && prefixes[0] == "" {
		return nil
	}
	return prefixes
}

// indexMatchingBracket returns the index of the ']' that closes the
// character class starting at glob[open] (which must be '['). Returns
// -1 if not found.
func indexMatchingBracket(glob string, open int) int {
	depth := 0
	for j := open; j < len(glob); j++ {
		switch glob[j] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// expandCharClass expands a POSIX-style character class body (the
// text between '[' and ']') into individual single-character strings.
//
// It handles:
//   - individual characters: abc  → ["a","b","c"]
//   - ranges:               a-f  → ["a","b","c","d","e","f"]
//
// Returns nil for negated classes (leading '!' or '^') or classes that
// contain POSIX named classes like [:alpha:].
func expandCharClass(body string) []string {
	if len(body) == 0 {
		return nil
	}
	// Negated class – can't enumerate what it matches.
	if body[0] == '!' || body[0] == '^' {
		return nil
	}
	// Named POSIX class – too complex to expand.
	if strings.Contains(body, "[:") {
		return nil
	}

	var out []string
	seen := make(map[byte]bool)
	addChar := func(c byte) {
		if !seen[c] {
			seen[c] = true
			out = append(out, string(c))
		}
	}

	for i := 0; i < len(body); i++ {
		if body[i] == '\\' && i+1 < len(body) {
			// Escaped character.
			i++
			addChar(body[i])
			continue
		}
		// Check for range: x-y
		if i+2 < len(body) && body[i+1] == '-' && body[i+2] != ']' {
			lo, hi := body[i], body[i+2]
			if lo > hi {
				return nil // invalid range
			}
			for c := lo; c <= hi; c++ {
				addChar(c)
			}
			i += 2
			continue
		}
		addChar(body[i])
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

// multiplyPrefixes returns the cross product of every existing prefix
// with every element in suffixes.
func multiplyPrefixes(prefixes, suffixes []string) []string {
	out := make([]string, 0, len(prefixes)*len(suffixes))
	for _, p := range prefixes {
		for _, s := range suffixes {
			out = append(out, p+s)
		}
	}
	return out
}

var (
	// Can't deal with
	//   / or ** in {}
	//   {{ regexp }}
	tooHardRe = regexp.MustCompile(`({[^{}]*(\*\*|/)[^{}]*})|\{\{|\}\}`)

	// Squash all /
	squashSlash = regexp.MustCompile(`/{2,}`)
)

// globToDirGlobs takes a file glob and turns it into a series of
// directory globs.  When matched with a directory (with a trailing /)
// this should answer the question as to whether this glob could be in
// this directory.
func globToDirGlobs(glob string) (out []string) {
	if tooHardRe.MatchString(glob) {
		// Can't figure this one out so return any directory might match
		fs.Infof(nil, "Can't figure out directory filters from %q: looking in all directories", glob)
		out = append(out, "/**")
		return out
	}

	// Get rid of multiple /s
	glob = squashSlash.ReplaceAllString(glob, "/")

	// Split on / or **
	// (** can contain /)
	for {
		i := strings.LastIndex(glob, "/")
		j := strings.LastIndex(glob, "**")
		what := ""
		if j > i {
			i = j
			what = "**"
		}
		if i < 0 {
			if len(out) == 0 {
				out = append(out, "/**")
			}
			break
		}
		glob = glob[:i]
		newGlob := glob + what + "/"
		if len(out) == 0 || out[len(out)-1] != newGlob {
			out = append(out, newGlob)
		}
	}

	return out
}

package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobStringToRegexp(t *testing.T) {
	for _, test := range []struct {
		in    string
		want  string
		error string
	}{
		{``, ``, ``},
		{`potato`, `potato`, ``},
		{`potato,sausage`, `potato,sausage`, ``},
		{`/potato`, `/potato`, ``},
		{`potato?sausage`, `potato.sausage`, ``},
		{`potat[oa]`, `potat[oa]`, ``},
		{`potat[a-z]or`, `potat[a-z]or`, ``},
		{`potat[[:alpha:]]or`, `potat[[:alpha:]]or`, ``},
		{`'.' '+' '(' ')' '|' '^' '$'`, `'\.' '\+' '\(' '\)' '\|' '\^' '\$'`, ``},
		{`*.jpg`, `.*\.jpg`, ``},
		{`a{b,c,d}e`, `a(b|c|d)e`, ``},
		{`potato**`, ``, `too many stars`},
		{`potato**sausage`, ``, `too many stars`},
		{`*.p[lm]`, `.*\.p[lm]`, ``},
		{`[\[\]]`, `[\[\]]`, ``},
		{`***potato`, ``, `too many stars`},
		{`***`, ``, `too many stars`},
		{`ab]c`, ``, `mismatched ']'`},
		{`ab[c`, ``, `mismatched '[' and ']'`},
		{`ab{x{cd`, ``, `can't nest`},
		{`ab{}}cd`, ``, `mismatched '{' and '}'`},
		{`ab}c`, ``, `mismatched '{' and '}'`},
		{`ab{c`, ``, `mismatched '{' and '}'`},
		{`*.{jpg,png,gif}`, `.*\.(jpg|png|gif)`, ``},
		{`[a--b]`, ``, `bad glob pattern`},
		{`a\*b`, `a\*b`, ``},
		{`a\\b`, `a\\b`, ``},
		{`a{{.*}}b`, `a(.*)b`, ``},
		{`a{{.*}`, ``, `mismatched '{{' and '}}'`},
		{`{{regexp}}`, `(regexp)`, ``},
		{`\{{{regexp}}`, `\{(regexp)`, ``},
		{`/{{regexp}}`, `/(regexp)`, ``},
		{`/{{\d{8}}}`, `/(\d{8})`, ``},
		{`/{{\}}}`, `/(\})`, ``},
		{`{{(?i)regexp}}`, `((?i)regexp)`, ``},
	} {
		for _, ignoreCase := range []bool{false, true} {
			for _, addAnchors := range []bool{false, true} {
				gotRe, err := GlobStringToRegexp(test.in, addAnchors, ignoreCase)
				if test.error == "" {
					require.NoError(t, err, test.in)
					prefix := ""
					suffix := ""
					if ignoreCase {
						prefix += "(?i)"
					}
					if addAnchors {
						prefix += "^"
						suffix += "$"
					}
					got := gotRe.String()
					assert.Equal(t, prefix+test.want+suffix, got, test.in)
				} else {
					require.Error(t, err, test.in)
					assert.Contains(t, err.Error(), test.error, test.in)
					assert.Nil(t, gotRe)
				}
			}
		}
	}
}

func TestGlobPathToRegexp(t *testing.T) {
	for _, test := range []struct {
		in    string
		want  string
		error string
	}{
		{``, `(^|/)$`, ``},
		{`potato`, `(^|/)potato$`, ``},
		{`potato,sausage`, `(^|/)potato,sausage$`, ``},
		{`/potato`, `^potato$`, ``},
		{`potato?sausage`, `(^|/)potato[^/]sausage$`, ``},
		{`potat[oa]`, `(^|/)potat[oa]$`, ``},
		{`potat[a-z]or`, `(^|/)potat[a-z]or$`, ``},
		{`potat[[:alpha:]]or`, `(^|/)potat[[:alpha:]]or$`, ``},
		{`'.' '+' '(' ')' '|' '^' '$'`, `(^|/)'\.' '\+' '\(' '\)' '\|' '\^' '\$'$`, ``},
		{`*.jpg`, `(^|/)[^/]*\.jpg$`, ``},
		{`a{b,c,d}e`, `(^|/)a(b|c|d)e$`, ``},
		{`potato**`, `(^|/)potato.*$`, ``},
		{`potato**sausage`, `(^|/)potato.*sausage$`, ``},
		{`*.p[lm]`, `(^|/)[^/]*\.p[lm]$`, ``},
		{`[\[\]]`, `(^|/)[\[\]]$`, ``},
		{`***potato`, ``, `too many stars`},
		{`***`, ``, `too many stars`},
		{`ab]c`, ``, `mismatched ']'`},
		{`ab[c`, ``, `mismatched '[' and ']'`},
		{`ab{x{cd`, ``, `can't nest`},
		{`ab{}}cd`, ``, `mismatched '{' and '}'`},
		{`ab}c`, ``, `mismatched '{' and '}'`},
		{`ab{c`, ``, `mismatched '{' and '}'`},
		{`*.{jpg,png,gif}`, `(^|/)[^/]*\.(jpg|png|gif)$`, ``},
		{`[a--b]`, ``, `bad glob pattern`},
		{`a\*b`, `(^|/)a\*b$`, ``},
		{`a\\b`, `(^|/)a\\b$`, ``},
		{`a{{.*}}b`, `(^|/)a(.*)b$`, ``},
		{`a{{.*}`, ``, `mismatched '{{' and '}}'`},
		{`{{regexp}}`, `(^|/)(regexp)$`, ``},
		{`\{{{regexp}}`, `(^|/)\{(regexp)$`, ``},
		{`/{{regexp}}`, `^(regexp)$`, ``},
		{`/{{\d{8}}}`, `^(\d{8})$`, ``},
		{`/{{\}}}`, `^(\})$`, ``},
		{`{{(?i)regexp}}`, `(^|/)((?i)regexp)$`, ``},
	} {
		for _, ignoreCase := range []bool{false, true} {
			gotRe, err := GlobPathToRegexp(test.in, ignoreCase)
			if test.error == "" {
				require.NoError(t, err, test.in)
				prefix := ""
				if ignoreCase {
					prefix = "(?i)"
				}
				got := gotRe.String()
				assert.Equal(t, prefix+test.want, got, test.in)
			} else {
				require.Error(t, err, test.in)
				assert.Contains(t, err.Error(), test.error, test.in)
				assert.Nil(t, gotRe)
			}
		}
	}
}

func TestGlobToDirGlobs(t *testing.T) {
	for _, test := range []struct {
		in   string
		want []string
	}{
		{`*`, []string{"/**"}},
		{`/*`, []string{"/"}},
		{`*.jpg`, []string{"/**"}},
		{`/*.jpg`, []string{"/"}},
		{`//*.jpg`, []string{"/"}},
		{`///*.jpg`, []string{"/"}},
		{`/a/*.jpg`, []string{"/a/", "/"}},
		{`/a//*.jpg`, []string{"/a/", "/"}},
		{`/a///*.jpg`, []string{"/a/", "/"}},
		{`/a/b/*.jpg`, []string{"/a/b/", "/a/", "/"}},
		{`a/*.jpg`, []string{"a/"}},
		{`a/b/*.jpg`, []string{"a/b/", "a/"}},
		{`*/*/*.jpg`, []string{"*/*/", "*/"}},
		{`a/b/`, []string{"a/b/", "a/"}},
		{`a/b`, []string{"a/"}},
		{`a/b/*.{jpg,png,gif}`, []string{"a/b/", "a/"}},
		{`/a/{jpg,png,gif}/*.{jpg,png,gif}`, []string{"/a/{jpg,png,gif}/", "/a/", "/"}},
		{`a/{a,a*b,a**c}/d/`, []string{"/**"}},
		{`/a/{a,a*b,a/c,d}/d/`, []string{"/**"}},
		{`/a/{{.*}}/d/`, []string{"/**"}},
		{`**`, []string{"**/"}},
		{`a**`, []string{"a**/"}},
		{`a**b`, []string{"a**/"}},
		{`a**b**c**d`, []string{"a**b**c**/", "a**b**/", "a**/"}},
		{`a**b/c**d`, []string{"a**b/c**/", "a**b/", "a**/"}},
		{`/A/a**b/B/c**d/C/`, []string{"/A/a**b/B/c**d/C/", "/A/a**b/B/c**d/", "/A/a**b/B/c**/", "/A/a**b/B/", "/A/a**b/", "/A/a**/", "/A/", "/"}},
		{`/var/spool/**/ncw`, []string{"/var/spool/**/", "/var/spool/", "/var/", "/"}},
		{`var/spool/**/ncw/`, []string{"var/spool/**/ncw/", "var/spool/**/", "var/spool/", "var/"}},
		{"/file1.jpg", []string{`/`}},
		{"/file2.png", []string{`/`}},
		{"/*.jpg", []string{`/`}},
		{"/*.png", []string{`/`}},
		{"/potato", []string{`/`}},
		{"/sausage1", []string{`/`}},
		{"/sausage2*", []string{`/`}},
		{"/sausage3**", []string{`/sausage3**/`, "/"}},
		{"/a/*.jpg", []string{`/a/`, "/"}},
	} {
		_, err := GlobPathToRegexp(test.in, false)
		assert.NoError(t, err)
		got := globToDirGlobs(test.in)
		assert.Equal(t, test.want, got, test.in)
	}
}

func TestGlobPrefixes(t *testing.T) {
	for _, test := range []struct {
		in   string
		want []string
	}{
		// Non-anchored patterns return nil (not safe to optimise).
		{`[0-1]**`, nil},
		{`**`, nil},
		{`*.jpg`, nil},
		{`abc**`, nil},

		// Anchored wildcard-only patterns have no useful prefix.
		{`/**`, nil},
		{`/*`, nil},
		{`/?`, nil},

		// Simple character class expansions.
		{`/[0-1]**`, []string{"0", "1"}},
		{`/[a-f]**`, []string{"a", "b", "c", "d", "e", "f"}},
		{`/[0-9]**`, []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}},
		{`/[abc]**`, []string{"a", "b", "c"}},

		// Literal prefix before wildcard.
		{`/abc**`, []string{"abc"}},
		{`/abc*`, []string{"abc"}},
		{`/abc?def`, []string{"abc"}},

		// Two character classes multiply together.
		{`/[01][ab]**`, []string{"0a", "0b", "1a", "1b"}},

		// Literal mixed with character class.
		{`/x[0-2]y**`, []string{"x0y", "x1y", "x2y"}},

		// Alternation with braces.
		{`/{a,b,c}**`, []string{"a", "b", "c"}},
		{`/prefix-{x,y}**`, []string{"prefix-x", "prefix-y"}},

		// Escaped characters.
		{`/a\*b**`, []string{"a*b"}},

		// Negated class returns nil (can't enumerate).
		{`/[!a-f]**`, nil},
		{`/[^a-f]**`, nil},

		// POSIX named class returns nil (too complex).
		{`/[[:alpha:]]**`, nil},

		// {{ regexp }} returns nil.
		{`/{{regexp}}**`, nil},

		// Exact match (no wildcard) still produces a prefix.
		{`/exact`, []string{"exact"}},

		// Pattern with a path separator before the wildcard.
		{`/dir/[0-1]**`, []string{"dir/0", "dir/1"}},
	} {
		got := GlobPrefixes(test.in)
		assert.Equal(t, test.want, got, test.in)
	}
}

func TestExpandCharClass(t *testing.T) {
	for _, test := range []struct {
		in   string
		want []string
	}{
		{"abc", []string{"a", "b", "c"}},
		{"a-f", []string{"a", "b", "c", "d", "e", "f"}},
		{"0-9", []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}},
		{"a-c0-2", []string{"a", "b", "c", "0", "1", "2"}},
		// Negated
		{"!a", nil},
		{"^a", nil},
		// Named POSIX class
		{"[:alpha:]", nil},
		// Empty
		{"", nil},
		// Single character
		{"x", []string{"x"}},
		// Duplicate characters
		{"aab", []string{"a", "b"}},
		// Escaped character
		{`\]`, []string{"]"}},
	} {
		got := expandCharClass(test.in)
		assert.Equal(t, test.want, got, test.in)
	}
}

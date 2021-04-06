package searchstring

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
)

// Token is the type of parsed token.
type Token int

const (
	// EOF is the end of file token.
	EOF Token = iota
	// STR is a string token.
	STR
	// STRQ is a quoted string
	STRQ
	// FIELD is a field token.
	FIELD
)

// eof is the EOF rune
var eof = rune(0)

// Scanner is the search string scanner.
type Scanner struct {
	r *bufio.Reader
}

// NewScanner returns a new instance of Scanner.
func NewScanner(input string) *Scanner {
	return &Scanner{r: bufio.NewReader(strings.NewReader(input))}
}

// next reads the next rune in the string.
func (s Scanner) next() rune {
	ch, _, err := s.r.ReadRune()
	if err != nil {
		return eof
	}
	return ch
}

// prev rewinds the scanner position to the previous rune.
func (s Scanner) prev() {
	s.r.UnreadRune()
}

// Scan scans the next token in the string.
func (s Scanner) Scan() (Token, string) {
	for {
		ch := s.next()
		if ch == eof {
			return EOF, ""
		}

		if ch == '"' {
			return STRQ, s.scanQuoted()
		}

		if isSpace(ch) {
			continue
		}

		s.prev()
		return s.scanString()
	}
}

// scanString scans a regular (unquoted) string.
func (s Scanner) scanString() (Token, string) {
	var b bytes.Buffer
	b.WriteRune(s.next())

loop:
	for {
		switch ch := s.next(); {
		case ch == eof:
			break loop
		case isSpace(ch):
			break loop
		case ch == ':':
			return FIELD, b.String()
		default:
			b.WriteRune(ch)
		}
	}

	return STR, b.String()
}

// scanQuoted scans a quoted string.
func (s Scanner) scanQuoted() string {
	// Scan until we find a closing " or EOF
	var b bytes.Buffer

loop:
	for {
		switch ch := s.next(); ch {
		case eof:
			break loop
		case '\\':
			c := s.next()
			if c == '"' {
				b.WriteRune(c)
			} else {
				b.WriteRune('\\')
				b.WriteRune(c)
			}
		case '"':
			break loop
		default:
			b.WriteRune(ch)
		}
	}

	return b.String()

}

// isSpace returns true if the rune is a space.
func isSpace(r rune) bool {
	if r <= '\u00FF' {
		// Obvious ASCII ones: \t through \r plus space. Plus two Latin-1 oddballs.
		switch r {
		case ' ', '\t', '\n', '\v', '\f', '\r':
			return true
		case '\u0085', '\u00A0':
			return true
		}
		return false
	}
	// High-valued ones.
	if '\u2000' <= r && r <= '\u200a' {
		return true
	}
	switch r {
	case '\u1680', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000':
		return true
	}
	return false
}

// SearchTerm is a search term part.
type SearchTerm struct {
	Field  string
	Value  string
	Quotes bool
}

// Parse returns a list of SearchTerm from input string.
func Parse(input string) ([]SearchTerm, error) {
	s := NewScanner(input)
	res := []SearchTerm{}

	var st *SearchTerm
loop:
	for {
		switch tok, value := s.Scan(); tok {
		case EOF:
			break loop
		case FIELD:
			if st != nil {
				return nil, errors.New("field followed by a field")
			}
			st = &SearchTerm{Field: value}
		case STR, STRQ:
			if st == nil {
				st = &SearchTerm{}
			}
			st.Value = value
			st.Quotes = tok == STRQ
			res = append(res, *st)
			st = nil
		}
	}

	return res, nil
}

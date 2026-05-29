package promptfilter

import (
	"fmt"
	"strings"

	"golang.org/x/text/unicode/norm"
)

const (
	MaxDictionaryChars = 20000
	MaxTerms           = 1000
	MaxTermChars       = 128
)

// ParseDictionary converts a newline-separated sensitive-word dictionary into
// normalized terms suitable for matching.
func ParseDictionary(raw string) ([]string, error) {
	if len([]rune(raw)) > MaxDictionaryChars {
		return nil, fmt.Errorf("length must be <= %d", MaxDictionaryChars)
	}

	terms := make([]string, 0)
	seen := make(map[string]struct{})
	for _, line := range strings.Split(raw, "\n") {
		term := Normalize(line)
		if term == "" {
			continue
		}
		if len([]rune(term)) > MaxTermChars {
			return nil, fmt.Errorf("contains a term longer than %d characters", MaxTermChars)
		}
		if _, exists := seen[term]; exists {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
		if len(terms) > MaxTerms {
			return nil, fmt.Errorf("must contain at most %d terms", MaxTerms)
		}
	}
	return terms, nil
}

// Normalize keeps matching predictable across common full-width and casing variants.
func Normalize(raw string) string {
	return strings.ToLower(strings.TrimSpace(norm.NFKC.String(raw)))
}

// Contains reports whether raw contains any already-normalized term.
func Contains(raw string, terms []string) bool {
	if len(terms) == 0 {
		return false
	}
	text := Normalize(raw)
	if text == "" {
		return false
	}
	for _, term := range terms {
		if term != "" && strings.Contains(text, term) {
			return true
		}
	}
	return false
}

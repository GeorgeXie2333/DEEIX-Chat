package promptfilter

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseDictionaryNormalizesLines(t *testing.T) {
	terms, err := ParseDictionary(" Foo \n\nＦｏｏ\r\n敏感词\nfoo\n")
	if err != nil {
		t.Fatalf("ParseDictionary() error = %v", err)
	}
	want := []string{"foo", "敏感词"}
	if len(terms) != len(want) {
		t.Fatalf("terms length = %d, want %d: %#v", len(terms), len(want), terms)
	}
	for index := range want {
		if terms[index] != want[index] {
			t.Fatalf("terms[%d] = %q, want %q", index, terms[index], want[index])
		}
	}
}

func TestContainsMatchesNormalizedSubstrings(t *testing.T) {
	terms, err := ParseDictionary("Block\n敏感词")
	if err != nil {
		t.Fatalf("ParseDictionary() error = %v", err)
	}
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "empty", text: "", want: false},
		{name: "case insensitive", text: "please BLOCK this", want: true},
		{name: "nfkc", text: "please Ｂｌｏｃｋ this", want: true},
		{name: "cjk", text: "这里有敏感词", want: true},
		{name: "miss", text: "ordinary message", want: false},
	}
	for _, tt := range tests {
		if got := Contains(tt.text, terms); got != tt.want {
			t.Fatalf("%s: Contains() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestParseDictionaryRejectsLimits(t *testing.T) {
	if _, err := ParseDictionary(strings.Repeat("x", MaxDictionaryChars+1)); err == nil {
		t.Fatal("expected total dictionary length validation error")
	}
	if _, err := ParseDictionary(strings.Repeat("x", MaxTermChars+1)); err == nil {
		t.Fatal("expected term length validation error")
	}
	lines := make([]string, MaxTerms+1)
	for index := range lines {
		lines[index] = fmt.Sprintf("term%d", index+1)
	}
	if _, err := ParseDictionary(strings.Join(lines, "\n")); err == nil {
		t.Fatal("expected term count validation error")
	}
}

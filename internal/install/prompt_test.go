package install

import (
	"strings"
	"testing"
)

func TestPromptYesNo(t *testing.T) {
	cases := []struct {
		name  string
		input string
		def   bool
		want  bool
	}{
		{"empty defaults true", "\n", true, true},
		{"empty defaults false", "\n", false, false},
		{"explicit yes", "y\n", false, true},
		{"explicit YES", "YES\n", false, true},
		{"explicit no", "n\n", true, false},
		{"garbage then yes", "wat\nyes\n", false, true},
		{"three garbage falls back to def", "a\nb\nc\n", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := PromptYesNo(strings.NewReader(tc.input), tc.def, nil)
			if got != tc.want {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestPromptYesNoCallsPromptFn(t *testing.T) {
	calls := 0
	PromptYesNo(strings.NewReader("garbage\nyes\n"), false, func(int) { calls++ })
	if calls != 2 {
		t.Errorf("promptFn calls = %d want 2", calls)
	}
}

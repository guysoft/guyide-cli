package install

import (
	"bufio"
	"io"
	"strings"
)

// PromptYesNo reads a y/n answer from r. Empty input falls back to def.
// Anything other than y/yes/n/no re-prompts up to 3 times before giving up
// and returning def.
//
// promptFn is invoked once per attempt so the caller can use whatever
// styled prefix it wants (writer.Info, raw fmt, etc.). It receives the
// attempt number (1-indexed) so the caller can phrase the retry.
func PromptYesNo(r io.Reader, def bool, promptFn func(attempt int)) bool {
	br := bufio.NewReader(r)
	for attempt := 1; attempt <= 3; attempt++ {
		if promptFn != nil {
			promptFn(attempt)
		}
		line, err := br.ReadString('\n')
		if err != nil && line == "" {
			return def
		}
		ans := strings.TrimSpace(strings.ToLower(line))
		switch ans {
		case "":
			return def
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
	}
	return def
}

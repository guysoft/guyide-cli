package install

import (
	"errors"
	"os"
	"path/filepath"

	embedfs "github.com/guysoft/guyide-cli/embed"
)

// WelcomePath is the canonical location of the first-launch cheatsheet.
func (p Paths) WelcomePath() string {
	return filepath.Join(p.Root, "WELCOME.md")
}

// WriteWelcome materialises the embedded WELCOME.md at ~/.guyide/WELCOME.md.
//
// It always overwrites: the file is content-addressed by the binary and
// users are not expected to edit it. If they want a personal copy they
// can `cp` it elsewhere.
func WriteWelcome(p Paths) error {
	if err := p.EnsureLayout(); err != nil {
		return err
	}
	body := embedfs.WelcomeMarkdown()
	if len(body) == 0 {
		return errors.New("install: embedded welcome markdown is empty")
	}
	tmp := p.WelcomePath() + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p.WelcomePath())
}

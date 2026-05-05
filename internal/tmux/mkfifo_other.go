//go:build !unix

package tmux

import "errors"

func mkfifo(path string, mode uint32) error {
	return errors.New("mkfifo not supported on this platform")
}

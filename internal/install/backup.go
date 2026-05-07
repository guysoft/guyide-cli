package install

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/guysoft/guyide-cli/pkg/schema"
)

// BackupRequest describes one backup invocation.
type BackupRequest struct {
	// Paths is the list of absolute paths to include. Missing paths
	// are silently skipped (newly-clean installs have nothing to back
	// up). Each path may be a file, directory, or symlink.
	Paths []string

	// Reason is recorded in the manifest entry: install|update|
	// uninstall|manual.
	Reason string

	// Component is the optional component name this backup is for;
	// empty for installer-wide backups.
	Component string

	// HomeDir overrides $HOME for path normalisation in the tarball.
	// Empty = use os.UserHomeDir(). Set in tests.
	HomeDir string
}

// CreateBackup writes a gzipped tarball under ~/.guyide/backups/ and
// returns a BackupEntry the caller should append to the manifest.
//
// The tarball stores paths relative to $HOME so a restore can untar
// straight into a homedir on any machine. If none of the requested
// paths exist, a backup with reason="empty" is still recorded so the
// manifest preserves the run history; the tarball will contain a single
// MANIFEST.txt explaining why it's empty.
func CreateBackup(p Paths, req BackupRequest) (schema.BackupEntry, error) {
	if req.Reason == "" {
		return schema.BackupEntry{}, errors.New("install: backup reason is required")
	}
	if err := p.EnsureLayout(); err != nil {
		return schema.BackupEntry{}, err
	}

	home := req.HomeDir
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return schema.BackupEntry{}, err
		}
		home = h
	}

	// Use RFC3339Nano so rapid successive backups (e.g. test suites)
	// don't collide. The ':' is replaced with '-' so the filename is
	// shell-safe across all our supported shells.
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	stamp := strings.ReplaceAll(ts, ":", "-")
	out := p.BackupAt(stamp)

	f, err := os.Create(out)
	if err != nil {
		return schema.BackupEntry{}, err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	included := 0
	for _, src := range req.Paths {
		n, err := addToTar(tw, src, home)
		if err != nil {
			return schema.BackupEntry{}, fmt.Errorf("backup %s: %w", src, err)
		}
		included += n
	}

	if included == 0 {
		// Always leave a marker so the tarball is non-empty.
		hdr := &tar.Header{
			Name:    "MANIFEST.txt",
			Mode:    0o644,
			Size:    int64(len(emptyMarker)),
			ModTime: time.Now().UTC(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return schema.BackupEntry{}, err
		}
		if _, err := tw.Write([]byte(emptyMarker)); err != nil {
			return schema.BackupEntry{}, err
		}
	}

	return schema.BackupEntry{
		Timestamp: time.Now().UTC(),
		Path:      out,
		Reason:    req.Reason,
		Component: req.Component,
	}, nil
}

const emptyMarker = "guyide backup: nothing to back up at this time.\n"

// addToTar walks src and writes each entry to tw, with paths
// normalised relative to home. Returns the number of entries written.
// A non-existent src is silently skipped (returns 0, nil).
func addToTar(tw *tar.Writer, src, home string) (int, error) {
	info, err := os.Lstat(src)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}

	count := 0
	walk := func(p string, fi fs.FileInfo, lerr error) error {
		if lerr != nil {
			return lerr
		}
		rel, err := relativeToHome(p, home)
		if err != nil {
			return err
		}
		var link string
		if fi.Mode()&fs.ModeSymlink != 0 {
			t, err := os.Readlink(p)
			if err != nil {
				return err
			}
			link = t
		}
		hdr, err := tar.FileInfoHeader(fi, link)
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		count++
		if fi.Mode().IsRegular() {
			f, err := os.Open(p)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}
		return nil
	}

	if info.IsDir() {
		err = filepath.Walk(src, func(p string, fi fs.FileInfo, lerr error) error {
			return walk(p, fi, lerr)
		})
	} else {
		err = walk(src, info, nil)
	}
	return count, err
}

func relativeToHome(p, home string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	homeAbs, err := filepath.Abs(home)
	if err != nil {
		return "", err
	}
	if rel, err := filepath.Rel(homeAbs, abs); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(filepath.Join("home", rel)), nil
	}
	// Outside $HOME: store under "abs" with leading slash stripped.
	return filepath.ToSlash(strings.TrimPrefix(abs, string(os.PathSeparator))), nil
}

// AppendBackup loads, appends, and re-saves the manifest in one step.
// Idempotent: if a BackupEntry with the same Path already exists it
// is left untouched. Returns the resulting manifest.
func AppendBackup(p Paths, entry schema.BackupEntry) (schema.Manifest, error) {
	m, err := LoadManifest(p)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return m, err
	}
	if errors.Is(err, fs.ErrNotExist) {
		// Initialise a fresh manifest.
		m = schema.Manifest{Schema: schema.ManifestSchema}
	}
	for _, b := range m.Backups {
		if b.Path == entry.Path {
			return m, nil
		}
	}
	// Newest first.
	m.Backups = append([]schema.BackupEntry{entry}, m.Backups...)
	if err := SaveManifest(p, m); err != nil {
		return m, err
	}
	return m, nil
}

// ListBackups returns the manifest's backup history newest-first.
// Convenience wrapper for `guyide uninstall --list-backups` and doctor.
func ListBackups(p Paths) ([]schema.BackupEntry, error) {
	m, err := LoadManifest(p)
	if err != nil {
		return nil, err
	}
	return m.Backups, nil
}

package persist

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// BaseDir returns the directory used for persistence. It respects the
// environment variable `NIGHTSHADE_DIR` for tests; otherwise uses
// $HOME/.nightshade
func BaseDir() string {
	if d := os.Getenv("NIGHTSHADE_DIR"); d != "" {
		return d
	}
	home := os.Getenv("HOME")
	return filepath.Join(home, ".nightshade")
}

func ensureDir(p string) error {
	return os.MkdirAll(p, 0o755)
}

func WriteJSON(path string, v interface{}) error {
	// Use atomic writer to avoid partial writes on crash.
	return WriteJSONAtomic(path, v, 0o644)
}

// WriteJSONAtomic marshals v and writes it atomically to path using a
// temporary file followed by rename. It fsyncs the file and its directory
// when possible to reduce risk of corruption.
func WriteJSONAtomic(path string, v interface{}, perm os.FileMode) error {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	// Create/truncate temp file
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	// Write bytes
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	// Sync file
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// Rename into place
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	// Attempt to fsync the directory to ensure rename durability (best-effort)
	dir := filepath.Dir(path)
	if dfd, err := os.Open(dir); err == nil {
		_ = dfd.Sync()
		_ = dfd.Close()
	}
	return nil
}

func ReadJSON(path string, v interface{}) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

package persist

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWriteJSONAtomic_CreatesFileAndNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	want := map[string]string{"hello": "world"}
	if err := WriteJSONAtomic(path, want, 0o644); err != nil {
		t.Fatalf("WriteJSONAtomic error: %v", err)
	}

	// final file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("final file missing: %v", err)
	}

	// tmp file should not remain
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Fatalf("tmp file still exists")
	}

	var got map[string]string
	if err := ReadJSON(path, &got); err != nil {
		t.Fatalf("ReadJSON error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("content mismatch: got=%v want=%v", got, want)
	}
}

func TestWriteJSONAtomic_OverwriteIsClean(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	first := map[string]string{"a": "one"}
	second := map[string]string{"a": "two", "b": "added"}

	if err := WriteJSONAtomic(path, first, 0o644); err != nil {
		t.Fatalf("first write error: %v", err)
	}
	if err := WriteJSONAtomic(path, second, 0o644); err != nil {
		t.Fatalf("second write error: %v", err)
	}

	var got map[string]string
	if err := ReadJSON(path, &got); err != nil {
		t.Fatalf("ReadJSON error: %v", err)
	}
	if !reflect.DeepEqual(got, second) {
		t.Fatalf("after overwrite content mismatch: got=%v want=%v", got, second)
	}
}

func TestWriteJSONAtomic_PartialTempDoesNotCorruptFinal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	want := map[string]string{"x": "y"}

	// Create a stray tmp file with garbage before calling WriteJSONAtomic
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte("not-json"), 0o644); err != nil {
		t.Fatalf("creating stray tmp failed: %v", err)
	}

	if err := WriteJSONAtomic(path, want, 0o644); err != nil {
		t.Fatalf("WriteJSONAtomic error: %v", err)
	}

	// tmp should be gone
	if _, err := os.Stat(tmp); err == nil {
		t.Fatalf("tmp file still exists after atomic write")
	}

	var got map[string]string
	if err := ReadJSON(path, &got); err != nil {
		t.Fatalf("ReadJSON error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("content mismatch after tmp preexists: got=%v want=%v", got, want)
	}
}

func TestWriteJSONAtomic_PermissionsNotMorePermissiveThanRequested(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	want := map[string]string{"p": "q"}
	perm := os.FileMode(0o640)

	if err := WriteJSONAtomic(path, want, perm); err != nil {
		t.Fatalf("WriteJSONAtomic error: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat final file: %v", err)
	}
	gotPerm := fi.Mode().Perm()

	// Ensure file is not more permissive than requested (i.e., no bits set outside perm)
	if gotPerm&^perm != 0 {
		t.Fatalf("file permissions too permissive: got=%o perm=%o", gotPerm, perm)
	}
}

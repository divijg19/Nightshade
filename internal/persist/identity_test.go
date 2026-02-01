package persist

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureIdentityCreatesFilesAndIsStable(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("NIGHTSHADE_DIR", dir)

	pub1, priv1, pubB641, err := EnsureIdentity()
	if err != nil {
		t.Fatalf("EnsureIdentity failed: %v", err)
	}
	// files exist
	pubPath := PublicKeyPath()
	privPath := PrivateKeyPath()
	if _, err := os.Stat(pubPath); err != nil {
		t.Fatalf("public.key not created: %v", err)
	}
	if _, err := os.Stat(privPath); err != nil {
		t.Fatalf("private.key not created: %v", err)
	}

	// public key length must be 32 bytes
	if len(pub1) != 32 {
		t.Fatalf("public key length != 32: %d", len(pub1))
	}

	// base64 encode matches returned pubB64
	if base64.StdEncoding.EncodeToString(pub1) != pubB641 {
		t.Fatalf("pubB64 mismatch")
	}

	// calling EnsureIdentity again should return same values
	pub2, priv2, pubB642, err := EnsureIdentity()
	if err != nil {
		t.Fatalf("EnsureIdentity second call failed: %v", err)
	}
	if pubB641 != pubB642 {
		t.Fatalf("pubB64 not stable across calls")
	}
	if string(pub1) != string(pub2) {
		t.Fatalf("public bytes changed across calls")
	}
	if string(priv1) != string(priv2) {
		t.Fatalf("private bytes changed across calls")
	}

	// Check permissions: private should be owner-read/write (0600) at least
	st, err := os.Stat(privPath)
	if err != nil {
		t.Fatalf("stat private key: %v", err)
	}
	if st.Mode().Perm()&0o600 != 0o600 {
		t.Fatalf("private.key perms not 0600: %v", st.Mode().Perm())
	}

	// public readable (0644) at least owner-read
	st2, err := os.Stat(pubPath)
	if err != nil {
		t.Fatalf("stat public key: %v", err)
	}
	if st2.Mode().Perm()&0o400 != 0o400 {
		t.Fatalf("public.key not owner-readable: %v", st2.Mode().Perm())
	}

	// ensure files are in expected directory
	expectedPriv := filepath.Join(dir, "identity", "private.key")
	if privPath != expectedPriv {
		t.Fatalf("private path unexpected: %s", privPath)
	}
}

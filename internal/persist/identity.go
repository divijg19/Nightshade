package persist

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
)

func identityDir() string {
    return filepath.Join(BaseDir(), "identity")
}

func PrivateKeyPath() string {
    return filepath.Join(identityDir(), "private.key")
}

func PublicKeyPath() string {
    return filepath.Join(identityDir(), "public.key")
}

// EnsureIdentity loads existing keys or generates and writes a new ed25519
// keypair. Returns public key bytes, private key bytes, and the base64
// public key string (AgentID).
func EnsureIdentity() (pub []byte, priv []byte, pubB64 string, err error) {
    dir := identityDir()
    if err := os.MkdirAll(dir, 0o755); err != nil {
        return nil, nil, "", err
    }

    pubPath := PublicKeyPath()
    privPath := PrivateKeyPath()

    // If both files exist, read and return
    if pb, err := os.ReadFile(pubPath); err == nil {
        if sb, err := os.ReadFile(privPath); err == nil {
            return pb, sb, base64.StdEncoding.EncodeToString(pb), nil
        }
    }

    // Otherwise generate new keypair
    pubk, privk, err := ed25519.GenerateKey(rand.Reader)
    if err != nil {
        return nil, nil, "", err
    }

    // Write files
    if err := os.WriteFile(privPath, privk, 0o600); err != nil {
        return nil, nil, "", err
    }
    if err := os.WriteFile(pubPath, pubk, 0o644); err != nil {
        return nil, nil, "", err
    }

    return pubk, privk, base64.StdEncoding.EncodeToString(pubk), nil
}

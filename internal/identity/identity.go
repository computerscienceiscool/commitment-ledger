package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Identity struct {
	Name       string `json:"name"`
	KeyID      string `json:"key_id"`
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

func LoadOrCreate(root string, name string) (Identity, ed25519.PublicKey, ed25519.PrivateKey, error) {
	path := filepath.Join(root, "config", "identities", slug(name)+".json")
	data, err := os.ReadFile(path)
	if err == nil {
		var ident Identity
		if err := json.Unmarshal(data, &ident); err != nil {
			return Identity{}, nil, nil, fmt.Errorf("parse identity %q: %w", path, err)
		}
		pub, err := base64.StdEncoding.DecodeString(ident.PublicKey)
		if err != nil {
			return Identity{}, nil, nil, fmt.Errorf("decode public key: %w", err)
		}
		priv, err := base64.StdEncoding.DecodeString(ident.PrivateKey)
		if err != nil {
			return Identity{}, nil, nil, fmt.Errorf("decode private key: %w", err)
		}
		return ident, ed25519.PublicKey(pub), ed25519.PrivateKey(priv), nil
	}
	if !os.IsNotExist(err) {
		return Identity{}, nil, nil, fmt.Errorf("read identity %q: %w", path, err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Identity{}, nil, nil, fmt.Errorf("generate identity: %w", err)
	}
	ident := Identity{
		Name:       name,
		KeyID:      slug(name) + "-ed25519-v1",
		PublicKey:  base64.StdEncoding.EncodeToString(pub),
		PrivateKey: base64.StdEncoding.EncodeToString(priv),
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Identity{}, nil, nil, fmt.Errorf("mkdir identities: %w", err)
	}
	out, err := json.MarshalIndent(ident, "", "  ")
	if err != nil {
		return Identity{}, nil, nil, fmt.Errorf("marshal identity: %w", err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return Identity{}, nil, nil, fmt.Errorf("write identity %q: %w", path, err)
	}
	return ident, pub, priv, nil
}

func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "anon"
	}
	return b.String()
}

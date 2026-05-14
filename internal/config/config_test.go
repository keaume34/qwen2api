package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplitCSV(t *testing.T) {
	cases := map[string][]string{
		"":             nil,
		"a":            {"a"},
		"a,b, c ,,d,":  {"a", "b", "c", "d"},
		" sk-1 , sk-2": {"sk-1", "sk-2"},
	}
	for in, want := range cases {
		got := splitCSV(in)
		if len(got) != len(want) {
			t.Errorf("splitCSV(%q) len=%d want=%d (%v)", in, len(got), len(want), got)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("splitCSV(%q)[%d]=%q want %q", in, i, got[i], want[i])
			}
		}
	}
}

func TestLoadEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"port":1234,"tokens":[{"value":"t1"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("QWEN2API_CONFIG_PATH", path)
	t.Setenv("QWEN2API_PORT", "9999")
	t.Setenv("QWEN2API_API_KEY", "sk-a,sk-b")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 9999 {
		t.Errorf("port: got %d want 9999", cfg.Port)
	}
	if len(cfg.APIKeys) != 2 {
		t.Errorf("api_keys len: got %d want 2", len(cfg.APIKeys))
	}
	if len(cfg.Tokens) != 1 || cfg.Tokens[0].Value != "t1" {
		t.Errorf("tokens from file lost: %+v", cfg.Tokens)
	}
}

func TestAuthorizedKey(t *testing.T) {
	open := Config{}
	if !open.AuthorizedKey("anything") {
		t.Error("open mode should accept any key")
	}
	closed := Config{APIKeys: []string{"sk-1", "sk-2"}}
	if !closed.AuthorizedKey("sk-1") {
		t.Error("known key rejected")
	}
	if closed.AuthorizedKey("sk-bad") {
		t.Error("unknown key accepted")
	}
	if closed.AuthorizedKey("") {
		t.Error("empty key accepted")
	}
}

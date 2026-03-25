package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadSelectedSubaccount(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg := &Config{SelectedSubaccount: "11111111-1111-1111-1111-111111111111"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.SelectedSubaccount != cfg.SelectedSubaccount {
		t.Fatalf("SelectedSubaccount = %q, want %q", got.SelectedSubaccount, cfg.SelectedSubaccount)
	}
}

func TestSelectedSubaccountUnsetPreservesZeroValue(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	if err := Save(&Config{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.SelectedSubaccount != "" {
		t.Fatalf("SelectedSubaccount = %q, want empty", got.SelectedSubaccount)
	}

	if _, err := os.Stat(filepath.Join(tmp, "qfex", "config.yaml")); err != nil {
		t.Fatalf("config file not written: %v", err)
	}
}

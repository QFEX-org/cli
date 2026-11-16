package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDaemonLogRepairsExistingPermissions(t *testing.T) {
	for _, existing := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "data", "daemon.log")
		if existing {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("old log\n"), 0644); err != nil {
				t.Fatal(err)
			}
		}
		file, err := openDaemonLog(path)
		if err != nil {
			t.Fatal(err)
		}
		file.Close()
		for name, mode := range map[string]os.FileMode{path: 0600, filepath.Dir(path): 0700} {
			info, err := os.Stat(name)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != mode {
				t.Fatalf("%s: got %o, want %o", name, info.Mode().Perm(), mode)
			}
		}
		if existing {
			content, err := os.ReadFile(path)
			if err != nil || string(content) != "old log\n" {
				t.Fatalf("existing log was not preserved: %q, %v", content, err)
			}
		}
	}
}

func TestDaemonLogRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "daemon.log")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if file, err := openDaemonLog(path); err == nil {
		file.Close()
		t.Fatal("opened symlink")
	}
	info, err := os.Stat(target)
	if err != nil || info.Mode().Perm() != 0644 {
		t.Fatalf("symlink target was changed: %v", err)
	}
}

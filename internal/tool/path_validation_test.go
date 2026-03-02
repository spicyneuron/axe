package tool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePath_ValidRelativePath(t *testing.T) {
	tmpdir := t.TempDir()
	sub := filepath.Join(tmpdir, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := validatePath(tmpdir, "subdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want, _ := filepath.EvalSymlinks(sub)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidatePath_DotPath(t *testing.T) {
	tmpdir := t.TempDir()

	got, err := validatePath(tmpdir, ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want, _ := filepath.EvalSymlinks(filepath.Clean(tmpdir))
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidatePath_NestedPath(t *testing.T) {
	tmpdir := t.TempDir()
	nested := filepath.Join(tmpdir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := validatePath(tmpdir, "a/b/c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want, _ := filepath.EvalSymlinks(nested)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidatePath_EmptyPath(t *testing.T) {
	tmpdir := t.TempDir()

	_, err := validatePath(tmpdir, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "path is required") {
		t.Errorf("error %q should contain %q", err.Error(), "path is required")
	}
}

func TestValidatePath_AbsolutePath(t *testing.T) {
	tmpdir := t.TempDir()

	_, err := validatePath(tmpdir, "/etc")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "absolute paths are not allowed") {
		t.Errorf("error %q should contain %q", err.Error(), "absolute paths are not allowed")
	}
}

func TestValidatePath_ParentTraversalEscape(t *testing.T) {
	tmpdir := t.TempDir()

	_, err := validatePath(tmpdir, "../../etc")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "path escapes workdir") {
		t.Errorf("error %q should contain %q", err.Error(), "path escapes workdir")
	}
}

func TestValidatePath_ParentTraversalWithinWorkdir(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpdir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := validatePath(tmpdir, "a/b/../b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want, _ := filepath.EvalSymlinks(filepath.Join(tmpdir, "a", "b"))
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidatePath_SymlinkWithinWorkdir(t *testing.T) {
	tmpdir := t.TempDir()
	realDir := filepath.Join(tmpdir, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	linkDir := filepath.Join(tmpdir, "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatal(err)
	}

	got, err := validatePath(tmpdir, "link")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want, _ := filepath.EvalSymlinks(realDir)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidatePath_SymlinkEscapingWorkdir(t *testing.T) {
	tmpdir := t.TempDir()
	outsideDir := t.TempDir() // separate tmpdir
	linkDir := filepath.Join(tmpdir, "escape")
	if err := os.Symlink(outsideDir, linkDir); err != nil {
		t.Fatal(err)
	}

	_, err := validatePath(tmpdir, "escape")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "path escapes workdir") {
		t.Errorf("error %q should contain %q", err.Error(), "path escapes workdir")
	}
}

func TestValidatePath_NonexistentPath(t *testing.T) {
	tmpdir := t.TempDir()

	_, err := validatePath(tmpdir, "no_such_dir")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestValidatePath_SiblingDirectoryPrefixOverlap(t *testing.T) {
	// Regression: workdir="/tmp/work" must NOT match "/tmp/worker".
	parent := t.TempDir()
	workdir := filepath.Join(parent, "work")
	sibling := filepath.Join(parent, "worker")
	if err := os.Mkdir(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := validatePath(workdir, "../worker")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "path escapes workdir") {
		t.Errorf("error %q should contain %q", err.Error(), "path escapes workdir")
	}
}

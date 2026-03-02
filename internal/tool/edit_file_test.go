package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/provider"
)

func TestEditFile_SingleReplace(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "hello.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := editFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "ef-1",
		Name: "edit_file",
		Arguments: map[string]string{
			"path":       "hello.txt",
			"old_string": "hello",
			"new_string": "goodbye",
		},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	data, err := os.ReadFile(filepath.Join(tmpdir, "hello.txt"))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "goodbye world" {
		t.Errorf("file content: got %q, want %q", string(data), "goodbye world")
	}

	want := "replaced 1 occurrence(s) in hello.txt"
	if result.Content != want {
		t.Errorf("Content: got %q, want %q", result.Content, want)
	}

	if result.CallID != "ef-1" {
		t.Errorf("CallID: got %q, want %q", result.CallID, "ef-1")
	}
}

func TestEditFile_ReplaceAll(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "repeat.txt"), []byte("aaa"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := editFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "ef-2",
		Name: "edit_file",
		Arguments: map[string]string{
			"path":        "repeat.txt",
			"old_string":  "a",
			"new_string":  "b",
			"replace_all": "true",
		},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	data, err := os.ReadFile(filepath.Join(tmpdir, "repeat.txt"))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "bbb" {
		t.Errorf("file content: got %q, want %q", string(data), "bbb")
	}

	want := "replaced 3 occurrence(s) in repeat.txt"
	if result.Content != want {
		t.Errorf("Content: got %q, want %q", result.Content, want)
	}
}

func TestEditFile_NotFoundError(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "target.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := editFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "ef-3",
		Name: "edit_file",
		Arguments: map[string]string{
			"path":       "target.txt",
			"old_string": "xyz",
			"new_string": "abc",
		},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "old_string not found in file") {
		t.Errorf("Content %q should contain 'old_string not found in file'", result.Content)
	}

	// Verify file unchanged.
	data, err := os.ReadFile(filepath.Join(tmpdir, "target.txt"))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("file content should be unchanged, got %q", string(data))
	}
}

func TestEditFile_MultipleMatchesWithoutReplaceAll(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "multi.txt"), []byte("abab"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := editFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "ef-4",
		Name: "edit_file",
		Arguments: map[string]string{
			"path":       "multi.txt",
			"old_string": "ab",
			"new_string": "cd",
		},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "found 2 times") {
		t.Errorf("Content %q should contain 'found 2 times'", result.Content)
	}

	// Verify file unchanged.
	data, err := os.ReadFile(filepath.Join(tmpdir, "multi.txt"))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "abab" {
		t.Errorf("file content should be unchanged, got %q", string(data))
	}
}

func TestEditFile_PathTraversalRejected(t *testing.T) {
	tmpdir := t.TempDir()

	entry := editFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "ef-5",
		Name: "edit_file",
		Arguments: map[string]string{
			"path":       "../../escape.txt",
			"old_string": "a",
			"new_string": "b",
		},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "path escapes workdir") {
		t.Errorf("Content %q should contain 'path escapes workdir'", result.Content)
	}
}

func TestEditFile_AbsolutePathRejected(t *testing.T) {
	tmpdir := t.TempDir()

	entry := editFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "ef-6",
		Name: "edit_file",
		Arguments: map[string]string{
			"path":       "/etc/passwd",
			"old_string": "a",
			"new_string": "b",
		},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "absolute paths") {
		t.Errorf("Content %q should mention absolute paths", result.Content)
	}
}

func TestEditFile_MissingPathArgument(t *testing.T) {
	tmpdir := t.TempDir()

	entry := editFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "ef-7",
		Name: "edit_file",
		Arguments: map[string]string{
			"old_string": "a",
			"new_string": "b",
		},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "path is required") {
		t.Errorf("Content %q should contain 'path is required'", result.Content)
	}
}

func TestEditFile_MissingOldStringArgument(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "target.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := editFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "ef-8",
		Name: "edit_file",
		Arguments: map[string]string{
			"path":       "target.txt",
			"new_string": "b",
		},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "old_string is required") {
		t.Errorf("Content %q should contain 'old_string is required'", result.Content)
	}
}

func TestEditFile_EmptyNewStringDeletesText(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "delete.txt"), []byte("remove me please"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := editFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "ef-9",
		Name: "edit_file",
		Arguments: map[string]string{
			"path":       "delete.txt",
			"old_string": "remove ",
			"new_string": "",
		},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	data, err := os.ReadFile(filepath.Join(tmpdir, "delete.txt"))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "me please" {
		t.Errorf("file content: got %q, want %q", string(data), "me please")
	}

	want := "replaced 1 occurrence(s) in delete.txt"
	if result.Content != want {
		t.Errorf("Content: got %q, want %q", result.Content, want)
	}
}

func TestEditFile_SymlinkEscapeRejected(t *testing.T) {
	tmpdir := t.TempDir()
	outsideDir := t.TempDir()

	// Write a file in the outside directory.
	if err := os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create symlink: workdir/link.txt -> outsideDir/secret.txt
	linkPath := filepath.Join(tmpdir, "link.txt")
	if err := os.Symlink(filepath.Join(outsideDir, "secret.txt"), linkPath); err != nil {
		t.Fatal(err)
	}

	entry := editFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "ef-10",
		Name: "edit_file",
		Arguments: map[string]string{
			"path":       "link.txt",
			"old_string": "original",
			"new_string": "modified",
		},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}

	// Verify outside file unchanged.
	data, err := os.ReadFile(filepath.Join(outsideDir, "secret.txt"))
	if err != nil {
		t.Fatalf("failed to read outside file: %v", err)
	}
	if string(data) != "original" {
		t.Errorf("outside file should be unchanged, got %q", string(data))
	}
}

func TestEditFile_CallIDPassthrough(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := editFileEntry()
	callID := "ef-unique-42"
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   callID,
		Name: "edit_file",
		Arguments: map[string]string{
			"path":       "test.txt",
			"old_string": "abc",
			"new_string": "xyz",
		},
	}, ExecContext{Workdir: tmpdir})

	if result.CallID != callID {
		t.Errorf("CallID: got %q, want %q", result.CallID, callID)
	}
}

func TestEditFile_NonexistentFileError(t *testing.T) {
	tmpdir := t.TempDir()

	entry := editFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "ef-12",
		Name: "edit_file",
		Arguments: map[string]string{
			"path":       "missing.txt",
			"old_string": "a",
			"new_string": "b",
		},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	// validatePath's EvalSymlinks will fail with ENOENT.
	if result.Content == "" {
		t.Error("expected non-empty error content")
	}
}

func TestEditFile_DirectoryPathRejected(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpdir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	entry := editFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "ef-13",
		Name: "edit_file",
		Arguments: map[string]string{
			"path":       "subdir",
			"old_string": "a",
			"new_string": "b",
		},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "path is a directory") {
		t.Errorf("Content %q should contain 'path is a directory'", result.Content)
	}
}

func TestEditFile_InvalidReplaceAllValue(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "target.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := editFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "ef-14",
		Name: "edit_file",
		Arguments: map[string]string{
			"path":        "target.txt",
			"old_string":  "hello",
			"new_string":  "hi",
			"replace_all": "banana",
		},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "invalid replace_all value") {
		t.Errorf("Content %q should contain 'invalid replace_all value'", result.Content)
	}

	// Verify file unchanged.
	data, err := os.ReadFile(filepath.Join(tmpdir, "target.txt"))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("file content should be unchanged, got %q", string(data))
	}
}

func TestEditFile_MultilineMatch(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "multi.txt"), []byte("line1\nline2\nline3"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := editFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "ef-15",
		Name: "edit_file",
		Arguments: map[string]string{
			"path":       "multi.txt",
			"old_string": "line1\nline2",
			"new_string": "replaced",
		},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	data, err := os.ReadFile(filepath.Join(tmpdir, "multi.txt"))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "replaced\nline3" {
		t.Errorf("file content: got %q, want %q", string(data), "replaced\nline3")
	}
}

package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/provider"
)

func TestWriteFile_CreateNewFile(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-1",
		Name:      "write_file",
		Arguments: map[string]string{"path": "output.txt", "content": "hello world"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	// Verify file on disk.
	data, err := os.ReadFile(filepath.Join(tmpdir, "output.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("file content: got %q, want %q", string(data), "hello world")
	}

	// Verify success message.
	want := "wrote 11 bytes to output.txt"
	if result.Content != want {
		t.Errorf("Content: got %q, want %q", result.Content, want)
	}

	// Verify CallID.
	if result.CallID != "wf-1" {
		t.Errorf("CallID: got %q, want %q", result.CallID, "wf-1")
	}
}

func TestWriteFile_OverwriteExisting(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "existing.txt"), []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-2",
		Name:      "write_file",
		Arguments: map[string]string{"path": "existing.txt", "content": "new content"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	// Verify old content is completely replaced.
	data, err := os.ReadFile(filepath.Join(tmpdir, "existing.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != "new content" {
		t.Errorf("file content: got %q, want %q", string(data), "new content")
	}

	want := "wrote 11 bytes to existing.txt"
	if result.Content != want {
		t.Errorf("Content: got %q, want %q", result.Content, want)
	}
}

func TestWriteFile_CreateWithNestedDirs(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-3",
		Name:      "write_file",
		Arguments: map[string]string{"path": "a/b/c/deep.txt", "content": "nested"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	// Verify file on disk.
	data, err := os.ReadFile(filepath.Join(tmpdir, "a", "b", "c", "deep.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != "nested" {
		t.Errorf("file content: got %q, want %q", string(data), "nested")
	}

	// Verify intermediate directories exist.
	for _, dir := range []string{"a", "a/b", "a/b/c"} {
		info, err := os.Stat(filepath.Join(tmpdir, dir))
		if err != nil {
			t.Errorf("directory %q does not exist: %v", dir, err)
		} else if !info.IsDir() {
			t.Errorf("%q is not a directory", dir)
		}
	}
}

func TestWriteFile_PathTraversalRejected(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-4",
		Name:      "write_file",
		Arguments: map[string]string{"path": "../../escape.txt", "content": "bad"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "path escapes workdir") {
		t.Errorf("Content %q should contain 'path escapes workdir'", result.Content)
	}
}

func TestWriteFile_AbsolutePathRejected(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-5",
		Name:      "write_file",
		Arguments: map[string]string{"path": "/tmp/absolute.txt", "content": "bad"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "absolute paths") {
		t.Errorf("Content %q should mention absolute paths", result.Content)
	}
}

func TestWriteFile_EmptyContent(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-6",
		Name:      "write_file",
		Arguments: map[string]string{"path": "empty.txt", "content": ""},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	// Verify file exists with 0 bytes.
	data, err := os.ReadFile(filepath.Join(tmpdir, "empty.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected 0 bytes, got %d", len(data))
	}

	want := "wrote 0 bytes to empty.txt"
	if result.Content != want {
		t.Errorf("Content: got %q, want %q", result.Content, want)
	}
}

func TestWriteFile_MissingContentArgument(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-7",
		Name:      "write_file",
		Arguments: map[string]string{"path": "nokey.txt"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	// Verify file exists with 0 bytes.
	data, err := os.ReadFile(filepath.Join(tmpdir, "nokey.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected 0 bytes, got %d", len(data))
	}

	want := "wrote 0 bytes to nokey.txt"
	if result.Content != want {
		t.Errorf("Content: got %q, want %q", result.Content, want)
	}
}

func TestWriteFile_MissingPathArgument(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-8",
		Name:      "write_file",
		Arguments: map[string]string{},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "path is required") {
		t.Errorf("Content %q should contain 'path is required'", result.Content)
	}
}

func TestWriteFile_SymlinkEscapeRejected(t *testing.T) {
	tmpdir := t.TempDir()
	outsideDir := t.TempDir()

	// Create symlink: workdir/link -> outsideDir
	linkPath := filepath.Join(tmpdir, "link")
	if err := os.Symlink(outsideDir, linkPath); err != nil {
		t.Fatal(err)
	}

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-9",
		Name:      "write_file",
		Arguments: map[string]string{"path": "link/escape.txt", "content": "bad"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}

	// Verify no file was created in the outside directory.
	escapedFile := filepath.Join(outsideDir, "escape.txt")
	if _, err := os.Stat(escapedFile); err == nil {
		t.Error("file was created outside workdir via symlink escape")
	}
}

func TestWriteFile_CallIDPassthrough(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	callID := "wf-unique-99"
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        callID,
		Name:      "write_file",
		Arguments: map[string]string{"path": "test.txt", "content": "x"},
	}, ExecContext{Workdir: tmpdir})

	if result.CallID != callID {
		t.Errorf("CallID: got %q, want %q", result.CallID, callID)
	}
}

func TestWriteFile_ByteCountAccurate(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-11",
		Name:      "write_file",
		Arguments: map[string]string{"path": "unicode.txt", "content": "日本語"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	// "日本語" is 3 characters × 3 bytes each = 9 bytes in UTF-8.
	want := "wrote 9 bytes to unicode.txt"
	if result.Content != want {
		t.Errorf("Content: got %q, want %q", result.Content, want)
	}

	// Verify on disk.
	data, err := os.ReadFile(filepath.Join(tmpdir, "unicode.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if len(data) != 9 {
		t.Errorf("file size: got %d bytes, want 9", len(data))
	}
}

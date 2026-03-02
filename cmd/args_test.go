package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestExactArgs_MissingArg_IncludesUsageHint(t *testing.T) {
	cmd := &cobra.Command{Use: "test <thing>"}
	validator := exactArgs(1)
	err := validator(cmd, []string{})
	if err == nil {
		t.Fatal("expected error for missing arg, got nil")
	}
	if !strings.Contains(err.Error(), "missing required argument: <thing>") {
		t.Errorf("error %q missing 'missing required argument: <thing>'", err.Error())
	}
	if !strings.Contains(err.Error(), "Usage:") {
		t.Errorf("error %q missing 'Usage:'", err.Error())
	}
}

func TestExactArgs_TooManyArgs_IncludesUsageHint(t *testing.T) {
	cmd := &cobra.Command{Use: "test <thing>"}
	validator := exactArgs(1)
	err := validator(cmd, []string{"a", "b", "c"})
	if err == nil {
		t.Fatal("expected error for too many args, got nil")
	}
	if !strings.Contains(err.Error(), "expected 1 argument, got 3") {
		t.Errorf("error %q missing 'expected 1 argument, got 3'", err.Error())
	}
	if !strings.Contains(err.Error(), "Usage:") {
		t.Errorf("error %q missing 'Usage:'", err.Error())
	}
}

func TestExactArgs_CorrectArgs_NoError(t *testing.T) {
	cmd := &cobra.Command{Use: "test <thing>"}
	validator := exactArgs(1)
	err := validator(cmd, []string{"value"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestExactArgs_NoPlaceholder_FallbackName(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	validator := exactArgs(1)
	err := validator(cmd, []string{})
	if err == nil {
		t.Fatal("expected error for missing arg, got nil")
	}
	if !strings.Contains(err.Error(), "missing required argument: argument") {
		t.Errorf("error %q missing 'missing required argument: argument'", err.Error())
	}
}

func TestExactArgs_RunCommand_MissingArg(t *testing.T) {
	resetRunCmd(t)
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing arg, got nil")
	}
	if !strings.Contains(err.Error(), "missing required argument: <agent>") {
		t.Errorf("error %q missing 'missing required argument: <agent>'", err.Error())
	}
	if !strings.Contains(err.Error(), "Usage:") {
		t.Errorf("error %q missing 'Usage:'", err.Error())
	}
}

func TestExactArgs_AgentsShowCommand_MissingArg(t *testing.T) {
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"agents", "show"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing arg, got nil")
	}
	if !strings.Contains(err.Error(), "missing required argument: <agent>") {
		t.Errorf("error %q missing 'missing required argument: <agent>'", err.Error())
	}
	if !strings.Contains(err.Error(), "Usage:") {
		t.Errorf("error %q missing 'Usage:'", err.Error())
	}
}

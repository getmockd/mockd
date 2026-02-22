package cli

import (
	"strings"
	"testing"
)

func runRootCommandForTest(args []string) error {
	jsonOutput = false

	if f := rootCmd.Flags().Lookup("help"); f != nil {
		f.Changed = false
		_ = f.Value.Set("false")
	}
	if f := rootCmd.Flags().Lookup("json"); f != nil {
		f.Changed = false
		_ = f.Value.Set("false")
	}
	if f := helpTopicCmd.Flags().Lookup("help"); f != nil {
		f.Changed = false
		_ = f.Value.Set("false")
	}
	if f := mcpCmd.Flags().Lookup("help"); f != nil {
		f.Changed = false
		_ = f.Value.Set("false")
	}

	rootCmd.SetArgs(args)
	defer rootCmd.SetArgs(nil)
	return rootCmd.Execute()
}

func TestHelpTopicCmd_RejectsExtraArgs(t *testing.T) {
	err := runRootCommandForTest([]string{"help", "config", "extra"})
	if err == nil {
		t.Fatal("expected error for extra help argument, got nil")
	}
	if !strings.Contains(err.Error(), "accepts at most 1 arg") {
		t.Fatalf("expected max-args error, got: %v", err)
	}
}

func TestMCPCmd_RejectsPositionalArgs(t *testing.T) {
	err := runRootCommandForTest([]string{"mcp", "extra"})
	if err == nil {
		t.Fatal("expected error for mcp positional arg, got nil")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("expected invalid-positional error, got: %v", err)
	}
}

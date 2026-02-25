package cli

import (
	"testing"
)

func TestStatefulCmdRegistered(t *testing.T) {
	// Verify the stateful command is properly registered on rootCmd
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "stateful" {
			found = true

			// Check subcommands
			subCmds := map[string]bool{}
			for _, sub := range cmd.Commands() {
				subCmds[sub.Name()] = true
			}

			if !subCmds["add"] {
				t.Error("stateful command should have 'add' subcommand")
			}
			if !subCmds["list"] {
				t.Error("stateful command should have 'list' subcommand")
			}
			if !subCmds["reset"] {
				t.Error("stateful command should have 'reset' subcommand")
			}
			break
		}
	}
	if !found {
		t.Error("stateful command should be registered on rootCmd")
	}
}

func TestStatefulAddCmdFlags(t *testing.T) {
	// Verify the add subcommand has the expected flags
	flags := statefulAddCmd.Flags()

	pathFlag := flags.Lookup("path")
	if pathFlag == nil {
		t.Error("stateful add should have --path flag")
	}

	idFieldFlag := flags.Lookup("id-field")
	if idFieldFlag == nil {
		t.Error("stateful add should have --id-field flag")
	}
}

func TestStatefulAddCmdRequiresArgs(t *testing.T) {
	// The add command requires exactly 1 argument (the resource name)
	err := statefulAddCmd.Args(statefulAddCmd, []string{})
	if err == nil {
		t.Error("stateful add should require exactly 1 argument")
	}

	err = statefulAddCmd.Args(statefulAddCmd, []string{"users"})
	if err != nil {
		t.Errorf("stateful add should accept 1 argument: %v", err)
	}

	err = statefulAddCmd.Args(statefulAddCmd, []string{"users", "extra"})
	if err == nil {
		t.Error("stateful add should reject 2 arguments")
	}
}

func TestStatefulResetCmdRequiresArgs(t *testing.T) {
	// The reset command requires exactly 1 argument (the resource name)
	err := statefulResetCmd.Args(statefulResetCmd, []string{})
	if err == nil {
		t.Error("stateful reset should require exactly 1 argument")
	}

	err = statefulResetCmd.Args(statefulResetCmd, []string{"users"})
	if err != nil {
		t.Errorf("stateful reset should accept 1 argument: %v", err)
	}
}

func TestStatefulListCmdFlags(t *testing.T) {
	// Verify the list subcommand has the expected flags
	flags := statefulListCmd.Flags()

	limitFlag := flags.Lookup("limit")
	if limitFlag == nil {
		t.Error("stateful list should have --limit flag")
	}
	if limitFlag.DefValue != "100" {
		t.Errorf("--limit default: got %s, want 100", limitFlag.DefValue)
	}

	offsetFlag := flags.Lookup("offset")
	if offsetFlag == nil {
		t.Error("stateful list should have --offset flag")
	}

	sortFlag := flags.Lookup("sort")
	if sortFlag == nil {
		t.Error("stateful list should have --sort flag")
	}

	orderFlag := flags.Lookup("order")
	if orderFlag == nil {
		t.Error("stateful list should have --order flag")
	}
}

func TestSoapAddCmdStatefulFlags(t *testing.T) {
	// Verify the soap add command has stateful flags
	flags := soapAddCmd.Flags()

	resFlag := flags.Lookup("stateful-resource")
	if resFlag == nil {
		t.Error("soap add should have --stateful-resource flag")
	}

	actionFlag := flags.Lookup("stateful-action")
	if actionFlag == nil {
		t.Error("soap add should have --stateful-action flag")
	}
}

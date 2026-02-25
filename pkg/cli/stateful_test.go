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

// --- Custom operation command tests ---

func TestCustomCmdRegistered(t *testing.T) {
	// Verify the custom command is registered under stateful
	found := false
	for _, cmd := range statefulCmd.Commands() {
		if cmd.Use == "custom" {
			found = true

			subCmds := map[string]bool{}
			for _, sub := range cmd.Commands() {
				subCmds[sub.Name()] = true
			}

			if !subCmds["list"] {
				t.Error("custom command should have 'list' subcommand")
			}
			if !subCmds["get"] {
				t.Error("custom command should have 'get' subcommand")
			}
			if !subCmds["add"] {
				t.Error("custom command should have 'add' subcommand")
			}
			if !subCmds["run"] {
				t.Error("custom command should have 'run' subcommand")
			}
			if !subCmds["delete"] {
				t.Error("custom command should have 'delete' subcommand")
			}
			break
		}
	}
	if !found {
		t.Error("custom command should be registered under stateful")
	}
}

func TestCustomGetCmdRequiresArgs(t *testing.T) {
	err := customGetCmd.Args(customGetCmd, []string{})
	if err == nil {
		t.Error("custom get should require exactly 1 argument")
	}

	err = customGetCmd.Args(customGetCmd, []string{"TransferFunds"})
	if err != nil {
		t.Errorf("custom get should accept 1 argument: %v", err)
	}
}

func TestCustomRunCmdRequiresArgs(t *testing.T) {
	err := customRunCmd.Args(customRunCmd, []string{})
	if err == nil {
		t.Error("custom run should require exactly 1 argument")
	}

	err = customRunCmd.Args(customRunCmd, []string{"TransferFunds"})
	if err != nil {
		t.Errorf("custom run should accept 1 argument: %v", err)
	}
}

func TestCustomDeleteCmdRequiresArgs(t *testing.T) {
	err := customDeleteCmd.Args(customDeleteCmd, []string{})
	if err == nil {
		t.Error("custom delete should require exactly 1 argument")
	}

	err = customDeleteCmd.Args(customDeleteCmd, []string{"TransferFunds"})
	if err != nil {
		t.Errorf("custom delete should accept 1 argument: %v", err)
	}
}

func TestCustomAddCmdFlags(t *testing.T) {
	flags := customAddCmd.Flags()

	fileFlag := flags.Lookup("file")
	if fileFlag == nil {
		t.Error("custom add should have --file flag")
	}

	defFlag := flags.Lookup("definition")
	if defFlag == nil {
		t.Error("custom add should have --definition flag")
	}
}

func TestCustomRunCmdFlags(t *testing.T) {
	flags := customRunCmd.Flags()

	inputFlag := flags.Lookup("input")
	if inputFlag == nil {
		t.Error("custom run should have --input flag")
	}

	inputFileFlag := flags.Lookup("input-file")
	if inputFileFlag == nil {
		t.Error("custom run should have --input-file flag")
	}
}

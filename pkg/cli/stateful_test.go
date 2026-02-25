package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
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
		t.Fatal("stateful list should have --limit flag")
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
			if !subCmds["validate"] {
				t.Error("custom command should have 'validate' subcommand")
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

func TestCustomValidateCmdFlags(t *testing.T) {
	flags := customValidateCmd.Flags()

	if flags.Lookup("file") == nil {
		t.Error("custom validate should have --file flag")
	}
	if flags.Lookup("definition") == nil {
		t.Error("custom validate should have --definition flag")
	}
	if flags.Lookup("input") == nil {
		t.Error("custom validate should have --input flag")
	}
	if flags.Lookup("input-file") == nil {
		t.Error("custom validate should have --input-file flag")
	}
	if flags.Lookup("fixtures-file") == nil {
		t.Error("custom validate should have --fixtures-file flag")
	}
	if flags.Lookup("check-resources") == nil {
		t.Error("custom validate should have --check-resources flag")
	}
	if flags.Lookup("check-expressions-runtime") == nil {
		t.Error("custom validate should have --check-expressions-runtime flag")
	}
	if flags.Lookup("strict") == nil {
		t.Error("custom validate should have --strict flag")
	}
}

func TestReadCustomOperationConfig_YAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transfer.yaml")
	content := []byte(`
name: TransferFunds
consistency: atomic
steps:
  - type: set
    var: total
    value: "input.amount * 2"
response:
  ok: "true"
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg, err := readCustomOperationConfig(path, "")
	if err != nil {
		t.Fatalf("readCustomOperationConfig: %v", err)
	}
	if cfg.Name != "TransferFunds" {
		t.Fatalf("name = %q", cfg.Name)
	}
	if cfg.Consistency != "atomic" {
		t.Fatalf("consistency = %q", cfg.Consistency)
	}
	if len(cfg.Steps) != 1 {
		t.Fatalf("steps = %d", len(cfg.Steps))
	}
}

func TestValidateCustomOperationLocally_Success(t *testing.T) {
	cfg := &config.CustomOperationConfig{
		Name:        "TransferFunds",
		Consistency: "atomic",
		Steps: []config.CustomStepConfig{
			{Type: "read", Resource: "accounts", ID: "input.sourceId", As: "source"},
			{Type: "set", Var: "newBalance", Value: "source.balance - input.amount"},
			{Type: "update", Resource: "accounts", ID: "input.sourceId", Set: map[string]string{"balance": "newBalance"}},
		},
		Response: map[string]string{"balance": "newBalance"},
	}

	res, err := validateCustomOperationLocally(cfg, map[string]interface{}{
		"sourceId": "acc-1",
		"amount":   float64(10),
	})
	if err != nil {
		t.Fatalf("validateCustomOperationLocally: %v", err)
	}
	if res.Name != "TransferFunds" {
		t.Fatalf("name = %q", res.Name)
	}
	if res.Consistency != "atomic" {
		t.Fatalf("consistency = %q", res.Consistency)
	}
	if len(res.ReferencedResources) != 1 || res.ReferencedResources[0] != "accounts" {
		t.Fatalf("resources = %#v", res.ReferencedResources)
	}
}

func TestValidateCustomOperationLocally_InvalidExpr(t *testing.T) {
	cfg := &config.CustomOperationConfig{
		Name: "BadExpr",
		Steps: []config.CustomStepConfig{
			{Type: "set", Var: "x", Value: "input."},
		},
	}

	if _, err := validateCustomOperationLocally(cfg, map[string]interface{}{}); err == nil {
		t.Fatal("expected validation error for bad expression")
	}
}

func TestValidateCustomOperationLocally_WarnsOnEmptyUpdateSet(t *testing.T) {
	cfg := &config.CustomOperationConfig{
		Name: "WarnOp",
		Steps: []config.CustomStepConfig{
			{Type: "update", Resource: "accounts", ID: `"acc-1"`, Set: map[string]string{}},
		},
	}

	res, err := validateCustomOperationLocally(cfg, map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected warning for empty update set")
	}
}

func TestReadCustomValidationFixtures_JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixtures.json")
	content := []byte(`{
		"vars": {"source": {"id": "acc-1", "balance": 500}},
		"resources": {"accounts": {"acc-1": {"id": "acc-1", "balance": 500}}}
	}`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	fixtures, err := readCustomValidationFixtures(path)
	if err != nil {
		t.Fatalf("readCustomValidationFixtures: %v", err)
	}
	if fixtures.Vars["source"] == nil {
		t.Fatal("expected vars.source fixture")
	}
	if fixtures.Resources["accounts"]["acc-1"] == nil {
		t.Fatal("expected resources.accounts.acc-1 fixture")
	}
}

func TestValidateCustomOperationRuntimeExpressions_WithFixtures(t *testing.T) {
	cfg := &config.CustomOperationConfig{
		Name: "TransferFunds",
		Steps: []config.CustomStepConfig{
			{Type: "read", Resource: "accounts", ID: "input.sourceId", As: "source"},
			{Type: "set", Var: "newBalance", Value: "source.balance - input.amount"},
			{Type: "update", Resource: "accounts", ID: "input.sourceId", Set: map[string]string{"balance": "newBalance"}, As: "updated"},
		},
		Response: map[string]string{
			"balance": "updated.balance",
		},
	}

	warnings, err := validateCustomOperationRuntimeExpressions(cfg, map[string]interface{}{
		"sourceId": "acc-1",
		"amount":   float64(100),
	}, &customValidationFixtures{
		Resources: map[string]map[string]map[string]interface{}{
			"accounts": {
				"acc-1": {"id": "acc-1", "balance": float64(500)},
			},
		},
	})
	if err != nil {
		t.Fatalf("runtime validation failed: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
}

func TestValidateCustomOperationRuntimeExpressions_WarnsWithoutFixtures(t *testing.T) {
	cfg := &config.CustomOperationConfig{
		Name: "WarnRuntime",
		Steps: []config.CustomStepConfig{
			{Type: "read", Resource: "accounts", ID: `"acc-1"`, As: "source"},
		},
		Response: map[string]string{
			"id": "source.id",
		},
	}

	warnings, err := validateCustomOperationRuntimeExpressions(cfg, map[string]interface{}{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected synthetic placeholder warning")
	}
}

func TestValidateCustomOperationRuntimeExpressions_CatchesRuntimeError(t *testing.T) {
	cfg := &config.CustomOperationConfig{
		Name: "BadRuntime",
		Steps: []config.CustomStepConfig{
			{Type: "set", Var: "x", Value: `"100"`},
		},
		Response: map[string]string{
			"bad": "x - 1",
		},
	}

	if _, err := validateCustomOperationRuntimeExpressions(cfg, map[string]interface{}{}, nil); err == nil {
		t.Fatal("expected runtime validation error")
	}
}

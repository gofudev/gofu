package analyzer

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func testdataDir(name string) string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "testdata", name)
}

func TestAnalyze(t *testing.T) {
	tests := []struct {
		name      string
		dir       string
		wantErr   bool
		wantDiags int
		wantMsgs  []string
	}{
		{
			name:      "allowed stdlib and gofu.dev imports pass",
			dir:       "valid",
			wantDiags: 0,
		},
		{
			name:      "blocked stdlib imports produce errors",
			dir:       "invalid",
			wantDiags: 4, // os, net, unsafe, github.com/foo/bar
			wantMsgs:  []string{"os", "net", "unsafe", "github.com/foo/bar"},
		},
		{
			name:      "testing in test file passes, in non-test file fails",
			dir:       "testing_edge",
			wantDiags: 1,
			wantMsgs:  []string{"testing"},
		},
		{
			name:      "mixed valid and invalid imports",
			dir:       "mixed",
			wantDiags: 1,
			wantMsgs:  []string{"os"},
		},
		{
			name:      "empty directory",
			dir:       "empty",
			wantDiags: 0,
		},
		{
			name:    "nonexistent directory",
			dir:     "nonexistent",
			wantErr: true,
		},
		{
			name:      "go statements rejected",
			dir:       "go_stmt",
			wantDiags: 3,
			wantMsgs:  []string{"goroutines are not allowed"},
		},
		{
			name:      "channel types and operations rejected",
			dir:       "channel_ops",
			wantDiags: 6,
			wantMsgs:  []string{"channels are not allowed", "channel send is not allowed", "channel receive is not allowed"},
		},
		{
			name:      "select statements rejected",
			dir:       "select_stmt",
			wantDiags: 3,
			wantMsgs:  []string{"select statements are not allowed", "channels are not allowed", "channel receive is not allowed"},
		},
		{
			name:      "init function declarations rejected",
			dir:       "init_func",
			wantDiags: 1,
			wantMsgs:  []string{"init functions are not allowed"},
		},
		{
			name:      "init method is allowed",
			dir:       "init_method",
			wantDiags: 0,
		},
		{
			name:      "recursive traversal catches sub-package violations",
			dir:       "subpkg",
			wantDiags: 1,
			wantMsgs:  []string{"os"},
		},
		{
			name:      "deeply nested sub-package violations detected",
			dir:       "nested",
			wantDiags: 1,
			wantMsgs:  []string{"net"},
		},
		{
			name:      "unparseable file produces error diagnostic",
			dir:       "unparseable",
			wantDiags: 2,
			wantMsgs:  []string{"parse error:", "os"},
		},
		{
			name:      "testing/fstest allowed in test file, blocked in non-test",
			dir:       "testing_sub",
			wantDiags: 1,
			wantMsgs:  []string{"testing/fstest"},
		},
		{
			name:      "gofu.dev/mallory/exploit and bare gofu.dev/something rejected",
			dir:       "gofu_narrow",
			wantDiags: 2,
			wantMsgs:  []string{"gofu.dev/mallory/exploit", "gofu.dev/something"},
		},
		{
			name:      "go directives blocked",
			dir:       "directives_blocked",
			wantDiags: 5,
			wantMsgs:  []string{"//go:linkname", "//go:noescape", "//go:generate", "//go:embed", "//go:build"},
		},
		{
			name:      "regular comments and gofu directives allowed",
			dir:       "directives_allowed",
			wantDiags: 0,
		},
		{
			name:      "directives in test files still rejected",
			dir:       "directives_test_file",
			wantDiags: 1,
			wantMsgs:  []string{"//go:linkname"},
		},
		{
			name:      "directives on function decl and top-of-file rejected",
			dir:       "directives_positions",
			wantDiags: 2,
			wantMsgs:  []string{"//go:build", "//go:noinline"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Analyze(testdataDir(tt.dir))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			diags := result.Diagnostics
			if len(diags) != tt.wantDiags {
				t.Errorf("got %d diagnostics, want %d", len(diags), tt.wantDiags)
				for _, d := range diags {
					t.Logf("  %s: %s", d.Pos, d.Msg)
				}
			}
			for _, msg := range tt.wantMsgs {
				found := false
				for _, d := range diags {
					if d.Severity != Error {
						t.Errorf("diagnostic %q has severity %d, want Error", d.Msg, d.Severity)
					}
					if contains(d.Msg, msg) {
						found = true
						if d.File == "" {
							t.Errorf("diagnostic for %q has empty File", msg)
						}
						if !contains(d.Msg, "parse error:") && d.Pos.Line == 0 {
							t.Errorf("diagnostic for %q has zero line position", msg)
						}
					}
				}
				if !found {
					t.Errorf("no diagnostic found mentioning %q", msg)
				}
			}
		})
	}
}

func TestRunnableValid(t *testing.T) {
	result, err := Analyze(testdataDir("runnable_valid"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("got %d diagnostics, want 0", len(result.Diagnostics))
		for _, d := range result.Diagnostics {
			t.Logf("  %s: %s", d.Pos, d.Msg)
		}
	}
	if len(result.Runnables) != 2 {
		t.Fatalf("got %d runnables, want 2", len(result.Runnables))
	}
	byName := map[string]Runnable{}
	for _, r := range result.Runnables {
		byName[r.Name] = r
	}

	add := byName["Add"]
	wantParams := []Field{{Name: "a", Type: "int"}, {Name: "b", Type: "int"}}
	wantReturns := []Field{{Type: "int"}}
	if !reflect.DeepEqual(add.Params, wantParams) {
		t.Errorf("Add params = %v, want %v", add.Params, wantParams)
	}
	if !reflect.DeepEqual(add.Returns, wantReturns) {
		t.Errorf("Add returns = %v, want %v", add.Returns, wantReturns)
	}

	greet := byName["Greet"]
	if len(greet.Params) != 1 || greet.Params[0].Name != "name" || greet.Params[0].Type != "string" {
		t.Errorf("Greet params = %v, want [{name string}]", greet.Params)
	}
	if len(greet.Returns) != 1 || greet.Returns[0].Type != "string" {
		t.Errorf("Greet returns = %v, want [{string}]", greet.Returns)
	}

	for _, r := range result.Runnables {
		if r.File == "" {
			t.Errorf("runnable %q has empty File", r.Name)
		}
		if r.Pos.Line == 0 {
			t.Errorf("runnable %q has zero line position", r.Name)
		}
	}
}

func TestRunnableUnexported(t *testing.T) {
	result, err := Analyze(testdataDir("runnable_unexported"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(result.Diagnostics))
	}
	if !contains(result.Diagnostics[0].Msg, "must be exported") {
		t.Errorf("got msg %q, want mention of 'must be exported'", result.Diagnostics[0].Msg)
	}
	if len(result.Runnables) != 0 {
		t.Errorf("got %d runnables, want 0", len(result.Runnables))
	}
}

func TestRunnableMethod(t *testing.T) {
	result, err := Analyze(testdataDir("runnable_method"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(result.Diagnostics))
	}
	if !contains(result.Diagnostics[0].Msg, "must not be a method") {
		t.Errorf("got msg %q, want mention of 'must not be a method'", result.Diagnostics[0].Msg)
	}
	if len(result.Runnables) != 0 {
		t.Errorf("got %d runnables, want 0", len(result.Runnables))
	}
}

func TestRunnableDuplicate(t *testing.T) {
	result, err := Analyze(testdataDir("runnable_duplicate"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(result.Diagnostics))
	}
	if !contains(result.Diagnostics[0].Msg, "duplicate runnable") {
		t.Errorf("got msg %q, want mention of 'duplicate runnable'", result.Diagnostics[0].Msg)
	}
	if len(result.Runnables) != 1 {
		t.Errorf("got %d runnables, want 1 (the first one)", len(result.Runnables))
	}
}

func TestRunnableNonFunc(t *testing.T) {
	result, err := Analyze(testdataDir("runnable_non_func"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(result.Diagnostics))
	}
	if !contains(result.Diagnostics[0].Msg, "must precede a function") {
		t.Errorf("got msg %q, want mention of 'must precede a function'", result.Diagnostics[0].Msg)
	}
	if len(result.Runnables) != 0 {
		t.Errorf("got %d runnables, want 0", len(result.Runnables))
	}
}

func TestRunnableMalformed(t *testing.T) {
	result, err := Analyze(testdataDir("runnable_malformed"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("got %d diagnostics, want 0", len(result.Diagnostics))
		for _, d := range result.Diagnostics {
			t.Logf("  %s: %s", d.Pos, d.Msg)
		}
	}
	if len(result.Runnables) != 0 {
		t.Errorf("got %d runnables, want 0 (malformed variants ignored)", len(result.Runnables))
	}
}

func TestRunnableGap(t *testing.T) {
	result, err := Analyze(testdataDir("runnable_gap"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("got %d diagnostics, want 0", len(result.Diagnostics))
	}
	if len(result.Runnables) != 0 {
		t.Errorf("got %d runnables, want 0 (blank line gap)", len(result.Runnables))
	}
}

func TestRunnableMultiParams(t *testing.T) {
	result, err := Analyze(testdataDir("runnable_multi_params"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("got %d diagnostics, want 0", len(result.Diagnostics))
	}
	if len(result.Runnables) != 2 {
		t.Fatalf("got %d runnables, want 2", len(result.Runnables))
	}
	byName := map[string]Runnable{}
	for _, r := range result.Runnables {
		byName[r.Name] = r
	}

	v := byName["Version"]
	if len(v.Params) != 0 {
		t.Errorf("Version params = %v, want empty", v.Params)
	}
	if !reflect.DeepEqual(v.Returns, []Field{{Type: "string"}}) {
		t.Errorf("Version returns = %v, want [{string}]", v.Returns)
	}

	d := byName["Divide"]
	wantParams := []Field{{Name: "a", Type: "float64"}, {Name: "b", Type: "float64"}}
	wantReturns := []Field{{Name: "result", Type: "float64"}, {Name: "err", Type: "error"}}
	if !reflect.DeepEqual(d.Params, wantParams) {
		t.Errorf("Divide params = %v, want %v", d.Params, wantParams)
	}
	if !reflect.DeepEqual(d.Returns, wantReturns) {
		t.Errorf("Divide returns = %v, want %v", d.Returns, wantReturns)
	}
}

func TestRunnableWithDiags(t *testing.T) {
	result, err := Analyze(testdataDir("runnable_with_diags"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1 (blocked import)", len(result.Diagnostics))
	}
	if !contains(result.Diagnostics[0].Msg, "os") {
		t.Errorf("got msg %q, want mention of 'os'", result.Diagnostics[0].Msg)
	}
	if len(result.Runnables) != 1 {
		t.Fatalf("got %d runnables, want 1", len(result.Runnables))
	}
	if result.Runnables[0].Name != "Greet" {
		t.Errorf("got runnable name %q, want Greet", result.Runnables[0].Name)
	}
}

func TestRunnableReturnFunc(t *testing.T) {
	result, err := Analyze(testdataDir("runnable_return_func"))
	if err != nil {
		t.Fatal(err)
	}
	// 1 return-type diagnostic only (func types aren't globally blocked)
	found := false
	for _, d := range result.Diagnostics {
		if contains(d.Msg, "runnable return type not allowed") {
			found = true
			if d.Severity != Error {
				t.Errorf("severity = %d, want Error", d.Severity)
			}
		}
	}
	if !found {
		t.Error("no diagnostic mentioning 'runnable return type not allowed'")
		for _, d := range result.Diagnostics {
			t.Logf("  %s: %s", d.Pos, d.Msg)
		}
	}
}

func TestRunnableReturnChan(t *testing.T) {
	result, err := Analyze(testdataDir("runnable_return_chan"))
	if err != nil {
		t.Fatal(err)
	}
	// Expect both: global "channels are not allowed" + return-type-specific diagnostic
	var foundGlobal, foundReturn bool
	for _, d := range result.Diagnostics {
		if contains(d.Msg, "channels are not allowed") {
			foundGlobal = true
		}
		if contains(d.Msg, "runnable return type not allowed") {
			foundReturn = true
			if d.Severity != Error {
				t.Errorf("severity = %d, want Error", d.Severity)
			}
		}
	}
	if !foundGlobal {
		t.Error("no diagnostic mentioning 'channels are not allowed'")
	}
	if !foundReturn {
		t.Error("no diagnostic mentioning 'runnable return type not allowed'")
	}
}

func TestRunnableReturnComplex(t *testing.T) {
	result, err := Analyze(testdataDir("runnable_return_complex"))
	if err != nil {
		t.Fatal(err)
	}
	var found64, found128 bool
	for _, d := range result.Diagnostics {
		if d.Severity != Error {
			t.Errorf("severity = %d, want Error for %q", d.Severity, d.Msg)
		}
		if contains(d.Msg, "complex64") {
			found64 = true
		}
		if contains(d.Msg, "complex128") {
			found128 = true
		}
	}
	if !found64 {
		t.Error("no diagnostic for complex64")
	}
	if !found128 {
		t.Error("no diagnostic for complex128")
	}
}

func TestRunnableReturnUnexportedFields(t *testing.T) {
	result, err := Analyze(testdataDir("runnable_return_unexported_fields"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range result.Diagnostics {
		if contains(d.Msg, "no exported fields") {
			found = true
			if d.Severity != Warning {
				t.Errorf("severity = %d, want Warning", d.Severity)
			}
		}
	}
	if !found {
		t.Error("no diagnostic mentioning 'no exported fields'")
		for _, d := range result.Diagnostics {
			t.Logf("  %s: %s", d.Pos, d.Msg)
		}
	}
}

func TestRunnableReturnCircular(t *testing.T) {
	result, err := Analyze(testdataDir("runnable_return_circular"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range result.Diagnostics {
		if contains(d.Msg, "circular") {
			found = true
			if d.Severity != Warning {
				t.Errorf("severity = %d, want Warning", d.Severity)
			}
		}
	}
	if !found {
		t.Error("no diagnostic mentioning 'circular'")
		for _, d := range result.Diagnostics {
			t.Logf("  %s: %s", d.Pos, d.Msg)
		}
	}
}

func TestRunnableReturnValid(t *testing.T) {
	result, err := Analyze(testdataDir("runnable_return_valid"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("got %d diagnostics, want 0", len(result.Diagnostics))
		for _, d := range result.Diagnostics {
			t.Logf("  %s: %s", d.Pos, d.Msg)
		}
	}
	if len(result.Runnables) != 6 {
		t.Errorf("got %d runnables, want 6", len(result.Runnables))
	}
}

func TestSecretDirectivesManifest(t *testing.T) {
	result, err := Analyze(testdataDir("secret_directives"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 {
		for _, d := range result.Diagnostics {
			t.Logf("  %s: %s", d.Pos, d.Msg)
		}
		t.Fatalf("got %d diagnostics, want 0", len(result.Diagnostics))
	}
	want := []string{"SLACK_TOKEN", "DATABASE_URL"}
	if !reflect.DeepEqual(result.Secrets, want) {
		t.Errorf("Secrets = %v, want %v", result.Secrets, want)
	}
}

func TestSecretMalformed(t *testing.T) {
	result, err := Analyze(testdataDir("secret_malformed"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 2 {
		for _, d := range result.Diagnostics {
			t.Logf("  %s: %s", d.Pos, d.Msg)
		}
		t.Fatalf("got %d diagnostics, want 2", len(result.Diagnostics))
	}
	for _, d := range result.Diagnostics {
		if d.Severity != Error {
			t.Errorf("severity = %d, want Error for %q", d.Severity, d.Msg)
		}
		if !contains(d.Msg, "malformed gofu:secret directive") {
			t.Errorf("got msg %q, want mention of 'malformed gofu:secret directive'", d.Msg)
		}
	}
}

func TestSecretDuplicate(t *testing.T) {
	result, err := Analyze(testdataDir("secret_duplicate"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 {
		for _, d := range result.Diagnostics {
			t.Logf("  %s: %s", d.Pos, d.Msg)
		}
		t.Fatalf("got %d diagnostics, want 1", len(result.Diagnostics))
	}
	d := result.Diagnostics[0]
	if d.Severity != Warning {
		t.Errorf("severity = %d, want Warning", d.Severity)
	}
	if !contains(d.Msg, "duplicate secret declaration") {
		t.Errorf("got msg %q, want mention of 'duplicate secret declaration'", d.Msg)
	}
	if !reflect.DeepEqual(result.Secrets, []string{"MY_KEY"}) {
		t.Errorf("Secrets = %v, want [MY_KEY]", result.Secrets)
	}
}

func TestSecretValidCall(t *testing.T) {
	result, err := Analyze(testdataDir("secret_valid_call"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 {
		for _, d := range result.Diagnostics {
			t.Logf("  %s: %s", d.Pos, d.Msg)
		}
		t.Fatalf("got %d diagnostics, want 0", len(result.Diagnostics))
	}
}

func TestSecretUndeclared(t *testing.T) {
	result, err := Analyze(testdataDir("secret_undeclared"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range result.Diagnostics {
		if contains(d.Msg, "not declared via //gofu:secret") {
			found = true
			if d.Severity != Error {
				t.Errorf("severity = %d, want Error", d.Severity)
			}
		}
	}
	if !found {
		t.Error("no diagnostic mentioning 'not declared via //gofu:secret'")
		for _, d := range result.Diagnostics {
			t.Logf("  %s: %s", d.Pos, d.Msg)
		}
	}
}

func TestSecretNonLiteral(t *testing.T) {
	result, err := Analyze(testdataDir("secret_nonliteral"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range result.Diagnostics {
		if contains(d.Msg, "credentials.Get requires a string literal") {
			found = true
			if d.Severity != Error {
				t.Errorf("severity = %d, want Error", d.Severity)
			}
		}
	}
	if !found {
		t.Error("no diagnostic mentioning 'credentials.Get requires a string literal'")
		for _, d := range result.Diagnostics {
			t.Logf("  %s: %s", d.Pos, d.Msg)
		}
	}
}

func TestSecretAlias(t *testing.T) {
	result, err := Analyze(testdataDir("secret_alias"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 {
		for _, d := range result.Diagnostics {
			t.Logf("  %s: %s", d.Pos, d.Msg)
		}
		t.Fatalf("got %d diagnostics, want 0 (alias should be resolved)", len(result.Diagnostics))
	}
}

func TestSecretNoImport(t *testing.T) {
	result, err := Analyze(testdataDir("secret_no_import"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 {
		for _, d := range result.Diagnostics {
			t.Logf("  %s: %s", d.Pos, d.Msg)
		}
		t.Fatalf("got %d diagnostics, want 0 (no credentials import = no call-site check)", len(result.Diagnostics))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

package autofix

import "testing"

func TestResolveRunner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		goos        string
		scriptPath  string
		wantKey     string
		wantCommand string
		wantArgs    []string
		wantErr     bool
	}{
		{
			name:        "windows bat uses cmd",
			goos:        "windows",
			scriptPath:  "scripts/fixes/restart.bat",
			wantKey:     "cmd",
			wantCommand: "cmd.exe",
			wantArgs:    []string{"/C"},
		},
		{
			name:        "windows cmd uses cmd",
			goos:        "windows",
			scriptPath:  "scripts/fixes/restart.cmd",
			wantKey:     "cmd",
			wantCommand: "cmd.exe",
			wantArgs:    []string{"/C"},
		},
		{
			name:        "windows sh uses bash",
			goos:        "windows",
			scriptPath:  "scripts/fixes/restart.sh",
			wantKey:     "bash",
			wantCommand: "bash",
		},
		{
			name:       "linux bat rejected",
			goos:       "linux",
			scriptPath: "scripts/fixes/restart.bat",
			wantErr:    true,
		},
		{
			name:        "linux sh uses bash",
			goos:        "linux",
			scriptPath:  "scripts/fixes/restart.sh",
			wantKey:     "bash",
			wantCommand: "bash",
		},
		{
			name:        "linux unknown extension falls back to bash",
			goos:        "linux",
			scriptPath:  "scripts/fixes/restart.script",
			wantKey:     "bash",
			wantCommand: "bash",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveRunner(tc.goos, tc.scriptPath)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveRunner returned error: %v", err)
			}
			if got.allowlistKey != tc.wantKey {
				t.Fatalf("allowlistKey mismatch: want %q got %q", tc.wantKey, got.allowlistKey)
			}
			if got.command != tc.wantCommand {
				t.Fatalf("command mismatch: want %q got %q", tc.wantCommand, got.command)
			}
			if len(got.args) != len(tc.wantArgs) {
				t.Fatalf("args length mismatch: want %d got %d", len(tc.wantArgs), len(got.args))
			}
			for i := range got.args {
				if got.args[i] != tc.wantArgs[i] {
					t.Fatalf("arg[%d] mismatch: want %q got %q", i, tc.wantArgs[i], got.args[i])
				}
			}
		})
	}
}

func TestNormalizeAllowedCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: "bash", want: "bash"},
		{in: " CMD.EXE ", want: "cmd"},
		{in: "  ", want: ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got := normalizeAllowedCommand(tc.in)
			if got != tc.want {
				t.Fatalf("normalizeAllowedCommand(%q): want %q got %q", tc.in, tc.want, got)
			}
		})
	}
}

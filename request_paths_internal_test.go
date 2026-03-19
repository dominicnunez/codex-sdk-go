package codex

import "testing"

func TestNormalizeAbsolutePath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "unix path is cleaned",
			input: "/tmp/../var//log",
			want:  "/var/log",
		},
		{
			name:  "unix path with duplicate leading slash stays unix",
			input: "//var/log",
			want:  "/var/log",
		},
		{
			name:  "windows drive path is cleaned on any platform",
			input: `C:\Users\kai\..\logs\app.txt`,
			want:  `C:\Users\logs\app.txt`,
		},
		{
			name:  "windows unc path is cleaned",
			input: `\\server\share\plugins\..\calendar`,
			want:  `\\server\share\calendar`,
		},
		{
			name:  "windows extended path is cleaned",
			input: `\\?\Volume{1234}\plugins\..\calendar`,
			want:  `\\?\Volume{1234}\calendar`,
		},
		{
			name:    "relative path is rejected",
			input:   "plugins/calendar",
			wantErr: true,
		},
		{
			name:    "drive-relative path is rejected",
			input:   `C:plugins\calendar`,
			wantErr: true,
		},
		{
			name:    "malformed unc path is rejected",
			input:   `\\server`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeAbsolutePath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeAbsolutePath(%q) error = nil, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeAbsolutePath(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeAbsolutePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeSandboxPolicyField(t *testing.T) {
	readOnly := &ReadOnlyAccessWrapper{Value: ReadOnlyAccessRestricted{
		ReadableRoots: []string{`C:\workspace\..\repo`},
	}}
	policy, err := normalizeSandboxPolicyField("sandboxPolicy", SandboxPolicyWorkspaceWrite{
		ReadOnlyAccess: readOnly,
		WritableRoots:  []string{"/tmp/../workspace"},
	})
	if err != nil {
		t.Fatalf("normalizeSandboxPolicyField() error = %v", err)
	}

	workspaceWrite, ok := policy.(SandboxPolicyWorkspaceWrite)
	if !ok {
		t.Fatalf("normalized policy type = %T, want SandboxPolicyWorkspaceWrite", policy)
	}
	if got := workspaceWrite.WritableRoots[0]; got != "/workspace" {
		t.Fatalf("WritableRoots[0] = %q, want /workspace", got)
	}

	restricted, ok := workspaceWrite.ReadOnlyAccess.Value.(ReadOnlyAccessRestricted)
	if !ok {
		t.Fatalf("ReadOnlyAccess type = %T, want ReadOnlyAccessRestricted", workspaceWrite.ReadOnlyAccess.Value)
	}
	if got := restricted.ReadableRoots[0]; got != `C:\repo` {
		t.Fatalf("ReadableRoots[0] = %q, want %q", got, `C:\repo`)
	}
}

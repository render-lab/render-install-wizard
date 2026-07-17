package detect

import "testing"

func TestContainsWSLSignature(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "wsl2 kernel string",
			content: "Linux version 5.15.90.1-microsoft-standard-WSL2 (oe-user@oe-host)",
			want:    true,
		},
		{
			name:    "microsoft lowercase",
			content: "5.10.16.3-microsoft-standard\n",
			want:    true,
		},
		{
			name:    "plain linux",
			content: "Linux version 6.8.0-generic (buildd@lcy02) #1 SMP",
			want:    false,
		},
		{
			name:    "empty",
			content: "",
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsWSLSignature(tt.content); got != tt.want {
				t.Errorf("containsWSLSignature(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

func TestDetectWSL(t *testing.T) {
	wslContent := func() string {
		return "Linux version 5.15.90.1-microsoft-standard-WSL2"
	}
	linuxContent := func() string {
		return "Linux version 6.8.0-generic"
	}

	if !detectWSL("linux", wslContent) {
		t.Errorf("detectWSL(linux, wsl) = false, want true")
	}
	if detectWSL("linux", linuxContent) {
		t.Errorf("detectWSL(linux, plain) = true, want false")
	}
	// Non-linux hosts are never WSL, even with a matching signature.
	if detectWSL("darwin", wslContent) {
		t.Errorf("detectWSL(darwin, wsl) = true, want false")
	}
}

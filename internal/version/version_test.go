package version

import "testing"

func TestString(t *testing.T) {
	tests := []struct {
		set  string
		want string
	}{
		{set: "1.2.0", want: "1.2.0"},
		{set: "", want: "dev"},
	}
	for _, tt := range tests {
		Version = tt.set
		if got := String(); got != tt.want {
			t.Errorf("Version %q: String() = %q, want %q", tt.set, got, tt.want)
		}
	}
}

func TestIsPackaged(t *testing.T) {
	defer func(old string) { Packaged = old }(Packaged)
	Packaged = ""
	if IsPackaged() {
		t.Error("a development build counts as packaged")
	}
	Packaged = "true"
	if !IsPackaged() {
		t.Error("a release build doesn't count as packaged")
	}
}

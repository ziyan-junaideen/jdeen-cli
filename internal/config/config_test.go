package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNormalizeAPIURL(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"https://api.jdeen.com":             "https://api.jdeen.com/v1",
		"https://api.jdeen.com/v1/":         "https://api.jdeen.com/v1",
		"http://api.jdeen.test:4020/custom": "http://api.jdeen.test:4020/custom/v1",
	}
	for input, expected := range tests {
		actual, err := NormalizeAPIURL(input)
		if err != nil {
			t.Fatalf("NormalizeAPIURL(%q): %v", input, err)
		}
		if actual != expected {
			t.Errorf("NormalizeAPIURL(%q) = %q, want %q", input, actual, expected)
		}
	}
	if _, err := NormalizeAPIURL("ftp://api.jdeen.com"); err == nil {
		t.Fatal("expected unsupported scheme to fail")
	}
}

func TestResolvePrecedenceAndProductionTLS(t *testing.T) {
	t.Setenv("JDEEN_PROFILE", "dev")
	t.Setenv("JDEEN_API_URL", "http://env.example.test")
	file := DefaultFile()
	runtimeConfig, err := Resolve(file, Overrides{APIURL: "http://flag.example.test"}, "test.toml")
	if err != nil {
		t.Fatal(err)
	}
	if runtimeConfig.ProfileName != "dev" || runtimeConfig.APIURL != "http://flag.example.test/v1" {
		t.Fatalf("unexpected runtime: %#v", runtimeConfig)
	}
	t.Setenv("JDEEN_API_URL", "")
	t.Setenv("JDEEN_PROFILE", "")
	insecure := true
	if _, err := Resolve(file, Overrides{ProfileName: "prod", InsecureSkipVerify: &insecure}, "test.toml"); err == nil {
		t.Fatal("expected insecure production URL to fail")
	}
}

func TestSaveUsesOwnerOnlyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode bits are not portable on Windows")
	}
	path := filepath.Join(t.TempDir(), "config", "config.toml")
	restore := SetPathForTest(path)
	defer restore()
	if err := Save(DefaultFile()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %o, want 600", info.Mode().Perm())
	}
}

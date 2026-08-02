package cli

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSkillDestinationsAlwaysUsesAgentsDirectory(t *testing.T) {
	home := t.TempDir()
	expected := []string{filepath.Join(home, ".agents", "skills", skillName)}
	if destinations := skillDestinations(home); !reflect.DeepEqual(destinations, expected) {
		t.Fatalf("skillDestinations() = %v, want %v", destinations, expected)
	}
}

func TestSkillDestinationsIncludesExistingClaudeDirectory(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("create .claude directory: %v", err)
	}
	expected := []string{
		filepath.Join(home, ".agents", "skills", skillName),
		filepath.Join(home, ".claude", "skills", skillName),
	}
	if destinations := skillDestinations(home); !reflect.DeepEqual(destinations, expected) {
		t.Fatalf("skillDestinations() = %v, want %v", destinations, expected)
	}
}

func TestInstallSkill(t *testing.T) {
	destination := filepath.Join(t.TempDir(), skillName)
	if err := installSkill(destination); err != nil {
		t.Fatalf("installSkill: %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(destination, "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed SKILL.md: %v", err)
	}
	if !strings.Contains(string(contents), "name: jdeen-cli") {
		t.Fatalf("installed SKILL.md does not contain jdeen-cli frontmatter: %s", contents)
	}
	if _, err := os.Stat(filepath.Join(destination, "agents", "openai.yaml")); err != nil {
		t.Fatalf("stat installed agents/openai.yaml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(destination, "stale.txt"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}
	if err := installSkill(destination); err != nil {
		t.Fatalf("refresh installSkill: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale file remains after refresh; stat error = %v", err)
	}
}

func TestSkillsCommandsInstallAndUninstallExpectedDirectories(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("create .claude directory: %v", err)
	}
	t.Setenv("HOME", home)

	installCommand := newSkillsInstallCommand()
	installCommand.SetOut(io.Discard)
	if err := installCommand.Execute(); err != nil {
		t.Fatalf("execute skills install: %v", err)
	}

	for _, path := range []string{
		filepath.Join(home, ".agents", "skills", skillName, "SKILL.md"),
		filepath.Join(home, ".claude", "skills", skillName, "SKILL.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected installed skill at %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".codex")); !os.IsNotExist(err) {
		t.Errorf("unexpected .codex directory; stat error = %v", err)
	}

	uninstallCommand := newSkillsUninstallCommand()
	uninstallCommand.SetOut(io.Discard)
	if err := uninstallCommand.Execute(); err != nil {
		t.Fatalf("execute skills uninstall: %v", err)
	}
	for _, path := range skillDestinations(home) {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("skill remains at %s; stat error = %v", path, err)
		}
	}
}

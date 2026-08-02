package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	projectskills "github.com/ziyan-junaideen/jdeen-cli/skills"
)

const skillName = "jdeen-cli"

func newSkillsCommand() *cobra.Command {
	command := &cobra.Command{Use: "skills", Short: "Manage agent skills for JDeen CLI"}
	command.AddCommand(newSkillsInstallCommand(), newSkillsUninstallCommand())
	return command
}

func newSkillsInstallCommand() *cobra.Command {
	return &cobra.Command{
		Use: "install", Short: "Install the JDeen CLI agent skill", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("find home directory: %w", err)
			}
			for _, destination := range skillDestinations(home) {
				if err := installSkill(destination); err != nil {
					return fmt.Errorf("install skill at %s: %w", destination, err)
				}
				fmt.Fprintf(command.OutOrStdout(), "Installed skill at %s\n", destination)
			}
			return nil
		},
	}
}

func newSkillsUninstallCommand() *cobra.Command {
	return &cobra.Command{
		Use: "uninstall", Short: "Uninstall the JDeen CLI agent skill", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("find home directory: %w", err)
			}
			for _, destination := range skillDestinations(home) {
				if err := os.RemoveAll(destination); err != nil {
					return fmt.Errorf("uninstall skill from %s: %w", destination, err)
				}
				fmt.Fprintf(command.OutOrStdout(), "Uninstalled skill from %s\n", destination)
			}
			return nil
		},
	}
}

func skillDestinations(home string) []string {
	destinations := []string{filepath.Join(home, ".agents", "skills", skillName)}
	claudeDirectory := filepath.Join(home, ".claude")
	if info, err := os.Stat(claudeDirectory); err == nil && info.IsDir() {
		destinations = append(destinations, filepath.Join(claudeDirectory, "skills", skillName))
	}
	return destinations
}

func installSkill(destination string) error {
	if err := os.RemoveAll(destination); err != nil {
		return err
	}
	return fs.WalkDir(projectskills.JDeenCLI, skillName, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(skillName, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		contents, err := fs.ReadFile(projectskills.JDeenCLI, path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, contents, 0o644)
	})
}

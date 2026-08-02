package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/ziyan-junaideen/jdeen-cli/internal/config"
	"github.com/ziyan-junaideen/jdeen-cli/internal/output"
)

func newProfilesCommand(options *globalOptions) *cobra.Command {
	profiles := &cobra.Command{Use: "profiles", Short: "Manage API profiles"}
	list := &cobra.Command{Use: "list", Short: "List configured profiles", RunE: func(command *cobra.Command, _ []string) error {
		_, file, err := config.Load(config.Overrides{})
		if err != nil {
			return err
		}
		if options.jsonOutput {
			return output.JSON(command.OutOrStdout(), file)
		}
		names := make([]string, 0, len(file.Profiles))
		for name := range file.Profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			marker := " "
			if name == file.ActiveProfile {
				marker = "*"
			}
			fmt.Fprintf(command.OutOrStdout(), "%s %s\t%s\n", marker, name, file.Profiles[name].APIURL)
		}
		return nil
	}}
	use := &cobra.Command{Use: "use <name>", Short: "Set the active profile", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		_, file, err := config.Load(config.Overrides{})
		if err != nil {
			return err
		}
		if _, ok := file.Profiles[args[0]]; !ok {
			return fmt.Errorf("profile %q is not configured", args[0])
		}
		file.ActiveProfile = args[0]
		if err := config.Save(file); err != nil {
			return err
		}
		fmt.Fprintf(command.OutOrStdout(), "Active profile is now %q\n", args[0])
		return nil
	}}
	var apiURL, caCert string
	var clearCACert bool
	set := &cobra.Command{Use: "set <name>", Short: "Create or update a profile", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		_, file, err := config.Load(config.Overrides{})
		if err != nil {
			return err
		}
		profile := file.Profiles[args[0]]
		if apiURL != "" {
			profile.APIURL, err = config.NormalizeAPIURL(apiURL)
			if err != nil {
				return err
			}
		}
		if caCert != "" && clearCACert {
			return fmt.Errorf("--ca-cert and --clear-ca-cert cannot be combined")
		}
		if caCert != "" {
			profile.CACert = caCert
		} else if clearCACert {
			profile.CACert = ""
		}
		if options.insecureSet {
			profile.InsecureSkipVerify = options.insecureSkipVerify
		}
		if profile.APIURL == "" {
			return fmt.Errorf("profile %q requires --api-url", args[0])
		}
		insecure := profile.InsecureSkipVerify
		if _, err := config.Resolve(
			config.File{ActiveProfile: args[0], Profiles: map[string]config.Profile{args[0]: profile}},
			config.Overrides{ProfileName: args[0], APIURL: profile.APIURL, CACert: profile.CACert, InsecureSkipVerify: &insecure},
			"",
		); err != nil {
			return err
		}
		file.Profiles[args[0]] = profile
		if err := config.Save(file); err != nil {
			return err
		}
		fmt.Fprintf(command.OutOrStdout(), "Saved profile %q\n", args[0])
		return nil
	}}
	set.Flags().StringVar(&apiURL, "api-url", "", "JDeen API base URL")
	set.Flags().StringVar(&caCert, "ca-cert", "", "CA certificate path")
	set.Flags().BoolVar(&clearCACert, "clear-ca-cert", false, "remove the configured CA certificate")
	profiles.AddCommand(list, use, set)
	return profiles
}

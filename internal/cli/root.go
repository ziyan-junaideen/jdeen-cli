package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/ziyan-junaideen/jdeen-cli/internal/config"
	"github.com/ziyan-junaideen/jdeen-cli/internal/jdeenapi"
	"github.com/ziyan-junaideen/jdeen-cli/internal/secrets"
)

var version = "dev"

type globalOptions struct {
	profileName        string
	apiURL             string
	caCert             string
	insecureSkipVerify bool
	insecureSet        bool
	jsonOutput         bool
}

func NewRootCommand() *cobra.Command {
	options := &globalOptions{}
	root := &cobra.Command{
		Use:           "jdeen",
		Short:         "Command line client for the JDeen JSON:API",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&options.profileName, "profile", "", "profile name")
	root.PersistentFlags().StringVar(&options.apiURL, "api-url", "", "JDeen API base URL")
	root.PersistentFlags().StringVar(&options.caCert, "ca-cert", "", "CA certificate path for local development")
	root.PersistentFlags().BoolVar(&options.insecureSkipVerify, "insecure-skip-verify", false, "skip TLS verification for non-production development endpoints")
	root.PersistentFlags().BoolVar(&options.jsonOutput, "json", false, "print JSON output")
	root.PersistentPreRunE = func(command *cobra.Command, _ []string) error {
		options.insecureSet = command.Flags().Changed("insecure-skip-verify") || command.InheritedFlags().Changed("insecure-skip-verify")
		return nil
	}
	root.AddCommand(newProfilesCommand(options))
	root.AddCommand(newAuthCommand(options))
	root.AddCommand(newResourceCommand(options, postDefinition()))
	root.AddCommand(newResourceCommand(options, categoryDefinition()))
	root.AddCommand(newResourceCommand(options, uploadDefinition()))
	root.AddCommand(newResourceCommand(options, commentDefinition()))
	root.AddCommand(newResourceCommand(options, userDefinition()))
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	return root
}

func loadRuntime(options *globalOptions) (config.Runtime, error) {
	overrides := config.Overrides{ProfileName: options.profileName, APIURL: options.apiURL, CACert: options.caCert}
	if options.insecureSet {
		overrides.InsecureSkipVerify = &options.insecureSkipVerify
	}
	runtime, _, err := config.Load(overrides)
	if err != nil {
		return config.Runtime{}, err
	}
	if runtime.InsecureSkipVerify {
		fmt.Fprintf(os.Stderr, "warning: TLS verification is disabled for profile %q\n", runtime.ProfileName)
	}
	return runtime, nil
}

func newAPIClient(options *globalOptions) (*jdeenapi.Client, config.Runtime, error) {
	runtime, err := loadRuntime(options)
	if err != nil {
		return nil, config.Runtime{}, err
	}
	accessToken := os.Getenv("JDEEN_ACCESS_TOKEN")
	var store secrets.Store
	store, storeErr := secrets.Open()
	if storeErr != nil && accessToken == "" {
		return nil, config.Runtime{}, storeErr
	}
	client, err := jdeenapi.New(jdeenapi.Config{
		APIURL: runtime.APIURL, ProfileName: runtime.ProfileName, CACert: runtime.CACert,
		InsecureSkipVerify: runtime.InsecureSkipVerify, AccessToken: accessToken,
		Version: version, Store: store,
	})
	return client, runtime, err
}

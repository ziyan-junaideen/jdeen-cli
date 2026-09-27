package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"
	"github.com/ziyan-junaideen/jdeen-cli/internal/jsonapi"
)

type tagFlags struct {
	name, slug string
	response   responseFlags
}

func (flags *tagFlags) bind(command *cobra.Command) {
	command.Flags().StringVar(&flags.name, "name", "", "tag name")
	command.Flags().StringVar(&flags.slug, "slug", "", "tag slug")
	flags.response.bind(command)
}

func addTagCreate(parent *cobra.Command, options *globalOptions) {
	flags := &tagFlags{}
	command := &cobra.Command{Use: "create", Short: "Create a tag", RunE: func(command *cobra.Command, _ []string) error {
		if flags.name == "" {
			return errors.New("--name is required")
		}
		attributes := map[string]any{"name": flags.name}
		setNonEmpty(attributes, "slug", flags.slug)
		query, err := flags.response.options()
		if err != nil {
			return err
		}
		client, _, err := newAPIClient(options)
		if err != nil {
			return err
		}
		resource, document, err := client.Create(context.Background(), "tags", jsonapi.ResourceObject{Type: "tags", Attributes: attributes}, query)
		if err != nil {
			return err
		}
		return renderWriteResult(command, options, resource, document)
	}}
	flags.bind(command)
	parent.AddCommand(command)
}

func addTagUpdate(parent *cobra.Command, options *globalOptions) {
	flags := &tagFlags{}
	command := &cobra.Command{Use: "update <id>", Short: "Update a tag", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if err := validateID(args[0]); err != nil {
			return err
		}
		attributes := map[string]any{}
		for name, value := range map[string]string{"name": flags.name, "slug": flags.slug} {
			if command.Flags().Changed(name) {
				attributes[name] = value
			}
		}
		if len(attributes) == 0 {
			return errors.New("at least one attribute change is required")
		}
		query, err := flags.response.options()
		if err != nil {
			return err
		}
		client, _, err := newAPIClient(options)
		if err != nil {
			return err
		}
		resource, document, err := client.Update(context.Background(), "tags", args[0], jsonapi.ResourceObject{Type: "tags", ID: args[0], Attributes: attributes}, query)
		if err != nil {
			return err
		}
		return renderWriteResult(command, options, resource, document)
	}}
	flags.bind(command)
	parent.AddCommand(command)
}

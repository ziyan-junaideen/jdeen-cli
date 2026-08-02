package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"
	"github.com/ziyan-junaideen/jdeen-cli/internal/jsonapi"
)

type categoryFlags struct {
	name, slug, description string
	clearDescription        bool
	response                responseFlags
}

func (flags *categoryFlags) bind(command *cobra.Command, create bool) {
	command.Flags().StringVar(&flags.name, "name", "", "category name")
	command.Flags().StringVar(&flags.slug, "slug", "", "category slug")
	command.Flags().StringVar(&flags.description, "description", "", "category description")
	if !create {
		command.Flags().BoolVar(&flags.clearDescription, "clear-description", false, "set description to null")
	}
	flags.response.bind(command)
}

func addCategoryCreate(parent *cobra.Command, options *globalOptions) {
	flags := &categoryFlags{}
	command := &cobra.Command{Use: "create", Short: "Create a category", RunE: func(command *cobra.Command, _ []string) error {
		if flags.name == "" {
			return errors.New("--name is required")
		}
		attributes := map[string]any{"name": flags.name}
		setNonEmpty(attributes, "slug", flags.slug)
		setNonEmpty(attributes, "description", flags.description)
		query, err := flags.response.options()
		if err != nil {
			return err
		}
		client, _, err := newAPIClient(options)
		if err != nil {
			return err
		}
		resource, document, err := client.Create(context.Background(), "categories", jsonapi.ResourceObject{Type: "categories", Attributes: attributes}, query)
		if err != nil {
			return err
		}
		return renderWriteResult(command, options, resource, document)
	}}
	flags.bind(command, true)
	parent.AddCommand(command)
}

func addCategoryUpdate(parent *cobra.Command, options *globalOptions) {
	flags := &categoryFlags{}
	command := &cobra.Command{Use: "update <id>", Short: "Update a category", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if err := validateID(args[0]); err != nil {
			return err
		}
		attributes := map[string]any{}
		for name, value := range map[string]string{"name": flags.name, "slug": flags.slug, "description": flags.description} {
			if command.Flags().Changed(name) {
				attributes[name] = value
			}
		}
		if flags.clearDescription {
			if command.Flags().Changed("description") {
				return errors.New("--description and --clear-description cannot be combined")
			}
			nullAttribute(attributes, "description")
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
		resource, document, err := client.Update(context.Background(), "categories", args[0], jsonapi.ResourceObject{Type: "categories", ID: args[0], Attributes: attributes}, query)
		if err != nil {
			return err
		}
		return renderWriteResult(command, options, resource, document)
	}}
	flags.bind(command, false)
	parent.AddCommand(command)
}

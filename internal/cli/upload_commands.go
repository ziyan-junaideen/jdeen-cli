package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/ziyan-junaideen/jdeen-cli/internal/jsonapi"
)

type uploadFlags struct {
	file, description, altText string
	clearAltText               bool
	response                   responseFlags
}

func (flags *uploadFlags) bindCreate(command *cobra.Command) {
	command.Flags().StringVar(&flags.file, "file", "", "image file to upload")
	command.Flags().StringVar(&flags.description, "description", "", "image description")
	command.Flags().StringVar(&flags.altText, "alt-text", "", "accessible image alternative text")
}

func (flags *uploadFlags) bindUpdate(command *cobra.Command) {
	command.Flags().StringVar(&flags.description, "description", "", "image description")
	command.Flags().StringVar(&flags.altText, "alt-text", "", "accessible image alternative text")
	command.Flags().BoolVar(&flags.clearAltText, "clear-alt-text", false, "set alt_text to null")
	flags.response.bind(command)
}

func addUploadCreate(parent *cobra.Command, options *globalOptions) {
	flags := &uploadFlags{}
	command := &cobra.Command{Use: "create", Short: "Upload a managed image", RunE: func(command *cobra.Command, _ []string) error {
		if flags.file == "" || flags.description == "" {
			return errors.New("--file and --description are required")
		}
		info, err := os.Stat(flags.file)
		if err != nil {
			return fmt.Errorf("inspect upload: %w", err)
		}
		if !info.Mode().IsRegular() {
			return errors.New("--file must name a regular file")
		}
		if info.Size() > 8_000_000 {
			return errors.New("upload exceeds the 8,000,000-byte API limit")
		}
		client, _, err := newAPIClient(options)
		if err != nil {
			return err
		}
		var altText *string
		if command.Flags().Changed("alt-text") {
			altText = stringPointer(flags.altText)
		}
		resource, document, checksum, err := client.Upload(context.Background(), flags.file, flags.description, altText)
		if err != nil {
			return err
		}
		if !options.jsonOutput {
			fmt.Fprintf(command.ErrOrStderr(), "SHA-256: %s\n", checksum)
		}
		return renderWriteResult(command, options, resource, document)
	}}
	flags.bindCreate(command)
	parent.AddCommand(command)
}

func addUploadUpdate(parent *cobra.Command, options *globalOptions) {
	flags := &uploadFlags{}
	command := &cobra.Command{Use: "update <id>", Short: "Update upload metadata", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if err := validateID(args[0]); err != nil {
			return err
		}
		attributes := map[string]any{}
		if command.Flags().Changed("description") {
			attributes["description"] = flags.description
		}
		if command.Flags().Changed("alt-text") {
			if flags.clearAltText {
				return errors.New("--alt-text and --clear-alt-text cannot be combined")
			}
			attributes["alt_text"] = flags.altText
		} else if flags.clearAltText {
			nullAttribute(attributes, "alt_text")
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
		resource, document, err := client.Update(context.Background(), "uploads", args[0], jsonapi.ResourceObject{Type: "uploads", ID: args[0], Attributes: attributes}, query)
		if err != nil {
			return err
		}
		return renderWriteResult(command, options, resource, document)
	}}
	flags.bindUpdate(command)
	parent.AddCommand(command)
}

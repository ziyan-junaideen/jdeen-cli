package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ziyan-junaideen/jdeen-cli/internal/jdeenapi"
	"github.com/ziyan-junaideen/jdeen-cli/internal/jsonapi"
)

type commentFlags struct {
	body, bodyFile, status, post, user, parent string
	response                                   responseFlags
}

func (flags *commentFlags) bind(command *cobra.Command, create bool) {
	command.Flags().StringVar(&flags.body, "body", "", "comment body")
	command.Flags().StringVar(&flags.bodyFile, "body-file", "", "read comment body from a file or - for stdin")
	command.Flags().StringVar(&flags.status, "status", "", "published or quarantined")
	if create {
		command.Flags().StringVar(&flags.post, "post", "", "post UUID")
		command.Flags().StringVar(&flags.user, "user", "", "commenter user UUID")
		command.Flags().StringVar(&flags.parent, "parent-comment", "", "parent comment UUID for a reply")
	}
	flags.response.bind(command)
}

func addCommentCreate(parent *cobra.Command, options *globalOptions) {
	flags := &commentFlags{}
	command := &cobra.Command{Use: "create", Short: "Create a comment or reply", RunE: func(command *cobra.Command, _ []string) error {
		body, err := readContent(command.InOrStdin(), flags.body, flags.bodyFile, "body", "body-file")
		if err != nil {
			return err
		}
		if body == "" || flags.post == "" || flags.user == "" {
			return errors.New("--post, --user, and one of --body or --body-file are required")
		}
		if err := validateIDs([]string{flags.post, flags.user}); err != nil {
			return err
		}
		if flags.parent != "" {
			if err := validateID(flags.parent); err != nil {
				return fmt.Errorf("parent comment: %w", err)
			}
		}
		if err := validateCommentStatus(flags.status); err != nil {
			return err
		}
		attributes := map[string]any{"body": body}
		setNonEmpty(attributes, "status", flags.status)
		relationships := map[string]jsonapi.RelationshipDocument{
			"post": jdeenapi.RelationshipOne("posts", flags.post),
			"user": jdeenapi.RelationshipOne("users", flags.user),
		}
		if flags.parent != "" {
			relationships["parent_comment"] = jdeenapi.RelationshipOne("post_comments", flags.parent)
		}
		query, err := flags.response.options()
		if err != nil {
			return err
		}
		client, _, err := newAPIClient(options)
		if err != nil {
			return err
		}
		resource, document, err := client.Create(context.Background(), "post_comments", jsonapi.ResourceObject{Type: "post_comments", Attributes: attributes, Relationships: relationships}, query)
		if err != nil {
			return err
		}
		return renderWriteResult(command, options, resource, document)
	}}
	flags.bind(command, true)
	parent.AddCommand(command)
}

func addCommentUpdate(parent *cobra.Command, options *globalOptions) {
	flags := &commentFlags{}
	command := &cobra.Command{Use: "update <id>", Short: "Update a comment", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if err := validateID(args[0]); err != nil {
			return err
		}
		body, err := readContent(command.InOrStdin(), flags.body, flags.bodyFile, "body", "body-file")
		if err != nil {
			return err
		}
		if err := validateCommentStatus(flags.status); err != nil {
			return err
		}
		attributes := map[string]any{}
		if command.Flags().Changed("body") || command.Flags().Changed("body-file") {
			attributes["body"] = body
		}
		if command.Flags().Changed("status") {
			attributes["status"] = flags.status
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
		resource, document, err := client.Update(context.Background(), "post_comments", args[0], jsonapi.ResourceObject{Type: "post_comments", ID: args[0], Attributes: attributes}, query)
		if err != nil {
			return err
		}
		return renderWriteResult(command, options, resource, document)
	}}
	flags.bind(command, false)
	parent.AddCommand(command)
}

func validateCommentStatus(status string) error {
	if status != "" && status != "published" && status != "quarantined" {
		return errors.New("--status must be published or quarantined")
	}
	return nil
}

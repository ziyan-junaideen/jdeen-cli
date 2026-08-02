package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ziyan-junaideen/jdeen-cli/internal/jdeenapi"
	"github.com/ziyan-junaideen/jdeen-cli/internal/jsonapi"
	"github.com/ziyan-junaideen/jdeen-cli/internal/output"
	"golang.org/x/term"
)

type resourceDefinition struct {
	use           string
	aliases       []string
	apiType       string
	short         string
	readOnly      bool
	relationships map[string]bool // true means to-many
	addCreate     func(*cobra.Command, *globalOptions)
	addUpdate     func(*cobra.Command, *globalOptions)
}

func newResourceCommand(options *globalOptions, definition resourceDefinition) *cobra.Command {
	resource := &cobra.Command{Use: definition.use, Aliases: definition.aliases, Short: definition.short}
	var listFlags queryFlags
	list := &cobra.Command{Use: "list", Short: "List " + definition.use, RunE: func(command *cobra.Command, _ []string) error {
		query, err := listFlags.options()
		if err != nil {
			return err
		}
		client, _, err := newAPIClient(options)
		if err != nil {
			return err
		}
		resources, document, err := client.List(context.Background(), definition.apiType, query)
		if err != nil {
			return err
		}
		if options.jsonOutput {
			return output.JSON(command.OutOrStdout(), document)
		}
		return output.Collection(command.OutOrStdout(), definition.apiType, resources, document.Links)
	}}
	listFlags.bind(list, true)

	var showFlags queryFlags
	show := &cobra.Command{Use: "show <id>", Short: "Show one " + strings.TrimSuffix(definition.use, "s"), Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if err := validateID(args[0]); err != nil {
			return err
		}
		query, err := showFlags.options()
		if err != nil {
			return err
		}
		client, _, err := newAPIClient(options)
		if err != nil {
			return err
		}
		resource, document, err := client.Show(context.Background(), definition.apiType, args[0], query)
		if err != nil {
			return err
		}
		if options.jsonOutput {
			return output.JSON(command.OutOrStdout(), document)
		}
		return output.Member(command.OutOrStdout(), resource)
	}}
	showFlags.bind(show, false)

	var relatedFlags queryFlags
	related := &cobra.Command{Use: "related <id> <relationship>", Short: "Fetch a related resource or collection", Args: cobra.ExactArgs(2), RunE: func(command *cobra.Command, args []string) error {
		if err := validateID(args[0]); err != nil {
			return err
		}
		many, ok := definition.relationships[args[1]]
		if !ok {
			return fmt.Errorf("unsupported %s relationship %q; allowed: %s", definition.apiType, args[1], relationshipNames(definition.relationships))
		}
		query, err := relatedFlags.options()
		if err != nil {
			return err
		}
		if !many && (query.PageSize > 0 || query.After != "" || query.Before != "" || query.PageURL != "") {
			return errors.New("pagination flags cannot be used with a to-one relationship")
		}
		client, _, err := newAPIClient(options)
		if err != nil {
			return err
		}
		data, document, err := client.Related(context.Background(), definition.apiType, args[0], args[1], query)
		if err != nil {
			return err
		}
		if options.jsonOutput {
			return output.JSON(command.OutOrStdout(), document)
		}
		resources, collection, err := jdeenapi.ParseRelated(data)
		if err != nil {
			return err
		}
		if len(resources) == 0 {
			fmt.Fprintln(command.OutOrStdout(), "No related resource")
			return nil
		}
		if collection {
			return output.Collection(command.OutOrStdout(), resources[0].Type, resources, document.Links)
		}
		return output.Member(command.OutOrStdout(), resources[0])
	}}
	relatedFlags.bind(related, true)
	resource.AddCommand(list, show, related)

	if !definition.readOnly {
		definition.addCreate(resource, options)
		definition.addUpdate(resource, options)
		resource.AddCommand(newDeleteCommand(options, definition))
	}
	return resource
}

func newDeleteCommand(options *globalOptions, definition resourceDefinition) *cobra.Command {
	var yes bool
	command := &cobra.Command{Use: "delete <id>", Short: "Permanently delete one " + strings.TrimSuffix(definition.use, "s"), Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if err := validateID(args[0]); err != nil {
			return err
		}
		if !yes {
			file, terminal := command.InOrStdin().(*os.File)
			if !terminal || !term.IsTerminal(int(file.Fd())) {
				return errors.New("deletion requires confirmation; pass --yes in non-interactive use")
			}
			fmt.Fprintf(command.ErrOrStderr(), "Permanently delete %s %s? [y/N] ", definition.apiType, args[0])
			answer, _ := bufio.NewReader(command.InOrStdin()).ReadString('\n')
			if !strings.EqualFold(strings.TrimSpace(answer), "y") && !strings.EqualFold(strings.TrimSpace(answer), "yes") {
				return errors.New("deletion cancelled")
			}
		}
		client, _, err := newAPIClient(options)
		if err != nil {
			return err
		}
		if err := client.Delete(context.Background(), definition.apiType, args[0]); err != nil {
			return err
		}
		return output.Deleted(command.OutOrStdout(), definition.apiType, args[0], options.jsonOutput)
	}}
	command.Flags().BoolVarP(&yes, "yes", "y", false, "skip the interactive confirmation")
	return command
}

func renderWriteResult(command *cobra.Command, options *globalOptions, resource jsonapi.Resource, document jsonapi.Document) error {
	if options.jsonOutput {
		return output.JSON(command.OutOrStdout(), document)
	}
	return output.Member(command.OutOrStdout(), resource)
}

type responseFlags struct {
	includes []string
	fields   []string
}

func (flags *responseFlags) bind(command *cobra.Command) {
	command.Flags().StringArrayVar(&flags.includes, "include", nil, "relationship path to include; repeat or comma-separate")
	command.Flags().StringArrayVar(&flags.fields, "fields", nil, "sparse fieldset as type=field1,field2; repeat by type")
}

func (flags responseFlags) options() (jdeenapi.QueryOptions, error) {
	query := queryFlags{includes: flags.includes, fields: flags.fields}
	return query.options()
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func validateID(id string) error {
	if !uuidPattern.MatchString(id) {
		return fmt.Errorf("invalid UUID %q", id)
	}
	return nil
}

func validateIDs(ids []string) error {
	for _, id := range ids {
		if err := validateID(id); err != nil {
			return err
		}
	}
	return nil
}

func relationshipNames(relationships map[string]bool) string {
	names := make([]string, 0, len(relationships))
	for name := range relationships {
		names = append(names, name)
	}
	sortStrings(names)
	return strings.Join(names, ", ")
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func readContent(reader io.Reader, value, filePath, valueFlag, fileFlag string) (string, error) {
	if value != "" && filePath != "" {
		return "", fmt.Errorf("--%s and --%s cannot be combined", valueFlag, fileFlag)
	}
	if filePath == "-" {
		contents, err := io.ReadAll(reader)
		return string(contents), err
	}
	if filePath != "" {
		contents, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", filePath, err)
		}
		return string(contents), nil
	}
	return value, nil
}

func stringPointer(value string) *string { return &value }

func nullAttribute(attributes map[string]any, name string) { attributes[name] = nil }

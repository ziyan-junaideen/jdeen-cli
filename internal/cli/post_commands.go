package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/ziyan-junaideen/jdeen-cli/internal/jdeenapi"
	"github.com/ziyan-junaideen/jdeen-cli/internal/jsonapi"
)

type postFlags struct {
	title, slug, state, content, contentFile, summary, format, publishedAt string
	featured                                                               bool
	author, bannerUpload                                                   string
	categories, tags                                                       []string
	clearSummary, clearCategories, clearTags, clearBanner                  bool
	response                                                               responseFlags
}

func (flags *postFlags) bind(command *cobra.Command, create bool) {
	command.Flags().StringVar(&flags.title, "title", "", "post title")
	command.Flags().StringVar(&flags.slug, "slug", "", "post slug")
	command.Flags().StringVar(&flags.state, "state", "", "draft, published, or archived")
	command.Flags().StringVar(&flags.content, "content", "", "post content")
	command.Flags().StringVar(&flags.contentFile, "content-file", "", "read post content from a file or - for stdin")
	command.Flags().StringVar(&flags.summary, "summary", "", "post summary")
	command.Flags().StringVar(&flags.format, "format", "", "markdown or html")
	command.Flags().StringVar(&flags.publishedAt, "published-at", "", "publication time as an RFC 3339 UTC timestamp")
	command.Flags().BoolVar(&flags.featured, "featured", false, "whether the post is featured")
	command.Flags().StringVar(&flags.author, "author", "", "author user UUID")
	command.Flags().StringArrayVar(&flags.categories, "category", nil, "category UUID; repeat to set multiple categories")
	command.Flags().StringArrayVar(&flags.tags, "tag", nil, "tag UUID; repeat to set multiple tags")
	command.Flags().StringVar(&flags.bannerUpload, "banner-upload", "", "banner upload UUID")
	if !create {
		command.Flags().BoolVar(&flags.clearSummary, "clear-summary", false, "set summary to null")
		command.Flags().BoolVar(&flags.clearCategories, "clear-categories", false, "replace categories with an empty set")
		command.Flags().BoolVar(&flags.clearTags, "clear-tags", false, "replace tags with an empty set")
		command.Flags().BoolVar(&flags.clearBanner, "clear-banner-upload", false, "set banner_upload to null")
	}
	flags.response.bind(command)
}

func addPostCreate(parent *cobra.Command, options *globalOptions) {
	flags := &postFlags{}
	command := &cobra.Command{Use: "create", Short: "Create a post", RunE: func(command *cobra.Command, _ []string) error {
		content, err := readContent(command.InOrStdin(), flags.content, flags.contentFile, "content", "content-file")
		if err != nil {
			return err
		}
		if flags.title == "" || content == "" || flags.author == "" {
			return errors.New("--title, --author, and one of --content or --content-file are required")
		}
		if err := validateID(flags.author); err != nil {
			return fmt.Errorf("author: %w", err)
		}
		if err := validateIDs(flags.categories); err != nil {
			return fmt.Errorf("category: %w", err)
		}
		if err := validateIDs(flags.tags); err != nil {
			return fmt.Errorf("tag: %w", err)
		}
		if flags.bannerUpload != "" {
			if err := validateID(flags.bannerUpload); err != nil {
				return fmt.Errorf("banner upload: %w", err)
			}
		}
		if err := validatePostEnums(flags.state, flags.format); err != nil {
			return err
		}
		if err := validatePublishedAt(flags.publishedAt); err != nil {
			return err
		}
		attributes := map[string]any{"title": flags.title, "content": content}
		setNonEmpty(attributes, "slug", flags.slug)
		setNonEmpty(attributes, "state", flags.state)
		setNonEmpty(attributes, "summary", flags.summary)
		setNonEmpty(attributes, "format", flags.format)
		setNonEmpty(attributes, "published_at", flags.publishedAt)
		if command.Flags().Changed("featured") {
			attributes["featured"] = flags.featured
		}
		relationships := map[string]jsonapi.RelationshipDocument{"author": jdeenapi.RelationshipOne("users", flags.author)}
		if len(flags.categories) > 0 {
			relationships["categories"] = jdeenapi.RelationshipMany("categories", flags.categories)
		}
		if len(flags.tags) > 0 {
			relationships["tags"] = jdeenapi.RelationshipMany("tags", flags.tags)
		}
		if flags.bannerUpload != "" {
			relationships["banner_upload"] = jdeenapi.RelationshipOne("uploads", flags.bannerUpload)
		}
		query, err := flags.response.options()
		if err != nil {
			return err
		}
		client, _, err := newAPIClient(options)
		if err != nil {
			return err
		}
		resource, document, err := client.Create(context.Background(), "posts", jsonapi.ResourceObject{Type: "posts", Attributes: attributes, Relationships: relationships}, query)
		if err != nil {
			return err
		}
		return renderWriteResult(command, options, resource, document)
	}}
	flags.bind(command, true)
	parent.AddCommand(command)
}

func addPostUpdate(parent *cobra.Command, options *globalOptions) {
	flags := &postFlags{}
	command := &cobra.Command{Use: "update <id>", Short: "Update a post", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if err := validateID(args[0]); err != nil {
			return err
		}
		content, err := readContent(command.InOrStdin(), flags.content, flags.contentFile, "content", "content-file")
		if err != nil {
			return err
		}
		if err := validatePostEnums(flags.state, flags.format); err != nil {
			return err
		}
		if err := validatePublishedAt(flags.publishedAt); err != nil {
			return err
		}
		attributes := map[string]any{}
		for name, value := range map[string]string{"title": flags.title, "slug": flags.slug, "state": flags.state, "summary": flags.summary, "format": flags.format} {
			if command.Flags().Changed(name) {
				attributes[name] = value
			}
		}
		if command.Flags().Changed("published-at") {
			attributes["published_at"] = flags.publishedAt
		}
		if command.Flags().Changed("content") || command.Flags().Changed("content-file") {
			attributes["content"] = content
		}
		if command.Flags().Changed("featured") {
			attributes["featured"] = flags.featured
		}
		if flags.clearSummary {
			if command.Flags().Changed("summary") {
				return errors.New("--summary and --clear-summary cannot be combined")
			}
			nullAttribute(attributes, "summary")
		}
		relationships := map[string]jsonapi.RelationshipDocument{}
		if flags.author != "" {
			if err := validateID(flags.author); err != nil {
				return err
			}
			relationships["author"] = jdeenapi.RelationshipOne("users", flags.author)
		}
		if command.Flags().Changed("category") {
			if flags.clearCategories {
				return errors.New("--category and --clear-categories cannot be combined")
			}
			if err := validateIDs(flags.categories); err != nil {
				return err
			}
			relationships["categories"] = jdeenapi.RelationshipMany("categories", flags.categories)
		} else if flags.clearCategories {
			relationships["categories"] = jdeenapi.RelationshipMany("categories", nil)
		}
		if command.Flags().Changed("tag") {
			if flags.clearTags {
				return errors.New("--tag and --clear-tags cannot be combined")
			}
			if err := validateIDs(flags.tags); err != nil {
				return err
			}
			relationships["tags"] = jdeenapi.RelationshipMany("tags", flags.tags)
		} else if flags.clearTags {
			relationships["tags"] = jdeenapi.RelationshipMany("tags", nil)
		}
		if flags.bannerUpload != "" {
			if flags.clearBanner {
				return errors.New("--banner-upload and --clear-banner-upload cannot be combined")
			}
			if err := validateID(flags.bannerUpload); err != nil {
				return err
			}
			relationships["banner_upload"] = jdeenapi.RelationshipOne("uploads", flags.bannerUpload)
		} else if flags.clearBanner {
			relationships["banner_upload"] = jdeenapi.RelationshipNull()
		}
		if len(attributes) == 0 && len(relationships) == 0 {
			return errors.New("at least one attribute or relationship change is required")
		}
		query, err := flags.response.options()
		if err != nil {
			return err
		}
		client, _, err := newAPIClient(options)
		if err != nil {
			return err
		}
		resource, document, err := client.Update(context.Background(), "posts", args[0], jsonapi.ResourceObject{Type: "posts", ID: args[0], Attributes: attributes, Relationships: relationships}, query)
		if err != nil {
			return err
		}
		return renderWriteResult(command, options, resource, document)
	}}
	flags.bind(command, false)
	parent.AddCommand(command)
}

func validatePublishedAt(value string) error {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return errors.New("--published-at must be an RFC 3339 UTC timestamp, for example 2020-04-15T10:30:00Z")
	}
	_, offset := parsed.Zone()
	if offset != 0 {
		return errors.New("--published-at must use UTC (Z or a zero offset)")
	}
	return nil
}

func validatePostEnums(state, format string) error {
	if state != "" && state != "draft" && state != "published" && state != "archived" {
		return errors.New("--state must be draft, published, or archived")
	}
	if format != "" && format != "markdown" && format != "html" {
		return errors.New("--format must be markdown or html")
	}
	return nil
}

func setNonEmpty(values map[string]any, key, value string) {
	if value != "" {
		values[key] = value
	}
}

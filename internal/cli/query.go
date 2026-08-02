package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ziyan-junaideen/jdeen-cli/internal/jdeenapi"
)

type queryFlags struct {
	filters     []string
	includes    []string
	fields      []string
	sort        string
	pageSize    int
	pageSizeSet bool
	after       string
	before      string
	pageURL     string
}

func (flags *queryFlags) bind(command *cobra.Command, pagination bool) {
	command.Flags().StringArrayVar(&flags.filters, "filter", nil, "filter as key=value; repeat for multiple filters")
	command.Flags().StringArrayVar(&flags.includes, "include", nil, "relationship path to include; repeat or comma-separate")
	command.Flags().StringArrayVar(&flags.fields, "fields", nil, "sparse fieldset as type=field1,field2; repeat by type")
	command.Flags().StringVar(&flags.sort, "sort", "", "comma-separated JSON:API sort fields")
	if pagination {
		command.Flags().Var(pageSizeValue{flags: flags}, "page-size", "page size from 1 to 100")
		command.Flags().StringVar(&flags.after, "after", "", "opaque cursor for the next page")
		command.Flags().StringVar(&flags.before, "before", "", "opaque cursor for the previous page")
		command.Flags().StringVar(&flags.pageURL, "page-url", "", "follow an exact pagination URL returned by the API")
	}
}

type pageSizeValue struct{ flags *queryFlags }

func (value pageSizeValue) String() string {
	if value.flags == nil || !value.flags.pageSizeSet {
		return ""
	}
	return strconv.Itoa(value.flags.pageSize)
}

func (value pageSizeValue) Set(raw string) error {
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("invalid page size %q", raw)
	}
	value.flags.pageSize = parsed
	value.flags.pageSizeSet = true
	return nil
}

func (pageSizeValue) Type() string { return "int" }

func (flags queryFlags) options() (jdeenapi.QueryOptions, error) {
	if flags.pageSizeSet && (flags.pageSize < 1 || flags.pageSize > 100) {
		return jdeenapi.QueryOptions{}, errors.New("--page-size must be between 1 and 100")
	}
	if flags.after != "" && flags.before != "" {
		return jdeenapi.QueryOptions{}, errors.New("--after and --before cannot be used together")
	}
	if flags.pageURL != "" && (len(flags.filters) > 0 || len(flags.includes) > 0 || len(flags.fields) > 0 || flags.sort != "" || flags.pageSize > 0 || flags.after != "" || flags.before != "") {
		return jdeenapi.QueryOptions{}, errors.New("--page-url cannot be combined with other query flags")
	}
	filters := map[string][]string{}
	for _, entry := range flags.filters {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return jdeenapi.QueryOptions{}, fmt.Errorf("invalid filter %q; expected key=value", entry)
		}
		filters[strings.TrimSpace(key)] = append(filters[strings.TrimSpace(key)], strings.TrimSpace(value))
	}
	fields := map[string][]string{}
	for _, entry := range flags.fields {
		resourceType, values, ok := strings.Cut(entry, "=")
		if !ok || strings.TrimSpace(resourceType) == "" || strings.TrimSpace(values) == "" {
			return jdeenapi.QueryOptions{}, fmt.Errorf("invalid fieldset %q; expected type=field1,field2", entry)
		}
		fields[strings.TrimSpace(resourceType)] = append(fields[strings.TrimSpace(resourceType)], splitCSV(values)...)
	}
	return jdeenapi.QueryOptions{
		Filters: filters, Includes: splitMany(flags.includes), Fields: fields, Sort: strings.TrimSpace(flags.sort),
		PageSize: func() int {
			if flags.pageSizeSet {
				return flags.pageSize
			}
			return 0
		}(), After: flags.after, Before: flags.before, PageURL: flags.pageURL,
	}, nil
}

func splitMany(values []string) []string {
	result := []string{}
	for _, value := range values {
		result = append(result, splitCSV(value)...)
	}
	return result
}

func splitCSV(value string) []string {
	result := []string{}
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

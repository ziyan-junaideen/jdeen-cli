package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ziyan-junaideen/jdeen-cli/internal/jdeenapi"
	"github.com/ziyan-junaideen/jdeen-cli/internal/jsonapi"
)

func JSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func Collection(writer io.Writer, resourceType string, resources []jsonapi.Resource, links jsonapi.Links) error {
	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	columns := collectionColumns(resourceType)
	fmt.Fprintln(table, strings.Join(columns, "\t"))
	for _, resource := range resources {
		values := make([]string, 0, len(columns))
		for _, column := range columns {
			if column == "ID" {
				values = append(values, resource.ID)
				continue
			}
			values = append(values, attributeText(resource, strings.ToLower(strings.ReplaceAll(column, " ", "_"))))
		}
		fmt.Fprintln(table, strings.Join(values, "\t"))
	}
	if err := table.Flush(); err != nil {
		return err
	}
	if links.Next != "" {
		fmt.Fprintf(writer, "Next: %s\n", links.Next)
	}
	if links.Previous != "" {
		fmt.Fprintf(writer, "Previous: %s\n", links.Previous)
	}
	return nil
}

func Member(writer io.Writer, resource jsonapi.Resource) error {
	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	fmt.Fprintf(table, "TYPE\t%s\nID\t%s\n", resource.Type, resource.ID)
	keys := make([]string, 0, len(resource.Attributes))
	for key := range resource.Attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(table, "%s\t%s\n", strings.ToUpper(key), rawText(resource.Attributes[key]))
	}
	relationships := make([]string, 0, len(resource.Relationships))
	for key := range resource.Relationships {
		relationships = append(relationships, key)
	}
	sort.Strings(relationships)
	for _, key := range relationships {
		fmt.Fprintf(table, "%s\t%s\n", strings.ToUpper(key), relationshipText(resource.Relationships[key]))
	}
	return table.Flush()
}

func Sessions(writer io.Writer, sessions []jdeenapi.APISession) error {
	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tDEVICE\tIP ADDRESS\tLAST USED\tEXPIRES")
	for _, session := range sessions {
		lastUsed := "-"
		if session.LastUsedAt != nil {
			lastUsed = session.LastUsedAt.Local().Format(time.RFC3339)
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", session.ID, session.DeviceName, session.IPAddress, lastUsed, session.ExpiresAt.Local().Format(time.RFC3339))
	}
	return table.Flush()
}

func Deleted(writer io.Writer, resourceType, id string, jsonOutput bool) error {
	if jsonOutput {
		return JSON(writer, map[string]any{"deleted": true, "type": resourceType, "id": id})
	}
	_, err := fmt.Fprintf(writer, "Deleted %s %s\n", resourceType, id)
	return err
}

func collectionColumns(resourceType string) []string {
	switch resourceType {
	case "posts":
		return []string{"ID", "TITLE", "STATE", "FORMAT", "FEATURED", "PUBLISHED AT"}
	case "categories":
		return []string{"ID", "NAME", "SLUG", "DESCRIPTION"}
	case "uploads":
		return []string{"ID", "ORIGINAL FILENAME", "CONTENT TYPE", "BYTE SIZE", "DESCRIPTION"}
	case "post_comments":
		return []string{"ID", "STATUS", "BODY", "CREATED AT"}
	case "users":
		return []string{"ID", "NAME", "JOB TITLE"}
	default:
		return []string{"ID"}
	}
}

func attributeText(resource jsonapi.Resource, key string) string {
	value, ok := resource.Attributes[key]
	if !ok {
		return "-"
	}
	return truncate(rawText(value), 72)
}

func rawText(value json.RawMessage) string {
	if len(value) == 0 || string(value) == "null" {
		return "-"
	}
	var decoded any
	if json.Unmarshal(value, &decoded) != nil {
		return string(value)
	}
	switch typed := decoded.(type) {
	case string:
		return strings.ReplaceAll(typed, "\n", "\\n")
	case map[string]any, []any:
		encoded, _ := json.Marshal(typed)
		return string(encoded)
	default:
		return fmt.Sprint(typed)
	}
}

func relationshipText(relationship jsonapi.Relationship) string {
	if len(relationship.Data) == 0 || string(relationship.Data) == "null" {
		return "-"
	}
	var many []jsonapi.ResourceIdentifier
	if json.Unmarshal(relationship.Data, &many) == nil {
		parts := make([]string, 0, len(many))
		for _, identifier := range many {
			parts = append(parts, identifier.Type+":"+identifier.ID)
		}
		return strings.Join(parts, ", ")
	}
	var one jsonapi.ResourceIdentifier
	if json.Unmarshal(relationship.Data, &one) == nil {
		return one.Type + ":" + one.ID
	}
	return rawText(relationship.Data)
}

func truncate(value string, maximum int) string {
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	return string(runes[:maximum-1]) + "…"
}

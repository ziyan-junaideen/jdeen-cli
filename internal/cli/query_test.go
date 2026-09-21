package cli

import (
	"strings"
	"testing"
)

func TestQueryFlagsParsing(t *testing.T) {
	flags := queryFlags{
		filters:  []string{"state=draft", "state=published", "author.name=Ziyan Junaideen"},
		includes: []string{"author,categories", "comments.user"}, fields: []string{"posts=title,state"},
		pageSize: 25, pageSizeSet: true,
	}
	options, err := flags.options()
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Filters["state"]) != 2 || len(options.Includes) != 3 || len(options.Fields["posts"]) != 2 {
		t.Fatalf("unexpected options: %#v", options)
	}
}

func TestQueryFlagsRejectConflicts(t *testing.T) {
	for _, flags := range []queryFlags{
		{after: "a", before: "b"},
		{pageURL: "https://api.jdeen.com/v1/posts", sort: "title"},
		{pageSize: 0, pageSizeSet: true},
		{pageSize: 101, pageSizeSet: true},
	} {
		if _, err := flags.options(); err == nil {
			t.Fatalf("expected conflict to fail: %#v", flags)
		}
	}
}

func TestUUIDValidation(t *testing.T) {
	if err := validateID("0ee67de0-b9e9-43dd-9c50-3804533ddf80"); err != nil {
		t.Fatal(err)
	}
	if err := validateID("not-a-uuid"); err == nil {
		t.Fatal("expected invalid UUID to fail")
	}
}

func TestReadContentFromStdin(t *testing.T) {
	content, err := readContent(strings.NewReader("# Post\n\nBody\n"), "", "-", "content", "content-file")
	if err != nil {
		t.Fatal(err)
	}
	if content != "# Post\n\nBody\n" {
		t.Fatalf("content = %q", content)
	}
}

func TestValidatePublishedAt(t *testing.T) {
	for _, value := range []string{"", "2020-04-15T10:30:00Z", "2020-04-15T10:30:00+00:00"} {
		if err := validatePublishedAt(value); err != nil {
			t.Errorf("validatePublishedAt(%q) returned %v", value, err)
		}
	}
	for _, value := range []string{"2020-04-15", "2020-04-15T10:30:00+05:30", "not-a-time"} {
		if err := validatePublishedAt(value); err == nil {
			t.Errorf("validatePublishedAt(%q) unexpectedly succeeded", value)
		}
	}
}

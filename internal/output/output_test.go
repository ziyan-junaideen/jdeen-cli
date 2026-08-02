package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ziyan-junaideen/jdeen-cli/internal/jsonapi"
)

func TestCollectionAndMemberOutput(t *testing.T) {
	resource := jsonapi.Resource{ID: "post-id", Type: "posts", Attributes: map[string]json.RawMessage{
		"title": json.RawMessage(`"A dependable post"`), "state": json.RawMessage(`"draft"`), "featured": json.RawMessage(`false`),
	}}
	var collection bytes.Buffer
	if err := Collection(&collection, "posts", []jsonapi.Resource{resource}, jsonapi.Links{Next: "https://api.jdeen.com/v1/posts?page=next"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"A dependable post", "draft", "Next: https://api.jdeen.com"} {
		if !strings.Contains(collection.String(), expected) {
			t.Errorf("collection output missing %q:\n%s", expected, collection.String())
		}
	}
	var member bytes.Buffer
	if err := Member(&member, resource); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(member.String(), "TITLE") || !strings.Contains(member.String(), "post-id") {
		t.Fatalf("unexpected member output:\n%s", member.String())
	}
}

func TestDeletedJSON(t *testing.T) {
	var buffer bytes.Buffer
	if err := Deleted(&buffer, "posts", "post-id", true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buffer.String(), `"deleted": true`) {
		t.Fatalf("unexpected JSON: %s", buffer.String())
	}
}

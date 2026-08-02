package jsonapi

import "encoding/json"

const MediaType = "application/vnd.api+json"

type Document struct {
	Data     json.RawMessage `json:"data,omitempty"`
	Errors   []Error         `json:"errors,omitempty"`
	Included json.RawMessage `json:"included,omitempty"`
	Links    Links           `json:"links,omitempty"`
	Meta     json.RawMessage `json:"meta,omitempty"`
	JSONAPI  json.RawMessage `json:"jsonapi,omitempty"`
}

type Links struct {
	Self        string `json:"self,omitempty"`
	First       string `json:"first,omitempty"`
	Next        string `json:"next,omitempty"`
	Previous    string `json:"prev,omitempty"`
	DescribedBy string `json:"describedby,omitempty"`
}

type Resource struct {
	ID            string                     `json:"id"`
	Type          string                     `json:"type"`
	Attributes    map[string]json.RawMessage `json:"attributes,omitempty"`
	Relationships map[string]Relationship    `json:"relationships,omitempty"`
	Links         json.RawMessage            `json:"links,omitempty"`
	Meta          json.RawMessage            `json:"meta,omitempty"`
}

type Relationship struct {
	Data  json.RawMessage `json:"data,omitempty"`
	Links json.RawMessage `json:"links,omitempty"`
	Meta  json.RawMessage `json:"meta,omitempty"`
}

type ResourceIdentifier struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type ResourceObject struct {
	Type          string                          `json:"type"`
	ID            string                          `json:"id,omitempty"`
	Attributes    map[string]any                  `json:"attributes,omitempty"`
	Relationships map[string]RelationshipDocument `json:"relationships,omitempty"`
}

type RelationshipDocument struct {
	Data any `json:"data"`
}

type WriteDocument struct {
	Data ResourceObject `json:"data"`
}

type Error struct {
	ID     string          `json:"id,omitempty"`
	Status any             `json:"status,omitempty"`
	Code   string          `json:"code,omitempty"`
	Title  string          `json:"title,omitempty"`
	Detail string          `json:"detail,omitempty"`
	Source json.RawMessage `json:"source,omitempty"`
	Meta   json.RawMessage `json:"meta,omitempty"`
}

func DecodeResource(data json.RawMessage) (Resource, error) {
	var resource Resource
	err := json.Unmarshal(data, &resource)
	return resource, err
}

func DecodeCollection(data json.RawMessage) ([]Resource, error) {
	var resources []Resource
	err := json.Unmarshal(data, &resources)
	return resources, err
}

func Identifier(resourceType, id string) ResourceIdentifier {
	return ResourceIdentifier{Type: resourceType, ID: id}
}

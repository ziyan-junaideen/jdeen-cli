package jdeenapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ziyan-junaideen/jdeen-cli/internal/jsonapi"
)

func (client *Client) List(ctx context.Context, resourceType string, query QueryOptions) ([]jsonapi.Resource, jsonapi.Document, error) {
	document, err := client.resourceRequest(ctx, http.MethodGet, resourceType, query, nil, "")
	if err != nil {
		return nil, jsonapi.Document{}, err
	}
	resources, err := jsonapi.DecodeCollection(document.Data)
	return resources, document, err
}

func (client *Client) Show(ctx context.Context, resourceType, id string, query QueryOptions) (jsonapi.Resource, jsonapi.Document, error) {
	path := resourceType + "/" + url.PathEscape(id)
	document, err := client.resourceRequest(ctx, http.MethodGet, path, query, nil, "")
	if err != nil {
		return jsonapi.Resource{}, jsonapi.Document{}, err
	}
	resource, err := jsonapi.DecodeResource(document.Data)
	return resource, document, err
}

func (client *Client) Related(ctx context.Context, resourceType, id, relationship string, query QueryOptions) (json.RawMessage, jsonapi.Document, error) {
	path := resourceType + "/" + url.PathEscape(id) + "/relationships/" + url.PathEscape(relationship)
	document, err := client.resourceRequest(ctx, http.MethodGet, path, query, nil, "")
	return document.Data, document, err
}

func (client *Client) Create(ctx context.Context, resourceType string, object jsonapi.ResourceObject, query QueryOptions) (jsonapi.Resource, jsonapi.Document, error) {
	body, err := json.Marshal(jsonapi.WriteDocument{Data: object})
	if err != nil {
		return jsonapi.Resource{}, jsonapi.Document{}, err
	}
	document, err := client.resourceRequest(ctx, http.MethodPost, resourceType, query, body, jsonapi.MediaType)
	if err != nil {
		return jsonapi.Resource{}, jsonapi.Document{}, err
	}
	resource, err := jsonapi.DecodeResource(document.Data)
	return resource, document, err
}

func (client *Client) Update(ctx context.Context, resourceType, id string, object jsonapi.ResourceObject, query QueryOptions) (jsonapi.Resource, jsonapi.Document, error) {
	body, err := json.Marshal(jsonapi.WriteDocument{Data: object})
	if err != nil {
		return jsonapi.Resource{}, jsonapi.Document{}, err
	}
	document, err := client.resourceRequest(ctx, http.MethodPatch, resourceType+"/"+url.PathEscape(id), query, body, jsonapi.MediaType)
	if err != nil {
		return jsonapi.Resource{}, jsonapi.Document{}, err
	}
	resource, err := jsonapi.DecodeResource(document.Data)
	return resource, document, err
}

func (client *Client) Delete(ctx context.Context, resourceType, id string) error {
	_, err := client.resourceRequest(ctx, http.MethodDelete, resourceType+"/"+url.PathEscape(id), QueryOptions{}, nil, "")
	return err
}

func (client *Client) Upload(ctx context.Context, filePath, description string, altText *string) (jsonapi.Resource, jsonapi.Document, string, error) {
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return jsonapi.Resource{}, jsonapi.Document{}, "", fmt.Errorf("read upload: %w", err)
	}
	digest := sha256.Sum256(contents)
	checksum := hex.EncodeToString(digest[:])
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(filePath)))
	if contentType == "" {
		contentType = http.DetectContentType(contents)
	}
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, strings.ReplaceAll(filepath.Base(filePath), `"`, `'`)))
	partHeader.Set("Content-Type", contentType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return jsonapi.Resource{}, jsonapi.Document{}, checksum, err
	}
	if _, err := part.Write(contents); err != nil {
		return jsonapi.Resource{}, jsonapi.Document{}, checksum, err
	}
	_ = writer.WriteField("description", description)
	if altText != nil {
		_ = writer.WriteField("alt_text", *altText)
	}
	if err := writer.Close(); err != nil {
		return jsonapi.Resource{}, jsonapi.Document{}, checksum, err
	}
	document, err := client.resourceRequest(ctx, http.MethodPost, "uploads", QueryOptions{}, body.Bytes(), writer.FormDataContentType())
	if err != nil {
		var apiError *APIError
		if !errors.As(err, &apiError) {
			return jsonapi.Resource{}, jsonapi.Document{}, checksum,
				fmt.Errorf("upload result is unknown (sha256 %s); check `jdeen uploads list --filter checksum_sha256=%s` before retrying: %w", checksum, checksum, err)
		}
		return jsonapi.Resource{}, jsonapi.Document{}, checksum, err
	}
	resource, err := jsonapi.DecodeResource(document.Data)
	return resource, document, checksum, err
}

func (client *Client) resourceRequest(ctx context.Context, method, path string, query QueryOptions, body []byte, contentType string) (jsonapi.Document, error) {
	rawURL, err := client.resourceURL(path, query)
	if err != nil {
		return jsonapi.Document{}, err
	}
	token, err := client.validAccessToken(ctx)
	if err != nil {
		return jsonapi.Document{}, err
	}
	requestContentType := contentType
	if requestContentType == "" {
		requestContentType = jsonapi.MediaType
	}
	response, responseBody, err := client.performResource(ctx, method, rawURL, requestContentType, body, token)
	if err != nil {
		return jsonapi.Document{}, err
	}
	if response.StatusCode == http.StatusUnauthorized && errorCode(responseBody) == "access_token_expired" {
		token, err = client.refresh(ctx, token)
		if err != nil {
			return jsonapi.Document{}, err
		}
		response, responseBody, err = client.performResource(ctx, method, rawURL, requestContentType, body, token)
		if err != nil {
			return jsonapi.Document{}, err
		}
	}
	if response.StatusCode >= 400 {
		code := errorCode(responseBody)
		if code == "invalid_token" || code == "invalid_grant" || code == "account_not_allowed" {
			client.clearStoredSessionForAuthFailure()
		}
		return jsonapi.Document{}, decodeAPIError(response.StatusCode, responseBody)
	}
	if response.StatusCode == http.StatusNoContent {
		return jsonapi.Document{}, nil
	}
	var document jsonapi.Document
	if err := json.Unmarshal(responseBody, &document); err != nil {
		return jsonapi.Document{}, fmt.Errorf("decode JSON:API response: %w", err)
	}
	return document, nil
}

func (client *Client) performResource(ctx context.Context, method, rawURL, contentType string, body []byte, token string) (*http.Response, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Accept", jsonapi.MediaType)
	request.Header.Set("User-Agent", client.userAgent)
	request.Header.Set("Authorization", "Bearer "+token)
	if len(body) > 0 {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	responseBody := new(bytes.Buffer)
	if _, err := responseBody.ReadFrom(response.Body); err != nil {
		return nil, nil, err
	}
	return response, responseBody.Bytes(), nil
}

func RelationshipOne(resourceType, id string) jsonapi.RelationshipDocument {
	return jsonapi.RelationshipDocument{Data: jsonapi.Identifier(resourceType, id)}
}

func RelationshipMany(resourceType string, ids []string) jsonapi.RelationshipDocument {
	identifiers := make([]jsonapi.ResourceIdentifier, 0, len(ids))
	for _, id := range ids {
		identifiers = append(identifiers, jsonapi.Identifier(resourceType, id))
	}
	return jsonapi.RelationshipDocument{Data: identifiers}
}

func RelationshipNull() jsonapi.RelationshipDocument {
	return jsonapi.RelationshipDocument{Data: nil}
}

func ParseRelated(data json.RawMessage) ([]jsonapi.Resource, bool, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == "" {
		return nil, false, nil
	}
	if strings.HasPrefix(trimmed, "[") {
		resources, err := jsonapi.DecodeCollection(data)
		return resources, true, err
	}
	resource, err := jsonapi.DecodeResource(data)
	if err != nil {
		return nil, false, err
	}
	return []jsonapi.Resource{resource}, false, nil
}

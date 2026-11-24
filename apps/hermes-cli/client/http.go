package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"hermes-messages"
)

type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *HTTPClient) Authenticate(
	ctx context.Context,
	username string,
	password []byte,
) (Session, error) {
	requestIDBytes := make([]byte, 16)
	if _, err := rand.Read(requestIDBytes); err != nil {
		return Session{}, fmt.Errorf("generate request ID: %w", err)
	}
	request := messages.Request[messages.AuthenticateInput]{
		Envelope: messages.Envelope{
			SchemaVersion: messages.SchemaVersion,
			RequestID:     hex.EncodeToString(requestIDBytes),
			Operation:     messages.Authenticate,
		},
		Payload: messages.AuthenticateInput{
			Username: username,
			Password: string(password),
		},
	}

	body, err := json.Marshal(request)
	if err != nil {
		return Session{}, fmt.Errorf("encode authentication request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+messages.LoginPath,
		bytes.NewReader(body),
	)
	if err != nil {
		return Session{}, fmt.Errorf("create authentication request: %w", err)
	}

	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return Session{}, fmt.Errorf("send authentication request: %w", err)
	}
	defer response.Body.Close()

	var result messages.Response[messages.AuthenticateResult]

	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return Session{}, fmt.Errorf("decode authentication response: %w", err)
	}

	if result.SchemaVersion != messages.SchemaVersion ||
		result.RequestID != request.RequestID ||
		result.Operation != request.Operation {
		return Session{}, fmt.Errorf("invalid authentication response envelope")
	}

	switch result.Status {
	case messages.StatusError:
		if result.Error == nil || result.Payload != nil || result.Error.Code == "" {
			return Session{}, fmt.Errorf("invalid authentication error response")
		}
		return Session{}, &APIError{
			Code: result.Error.Code,
		}

	case messages.StatusSuccess:
		if response.StatusCode != http.StatusOK {
			return Session{}, fmt.Errorf(
				"unexpected authentication HTTP status: %d",
				response.StatusCode,
			)
		}
		if result.Payload == nil || result.Error != nil {
			return Session{}, fmt.Errorf("invalid authentication success response")
		}

		payload := result.Payload
		if payload.UserID == "" || payload.Token == "" || payload.ExpiresAt.IsZero() {
			return Session{}, fmt.Errorf("incomplete authentication session")
		}

		return Session{
			UserID:    payload.UserID,
			Token:     payload.Token,
			ExpiresAt: payload.ExpiresAt,
		}, nil

	default:
		return Session{}, fmt.Errorf("unknown authentication response status")
	}
}

func (c *HTTPClient) List(
	ctx context.Context,
	session Session,
) (DocumentPage, error) {
	requestIDBytes := make([]byte, 16)
	if _, err := rand.Read(requestIDBytes); err != nil {
		return DocumentPage{}, fmt.Errorf("generate request ID: %w", err)
	}
	request := messages.Request[messages.ListDocumentsInput]{
		Envelope: messages.Envelope{
			SchemaVersion: messages.SchemaVersion,
			RequestID:     hex.EncodeToString(requestIDBytes),
			Operation:     messages.ListDocuments,
		},
		Payload: messages.ListDocumentsInput{Page: &messages.PageInput{}},
	}

	body, err := json.Marshal(request)
	if err != nil {
		return DocumentPage{}, fmt.Errorf("encode document list request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+messages.ListDocumentsPath,
		bytes.NewReader(body),
	)
	if err != nil {
		return DocumentPage{}, fmt.Errorf("create document list request: %w", err)
	}

	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+session.Token)

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return DocumentPage{}, fmt.Errorf("send document list request: %w", err)
	}
	defer response.Body.Close()

	var result messages.Response[messages.ListDocumentsResult]

	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return DocumentPage{}, fmt.Errorf("decode document list response: %w", err)
	}

	if result.SchemaVersion != messages.SchemaVersion ||
		result.RequestID != request.RequestID ||
		result.Operation != request.Operation {
		return DocumentPage{}, fmt.Errorf("invalid document list response envelope")
	}

	switch result.Status {
	case messages.StatusError:
		if result.Error == nil || result.Payload != nil || result.Error.Code == "" {
			return DocumentPage{}, fmt.Errorf("invalid document list error response")
		}
		return DocumentPage{}, &APIError{Code: result.Error.Code}
	case messages.StatusSuccess:
		if response.StatusCode != http.StatusOK {
			return DocumentPage{}, fmt.Errorf("unexpected document list HTTP status: %d", response.StatusCode)
		}
		if result.Error != nil || result.Payload == nil || result.Payload.Page == nil || result.Payload.Page.Items == nil {
			return DocumentPage{}, fmt.Errorf("invalid document list success response")
		}
		return *result.Payload.Page, nil
	default:
		return DocumentPage{}, fmt.Errorf("unknown document list response status")
	}
}

func (c *HTTPClient) Read(
	ctx context.Context,
	session Session,
	documentID string,
) (Document, error) {
	requestIDBytes := make([]byte, 16)
	if _, err := rand.Read(requestIDBytes); err != nil {
		return Document{}, fmt.Errorf("generate request ID: %w", err)
	}
	request := messages.Request[messages.GetDocumentInput]{
		Envelope: messages.Envelope{
			SchemaVersion: messages.SchemaVersion,
			RequestID:     hex.EncodeToString(requestIDBytes),
			Operation:     messages.GetDocument,
		},
		Payload: messages.GetDocumentInput{DocumentID: documentID},
	}

	body, err := json.Marshal(request)
	if err != nil {
		return Document{}, fmt.Errorf("encode document read request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+messages.GetDocumentPath,
		bytes.NewReader(body),
	)
	if err != nil {
		return Document{}, fmt.Errorf("create document read request: %w", err)
	}

	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+session.Token)

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return Document{}, fmt.Errorf("send document read request: %w", err)
	}
	defer response.Body.Close()

	var result messages.Response[messages.GetDocumentResult]

	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return Document{}, fmt.Errorf("decode document read response: %w", err)
	}

	if result.SchemaVersion != messages.SchemaVersion ||
		result.RequestID != request.RequestID ||
		result.Operation != request.Operation {
		return Document{}, fmt.Errorf("invalid document read response envelope")
	}

	switch result.Status {
	case messages.StatusError:
		if result.Error == nil || result.Payload != nil || result.Error.Code == "" {
			return Document{}, fmt.Errorf("invalid document read error response")
		}
		return Document{}, &APIError{Code: result.Error.Code}
	case messages.StatusSuccess:
		if response.StatusCode != http.StatusOK {
			return Document{}, fmt.Errorf("unexpected document read HTTP status: %d", response.StatusCode)
		}
		if result.Error != nil || result.Payload == nil || result.Payload.Document == nil {
			return Document{}, fmt.Errorf("invalid document read success response")
		}
		document := result.Payload.Document
		return Document{Summary: document.DocumentSummary, Content: document.Content}, nil
	default:
		return Document{}, fmt.Errorf("unknown document read response status")
	}
}

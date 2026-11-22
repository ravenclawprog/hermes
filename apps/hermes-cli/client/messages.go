package client

import "time"

type authenticatePayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authenticateRequest struct {
	SchemaVersion int                 `json:"schema_version"`
	RequestID     string              `json:"request_id"`
	Operation     string              `json:"operation"`
	Payload       authenticatePayload `json:"payload"`
}

type authenticateResult struct {
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type responseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type authenticateResponse struct {
	SchemaVersion int                 `json:"schema_version"`
	RequestID     string              `json:"request_id"`
	Operation     string              `json:"operation"`
	Status        string              `json:"status"`
	Payload       *authenticateResult `json:"payload"`
	Error         *responseError      `json:"error"`
}

type documentListRequest struct {
	SchemaVersion int                 `json:"schema_version"`
	RequestID     string              `json:"request_id"`
	Operation     string              `json:"operation"`
	Payload       documentListPayload `json:"payload"`
}

type documentListPayload struct {
	Page struct{} `json:"page"`
}

type documentListResult struct {
	Page *DocumentPage `json:"page"`
}

type documentListResponse struct {
	SchemaVersion int                 `json:"schema_version"`
	RequestID     string              `json:"request_id"`
	Operation     string              `json:"operation"`
	Status        string              `json:"status"`
	Payload       *documentListResult `json:"payload"`
	Error         *responseError      `json:"error"`
}

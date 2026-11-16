package client

type APIError struct {
	Code string
}

func (e *APIError) Error() string {
	return "server error: " + e.Code
}

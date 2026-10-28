package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
)

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return strconv.Itoa(e.Status) + ": " + e.Message
}

func readError(response *http.Response) error {
	var buffer bytes.Buffer
	_, _ = buffer.ReadFrom(response.Body)
	message := buffer.String()

	var wrapped struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(buffer.Bytes(), &wrapped) == nil && (wrapped.Message != "" || wrapped.Error != "") {
		if wrapped.Message != "" {
			message = wrapped.Message
		}
		return &APIError{Status: response.StatusCode, Code: wrapped.Error, Message: message}
	}
	var bare string
	if json.Unmarshal(buffer.Bytes(), &bare) == nil && bare != "" {
		message = bare
	}

	return &APIError{Status: response.StatusCode, Message: message}
}

package apis

import (
	"encoding/json"
	"log"
	"net/http"
)

type Response struct {
	StatusCode uint32      `json:"status_code"`
	Message    string      `json:"message"`
	Data       interface{} `json:"data"`
}

type ErrorCodeWithMessage struct {
	HttpStatus int
	ErrorCode  uint32
	Message    string
}

func SendHttpResponse(w http.ResponseWriter, m *Response, httpCode int) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")

	w.WriteHeader(httpCode)
	if m != nil {
		err := json.NewEncoder(w).Encode(m)
		if err != nil {
			log.Printf("Failed to encode response: %v\n", err)
			return
		}
	}
}

func SendErrorResponse(w http.ResponseWriter, err *ErrorCodeWithMessage, describe string) {
	SendHttpResponse(w, &Response{
		StatusCode: err.ErrorCode,
		Message:    err.Message,
		Data:       describe,
	}, err.HttpStatus)
}

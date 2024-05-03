package apis

import (
	"net/http"
)

func Healthz(w http.ResponseWriter, r *http.Request) {
	SendHttpResponse(w, &Response{
		StatusCode: 0,
		Message:    "OK",
		Data:       "Alive",
	}, http.StatusOK)
}

package server

import (
	"net/http"
	"usercenter/server/apis"

	"github.com/gorilla/mux"
)

var (
	healthzPath  = "/healthz"
	deviceLogin  = "/device/login"
	deviceLogout = "/device/logout"
)

func NewRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	router.
		Methods(http.MethodGet).
		Path(healthzPath).
		Name("healthz").
		HandlerFunc(apis.Healthz)

	router.
		Methods(http.MethodPost).
		Path(deviceLogin).
		Name("deviceLogin").
		HandlerFunc(apis.DeviceLogin)

	router.
		Methods(http.MethodPost).
		Path(deviceLogout).
		Name("deviceLogout").
		HandlerFunc(apis.DeviceLogout)

	return router
}

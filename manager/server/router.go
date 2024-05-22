package server

import (
	"manager/server/apis"
	"net/http"

	"github.com/gorilla/mux"
)

var (
	healthzPath     = "/healthz"
	instanceApply   = "/instance/apply"
	instanceRelease = "/instance/release"
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
		Path(instanceApply).
		Name("instanceApply").
		HandlerFunc(apis.InstanceApply)
	router.
		Methods(http.MethodPost).
		Path(instanceRelease).
		Name("instanceRelease").
		HandlerFunc(apis.InstanceRelease)

	return router
}

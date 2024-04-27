package server

import (
	"github.com/gorilla/mux"
	"manager/server/apis"
	"net/http"
)

var (
	healthzPath         = "/healthz"
	deploymentApplyPath = "/deployment/apply"
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
		Path(deploymentApplyPath).
		Name("deploymentApply").
		HandlerFunc(apis.DeploymentApply)

	return router
}

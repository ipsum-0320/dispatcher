package server

import (
	"manager/server/apis"
	"net/http"

	"github.com/gorilla/mux"
)

var (
	healthzPath    = "/healthz"
	instanceManage = "/instance/manage"
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
		Path(instanceManage).
		Name("instanceManage").
		HandlerFunc(apis.InstanceManage)
	router.
		Methods(http.MethodGet).
		Path("/bounce/rate").
		Name("bounceRate").
		HandlerFunc(apis.BounceRate)
	return router
}

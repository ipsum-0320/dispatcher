package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
)

func main() {
	r := mux.NewRouter()
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("./"))))
	fmt.Printf("Starting server at http://10.10.103.51:8080\n")
	http.ListenAndServe("10.10.103.51:8080", r)
}

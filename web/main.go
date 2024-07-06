package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
)

func main() {
	r := mux.NewRouter()
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("./"))))
	fmt.Printf("Starting server at http://localhost:8080\n")
	http.ListenAndServe("localhost:8080", r)
}

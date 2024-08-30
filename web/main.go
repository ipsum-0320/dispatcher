package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("./"))))
	fmt.Printf("Starting server at http://0.0.0.0:5678\n")
	http.ListenAndServe("0.0.0.0:5678", r)
}

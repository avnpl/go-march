package main

import (
	"fmt"
	"net/http"
)

var PORT = ":8000"

func main() {
	fmt.Println("Starting server...")
	mux := http.NewServeMux()

	mux.HandleFunc("GET /comment", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Return all comments")
	})

	mux.HandleFunc("GET /comment/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		fmt.Fprintf(w, "Return comment with ID : %s", id)
	})

	mux.HandleFunc("POST /comment", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Posting a comment")
	})

	if err := http.ListenAndServe(PORT, mux); err != nil {
		fmt.Println(err.Error())
	}
}

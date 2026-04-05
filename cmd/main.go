package main

import (
	"net/http"

	"fmt"

	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World"))
	})

	port := 8080
	fmt.Println("Server is running on port 8080")
	http.ListenAndServe(fmt.Sprintf(":%d", port), r)

}

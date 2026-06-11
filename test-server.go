package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	fmt.Println("Test server starting on :9999")
	if err := http.ListenAndServe(":9999", nil); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

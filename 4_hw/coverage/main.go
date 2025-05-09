package main

import (
	"fmt"
	"net/http"
)

func main() {

	http.HandleFunc("/", SearchServer)

	fmt.Println("starting server at :8080")
	http.ListenAndServe(":8080", nil)
}

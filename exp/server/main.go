package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("GET /render/", RenderHandler)

	fmt.Println("Exp server")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func RenderHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("RenderHandler")

    url := r.URL.Query().Get("url")
    log.Printf("url: %s", url)

	// pathString := r.PathValue("pathname")
	// fmt.Println("pathString: ", pathString)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hello, World!"))
}

package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
)

func main() {
	http.HandleFunc("GET /render", RenderHandler)

	fmt.Println("Exp server")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func RenderHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("RenderHandler")

	urlParam := r.URL.Query().Get("url")
	log.Printf("url param: %s", urlParam)

	encodedUrl := url.QueryEscape(urlParam)
	decodedUrl, err := url.QueryUnescape(urlParam)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("encodedUrl: %s", encodedUrl)
	log.Printf("decodedUrl: %s", decodedUrl)

	// pathString := r.PathValue("pathname")
	// fmt.Println("pathString: ", pathString)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hello, World!"))
}

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/liuminhaw/renderer"
	"github.com/liuminhaw/wrenderer/cmd/server/awsLambda"
)

func HttpHandler(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	url := r.URL.Query().Get("url")
	log.Printf("url: %s", url)
	if url == "" {
		http.Error(w, "Missing url parameter", http.StatusBadRequest)
		return
	}

	// Render the page
	browserContext := renderer.BrowserContext{
		DebugMode: true,
		Container: true,
	}
	rendererContext := renderer.RendererContext{
		Headless:       false,
		WindowWidth:    1000,
		WindowHeight:   1000,
		Timeout:        30,
		ImageLoad:      false,
		SkipFrameCount: 0,
	}
	ctx := context.Background()
	ctx = renderer.WithBrowserContext(ctx, &browserContext)
	ctx = renderer.WithRendererContext(ctx, &rendererContext)

	context, err := renderer.RenderPage(ctx, url)
	if err != nil {
		log.Printf("Render Failed: %s", err)
		http.Error(w, "Render Failed", http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write(context)
}

func main() {
	if _, exists := os.LookupEnv("AWS_LAMBDA_RUNTIME_API"); exists {
		fmt.Println("Running in AWS Lambda custom image")
		lambda.Start(awsLambda.LambdaHandler)
	} else {
		fmt.Println("server listening on 8080 port")
		http.HandleFunc("/render/", HttpHandler)
		log.Fatal(http.ListenAndServe(":8080", nil))
	}
}

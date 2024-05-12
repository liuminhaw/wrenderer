package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/liuminhaw/renderer"
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
		Container: false,
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

func LambdaHandler(event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get query parameters
	url := event.QueryStringParameters["url"]
	log.Printf("url: %s", url)
	if url == "" {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       "Missing url parameter",
		}, nil
	}

	// Render the page
	browserContext := renderer.BrowserContext{
		DebugMode: true,
		Container: true,
		// SingleProcess:   true,
	}
	rendererContext := renderer.RendererContext{
		Headless:       true,
		WindowWidth:    1000,
		WindowHeight:   1000,
		Timeout:        30,
		ImageLoad:      true,
		SkipFrameCount: 0,
	}
	ctx := context.Background()
	ctx = renderer.WithBrowserContext(ctx, &browserContext)
	ctx = renderer.WithRendererContext(ctx, &rendererContext)

	context, err := renderer.RenderPage(ctx, url)
	if err != nil {
		log.Printf("Render Failed: %s", err)
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Render Failed",
		}, nil
	}

	// Regular expressions for matching base64 images and SVG content
	regexpBase64 := regexp.MustCompile(`"data:image\/.*?;base64.*?"`)
	regexpSVG := regexp.MustCompile(`\<svg.*?\>.*?\<\/svg\>`)

	// Replacing base64 images with empty strings and SVG content with empty <svg></svg> tags
	newContext := regexpBase64.ReplaceAllString(string(context), `""`)
	newContext = regexpSVG.ReplaceAllString(newContext, `<svg></svg>`)

    // TODO: Upload rendered result to S3
    // TODO: Return S3 URL for modify request settings

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(context),
	}, nil
}

func main() {
	if _, exists := os.LookupEnv("AWS_LAMBDA_RUNTIME_API"); exists {
		fmt.Println("Running in AWS Lambda custom image")
		lambda.Start(LambdaHandler)
	} else {
		fmt.Println("server listening on 8080 port")
		http.HandleFunc("/render/", HttpHandler)
		log.Fatal(http.ListenAndServe(":8080", nil))
	}
}

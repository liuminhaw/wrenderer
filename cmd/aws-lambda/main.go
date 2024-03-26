package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	// "github.com/liuminhaw/renderer"
	"github.com/liuminhaw/wrenderer/renderer"
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
		// BrowserExecPath: "/home/haw/Programs/headless-chromium/chromium",
		NoSandbox:       true,
		DebugMode:       true,
		// SingleProcess: true,
		// NoSandbox:       false,
		// DebugMode:       true,
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
		// BrowserExecPath: "/home/haw/Programs/headless-chromium/chromium",
		// BrowserExecPath: "chromium",
		NoSandbox:       true,
		DebugMode:       true,
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

	// chromiumLog()
	// fmt.Println("Cleaning up temporary directories...")
	// cleanTmp("/tmp", "chromedp")

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(context),
	}, nil
}

func main() {
	// Start a HTTP server
	http.HandleFunc("/", HttpHandler)

    if _, exists := os.LookupEnv("AWS_LAMBDA_RUNTIME_API"); exists {
        fmt.Println("Running in AWS Lambda custom image")
        lambda.Start(LambdaHandler)
    } else {
        fmt.Println("Running in local")
        log.Fatal(http.ListenAndServe(":8080", nil))
    }
	// log.Fatal(http.ListenAndServe(":8080", nil))
	// lambda.Start(LambdaHandler)
	//    chromiumLog()
	//    fmt.Println("Cleaning up temporary directories...")
	// cleanTmp("/tmp", "chromedp")
}

func cleanTmp(baseDir string, prefix string) error {
	// Walk the base directory to find directories that match the prefix
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("cleanTmp: walk: %w", err)
		}
		if info.IsDir() && filepath.Base(path)[:len(prefix)] == prefix {
			// Found a directory that matches the prefix; attempt to remove it
			if err := os.RemoveAll(path); err != nil {
				// Handle the error if the directory cannot be removed
				return fmt.Errorf("cleanTmp: walk: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		// Handle any errors that occurred during the walk
		return fmt.Errorf("cleanTmp: %w", err)
	}

	return nil
}

func chromiumLog() {
	// Step 1: Find the path of the 'chromium' command
	chromiumPath, err := exec.LookPath("chromium")
	if err != nil {
		fmt.Println("Error finding 'chromium':", err)
		return
	}

	// Step 2: Extract the directory from the command path
	chromiumDir := filepath.Dir(chromiumPath)
    fmt.Println("chromiumDir: ", chromiumDir)

	// Step 3: List the files in the directory
	files, err := os.ReadDir(chromiumDir)
	if err != nil {
		fmt.Println("Error reading directory:", err)
		return
	}

	// Step 4: Check for 'chrome_debug.log' and display its contents
	for _, file := range files {
		fmt.Printf("file: %s\n", file.Name())
		if file.Name() == "chrome_debug.log" {
			fmt.Println("'chrome_debug.log' found. Displaying contents:")
			logPath := filepath.Join(chromiumDir, "chrome_debug.log")
			content, err := os.ReadFile(logPath)
			if err != nil {
				fmt.Println("Error reading file:", err)
				return
			}
			fmt.Println(string(content))
			return
		}
	}

	fmt.Println("'chrome_debug.log' not found in the directory.")
}

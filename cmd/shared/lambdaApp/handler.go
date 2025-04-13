package lambdaApp

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/liuminhaw/renderer"
	"github.com/liuminhaw/wrenderer/cmd/shared"
	"github.com/liuminhaw/wrenderer/wrender"
)

// RenderUrl will check if the given url is already rendered and cached in S3 bucket.
// If not, it will render the url and upload the result to S3 bucket for caching.
// existenceCheck is a flag to check the existence of the object in S3 bucket.
// If the flag is set to false, the object will be rendered and uploaded to S3 bucket
// no matter if the object already exists in the bucket.
// The cached object path will be returned if no error occurred, otherwise an error
// will be returned.
// func (app *Application) RenderUrl(url string, existenceCheck bool) (string, error) {
func RenderUrl(url string, existenceCheck bool, logger *slog.Logger) (string, error) {
	loader, err := shared.NewConfLoader(shared.S3Service)
	if err != nil {
		return "", err
	}

	// Check if object exists
	render, err := wrender.NewWrender(url, wrender.CachedPagePrefix)
	if err != nil {
		return "", err
	}
	caching := wrender.NewS3Caching(
		loader.Clients.S3,
		render.GetPrefixPath(),
		render.CachePath,
		wrender.S3CachingMeta{
			Bucket:      loader.EnvConf.S3BucketName,
			Region:      loader.EnvConf.S3BucketRegion,
			ContentType: wrender.HtmlContentType,
		},
	)

	// var exists bool
	if existenceCheck {
		exists, err := caching.Exists()
		if err != nil {
			return "", err
		}
		if exists {
			return caching.CachedPath, nil
		}
	}

	// Render the page
	content, err := renderPage(url, logger)
	if err != nil {
		return "", err
	}

	// Check if rendered result is empty
	if len(content) == 0 {
		return "", fmt.Errorf("empty content render result")
	}

	// Upload rendered result to S3
	contentReader := bytes.NewReader(content)
	if err := caching.Update(contentReader); err != nil {
		return "", err
	}

	return caching.CachedPath, nil
}

func renderPage(urlParam string, logger *slog.Logger) ([]byte, error) {
	idleType, exists := os.LookupEnv("WRENDERER_IDLE_TYPE")
	if !exists {
		idleType = "networkIdle"
	}

	var windowWidth, windowHeight int
	var err error
	windowWidthConfig, exists := os.LookupEnv("WRENDERER_WINDOW_WIDTH")
	if !exists {
		windowWidth = 1920
	} else {
		windowWidth, err = strconv.Atoi(windowWidthConfig)
		if err != nil {
			return nil, fmt.Errorf("renderPage: %w", err)
		}
	}
	windowHeightConfig, exists := os.LookupEnv("WRENDERER_WINDOW_HEIGHT")
	if !exists {
		windowHeight = 1080
	} else {
		windowHeight, err = strconv.Atoi(windowHeightConfig)
		if err != nil {
			return nil, fmt.Errorf("renderPage: %w", err)
		}
	}

	userAgent, exists := os.LookupEnv("WRENDERER_USER_AGENT")
	if !exists {
		userAgent = ""
	}

	r := renderer.NewRenderer(renderer.WithLogger(logger))
	content, err := r.RenderPage(urlParam, &renderer.RendererOption{
		BrowserOpts: renderer.BrowserConf{
			IdleType:  idleType,
			Container: true,
		},
		Headless:     true,
		WindowWidth:  windowWidth,
		WindowHeight: windowHeight,
		Timeout:      30,
		UserAgent:    userAgent,
	})
	if err != nil {
		return nil, fmt.Errorf("renderPage: %w", err)
	}

	return content, nil
}

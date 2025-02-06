package lambdaApp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/liuminhaw/renderer"
	"github.com/liuminhaw/sitemapHelper"
	"github.com/liuminhaw/wrenderer/wrender"
)

type Application struct {
	Logger *slog.Logger
}

// RenderUrl will check if the given url is already rendered and cached in S3 bucket.
// If not, it will render the url and upload the result to S3 bucket for caching.
// The cached object path will be returned if no error occurred, otherwise an error
// will be returned.
func (app *Application) RenderUrl(url string) (string, error) {
	render, err := wrender.NewWrender(url)
	if err != nil {
		return "", err
	}

	// Check if object exists
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return "", err
	}
	client := s3.NewFromConfig(cfg)

	exists, err := CheckObjectExists(client, render.CachePath)
	if err != nil {
		return "", err
	}
	if exists {
		return render.CachePath, nil
	}

	// Render the page
	content, err := app.renderPage(url)
	if err != nil {
		return "", err
	}

	// Check if rendered result is empty
	if len(content) == 0 {
		return "", fmt.Errorf("empty content render result")
	}

	// Upload rendered result to S3
	contentReader := bytes.NewReader(content)
	err = uploadToS3(client, render.CachePath, contentReader)
	if err != nil {
		return "", err
	}

	return render.CachePath, nil
}

type workerQueuePayload struct {
	TargetUrl string `json:"targetUrl"`
}

func (app *Application) RenderSitemap(url string) error {
	resp, err := http.Get(url)
	if err != nil {
        return err
	}

	entries, err := sitemapHelper.ParseSitemap(resp.Body)
	if err != nil {
        return err
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
        return err
	}
	client := sqs.NewFromConfig(cfg)

	for _, entry := range entries {
		app.Logger.Debug(fmt.Sprintf("Entry: %s", entry.Loc))
		payload, err := json.Marshal(workerQueuePayload{TargetUrl: entry.Loc})
		if err != nil {
            return err
		}

		messageId, err := sendMessageToQueue(client, string(payload))
		if err != nil {
            return err
		}
		app.Logger.Debug(
			fmt.Sprintf("Message id %s successfully sent", messageId),
			slog.String("payload", string(payload)),
		)
	}

    return nil
}

func (app *Application) DeleteUrlRenderCache(url string) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}
	client := s3.NewFromConfig(cfg)

	render, err := wrender.NewWrender(url)
	if err != nil {
		return err
	}

	// Remove the object from s3
	if err := deleteObjectFromS3(client, render.CachePath); err != nil {
		return err
	}
	// Remove the host prefix if no more objects are left
	prefix := fmt.Sprintf("%s/", render.GetPrefixPath())
	empty, err := checkDomainEmpty(client, prefix)
	if err != nil {
		return err
	}
	if empty {
		if err := deletePrefixFromS3(client, prefix); err != nil {
			return err
		}
	}

	return nil
}

func (app *Application) DeleteDomainRenderCache(domain string) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}
	client := s3.NewFromConfig(cfg)

	render, err := wrender.NewWrender(domain)
	if err != nil {
		return err
	}

	prefix := fmt.Sprintf("%s/", render.GetPrefixPath())
	if err := deletePrefixFromS3(client, prefix); err != nil {
		return err
	}

	return nil
}

func (app *Application) renderPage(urlParam string) ([]byte, error) {
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

	r := renderer.NewRenderer(renderer.WithLogger(app.Logger))
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

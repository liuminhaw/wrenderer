package renderer

import (
	"context"
	"errors"
)

type key string

const (
	browserKey  key = "browserContext"
	rendererKey key = "renderContext"
	pdfKey      key = "pdfContext"
)

var (
	ErrBrowserContextNotFound  = errors.New("browser context not found")
	ErrRendererContextNotFound = errors.New("renderer context not found")
	ErrPdfContextNotFound      = errors.New("pdf context not found")
)

// BrowserContext is use for general browser settings
type BrowserContext struct {
	IdleType        string
	BrowserExecPath string
	NoSandbox       bool
	DebugMode       bool
}

// WithBrowserContext add BrowserContext with browserKey to context and return
// new context value
func WithBrowserContext(ctx context.Context, bc *BrowserContext) context.Context {
	return context.WithValue(ctx, browserKey, bc)
}

// GetBrowserContext read and return BrowserContext from input context
// ErrBrowserContextNotFound is returned if browserKey not exist
func GetBrowserContext(ctx context.Context) (*BrowserContext, error) {
	val := ctx.Value(browserKey)

	browserContext, ok := val.(*BrowserContext)
	if !ok {
		return nil, ErrBrowserContextNotFound
	}

	return browserContext, nil
}

// RendererContext is use for renderer settings
type RendererContext struct {
	Headless       bool
	WindowWidth    int
	WindowHeight   int
	Timeout        int
	ImageLoad      bool
	SkipFrameCount int
}

// WithRendererContext add RendererContext with rendererKey to context and return
// new context value
func WithRendererContext(ctx context.Context, rc *RendererContext) context.Context {
	return context.WithValue(ctx, rendererKey, rc)
}

// GetRendererContext read and return RendererContext from input context
// ErrRendererContextNotFound is returned if rendererKey not exist
func GetRendererContext(ctx context.Context) (*RendererContext, error) {
	val := ctx.Value(rendererKey)

	rendererContext, ok := val.(*RendererContext)
	if !ok {
		return nil, ErrRendererContextNotFound
	}

	return rendererContext, nil
}

// PdfContext is use for print PDF settings
type PdfContext struct {
	Landscape           bool
	DisplayHeaderFooter bool
	PaperWidthCm        float64
	PaperHeightCm       float64
	MarginTopCm         float64
	MarginBottomCm      float64
	MarginLeftCm        float64
	MarginRightCm       float64
}

// WithPdfContext add PdfContext with pdfKey to context and return
// new context value
func WithPdfContext(ctx context.Context, pc *PdfContext) context.Context {
	return context.WithValue(ctx, pdfKey, pc)
}

// GetPdfContext read and return PdfContext from input context
// ErrPdfContextNotFound is returned if pdfKey not exist
func GetPdfContext(ctx context.Context) (*PdfContext, error) {
	val := ctx.Value(pdfKey)

	pdfContext, ok := val.(*PdfContext)
	if !ok {
		return nil, ErrPdfContextNotFound
	}

	return pdfContext, nil
}

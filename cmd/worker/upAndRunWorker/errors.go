package upAndRunWorker

import "fmt"

type HandlerError struct {
	err    error
	source string
}

func (h *HandlerError) Error() string {
    return fmt.Sprintf("source: %s, err: %v", h.source, h.err)
}

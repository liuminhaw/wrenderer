package shared

type RenderStatusResp struct {
	Status     string   `json:"status"`
	Details    []string `json:"details,omitempty"`
	StatusCode int      `json:"-"`
}

type RespErrorMessage struct {
	Message string `json:"message"`
}

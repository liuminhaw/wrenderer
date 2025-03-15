package shared

type RenderStatusResp struct {
	Status  string   `json:"status"`
	Details []string `json:"details,omitempty"`
}

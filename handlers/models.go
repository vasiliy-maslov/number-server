package handlers

type NumberRequest struct {
	Number int `json:"number"`
}

type NumberResponse struct {
	Status string `json:"status"`
}

type ResetResponse struct {
	Status string `json:"status"`
}

type HealthResponse struct {
	ServerStatus string `json:"status"`
	DBStatus     string `json:"db"`
}

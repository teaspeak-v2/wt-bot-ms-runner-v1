package models

// SpawnResponse is returned when a bot container is spawned.
type SpawnResponse struct {
	ContainerID string `json:"container_id"`
	Status      string `json:"status"`
}

// StopResponse is returned when a bot container is stopped.
type StopResponse struct {
	Message string `json:"message"`
}

// MessageResponse is a generic message response.
type MessageResponse struct {
	Message string `json:"message"`
}

// ErrorResponse is a generic error response.
type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

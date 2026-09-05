package handlers

// SwaggerUnifiedModelRequest documents user-editable fields only. Kind, source
// and reason are derived by the settings service and are not writable inputs.
type SwaggerUnifiedModelRequest struct {
	ID       string `json:"id" example:"vendor/image-model"`
	Protocol string `json:"protocol" enums:"images,chat-image,speech,videos"`
	Enabled  bool   `json:"enabled"`
}

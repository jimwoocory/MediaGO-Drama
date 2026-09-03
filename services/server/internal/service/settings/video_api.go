package settings

import (
	"context"
	"fmt"
	"strings"
)

const (
	videoAPIBaseURLSettingKey = "generation.videoapi.base_url"
	videoAPIModelSettingKey   = "generation.videoapi.model"
)

// VideoAPISettings stores a third-party asynchronous video endpoint.
type VideoAPISettings struct {
	BaseURL string `json:"baseURL"`
	Model   string `json:"model"`
}

func (service *Settings) GetVideoAPISettings(ctx context.Context) (VideoAPISettings, error) {
	_ = ctx
	if service == nil || service.appSettings == nil {
		return VideoAPISettings{}, nil
	}
	baseURL, _, err := service.appSettings.GetAppSetting(videoAPIBaseURLSettingKey)
	if err != nil {
		return VideoAPISettings{}, err
	}
	model, _, err := service.appSettings.GetAppSetting(videoAPIModelSettingKey)
	if err != nil {
		return VideoAPISettings{}, err
	}
	return VideoAPISettings{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Model:   strings.TrimSpace(model),
	}, nil
}

func (service *Settings) SetVideoAPISettings(ctx context.Context, input VideoAPISettings) (VideoAPISettings, error) {
	_ = ctx
	if service == nil || service.appSettings == nil {
		return VideoAPISettings{}, ErrAppSettingStoreMissing
	}
	baseURL, err := normalizeOpenAICompatibleBaseURL(input.BaseURL)
	if err != nil {
		return VideoAPISettings{}, fmt.Errorf("%w: %v", ErrAgentModelInvalid, err)
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		return VideoAPISettings{}, fmt.Errorf("%w: video Model ID is required", ErrAgentModelInvalid)
	}
	for key, value := range map[string]string{
		videoAPIBaseURLSettingKey: baseURL,
		videoAPIModelSettingKey:   model,
	} {
		if err := service.appSettings.SetAppSetting(key, value); err != nil {
			return VideoAPISettings{}, err
		}
	}
	return VideoAPISettings{BaseURL: baseURL, Model: model}, nil
}

package settings

import (
	"context"
	"fmt"
	"strings"
)

const (
	speechAPIBaseURLSettingKey = "generation.speechapi.base_url"
	speechAPIModelSettingKey   = "generation.speechapi.model"
	speechAPIVoiceSettingKey   = "generation.speechapi.voice"
	defaultSpeechAPIModel      = "gpt-4o-mini-tts"
	defaultSpeechAPIVoice      = "alloy"
)

// SpeechAPISettings stores the OpenAI-compatible third-party TTS endpoint.
type SpeechAPISettings struct {
	BaseURL string `json:"baseURL"`
	Model   string `json:"model"`
	Voice   string `json:"voice"`
}

func (service *Settings) GetSpeechAPISettings(ctx context.Context) (SpeechAPISettings, error) {
	_ = ctx
	result := SpeechAPISettings{Model: defaultSpeechAPIModel, Voice: defaultSpeechAPIVoice}
	if service == nil || service.appSettings == nil {
		return result, nil
	}
	baseURL, _, err := service.appSettings.GetAppSetting(speechAPIBaseURLSettingKey)
	if err != nil {
		return SpeechAPISettings{}, err
	}
	model, _, err := service.appSettings.GetAppSetting(speechAPIModelSettingKey)
	if err != nil {
		return SpeechAPISettings{}, err
	}
	voice, _, err := service.appSettings.GetAppSetting(speechAPIVoiceSettingKey)
	if err != nil {
		return SpeechAPISettings{}, err
	}
	result.BaseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.TrimSpace(model) != "" {
		result.Model = strings.TrimSpace(model)
	}
	if strings.TrimSpace(voice) != "" {
		result.Voice = strings.TrimSpace(voice)
	}
	return result, nil
}

func (service *Settings) SetSpeechAPISettings(ctx context.Context, input SpeechAPISettings) (SpeechAPISettings, error) {
	_ = ctx
	if service == nil || service.appSettings == nil {
		return SpeechAPISettings{}, ErrAppSettingStoreMissing
	}
	baseURL, err := normalizeOpenAICompatibleBaseURL(input.BaseURL)
	if err != nil {
		return SpeechAPISettings{}, fmt.Errorf("%w: %v", ErrAgentModelInvalid, err)
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = defaultSpeechAPIModel
	}
	voice := strings.TrimSpace(input.Voice)
	if voice == "" {
		voice = defaultSpeechAPIVoice
	}
	for key, value := range map[string]string{
		speechAPIBaseURLSettingKey: baseURL,
		speechAPIModelSettingKey:   model,
		speechAPIVoiceSettingKey:   voice,
	} {
		if err := service.appSettings.SetAppSetting(key, value); err != nil {
			return SpeechAPISettings{}, err
		}
	}
	return SpeechAPISettings{BaseURL: baseURL, Model: model, Voice: voice}, nil
}

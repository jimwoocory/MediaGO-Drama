package speechapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mediago-dev/mediago-drama/packages/core/pkg/generation"
)

const defaultHTTPTimeout = 90 * time.Second

type Config struct {
	BaseURL    string
	APIKey     string
	Model      string
	Voice      string
	HTTPClient *http.Client
}

type Provider struct {
	baseURL string
	apiKey  string
	model   string
	voice   string
	client  *http.Client
}

type speechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
}

func NewProvider(config Config) (*Provider, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("speech API base URL is not configured")
	}
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, generation.ErrMissingAPIKey
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &Provider{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(config.APIKey),
		model:   valueOrDefault(strings.TrimSpace(config.Model), "gpt-4o-mini-tts"),
		voice:   valueOrDefault(strings.TrimSpace(config.Voice), "alloy"),
		client:  client,
	}, nil
}

func (provider *Provider) Name() string { return generation.ProviderSpeechAPI }

func (provider *Provider) Generate(ctx context.Context, request generation.Request) (generation.Response, error) {
	if request.Kind != generation.KindAudio {
		return generation.Response{}, fmt.Errorf("speech API only supports audio generation")
	}
	if strings.TrimSpace(request.Prompt) == "" {
		return generation.Response{}, generation.ErrMissingPrompt
	}
	format := strings.ToLower(paramString(request.Params, "format"))
	if format == "" {
		format = "mp3"
	}
	payload := speechRequest{
		Model:          provider.model,
		Input:          request.Prompt,
		Voice:          provider.voice,
		ResponseFormat: format,
		Speed:          paramFloat(request.Params, "speed", 1),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return generation.Response{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.baseURL+"/audio/speech", bytes.NewReader(body))
	if err != nil {
		return generation.Response{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+provider.apiKey)
	response, err := provider.client.Do(httpRequest)
	if err != nil {
		return generation.Response{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return generation.Response{}, generation.HTTPErrorFromResponse(generation.ProviderSpeechAPI, response)
	}
	audio, err := io.ReadAll(io.LimitReader(response.Body, 64<<20))
	if err != nil {
		return generation.Response{}, err
	}
	if len(audio) == 0 {
		return generation.Response{}, fmt.Errorf("speech API returned no audio")
	}
	mimeType := strings.TrimSpace(response.Header.Get("Content-Type"))
	if mimeType == "" || strings.EqualFold(mimeType, "application/octet-stream") {
		mimeType = audioMIMEType(format)
	}
	return generation.Response{
		ID:     request.RouteID,
		Status: "completed",
		Model:  provider.model,
		Assets: []generation.Asset{{
			Kind:     generation.KindAudio,
			Base64:   base64.StdEncoding.EncodeToString(audio),
			MIMEType: mimeType,
			Metadata: map[string]any{"voice": provider.voice, "format": format},
		}},
	}, nil
}

func (provider *Provider) Get(context.Context, string) (generation.Response, error) {
	return generation.Response{}, fmt.Errorf("speech API does not expose async task status")
}

func paramString(params map[string]any, key string) string {
	if params == nil {
		return ""
	}
	value, _ := params[key].(string)
	return strings.TrimSpace(value)
}

func paramFloat(params map[string]any, key string, fallback float64) float64 {
	if params == nil {
		return fallback
	}
	switch value := params[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return fallback
	}
}

func audioMIMEType(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "wav":
		return "audio/wav"
	case "flac":
		return "audio/flac"
	default:
		return "audio/mpeg"
	}
}

func valueOrDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

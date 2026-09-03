// Package videoapi adapts a configurable OpenAI-style asynchronous video API.
package videoapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mediago-dev/mediago-drama/packages/core/pkg/generation"
)

const defaultHTTPTimeout = 120 * time.Second

type Config struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

type Provider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

type videoPayload struct {
	ID       string         `json:"id"`
	Object   string         `json:"object"`
	Model    string         `json:"model"`
	Status   string         `json:"status"`
	Progress int            `json:"progress"`
	VideoURL string         `json:"video_url"`
	URL      string         `json:"url"`
	Error    any            `json:"error"`
	Metadata map[string]any `json:"meta_data"`
	Output   *struct {
		URL string `json:"url"`
	} `json:"output"`
}

type videoEnvelope struct {
	videoPayload
	Data *videoPayload `json:"data"`
}

func NewProvider(config Config) (*Provider, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("video API base URL is not configured")
	}
	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey == "" {
		return nil, generation.ErrMissingAPIKey
	}
	model := strings.TrimSpace(config.Model)
	if model == "" {
		return nil, fmt.Errorf("video API model is not configured")
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &Provider{baseURL: baseURL, apiKey: apiKey, model: model, client: client}, nil
}

func (provider *Provider) Name() string { return generation.ProviderVideoAPI }

func (provider *Provider) Generate(ctx context.Context, request generation.Request) (generation.Response, error) {
	if request.Kind != generation.KindVideo {
		return generation.Response{}, fmt.Errorf("video API only supports video generation")
	}
	if strings.TrimSpace(request.Prompt) == "" {
		return generation.Response{}, generation.ErrMissingPrompt
	}
	if len(request.ReferenceURLs) > 0 {
		return generation.Response{}, fmt.Errorf("video API compatible route does not support reference URLs")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", provider.model); err != nil {
		return generation.Response{}, err
	}
	if err := writer.WriteField("prompt", request.Prompt); err != nil {
		return generation.Response{}, err
	}
	if err := writer.Close(); err != nil {
		return generation.Response{}, err
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.baseURL+"/videos", &body)
	if err != nil {
		return generation.Response{}, err
	}
	httpRequest.Header.Set("Content-Type", writer.FormDataContentType())
	httpRequest.Header.Set("Authorization", provider.authorization())
	return provider.do(httpRequest, "submitted")
}

func (provider *Provider) Get(ctx context.Context, id string) (generation.Response, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return generation.Response{}, fmt.Errorf("video generation id is required")
	}
	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		provider.baseURL+"/videos/"+url.PathEscape(id),
		nil,
	)
	if err != nil {
		return generation.Response{}, err
	}
	httpRequest.Header.Set("Authorization", provider.authorization())
	return provider.do(httpRequest, "running")
}

func (provider *Provider) do(request *http.Request, fallbackStatus string) (generation.Response, error) {
	response, err := provider.client.Do(request)
	if err != nil {
		return generation.Response{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return generation.Response{}, generation.HTTPErrorFromResponse(generation.ProviderVideoAPI, response)
	}
	var envelope videoEnvelope
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return generation.Response{}, err
	}
	payload := envelope.videoPayload
	if payload.ID == "" && envelope.Data != nil {
		payload = *envelope.Data
	}
	videoURL := strings.TrimSpace(payload.VideoURL)
	if videoURL == "" {
		videoURL = strings.TrimSpace(payload.URL)
	}
	if videoURL == "" && payload.Output != nil {
		videoURL = strings.TrimSpace(payload.Output.URL)
	}
	status := normalizeStatus(payload.Status, fallbackStatus, videoURL != "")
	assets := []generation.Asset{}
	if videoURL != "" {
		assets = append(assets, generation.Asset{Kind: generation.KindVideo, URL: videoURL})
	}
	metadata := map[string]any{
		"object":   payload.Object,
		"progress": payload.Progress,
	}
	if payload.Metadata != nil {
		metadata["meta_data"] = payload.Metadata
	}
	if payload.Error != nil {
		metadata["error"] = payload.Error
	}
	return generation.Response{
		ID:       strings.TrimSpace(payload.ID),
		Status:   status,
		Model:    firstNonEmpty(strings.TrimSpace(payload.Model), provider.model),
		Assets:   assets,
		Metadata: metadata,
	}, nil
}

func (provider *Provider) authorization() string {
	if strings.HasPrefix(strings.ToLower(provider.apiKey), "bearer ") {
		return provider.apiKey
	}
	return "Bearer " + provider.apiKey
}

func normalizeStatus(value, fallback string, hasVideo bool) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "completed", "complete", "succeeded", "success", "done":
		return "completed"
	case "failed", "failure", "error", "cancelled", "canceled":
		return "failed"
	case "queued", "pending", "submitted", "created":
		return "submitted"
	case "processing", "running", "in_progress", "in-progress":
		return "running"
	case "":
		if hasVideo {
			return "completed"
		}
		return fallback
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

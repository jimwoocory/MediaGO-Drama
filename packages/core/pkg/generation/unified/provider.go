// Package unified executes media models using one shared aggregation credential.
package unified

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mediago-dev/mediago-drama/packages/core/pkg/generation"
	"github.com/mediago-dev/mediago-drama/packages/core/pkg/generation/speechapi"
	"github.com/mediago-dev/mediago-drama/packages/core/pkg/generation/videoapi"
)

// Config holds credentials and one resolved model route.
type Config struct {
	BaseURL, APIKey string
	Route           generation.ModelRoute
	HTTPClient      *http.Client
}

// Provider executes one model while preserving its exact ID.
type Provider struct{ config Config }

// NewProvider validates a unified provider configuration.
func NewProvider(config Config) (*Provider, error) {
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if config.BaseURL == "" {
		return nil, fmt.Errorf("统一接口 Base URL 尚未配置")
	}
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, generation.ErrMissingAPIKey
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 10 * time.Minute}
	}
	return &Provider{config: config}, nil
}

// Name returns the shared credential provider ID.
func (p *Provider) Name() string { return generation.ProviderUnified }

// Generate dispatches by the selected wire protocol.
func (p *Provider) Generate(ctx context.Context, r generation.Request) (generation.Response, error) {
	if err := generation.ValidateRequestForRoute(r, p.config.Route); err != nil {
		return generation.Response{}, err
	}
	if strings.TrimSpace(r.Prompt) == "" {
		return generation.Response{}, generation.ErrMissingPrompt
	}
	r.Model = p.config.Route.Model
	switch p.config.Route.Adapter {
	case "unified.speech":
		provider, err := speechapi.NewProvider(speechapi.Config{BaseURL: p.config.BaseURL, APIKey: p.config.APIKey, Model: r.Model, HTTPClient: p.config.HTTPClient})
		if err != nil {
			return generation.Response{}, err
		}
		response, err := provider.Generate(ctx, r)
		// Speech has no upstream task ID. Let the workbench allocate a fresh ID
		// instead of reusing the route ID across every generated clip.
		response.ID = ""
		return response, err
	case "unified.videos":
		provider, err := p.video()
		if err != nil {
			return generation.Response{}, err
		}
		response, err := provider.Generate(ctx, r)
		return p.withVideoRoute(response), err
	case "unified.images", "unified.chat-image":
		return p.image(ctx, r)
	default:
		return generation.Response{}, fmt.Errorf("未适配的统一接口协议 %q", p.config.Route.Adapter)
	}
}

func (p *Provider) video() (*videoapi.Provider, error) {
	return videoapi.NewProvider(videoapi.Config{BaseURL: p.config.BaseURL, APIKey: p.config.APIKey, Model: p.config.Route.Model, HTTPClient: p.config.HTTPClient})
}

// Get queries an existing video task without resubmitting it.
func (p *Provider) Get(ctx context.Context, id string) (generation.Response, error) {
	if p.config.Route.Adapter != "unified.videos" {
		return generation.Response{}, fmt.Errorf("此模型不支持异步查询")
	}
	provider, err := p.video()
	if err != nil {
		return generation.Response{}, err
	}
	id = strings.TrimPrefix(id, p.config.Route.ID+":")
	response, err := provider.Get(ctx, id)
	return p.withVideoRoute(response), err
}

func (p *Provider) withVideoRoute(response generation.Response) generation.Response {
	if response.ID != "" {
		response.ID = p.config.Route.ID + ":" + response.ID
	}
	return response
}

type imageURL struct {
	URL string `json:"url"`
}
type imagePart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}
type imageMessage struct {
	Role    string      `json:"role"`
	Content []imagePart `json:"content"`
}

func (p *Provider) image(ctx context.Context, r generation.Request) (generation.Response, error) {
	endpoint := "/images/generations"
	var payload []byte
	var err error
	if p.config.Route.Adapter == "unified.chat-image" {
		endpoint = "/chat/completions"
		parts := []imagePart{{Type: "text", Text: r.Prompt}}
		for _, url := range r.ReferenceURLs {
			parts = append(parts, imagePart{Type: "image_url", ImageURL: &imageURL{URL: url}})
		}
		payload, err = json.Marshal(struct {
			Model      string         `json:"model"`
			Messages   []imageMessage `json:"messages"`
			Modalities []string       `json:"modalities"`
		}{r.Model, []imageMessage{{"user", parts}}, []string{"image", "text"}})
	} else {
		payload, err = json.Marshal(struct {
			Model  string `json:"model"`
			Prompt string `json:"prompt"`
			N      int    `json:"n"`
		}{r.Model, r.Prompt, 1})
	}
	if err != nil {
		return generation.Response{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.BaseURL+endpoint, bytes.NewReader(payload))
	if err != nil {
		return generation.Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.config.APIKey))
	resp, err := p.config.HTTPClient.Do(req)
	if err != nil {
		return generation.Response{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return generation.Response{}, generation.HTTPErrorFromResponse(p.Name(), resp)
	}
	var envelope struct {
		Data []struct {
			URL string `json:"url"`
			B64 string `json:"b64_json"`
		} `json:"data"`
		Choices []struct {
			Message struct {
				Images  []imagePart     `json:"images"`
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&envelope); err != nil {
		return generation.Response{}, fmt.Errorf("解析统一接口图片响应: %w", err)
	}
	assets := []generation.Asset{}
	for _, item := range envelope.Data {
		if item.URL != "" {
			assets = append(assets, generation.Asset{Kind: generation.KindImage, URL: item.URL})
		} else if item.B64 != "" {
			assets = append(assets, generation.Asset{Kind: generation.KindImage, Base64: item.B64, MIMEType: "image/png"})
		}
	}
	for _, choice := range envelope.Choices {
		parts := choice.Message.Images
		var content []imagePart
		if json.Unmarshal(choice.Message.Content, &content) == nil {
			parts = append(parts, content...)
		}
		for _, part := range parts {
			if part.ImageURL != nil && part.ImageURL.URL != "" {
				url := part.ImageURL.URL
				if strings.HasPrefix(url, "data:image/") {
					header, data, ok := strings.Cut(url, ",")
					if ok && strings.HasSuffix(header, ";base64") {
						assets = append(assets, generation.Asset{Kind: generation.KindImage, Base64: data, MIMEType: strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")})
					}
				} else {
					assets = append(assets, generation.Asset{Kind: generation.KindImage, URL: url})
				}
			}
		}
	}
	if len(assets) == 0 {
		return generation.Response{}, fmt.Errorf("统一接口未返回图片；请核对生图协议，文本回复不算生图成功")
	}
	return generation.Response{Status: "completed", Model: r.Model, Assets: assets}, nil
}

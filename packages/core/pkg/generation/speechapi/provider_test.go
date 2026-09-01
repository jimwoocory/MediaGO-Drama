package speechapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mediago-dev/mediago-drama/packages/core/pkg/generation"
)

func TestProviderGeneratesOpenAICompatibleSpeech(t *testing.T) {
	var authorization string
	var payload speechRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/audio/speech" {
			t.Fatalf("path = %q, want /v1/audio/speech", request.URL.Path)
		}
		authorization = request.Header.Get("Authorization")
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		writer.Header().Set("Content-Type", "audio/mpeg")
		_, _ = writer.Write([]byte("audio-bytes"))
	}))
	defer server.Close()

	provider, err := NewProvider(Config{
		BaseURL: server.URL + "/v1/",
		APIKey:  "sk-speech",
		Model:   "custom-tts",
		Voice:   "nova",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	response, err := provider.Generate(context.Background(), generation.Request{
		Kind:    generation.KindAudio,
		RouteID: generation.RouteSpeechAPICompatible,
		Prompt:  "hello",
		Params: map[string]any{
			"speed":  1.25,
			"format": "mp3",
		},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if authorization != "Bearer sk-speech" {
		t.Fatalf("Authorization = %q", authorization)
	}
	if payload.Model != "custom-tts" || payload.Voice != "nova" || payload.Input != "hello" || payload.Speed != 1.25 || payload.ResponseFormat != "mp3" {
		t.Fatalf("payload = %#v", payload)
	}
	if len(response.Assets) != 1 || response.Assets[0].MIMEType != "audio/mpeg" {
		t.Fatalf("response = %#v", response)
	}
	if got, _ := base64.StdEncoding.DecodeString(response.Assets[0].Base64); string(got) != "audio-bytes" {
		t.Fatalf("audio = %q", string(got))
	}
}

package gemini_test

import (
	"RiderMate-Bot-Service/llms"
	"RiderMate-Bot-Service/pkg/gemini"
	"testing"
)

func TestNewGeminiLiveClient(t *testing.T) {
	pageConfig := llms.PageConfig{
		PageName:      "RIDER_SUPPORT",
		IsCustomModel: false,
		BrandName:     "RiderMate",
		Instruction:   "Provide instant delivery rider assistance.",
	}

	client, err := gemini.NewGeminiLiveClient("test-conversation", llms.AudioModeAudio, pageConfig)
	if err != nil {
		t.Fatalf("Failed to create GeminiLiveClient: %v", err)
	}

	if client == nil {
		t.Fatal("Expected client to be non-nil")
	}

	if client.Model() != gemini.DefaultAudioModel {
		t.Errorf("Expected default model %s, got %s", gemini.DefaultAudioModel, client.Model())
	}

	client.SetModel(gemini.BidiFlash001Model)
	if client.Model() != gemini.BidiFlash001Model {
		t.Errorf("Expected model %s after SetModel, got %s", gemini.BidiFlash001Model, client.Model())
	}
}

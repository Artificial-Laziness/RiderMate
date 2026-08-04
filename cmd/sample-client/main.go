package main

import (
	"RiderMate-Bot-Service/llms"
	"RiderMate-Bot-Service/pkg/gemini"
	"RiderMate-Bot-Service/tools"
	"fmt"
	"log"
)

func main() {
	fmt.Println("Starting RiderMate Gemini Live Client Sample...")

	// 1. Initialize Page Configuration
	pageConfig := llms.PageConfig{
		PageName:      "RIDER_SUPPORT",
		IsCustomModel: false,
		BrandName:     "RiderMate",
		Instruction:   "Provide instant, hands-free delivery rider support for active orders, payouts, and route guidance.",
	}

	// 2. Instantiate the Gemini Live Client
	client, err := gemini.NewGeminiLiveClient("conversation-rider-12345", llms.AudioModeAudio, pageConfig)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// 3. Define Callback implementations
	callbacks := llms.Callbacks{
		ConversationProvider:  &mockConversationProvider{},
		TokenCalculator:       &mockTokenCalculator{},
		ToolExecutionProvider: &mockToolExecutionProvider{},
		AudioQueueClearer:     &mockAudioQueueClearer{},
	}
	client.SetCallbacks(callbacks)

	fmt.Printf("RiderMate Gemini Live Client created successfully: model=%s\n", client.Model())
	fmt.Println("Rider voice session and VAD setup initialized.")
}

// Mock Implementations for Callback Interfaces

type mockConversationProvider struct{}

func (m *mockConversationProvider) AddAssistantMessage(text, msgType string) {
	fmt.Printf("[Mock Conversation] AddAssistantMessage: %s (%s)\n", text, msgType)
}
func (m *mockConversationProvider) AddUserMessage(text, msgType string) {
	fmt.Printf("[Mock Conversation] AddUserMessage: %s (%s)\n", text, msgType)
}
func (m *mockConversationProvider) CompleteUserMessage() {
	fmt.Println("[Mock Conversation] CompleteUserMessage")
}
func (m *mockConversationProvider) CompleteAssistantMessage() {
	fmt.Println("[Mock Conversation] CompleteAssistantMessage")
}
func (m *mockConversationProvider) AccumulateUserText(text string) {
	fmt.Printf("[Mock Conversation] AccumulateUserText: %s\n", text)
}
func (m *mockConversationProvider) AccumulateAssistantText(text string) {
	fmt.Printf("[Mock Conversation] AccumulateAssistantText: %s\n", text)
}
func (m *mockConversationProvider) CompleteCurrentMessage() {
	fmt.Println("[Mock Conversation] CompleteCurrentMessage")
}

type mockTokenCalculator struct{}

func (m *mockTokenCalculator) AddTokens(details []llms.TokenDetail) {
	for _, detail := range details {
		fmt.Printf("[Mock TokenCalc] Added %d %s tokens (type=%s, model=%s)\n",
			detail.TokenCount, detail.TokenModality, detail.TokenType, detail.ModelName)
	}
}

type mockToolExecutionProvider struct{}

func (m *mockToolExecutionProvider) ExecuteTool(name, arguments string, tool tools.CustomTools) (string, error) {
	fmt.Printf("[Mock ToolExec] Executing tool '%s' with args '%s'\n", name, arguments)
	return fmt.Sprintf(`{"status": "success", "message": "Result of tool %s"}`, name), nil
}

type mockAudioQueueClearer struct{}

func (m *mockAudioQueueClearer) ClearAudioQueues() {
	fmt.Println("[Mock AudioQueue] Clearing all audio queues on playback device")
}

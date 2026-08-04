package llms

import "RiderMate-Bot-Service/tools"

// AudioMode defines the modality for audio
type AudioMode string

const (
	AudioModeAudio AudioMode = "audio"
	AudioModeText  AudioMode = "text"
)

// Token types and modalities
type TokenType string
type TokenModality string

const (
	TokenTypeInput  TokenType = "input"
	TokenTypeCached TokenType = "cached"
	TokenTypeOutput TokenType = "output"

	TokenModalityAudio TokenModality = "audio"
	TokenModalityText  TokenModality = "text"
)

// Output types
type OutputType string

const (
	OutputTypeAudioDone    OutputType = "audio_done"
	OutputTypeText         OutputType = "text"
	OutputTypeAudio        OutputType = "audio"
	OutputTypeFunction     OutputType = "function"
	OutputTypeControl      OutputType = "control"
	OutputTypeCTA          OutputType = "cta"
	OutputTypeEndCall      OutputType = "end_call"
	OutputTypeForceEndCall OutputType = "force_end_call"
)

// TokenDetail tracks token usage metadata
type TokenDetail struct {
	TokenType     TokenType
	TokenModality TokenModality
	TokenCount    int
	ModelName     string
}

// AudioInput holds raw input audio data
type AudioInput struct {
	ByteData []byte
}

// AudioOutput represents the standard output format for the client
type AudioOutput struct {
	Type                   OutputType
	Text                   string
	Data                   string
	Tool                   interface{}
	GeminiResumableSession string
}

// PageConfig holds details about the client's current location/journey state
type PageConfig struct {
	PageName      string
	IsCustomModel bool
	BrandName     string
	Instruction   string
}

// Interfaces for Callbacks

type ConversationProvider interface {
	AddAssistantMessage(text, msgType string)
	AddUserMessage(text, msgType string)
	CompleteUserMessage()
	CompleteAssistantMessage()
	AccumulateUserText(text string)
	AccumulateAssistantText(text string)
	CompleteCurrentMessage()
}

type TokenCalculator interface {
	AddTokens(details []TokenDetail)
}

type ToolExecutionProvider interface {
	ExecuteTool(name, arguments string, tool tools.CustomTools) (string, error)
}

type AudioQueueClearer interface {
	ClearAudioQueues()
}

// Callbacks groups all provider interfaces used by the client
type Callbacks struct {
	ConversationProvider  ConversationProvider
	TokenCalculator       TokenCalculator
	ToolExecutionProvider ToolExecutionProvider
	AudioQueueClearer     AudioQueueClearer
}

// Global System Instruction Prompts
var BaseInstructionPromptPSGeminiJustAudio = "You are a helpful delivery rider voice assistant for RiderMate. Speak concisely and clearly."

func GetBaseInstructionPromptPSGemini() string {
	return "You are a helpful hands-free voice assistant for BRAND_NAME delivery riders. Support riders with active order status, delivery routes, payout queries, store delays, and rider support escalation."
}

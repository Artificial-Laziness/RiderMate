package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"RiderMate-Bot-Service/llms"
	"RiderMate-Bot-Service/pkg/gemini"
	"RiderMate-Bot-Service/tools"
)

func main() {
	backendFlag := flag.String("backend", "vertex", "Gemini Live backend: 'vertex' or 'aistudio'")
	modelFlag := flag.String("model", "", "Model name override")
	turnsFlag := flag.Int("turns", 2, "Number of test turns to run (default 2)")
	cidFlag := flag.String("cid", "conversation-rider-12345", "Conversation ID")
	triggerFlag := flag.Int64("trigger-tokens", 0, "Context window compression trigger token threshold (0 = default 5000)")
	targetFlag := flag.Int64("target-tokens", 0, "Context window compression sliding window target token count (0 = default 2500)")
	inputFlag := flag.String("input", "text", "User turn input mode: 'text' (clientContent) or 'audio' (realtimeInput 16kHz PCM streamed in real time)")
	audioDirFlag := flag.String("audio-dir", "testdata/rider_audio", "Directory containing turnNN.pcm (16kHz mono s16le) rider utterances for -input=audio")
	trailingSilenceFlag := flag.Duration("trailing-silence", 1500*time.Millisecond, "Silence streamed after each audio utterance so server-side VAD detects end of speech")
	flag.Parse()
	audioInput := strings.EqualFold(*inputFlag, "audio")

	backend := gemini.BackendVertex
	if strings.EqualFold(*backendFlag, "aistudio") || strings.EqualFold(*backendFlag, "ais") {
		backend = gemini.BackendAIStudio
	}

	fmt.Printf("====================================================\n")
	fmt.Printf("  RiderMate Live Client - Backend: %s\n", strings.ToUpper(string(backend)))
	fmt.Printf("====================================================\n")

	// 1. Initialize Page Configuration
	pageConfig := llms.PageConfig{
		PageName:      "RIDER_SUPPORT",
		IsCustomModel: false,
		BrandName:     "RiderMate",
		Instruction:   "",
	}

	// 2. Instantiate the Gemini Live Client
	client, err := gemini.NewGeminiLiveClient(*cidFlag, llms.AudioModeAudio, pageConfig)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	client.SetBackend(backend)
	if *modelFlag != "" {
		client.SetModel(*modelFlag)
	}
	if *triggerFlag > 0 && *targetFlag > 0 {
		client.SetContextWindowCompression(*triggerFlag, *targetFlag)
		fmt.Printf("ContextWindowCompression configured: TriggerTokens=%d, TargetTokens=%d\n", *triggerFlag, *targetFlag)
	}

	// 3. Define Callback implementations
	turnDoneCh := make(chan struct{}, 10)
	tokenTracker := &tokenUsageTracker{}
	convProvider := &sampleConversationProvider{turnDoneCh: turnDoneCh}
	toolProvider := &sampleToolExecutionProvider{tracker: tokenTracker}

	callbacks := llms.Callbacks{
		ConversationProvider:  convProvider,
		TokenCalculator:       tokenTracker,
		ToolExecutionProvider: toolProvider,
		AudioQueueClearer:     &sampleAudioQueueClearer{},
	}
	client.SetCallbacks(callbacks)

	// 4. Listen to output channels
	go func() {
		for out := range client.ReadTexts() {
			if out.Text != "" {
				fmt.Printf("[Assistant Text]: %s\n", out.Text)
			}
		}
	}()

	var audioBytesTotal int
	var audioMu sync.Mutex
	go func() {
		for out := range client.ReadAudios() {
			if out.Type == llms.OutputTypeAudio && len(out.Data) > 0 {
				audioMu.Lock()
				audioBytesTotal += len(out.Data)
				audioMu.Unlock()
			}
		}
	}()

	// 5. Connect to Backend
	fmt.Printf("Connecting to %s (model=%s)...\n", backend, client.Model())
	if err := client.Connect(); err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	fmt.Printf("Connected successfully! Ready for live session.\n\n")

	// Pause briefly to let setup settle
	time.Sleep(1 * time.Second)

	// 6. Execute Conversation Turns
	prompts := []string{
		"Hello RiderMate, who are you and how can you help me during delivery?",
		"What is my current active order destination? Answer in 10 words.",
		"The restaurant is delaying the food pickup. What should I do?",
		"Can you notify the customer about this 10 minute restaurant delay?",
		"The restaurant manager says 5 more minutes. Please record this wait time.",
		"I received the order packages now. How do I confirm store departure?",
		"What is the fastest route to the customer location?",
		"There is heavy traffic on the main avenue. Any alternative route?",
		"I arrived at the customer apartment complex. Can you call the customer?",
		"The customer is not answering the phone. What is the standard protocol?",
		"I left the order at the security gate as requested. How do I take the proof photo?",
		"The delivery is now marked complete. What was my payout for this trip?",
		"Do I have any bonus incentive active for completing orders this evening?",
		"What is my next assigned delivery order?",
		"Thank you RiderMate, I am ending the call now. Goodbye.",
	}

	maxTurns := *turnsFlag
	if maxTurns > len(prompts) {
		maxTurns = len(prompts)
	}

	callStart := time.Now()

	for i := 0; i < maxTurns; i++ {
		turnNum := i + 1
		prompt := prompts[i]
		fmt.Printf("----------------------------------------------------\n")
		fmt.Printf(">>> TURN %d/%d: Sending prompt: %q\n", turnNum, maxTurns, prompt)
		fmt.Printf("----------------------------------------------------\n")

		// Drain any stray signals in turnDoneCh before sending
	drainLoop:
		for {
			select {
			case <-turnDoneCh:
			default:
				break drainLoop
			}
		}

		tokenTracker.StartTurn(turnNum)

		if audioInput {
			pcmPath := filepath.Join(*audioDirFlag, fmt.Sprintf("turn%02d.pcm", turnNum))
			if err := streamAudioTurn(client, pcmPath, *trailingSilenceFlag); err != nil {
				fmt.Printf("Error streaming audio for turn %d: %v\n", turnNum, err)
				break
			}
		} else if err := client.SendTextUpdate(prompt, "user", true); err != nil {
			fmt.Printf("Error sending turn %d: %v\n", turnNum, err)
			break
		}

		// Wait for turn completion or timeout
		select {
		case <-turnDoneCh:
			time.Sleep(300 * time.Millisecond)
			fmt.Printf(">>> TURN %d COMPLETED by model.\n\n", turnNum)
		case <-time.After(30 * time.Second):
			fmt.Printf(">>> TURN %d TIMED OUT after 30s.\n\n", turnNum)
		}

		tokenTracker.EndTurn(turnNum)
		// Natural pause between turns to model a real rider conversation without overlapping/interrupting
		time.Sleep(1500 * time.Millisecond)
	}

	callDuration := time.Since(callStart)

	// 7. Print Session Metrics
	fmt.Printf("====================================================\n")
	fmt.Printf("  SESSION TOKEN USAGE & COST SUMMARY (%s, %s, Duration: %s)\n",
		strings.ToUpper(string(backend)), client.Model(), callDuration.Round(time.Millisecond))
	fmt.Printf("====================================================\n")
	tokenTracker.PrintSummary(client.Model())
	audioMu.Lock()
	fmt.Printf("Total Audio Payload Received: %d bytes (base64)\n", audioBytesTotal)
	audioMu.Unlock()
	fmt.Printf("====================================================\n")
}

type turnSnapshot struct {
	turnNum           int
	eventsCount       int
	activeContext     int
	activeText        int
	activeAudio       int
	netInputDelta     int
	candText          int
	candAudio         int
	naiveSummedPrompt int
	billedPromptText  int
	billedPromptAudio int
	cached            int
	toolsCalled       []string
}

// Token usage tracker
type tokenUsageTracker struct {
	mu                       sync.Mutex
	currentTurn              int
	currentTools             []string
	initialPromptWatermark   int
	prevTurnWatermark        int
	lastPromptWatermark      int
	lastPromptTextWatermark  int
	lastPromptAudioWatermark int
	peakPromptWatermark      int
	turnStartNaivePrompt     int
	turnStartBilledText      int
	turnStartBilledAudio     int
	turnStartCT              int
	turnStartCA              int
	turnStartEvents          int
	naiveSummedPrompt        int
	billedPromptText         int
	billedPromptAudio        int
	candText                 int
	candAudio                int
	cached                   int
	callCount                int
	turnHistory              []turnSnapshot
}

func (t *tokenUsageTracker) StartTurn(turnNum int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.currentTurn = turnNum
	t.currentTools = nil
	t.turnStartNaivePrompt = t.naiveSummedPrompt
	t.turnStartBilledText = t.billedPromptText
	t.turnStartBilledAudio = t.billedPromptAudio
	t.turnStartCT = t.candText
	t.turnStartCA = t.candAudio
	t.turnStartEvents = t.callCount
}

func (t *tokenUsageTracker) RecordToolCall(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.currentTools = append(t.currentTools, name)
}

func (t *tokenUsageTracker) EndTurn(turnNum int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	netDelta := t.lastPromptWatermark - t.prevTurnWatermark
	if netDelta < 0 {
		netDelta = 0
	}
	toolsCopy := append([]string(nil), t.currentTools...)
	snap := turnSnapshot{
		turnNum:           turnNum,
		eventsCount:       t.callCount - t.turnStartEvents,
		activeContext:     t.lastPromptWatermark,
		activeText:        t.lastPromptTextWatermark,
		activeAudio:       t.lastPromptAudioWatermark,
		netInputDelta:     netDelta,
		candText:          t.candText - t.turnStartCT,
		candAudio:         t.candAudio - t.turnStartCA,
		naiveSummedPrompt: t.naiveSummedPrompt - t.turnStartNaivePrompt,
		billedPromptText:  t.billedPromptText - t.turnStartBilledText,
		billedPromptAudio: t.billedPromptAudio - t.turnStartBilledAudio,
		cached:            t.cached,
		toolsCalled:       toolsCopy,
	}
	t.prevTurnWatermark = t.lastPromptWatermark
	t.turnHistory = append(t.turnHistory, snap)
}

func (t *tokenUsageTracker) AddTokens(details []llms.TokenDetail) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.callCount++
	var eventPromptText, eventPromptAudio int
	for _, d := range details {
		switch d.TokenType {
		case llms.TokenTypeInput:
			if d.TokenModality == llms.TokenModalityAudio {
				eventPromptAudio += d.TokenCount
			} else {
				eventPromptText += d.TokenCount
			}
		case llms.TokenTypeOutput:
			if d.TokenModality == llms.TokenModalityAudio {
				t.candAudio += d.TokenCount
			} else {
				t.candText += d.TokenCount
			}
		case llms.TokenTypeCached:
			t.cached += d.TokenCount
		}
		fmt.Printf("  [Token Event Turn %d]: Type=%-7s Modality=%-5s Count=%-5d Model=%s\n",
			t.currentTurn, d.TokenType, d.TokenModality, d.TokenCount, d.ModelName)
	}

	eventPromptTotal := eventPromptText + eventPromptAudio
	if eventPromptTotal > 0 {
		if t.initialPromptWatermark == 0 {
			t.initialPromptWatermark = eventPromptTotal
		}
		t.lastPromptWatermark = eventPromptTotal
		t.lastPromptTextWatermark = eventPromptText
		t.lastPromptAudioWatermark = eventPromptAudio
		if eventPromptTotal > t.peakPromptWatermark {
			t.peakPromptWatermark = eventPromptTotal
		}
		t.naiveSummedPrompt += eventPromptTotal
		t.billedPromptText += eventPromptText
		t.billedPromptAudio += eventPromptAudio
	}
}

func (t *tokenUsageTracker) PrintSummary(modelName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	totalCand := t.candText + t.candAudio
	totalBilledTokens := t.naiveSummedPrompt + totalCand

	fmt.Printf("\n--- Per-Turn Token Breakdown (Normalized vs. Billed Event Sum) ---\n")
	fmt.Printf("| Turn | Events | Active Context (Text / Audio) | Billed Input (Text / Audio) | Out Text | Out Audio | Tools Called |\n")
	fmt.Printf("|------|--------|-------------------------------|-----------------------------|----------|-----------|--------------|\n")
	var totalToolsCalled int
	for _, s := range t.turnHistory {
		toolsStr := "-"
		if len(s.toolsCalled) > 0 {
			toolsStr = strings.Join(s.toolsCalled, ", ")
			totalToolsCalled += len(s.toolsCalled)
		}
		fmt.Printf("| %-4d | %-6d | %-7d (%-5d / %-5d)     | %-7d (%-5d / %-5d)   | %-8d | %-9d | %-12s |\n",
			s.turnNum,
			s.eventsCount,
			s.activeContext, s.activeText, s.activeAudio,
			s.naiveSummedPrompt, s.billedPromptText, s.billedPromptAudio,
			s.candText,
			s.candAudio,
			toolsStr,
		)
	}
	fmt.Println()

	fmt.Printf("=== 1. NORMALIZED / TRUE SESSION ACCOUNTING ===\n")
	fmt.Printf("  Initial Setup Prompt Watermark: %d tokens\n", t.initialPromptWatermark)
	fmt.Printf("  Final Active Context Window:    %d tokens (Text: %d, Audio: %d)\n",
		t.lastPromptWatermark, t.lastPromptTextWatermark, t.lastPromptAudioWatermark)
	fmt.Printf("  Peak Context Window Reached:    %d tokens\n", t.peakPromptWatermark)
	fmt.Printf("  Total Candidate Output Tokens:  %d tokens (Text/Thoughts: %d, Audio: %d)\n",
		totalCand, t.candText, t.candAudio)
	fmt.Printf("  Cached Content Tokens:          %d tokens\n", t.cached)
	fmt.Printf("  True Session Total Footprint:   %d tokens (Final Context + Total Output)\n\n",
		t.lastPromptWatermark+totalCand)

	fmt.Printf("=== 2. CUMULATIVE BILLED TOKENS (ALL %d MODEL INVOCATIONS ACROSS %d TURNS, %d TOOL CALLS) ===\n",
		t.callCount, len(t.turnHistory), totalToolsCalled)
	fmt.Printf("  Billed Input Text Tokens:       %d tokens\n", t.billedPromptText)
	fmt.Printf("  Billed Input Audio Tokens:      %d tokens\n", t.billedPromptAudio)
	fmt.Printf("  Total Billed Input Tokens:      %d tokens\n", t.naiveSummedPrompt)
	fmt.Printf("  Billed Output Text/Thoughts:    %d tokens\n", t.candText)
	fmt.Printf("  Billed Output Audio Tokens:     %d tokens\n", t.candAudio)
	fmt.Printf("  Total Billed Output Tokens:     %d tokens\n", totalCand)
	fmt.Printf("  GRAND TOTAL BILLED TOKENS:      %d tokens\n", totalBilledTokens)
	if t.lastPromptWatermark > 0 {
		inflation := float64(t.naiveSummedPrompt) / float64(t.lastPromptWatermark)
		fmt.Printf("  Prompt Inflation Multiplier:    %.2fx over true final context window\n\n", inflation)
	}

	priceTextInput := 0.50
	priceAudioInput := 3.00
	priceTextOutput := 2.00
	priceAudioOutput := 12.00
	modelNameStr := "Gemini 2.5 Flash Live"

	if strings.Contains(modelName, "3.8") {
		modelNameStr = "Gemini 3.8 Live"
		priceTextInput = 0.75
		priceAudioInput = 3.00
		priceTextOutput = 4.50
		priceAudioOutput = 12.00
	}

	costBilledTextIn := (float64(t.billedPromptText) / 1_000_000.0) * priceTextInput
	costBilledAudioIn := (float64(t.billedPromptAudio) / 1_000_000.0) * priceAudioInput
	costOutText := (float64(t.candText) / 1_000_000.0) * priceTextOutput
	costOutAudio := (float64(t.candAudio) / 1_000_000.0) * priceAudioOutput
	totalBilledCost := costBilledTextIn + costBilledAudioIn + costOutText + costOutAudio

	fmt.Printf("=== 3. CALL COST BREAKDOWN (%s Official Pricing) ===\n", modelNameStr)
	fmt.Printf("  Pricing Rates:\n")
	fmt.Printf("    - Text Input:            $%.2f / 1M tokens\n", priceTextInput)
	fmt.Printf("    - Audio Input:           $%.2f / 1M tokens\n", priceAudioInput)
	fmt.Printf("    - Text/Thoughts Output:  $%.2f / 1M tokens\n", priceTextOutput)
	fmt.Printf("    - Audio Output:          $%.2f / 1M tokens\n\n", priceAudioOutput)
	fmt.Printf("  Component Costs (Cumulative Billed):\n")
	fmt.Printf("    - Input Text Cost:       $%.6f (%d tokens)\n", costBilledTextIn, t.billedPromptText)
	fmt.Printf("    - Input Audio Cost:      $%.6f (%d tokens)\n", costBilledAudioIn, t.billedPromptAudio)
	fmt.Printf("    - Output Text Cost:      $%.6f (%d tokens)\n", costOutText, t.candText)
	fmt.Printf("    - Output Audio Cost:     $%.6f (%d tokens)\n", costOutAudio, t.candAudio)
	fmt.Printf("  --------------------------------------------------------\n")
	fmt.Printf("  TOTAL COST FOR 1 CALL:     $%.6f USD (~%.2f cents / ~₹%.2f INR @ ₹86/$)\n",
		totalBilledCost, totalBilledCost*100, totalBilledCost*86.0)
}

// streamAudioTurn streams a 16kHz mono s16le PCM utterance over realtimeInput in 100ms chunks at
// real-time pace, followed by trailing silence so the server-side VAD can detect end of speech.
// With TURN_INCLUDES_ONLY_ACTIVITY, the trailing silence should not be part of the user turn.
func streamAudioTurn(client *gemini.GeminiLiveClient, pcmPath string, trailingSilence time.Duration) error {
	pcm, err := os.ReadFile(pcmPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", pcmPath, err)
	}
	const bytesPerSecond = 16000 * 2
	const chunkBytes = bytesPerSecond / 10 // 100ms
	silenceBytes := int(trailingSilence.Seconds() * bytesPerSecond)
	silenceBytes -= silenceBytes % 2
	fmt.Printf("[Audio Input]: streaming %s (%.2fs speech + %.2fs silence)\n",
		filepath.Base(pcmPath), float64(len(pcm))/bytesPerSecond, trailingSilence.Seconds())

	payload := append(append([]byte(nil), pcm...), make([]byte, silenceBytes)...)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for off := 0; off < len(payload); off += chunkBytes {
		end := off + chunkBytes
		if end > len(payload) {
			end = len(payload)
		}
		if err := client.SendAudio(llms.AudioInput{ByteData: payload[off:end]}); err != nil {
			return fmt.Errorf("send audio chunk: %w", err)
		}
		<-ticker.C
	}
	return nil
}

// Conversation provider
type sampleConversationProvider struct {
	turnDoneCh chan struct{}
}

func (m *sampleConversationProvider) AddAssistantMessage(text, msgType string) {}
func (m *sampleConversationProvider) AddUserMessage(text, msgType string)      {}
func (m *sampleConversationProvider) CompleteUserMessage()                     {}
func (m *sampleConversationProvider) CompleteAssistantMessage() {
	select {
	case m.turnDoneCh <- struct{}{}:
	default:
	}
}
func (m *sampleConversationProvider) AccumulateUserText(text string) {
	fmt.Printf("[User Transcript]: %s\n", text)
}
func (m *sampleConversationProvider) AccumulateAssistantText(text string) {}
func (m *sampleConversationProvider) CompleteCurrentMessage() {
	select {
	case m.turnDoneCh <- struct{}{}:
	default:
	}
}

// Tool execution provider
type sampleToolExecutionProvider struct {
	tracker *tokenUsageTracker
}

func (m *sampleToolExecutionProvider) ExecuteTool(name, arguments string, tool tools.CustomTools) (string, error) {
	fmt.Printf("[ToolExec] Executing tool '%s' with args: %s\n", name, arguments)
	if m.tracker != nil {
		m.tracker.RecordToolCall(name)
	}
	return `{"ok":true,"id":"1234","dest":"45 Garden St","next":"Burger Barn to 102 Oak Ave","payout":"$7.50","bonus":"peak active"}`, nil
}

// Audio queue clearer
type sampleAudioQueueClearer struct{}

func (m *sampleAudioQueueClearer) ClearAudioQueues() {
	fmt.Println("[AudioQueue] Playback queue flushed")
}

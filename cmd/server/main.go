package main

import (
	"RiderMate-Bot-Service/llms"
	"RiderMate-Bot-Service/pkg/gemini"
	"RiderMate-Bot-Service/tools"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow development testing
	},
}

// TokenDetailJSON represents a structured token measurement
type TokenDetailJSON struct {
	Type     string `json:"type"`
	Modality string `json:"modality"`
	Count    int    `json:"count"`
}

// WSMessage represents standard JSON messaging protocol with the client SPA
type WSMessage struct {
	Type         string            `json:"type"`
	Text         string            `json:"text,omitempty"`
	Role         string            `json:"role,omitempty"`
	Data         string            `json:"data,omitempty"` // Base64 audio chunk
	ToolName     string            `json:"toolName,omitempty"`
	ToolArgs     map[string]any    `json:"toolArgs,omitempty"`
	SessionID    string            `json:"sessionId,omitempty"`
	TokenDetails []TokenDetailJSON `json:"tokenDetails,omitempty"`
}

func main() {
	http.HandleFunc("/ws", handleWS)
	http.Handle("/", http.FileServer(http.Dir("./frontend/dist")))

	port := ":8080"
	log.Printf("Starting WebSocket Live Proxy Server on http://localhost%s...\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket Upgrade failed: %v\n", err)
		return
	}
	defer ws.Close()

	log.Println("New browser client connected to Real-time Stream Proxy")

	// Config
	pageConfig := llms.PageConfig{
		PageName:      "POSTSALES",
		IsCustomModel: false,
		BrandName:     "RiderMate",
		Instruction:   "Provide instant, voice-first assistance to delivery riders on active orders, earnings, route guidance, and support.",
	}

	client, err := gemini.NewGeminiLiveClient("conn-"+fmt.Sprintf("%d", time.Now().Unix()), llms.AudioModeAudio, pageConfig)
	if err != nil {
		log.Printf("Failed to create GeminiLiveClient: %v\n", err)
		return
	}
	defer client.Close()

	// Instantiate ws sender lock to avoid concurrent writes
	var wsMu sync.Mutex
	writeJSON := func(msg WSMessage) error {
		wsMu.Lock()
		defer wsMu.Unlock()
		return ws.WriteJSON(msg)
	}

	// Flag to track if we should operate in live mode or fallback sandbox/mock simulator mode
	var liveMode int32 = 0

	// Custom Provider implementations for callbacks
	callbacks := llms.Callbacks{
		ConversationProvider: &wsConversationProvider{writeJSON: writeJSON},
		TokenCalculator:      &wsTokenCalculator{writeJSON: writeJSON},
		AudioQueueClearer:    &wsAudioQueueClearer{writeJSON: writeJSON},
	}
	callbacks.ToolExecutionProvider = &wsToolExecutionProvider{
		writeJSON: writeJSON,
		callbacks: callbacks,
		client:    client,
	}
	client.SetCallbacks(callbacks)

	// Try establishing the live stream with Vertex AI
	err = client.Connect()
	if err != nil {
		log.Printf("[SANDBOX MODE] Failed to connect to actual Vertex AI stream (%v). Defaulting to Premium Offline Sandbox emulations!\n", err)
		atomic.StoreInt32(&liveMode, 0)
		_ = writeJSON(WSMessage{
			Type:      "status",
			Text:      "connected_sandbox",
			SessionID: client.ConversationID(),
		})
	} else {
		log.Println("[LIVE MODE] Successfully opened live bidirectional stream to Vertex AI!")
		atomic.StoreInt32(&liveMode, 1)
		_ = writeJSON(WSMessage{
			Type:      "status",
			Text:      "connected_live",
			SessionID: client.ConversationID(),
		})
	}

	// Context for graceful worker coordination
	wg := sync.WaitGroup{}
	stopChan := make(chan struct{})
	defer close(stopChan)

	if atomic.LoadInt32(&liveMode) == 1 {
		// --- LIVE MODE ROUTINES ---
		wg.Add(2)

		// Goroutine 1: Read audio output and transcripts from Gemini and relay to browser
		go func() {
			defer wg.Done()
			audioChan := client.ReadAudios()
			textChan := client.ReadTexts()

			for {
				select {
				case <-stopChan:
					return
				case audio, ok := <-audioChan:
					if !ok {
						return
					}
					var t string
					switch audio.Type {
					case llms.OutputTypeAudio:
						t = "audio"
					case llms.OutputTypeAudioDone:
						t = "audio_done"
					case llms.OutputTypeControl:
						t = "control"
					case llms.OutputTypeFunction:
						t = "tool_call"
					case llms.OutputTypeCTA:
						t = "cta"
					case llms.OutputTypeEndCall:
						t = "end_call"
					}
					_ = writeJSON(WSMessage{
						Type: t,
						Data: audio.Data,
						Text: audio.Text,
					})
				case text, ok := <-textChan:
					if !ok {
						return
					}
					_ = writeJSON(WSMessage{
						Type: "text",
						Text: text.Text,
					})
				}
			}
		}()

		// Goroutine 2: Receive messages from browser and stream to Vertex AI
		go func() {
			defer wg.Done()
			for {
				mt, message, err := ws.ReadMessage()
				if err != nil {
					log.Printf("Browser read failed: %v\n", err)
					return
				}

				if mt == websocket.BinaryMessage {
					// Audio stream frame from browser mic
					_ = client.SendAudio(llms.AudioInput{ByteData: message})
				} else if mt == websocket.TextMessage {
					var browserMsg WSMessage
					if err := json.Unmarshal(message, &browserMsg); err == nil {
						switch browserMsg.Type {
						case "text_input":
							_ = client.SendTextUpdate(browserMsg.Text, "user", true)
						case "force_end_call":
							_ = client.ForceEndCall()
						case "interrupt":
							client.HandleUserInterrupt()
						}
					}
				}
			}
		}()

		wg.Wait()
	} else {
		// --- OFFLINE SANDBOX SIMULATOR MODE ---
		// We emulate real responses to demonstrate visual waveforms, transcripts, and tool activation!
		log.Println("Starting mock simulator session listener")
		for {
			mt, message, err := ws.ReadMessage()
			if err != nil {
				break
			}

			if mt == websocket.BinaryMessage {
				// User is speaking (sending mic chunks). We discard since we are a simulator,
				// but we can animate a waveform visualizer based on active stream receipt!
				continue
			}

			if mt == websocket.TextMessage {
				var browserMsg WSMessage
				if err := json.Unmarshal(message, &browserMsg); err != nil {
					continue
				}

				switch browserMsg.Type {
				case "text_input":
					// Mirror User input immediately
					_ = writeJSON(WSMessage{
						Type: "transcript_user",
						Text: browserMsg.Text,
					})

					// Run mock response timeline
					go emulateSandboxDialogue(browserMsg.Text, writeJSON)

				case "force_end_call":
					_ = writeJSON(WSMessage{
						Type: "transcript_assistant",
						Text: "Ending session. Thank you for using RiderMate Support. Drive safely!",
					})
					streamSineMockAudio(400, 1.0, writeJSON) // play a sound
					_ = writeJSON(WSMessage{Type: "end_call"})

				case "interrupt":
					_ = writeJSON(WSMessage{Type: "control", Text: "clear"})
					_ = writeJSON(WSMessage{
						Type: "transcript_assistant",
						Text: "[Interrupted by Rider...]",
					})
				}
			}
		}
	}
}

// emulateSandboxDialogue builds a natural conversation timeline for testing UI components
func emulateSandboxDialogue(prompt string, writeJSON func(WSMessage) error) {
	time.Sleep(1 * time.Second)

	// 1. Send assistant start typing/transcription
	_ = writeJSON(WSMessage{
		Type: "text",
		Text: "Checking that for you right now, rider. ",
	})

	// Send mock structured tokens measurement for visualization
	_ = writeJSON(WSMessage{
		Type: "metrics",
		TokenDetails: []TokenDetailJSON{
			{Type: "input", Modality: "text", Count: 184},
			{Type: "input", Modality: "audio", Count: 420},
			{Type: "output", Modality: "text", Count: 75},
			{Type: "output", Modality: "audio", Count: 960},
		},
	})

	// 2. Play minor voice chime/audio
	streamSineMockAudio(440, 0.5, writeJSON)

	time.Sleep(1 * time.Second)

	// 3. Emulate tool activation if user asked for cancellation/status/earning/support
	if containsAny(prompt, "cancel", "refund", "status", "order") {
		_ = writeJSON(WSMessage{
			Type:     "tool_call",
			ToolName: "BookingCancellationAgent",
			ToolArgs: map[string]any{
				"orderId": "RD892104",
				"reason":  "Store closed / rider emergency",
			},
		})
		time.Sleep(2 * time.Second)

		// Tool complete result
		_ = writeJSON(WSMessage{
			Type: "text",
			Text: "I found order RD892104. The order cancellation has been verified, and a delay compensation fee of ₹45 has been credited to your rider account.",
		})
		streamSineMockAudio(600, 1.2, writeJSON)
	} else if containsAny(prompt, "agent", "human", "transfer", "support") {
		// Trigger human handoff tool
		_ = writeJSON(WSMessage{
			Type:     "tool_call",
			ToolName: "AgentConnectTool",
			ToolArgs: map[string]any{
				"queue": "rider_emergency_support",
			},
		})
		time.Sleep(1500 * time.Millisecond)
		_ = writeJSON(WSMessage{
			Type: "text",
			Text: "Connecting you directly to Rider Support Priority Line. Please hold on and stay safe.",
		})
		streamSineMockAudio(440, 2.0, writeJSON)
		_ = writeJSON(WSMessage{Type: "cta", Text: "Connected to Rider Support"})
	} else if containsAny(prompt, "earning", "payout", "pay", "money") {
		_ = writeJSON(WSMessage{
			Type:     "tool_call",
			ToolName: "EnquiryTool",
			ToolArgs: map[string]any{
				"query": "weekly_payout_details",
			},
		})
		time.Sleep(1500 * time.Millisecond)
		_ = writeJSON(WSMessage{
			Type: "text",
			Text: "Your total earnings today are ₹1,480 across 12 completed orders, including ₹150 in peak incentive bonuses.",
		})
		streamSineMockAudio(520, 1.0, writeJSON)
	} else {
		// Generic assistant response
		_ = writeJSON(WSMessage{
			Type: "text",
			Text: "I can assist you with active order updates, store delays, earnings queries, or connecting to Rider Support. What do you need?",
		})
		streamSineMockAudio(520, 0.8, writeJSON)
	}

	_ = writeJSON(WSMessage{Type: "audio_done"})
	_ = writeJSON(WSMessage{Type: "transcript_assistant", Text: ""})
}

// streamSineMockAudio synthesizes clean 16kHz sine waves to let the browser play real sounds in sandbox mode
func streamSineMockAudio(freq float64, durationSec float64, writeJSON func(WSMessage) error) {
	const sampleRate = 16000
	totalSamples := int(sampleRate * durationSec)
	chunkSize := 1600 // 100ms frames

	for i := 0; i < totalSamples; i += chunkSize {
		limit := i + chunkSize
		if limit > totalSamples {
			limit = totalSamples
		}

		samplesCount := limit - i
		pcmData := make([]byte, samplesCount*2) // 16-bit PCM

		for j := 0; j < samplesCount; j++ {
			t := float64(i+j) / float64(sampleRate)
			val := math.Sin(2 * math.Pi * freq * t)

			// Scale to signed 16-bit
			ival := int16(val * 16384.0) // half volume
			pcmData[j*2] = byte(ival & 0xff)
			pcmData[j*2+1] = byte((ival >> 8) & 0xff)
		}

		encoded := base64.StdEncoding.EncodeToString(pcmData)
		_ = writeJSON(WSMessage{
			Type: "audio",
			Data: encoded,
		})

		time.Sleep(100 * time.Millisecond) // Pacify 100ms chunks
	}
}

func containsAny(s string, terms ...string) bool {
	lower := s
	// Simple lowercase converter for basic routing
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			b := make([]byte, len(s))
			for j := 0; j < len(s); j++ {
				if s[j] >= 'A' && s[j] <= 'Z' {
					b[j] = s[j] + 32
				} else {
					b[j] = s[j]
				}
			}
			lower = string(b)
			break
		}
	}
	for _, term := range terms {
		// basic substring match
		if containsSub(lower, term) {
			return true
		}
	}
	return false
}

func containsSub(s, term string) bool {
	if len(term) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(term); i++ {
		if s[i:i+len(term)] == term {
			return true
		}
	}
	return false
}

// WS Callback structures forwarding events to WebSocket JSON packets

type wsConversationProvider struct {
	writeJSON func(WSMessage) error
}

func (w *wsConversationProvider) AddAssistantMessage(text, msgType string) {
	_ = w.writeJSON(WSMessage{Type: "transcript_assistant", Text: text})
}
func (w *wsConversationProvider) AddUserMessage(text, msgType string) {
	_ = w.writeJSON(WSMessage{Type: "transcript_user", Text: text})
}
func (w *wsConversationProvider) CompleteUserMessage() {
	_ = w.writeJSON(WSMessage{Type: "transcript_user", Text: ""})
}
func (w *wsConversationProvider) CompleteAssistantMessage() {
	_ = w.writeJSON(WSMessage{Type: "transcript_assistant", Text: ""})
}
func (w *wsConversationProvider) AccumulateUserText(text string) {
	_ = w.writeJSON(WSMessage{Type: "stream_transcript_user", Text: text})
}
func (w *wsConversationProvider) AccumulateAssistantText(text string) {
	_ = w.writeJSON(WSMessage{Type: "stream_transcript_assistant", Text: text})
}
func (w *wsConversationProvider) CompleteCurrentMessage() {}

type wsTokenCalculator struct {
	writeJSON func(WSMessage) error
}

func (w *wsTokenCalculator) AddTokens(details []llms.TokenDetail) {
	var list []TokenDetailJSON
	for _, detail := range details {
		list = append(list, TokenDetailJSON{
			Type:     string(detail.TokenType),
			Modality: string(detail.TokenModality),
			Count:    detail.TokenCount,
		})
	}
	_ = w.writeJSON(WSMessage{
		Type:         "metrics",
		TokenDetails: list,
	})
}

type wsAudioQueueClearer struct {
	writeJSON func(WSMessage) error
}

func (w *wsAudioQueueClearer) ClearAudioQueues() {
	_ = w.writeJSON(WSMessage{Type: "control", Text: "clear"})
}

type wsToolExecutionProvider struct {
	writeJSON func(WSMessage) error
	callbacks llms.Callbacks
	client    *gemini.GeminiLiveClient
}

func (w *wsToolExecutionProvider) ExecuteTool(name, arguments string, tool tools.CustomTools) (string, error) {
	log.Printf("Executing tool %s with args: %s\n", name, arguments)

	var argsMap map[string]any
	_ = json.Unmarshal([]byte(arguments), &argsMap)

	// Send visual log to browser SPA
	_ = w.writeJSON(WSMessage{
		Type:     "tool_call",
		ToolName: name,
		ToolArgs: argsMap,
	})

	// Perform standard mock behavior or let it execute
	time.Sleep(1 * time.Second)
	result := fmt.Sprintf(`{"status": "success", "result": "Mock result for tool %s"}`, name)
	return result, nil
}

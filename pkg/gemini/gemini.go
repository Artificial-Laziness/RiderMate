package gemini

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"RiderMate-Bot-Service/llms"
	"RiderMate-Bot-Service/tools"
	"RiderMate-Bot-Service/tools/riderSupport/gemini"
	"RiderMate-Bot-Service/tools/searchTool"
	"RiderMate-Bot-Service/utils/apiLogUtils"
	"RiderMate-Bot-Service/utils/consul"

	"cloud.google.com/go/auth/credentials"
	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
	"google.golang.org/genai"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary
var eventIDCounter uint64

// Constants
const (
	DefaultAudioModel = "gemini-2.5-flash-native-audio-preview"
	DefaultQueueSize  = 10000
)

// Default token configuration
var (
	defaultTriggerTokens = int64(25600)
	defaultTargetTokens  = int64(12800)
)

// FunctionCallAccumulator holds accumulating function call data
type FunctionCallAccumulator struct {
	CallID    string
	Name      string
	Arguments string
	Complete  bool
}

// Barebones Bidi WebSocket Protocol Structs
type bidiSetupPayload struct {
	Setup bidiSetupConfig `json:"setup"`
}

type bidiSetupConfig struct {
	Model             string                `json:"model"`
	GenerationConfig  *bidiGenerationConfig `json:"generationConfig,omitempty"`
	SystemInstruction *bidiContent          `json:"systemInstruction,omitempty"`
	Tools             []*bidiTool           `json:"tools,omitempty"`
}

type bidiGenerationConfig struct {
	ResponseModalities []string          `json:"responseModalities,omitempty"`
	Temperature        *float32          `json:"temperature,omitempty"`
	SpeechConfig       *bidiSpeechConfig `json:"speechConfig,omitempty"`
}

type bidiSpeechConfig struct {
	VoiceConfig *bidiVoiceConfig `json:"voiceConfig,omitempty"`
}

type bidiVoiceConfig struct {
	PrebuiltVoiceConfig *bidiPrebuiltVoiceConfig `json:"prebuiltVoiceConfig,omitempty"`
}

type bidiPrebuiltVoiceConfig struct {
	VoiceName string `json:"voiceName,omitempty"`
}

type bidiContent struct {
	Role  string      `json:"role,omitempty"`
	Parts []*bidiPart `json:"parts,omitempty"`
}

type bidiPart struct {
	Text       string          `json:"text,omitempty"`
	InlineData *bidiInlineData `json:"inlineData,omitempty"`
}

type bidiInlineData struct {
	MimeType string `json:"mimeType,omitempty"`
	Data     []byte `json:"data,omitempty"`
}

type bidiTool struct {
	FunctionDeclarations []*bidiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type bidiFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type bidiServerMessage struct {
	ServerContent *bidiServerContent `json:"serverContent,omitempty"`
	ToolCall      *bidiToolCall      `json:"toolCall,omitempty"`
}

type bidiServerContent struct {
	ModelTurn    *bidiContent `json:"modelTurn,omitempty"`
	Interrupted  bool         `json:"interrupted,omitempty"`
	TurnComplete bool         `json:"turnComplete,omitempty"`
}

type bidiToolCall struct {
	FunctionCalls []*bidiFunctionCall `json:"functionCalls,omitempty"`
}

type bidiFunctionCall struct {
	ID   string                 `json:"id,omitempty"`
	Name string                 `json:"name,omitempty"`
	Args map[string]interface{} `json:"args,omitempty"`
}

// GeminiLiveClient handles real-time communication with Gemini Live API via barebones WebSocket
type GeminiLiveClient struct {
	id string

	// Direct WebSocket connection
	conn  *websocket.Conn
	model string

	// Configuration
	audioMode     llms.AudioMode
	sessionConfig *genai.LiveConnectConfig
	tools         map[string]tools.CustomTools
	MimeType      string

	// Output channels
	audioOutput chan llms.AudioOutput
	textOutput  chan llms.AudioOutput

	// Context and synchronization
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex

	brandName string

	// Connection state
	connected bool

	// Event handling
	eventQueue chan *genai.LiveServerMessage

	// Conversation tracking
	conversationId string
	userSpeaking   int32 // 0 = false, 1 = true

	// Response tracking
	responseInProgress int32
	currentResponseID  string // Track current response being processed

	// Callbacks for interacting with handler
	callbacks llms.Callbacks

	// Function call accumulation fields
	currentFunctionCall *FunctionCallAccumulator
	functionCallMu      sync.RWMutex

	// Unified write queues for thread-safe operations
	writeQueue chan interface{} // All operations go through this
	writeMu    sync.Mutex       // Single mutex for operations

	// Response coordination
	pendingResponse   interface{}
	pendingResponseMu sync.Mutex

	// JSON stream pooling
	jsonStream *jsoniter.Stream

	ResumptionHandle string

	customModel bool
}

// NewGeminiLiveClient creates a new instance of GeminiLiveClient
func NewGeminiLiveClient(conversationId string, audioMode llms.AudioMode, pageConfig llms.PageConfig) (*GeminiLiveClient, error) {
	ctx, cancel := context.WithCancel(context.Background())

	geminiClient := &GeminiLiveClient{
		model:          DefaultAudioModel,
		audioMode:      audioMode,
		tools:          make(map[string]tools.CustomTools),
		audioOutput:    make(chan llms.AudioOutput, DefaultQueueSize),
		textOutput:     make(chan llms.AudioOutput, DefaultQueueSize),
		ctx:            ctx,
		cancel:         cancel,
		connected:      false,
		eventQueue:     make(chan *genai.LiveServerMessage, DefaultQueueSize),
		conversationId: conversationId,
		callbacks:      llms.Callbacks{},
		writeQueue:     make(chan interface{}, 200),
		MimeType:       "audio/pcm;rate=16000",
		jsonStream:     jsoniter.NewStream(jsoniter.ConfigCompatibleWithStandardLibrary, nil, 512),
		customModel:    pageConfig.IsCustomModel,
		brandName:      pageConfig.BrandName,
	}

	geminiClient.initializeSession("", pageConfig.IsCustomModel)

	if !pageConfig.IsCustomModel {
		geminiClient.setupAllTools(pageConfig)
	}

	return geminiClient, nil
}

func (c *GeminiLiveClient) initializeSession(resumptionHandle string, isCustomModel bool) {
	modalities := []genai.Modality{genai.ModalityText}
	if c.audioMode == llms.AudioModeAudio {
		modalities = []genai.Modality{genai.ModalityAudio}
	}

	systemPrompt := strings.ReplaceAll(llms.GetBaseInstructionPromptRiderSupport(), "BRAND_NAME", c.brandName)
	if isCustomModel {
		systemPrompt = llms.BaseInstructionPromptRiderSupportJustAudio
	}

	sessionConfig := &genai.LiveConnectConfig{
		ResponseModalities:       modalities,
		InputAudioTranscription:  &genai.AudioTranscriptionConfig{},
		OutputAudioTranscription: &genai.AudioTranscriptionConfig{},
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				genai.NewPartFromText(systemPrompt),
			},
			Role: "user",
		},
		Temperature: genai.Ptr(float32(0.6)),
		SpeechConfig: &genai.SpeechConfig{
			LanguageCode: "en-IN",
			VoiceConfig: &genai.VoiceConfig{
				PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{
					VoiceName: "Aoede",
				},
			},
		},
		ContextWindowCompression: &genai.ContextWindowCompressionConfig{
			TriggerTokens: &defaultTriggerTokens,
			SlidingWindow: &genai.SlidingWindow{
				TargetTokens: &defaultTargetTokens,
			},
		},
		SessionResumption: &genai.SessionResumptionConfig{
			Handle: resumptionHandle,
		},
		RealtimeInputConfig: &genai.RealtimeInputConfig{
			TurnCoverage: genai.TurnCoverageTurnIncludesOnlyActivity,
			AutomaticActivityDetection: &genai.AutomaticActivityDetection{
				Disabled: false,
			},
		},
		Tools: []*genai.Tool{},
	}
	c.sessionConfig = sessionConfig
}

func (c *GeminiLiveClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *GeminiLiveClient) ConversationID() string {
	return c.conversationId
}

func (c *GeminiLiveClient) SetCallbacks(callbacks llms.Callbacks) {
	c.callbacks = callbacks
}

func (c *GeminiLiveClient) SetModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.model = model
}

func (c *GeminiLiveClient) Model() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.model
}

func (c *GeminiLiveClient) setupAllTools(pageConfig llms.PageConfig) {
	allTools := make(map[string]tools.CustomTools, 16)

	if pageConfig.PageName == "RIDER_SUPPORT" || pageConfig.PageName == "" {
		syso := strings.ReplaceAll(llms.GetBaseInstructionPromptRiderSupport(), "BRAND_NAME", c.brandName) + pageConfig.Instruction

		c.sessionConfig.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{
				genai.NewPartFromText(syso),
			},
			Role: "model",
		}

		cancelTool := gemini.OrderCancellationAgent{}
		orderReceiptTool := gemini.OrderReceiptTool{}
		storeArrivalTool := gemini.StoreArrivalTool{}
		noteCorrectionTool := gemini.ContactNoteTool{}
		specialClaimTool := gemini.SpecialClaimTool{}
		trackerTool := gemini.ObservabilityTool{}
		rescheduleTool := gemini.DateChangeAgent{}
		agentTool := gemini.AgentConnectTool{}
		enquiryTool := gemini.EnquiryTool{}
		riderBaggageTool := gemini.BaggageTool{}
		addonPurchaseTool := gemini.AddonPurchaseAgent{}
		orderStatusTool := gemini.OrderStatusTool{}
		endCallTool := gemini.EndCallTool{}

		allTools[cancelTool.GetName()] = cancelTool
		allTools[orderReceiptTool.GetName()] = orderReceiptTool
		allTools[storeArrivalTool.GetName()] = storeArrivalTool
		allTools[noteCorrectionTool.GetName()] = noteCorrectionTool
		allTools[specialClaimTool.GetName()] = specialClaimTool
		allTools[trackerTool.GetName()] = trackerTool
		allTools[rescheduleTool.GetName()] = rescheduleTool
		allTools[agentTool.GetName()] = agentTool
		allTools[enquiryTool.GetName()] = enquiryTool
		allTools[riderBaggageTool.GetName()] = riderBaggageTool
		allTools[addonPurchaseTool.GetName()] = addonPurchaseTool
		allTools[orderStatusTool.GetName()] = orderStatusTool
		allTools[endCallTool.GetName()] = endCallTool
	} else {
		var (
			deliverySearchTool  = searchTool.SearchTool{}
			deliveryInfoTool    = tools.DeliveryInformationTool{}
			payoutTool          = tools.PayoutTool{}
			thermalBagTool      = tools.ThermalBagTool{}
			orderCancelTool     = tools.OrderCancellationTool{}
			timeExtensionTool   = tools.TimeExtensionTool{}
			storeWaitTool       = tools.StoreWaitTimeTool{}
			bonusIncentiveTool  = tools.BonusIncentiveTool{}
			cateringBoxTool     = tools.CateringBoxTool{}
			vehicleSupportTool  = tools.VehicleSupportTool{}
			incidentReportTool  = tools.IncidentReportTool{}
			routeAssistanceTool = tools.RouteAssistanceTool{}
			riderGearTool       = tools.RiderGearTool{}
			riderPayoutTool     = tools.RiderPayoutTool{}
			customerContactTool = tools.CustomerContactTool{}
		)

		allTools[deliverySearchTool.GetName()] = deliverySearchTool
		allTools[deliveryInfoTool.GetName()] = deliveryInfoTool
		allTools[payoutTool.GetName()] = payoutTool
		allTools[thermalBagTool.GetName()] = thermalBagTool
		allTools[orderCancelTool.GetName()] = orderCancelTool
		allTools[timeExtensionTool.GetName()] = timeExtensionTool
		allTools[storeWaitTool.GetName()] = storeWaitTool
		allTools[bonusIncentiveTool.GetName()] = bonusIncentiveTool
		allTools[cateringBoxTool.GetName()] = cateringBoxTool
		allTools[vehicleSupportTool.GetName()] = vehicleSupportTool
		allTools[incidentReportTool.GetName()] = incidentReportTool
		allTools[routeAssistanceTool.GetName()] = routeAssistanceTool
		allTools[riderGearTool.GetName()] = riderGearTool
		allTools[riderPayoutTool.GetName()] = riderPayoutTool
		allTools[customerContactTool.GetName()] = customerContactTool
	}

	c.tools = make(map[string]tools.CustomTools)
	for name, tool := range allTools {
		c.tools[name] = tool
	}

	c.sessionConfig.Tools = make([]*genai.Tool, 0, len(c.tools))
	if len(c.tools) > 0 {
		functionDeclarations := make([]*genai.FunctionDeclaration, 0, len(c.tools))
		for _, tool := range c.tools {
			functionDeclarations = append(functionDeclarations, tooToFunctionDeclaration(tool))
		}
		c.sessionConfig.Tools = append(c.sessionConfig.Tools, &genai.Tool{
			FunctionDeclarations: functionDeclarations,
			CodeExecution:        nil,
		})
	}
}

func tooToFunctionDeclaration(tool tools.CustomTools) *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        tool.GetName(),
		Description: tool.GetDescription(),
		Parameters:  nil,
	}
}

func (c *GeminiLiveClient) Connect() error {
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	// 1. Fetch Service Account credentials from consul / config
	creds := consul.ApiKeys.GeminiCredsVB
	credsBytes, _ := json.Marshal(creds)

	// 2. Generate signed OAuth2 access token
	var tokenValue string
	options := credentials.DetectOptions{
		CredentialsJSON: credsBytes,
		Scopes:          []string{"https://www.googleapis.com/auth/cloud-platform"},
	}
	credsFromFile, err := credentials.DetectDefault(&options)
	if err != nil {
		apiLogUtils.LogGeneralError("Failed to detect credentials: %v", err)
	} else {
		tok, errTok := credsFromFile.Token(c.ctx)
		if errTok != nil {
			apiLogUtils.LogGeneralError("Failed to obtain token from credentials: %v", errTok)
		} else if tok != nil {
			tokenValue = tok.Value
		}
	}

	if tokenValue == "" {
		tokenValue = "mock-bearer-token"
	}

	projectID, ok := creds["project_id"].(string)
	if !ok || projectID == "" {
		projectID = "mock-bard-pilot-project-id"
	}
	location := "us-central1"

	// 3. Construct barebones Vertex AI WebSocket URL
	wsURL := fmt.Sprintf("wss://%s-aiplatform.googleapis.com/ws/google.cloud.aiplatform.v1.LlmBidiService/BidiGenerateContent", location)

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+tokenValue)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(c.ctx, wsURL, headers)
	if err != nil {
		apiLogUtils.LogGeneralError("Failed to dial WebSocket %s: %v", wsURL, err)
		c.cancel()
		return fmt.Errorf("failed to connect to Gemini Live WebSocket: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()

	// 4. Transmit initial Bidi setup payload over WebSocket
	if err := c.sendSetupPayload(projectID, location); err != nil {
		_ = c.Close()
		return fmt.Errorf("failed to send setup payload: %w", err)
	}

	go c.receiveResponses()
	go c.processEventQueue()
	go c.handleWebSocketWrites()
	go c.heartbeat()

	apiLogUtils.LogGeneralInfo(c.conversationId + " Successfully connected to Gemini Live via Barebones WebSocket")
	return nil
}

func (c *GeminiLiveClient) sendSetupPayload(projectID, location string) error {
	fullModel := fmt.Sprintf("projects/%s/locations/%s/publishers/google/models/%s", projectID, location, c.model)

	var modalities []string
	for _, m := range c.sessionConfig.ResponseModalities {
		modalities = append(modalities, string(m))
	}

	setupMsg := bidiSetupPayload{
		Setup: bidiSetupConfig{
			Model: fullModel,
			GenerationConfig: &bidiGenerationConfig{
				ResponseModalities: modalities,
				Temperature:        c.sessionConfig.Temperature,
			},
		},
	}

	if c.sessionConfig.SpeechConfig != nil &&
		c.sessionConfig.SpeechConfig.VoiceConfig != nil &&
		c.sessionConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig != nil {
		setupMsg.Setup.GenerationConfig.SpeechConfig = &bidiSpeechConfig{
			VoiceConfig: &bidiVoiceConfig{
				PrebuiltVoiceConfig: &bidiPrebuiltVoiceConfig{
					VoiceName: c.sessionConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName,
				},
			},
		}
	}

	if c.sessionConfig.SystemInstruction != nil {
		var parts []*bidiPart
		for _, p := range c.sessionConfig.SystemInstruction.Parts {
			if p != nil && p.Text != "" {
				parts = append(parts, &bidiPart{Text: p.Text})
			}
		}
		setupMsg.Setup.SystemInstruction = &bidiContent{
			Role:  c.sessionConfig.SystemInstruction.Role,
			Parts: parts,
		}
	}

	if len(c.sessionConfig.Tools) > 0 {
		for _, tool := range c.sessionConfig.Tools {
			if tool != nil && len(tool.FunctionDeclarations) > 0 {
				var fds []*bidiFunctionDeclaration
				for _, fd := range tool.FunctionDeclarations {
					if fd != nil {
						fds = append(fds, &bidiFunctionDeclaration{
							Name:        fd.Name,
							Description: fd.Description,
						})
					}
				}
				setupMsg.Setup.Tools = append(setupMsg.Setup.Tools, &bidiTool{
					FunctionDeclarations: fds,
				})
			}
		}
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("websocket connection is nil")
	}
	return c.conn.WriteJSON(setupMsg)
}

func (c *GeminiLiveClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false
	if c.conn != nil {
		_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = c.conn.Close()
		c.conn = nil
	}
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

func (c *GeminiLiveClient) SendAudio(audioData llms.AudioInput) error {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return fmt.Errorf("client not connected")
	}
	c.mu.RUnlock()

	msg := map[string]interface{}{
		"realtimeInput": map[string]interface{}{
			"mediaChunks": []map[string]interface{}{
				{
					"mimeType": c.MimeType,
					"data":     base64.StdEncoding.EncodeToString(audioData.ByteData),
				},
			},
		},
	}

	return c.sendEvent(msg)
}

func (c *GeminiLiveClient) SendTextUpdate(text, role string, generateResponse bool) error {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return fmt.Errorf("client not connected")
	}
	c.mu.RUnlock()

	if role == "assistant" {
		if c.callbacks.ConversationProvider != nil {
			c.callbacks.ConversationProvider.AddAssistantMessage(text, "text")
		}
	} else {
		if c.callbacks.ConversationProvider != nil {
			c.callbacks.ConversationProvider.AddUserMessage(text, "text")
		}
	}

	msg := map[string]interface{}{
		"clientContent": map[string]interface{}{
			"turns": []map[string]interface{}{
				{
					"role": role,
					"parts": []map[string]interface{}{
						{"text": text},
					},
				},
			},
			"turnComplete": generateResponse,
		},
	}
	return c.sendEvent(msg)
}

func (c *GeminiLiveClient) ForceEndCall() error {
	output := llms.AudioOutput{
		Type: llms.OutputTypeEndCall,
	}
	select {
	case c.audioOutput <- output:
	default:
	}
	return nil
}

func (c *GeminiLiveClient) HandleUserInterrupt() {
	atomic.StoreInt32(&c.userSpeaking, 1)
	if c.callbacks.AudioQueueClearer != nil {
		c.callbacks.AudioQueueClearer.ClearAudioQueues()
	}
}

func (c *GeminiLiveClient) ReadAudios() <-chan llms.AudioOutput {
	return c.audioOutput
}

func (c *GeminiLiveClient) ReadTexts() <-chan llms.AudioOutput {
	return c.textOutput
}

func (c *GeminiLiveClient) sendEvent(event interface{}) error {
	select {
	case c.writeQueue <- event:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c *GeminiLiveClient) handleWebSocketWrites() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case event := <-c.writeQueue:
			c.writeMu.Lock()
			if c.conn != nil {
				_ = c.conn.WriteJSON(event)
			}
			c.writeMu.Unlock()
		}
	}
}

func (c *GeminiLiveClient) receiveResponses() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()
			if conn == nil {
				return
			}

			_, messageBytes, err := conn.ReadMessage()
			if err != nil {
				c.mu.RLock()
				isConn := c.connected
				c.mu.RUnlock()
				if !isConn {
					return
				}
				apiLogUtils.LogGeneralError("WebSocket read error: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}

			var serverMsg bidiServerMessage
			if err := json.Unmarshal(messageBytes, &serverMsg); err != nil {
				apiLogUtils.LogGeneralError("Failed to unmarshal server message: %v", err)
				continue
			}

			genaiMsg := convertBidiToServerMessage(&serverMsg)
			select {
			case c.eventQueue <- genaiMsg:
			default:
				go c.processServerResponse(genaiMsg)
			}
		}
	}
}

func convertBidiToServerMessage(msg *bidiServerMessage) *genai.LiveServerMessage {
	if msg == nil {
		return nil
	}
	res := &genai.LiveServerMessage{}
	if msg.ServerContent != nil {
		res.ServerContent = &genai.LiveServerContent{
			Interrupted:  msg.ServerContent.Interrupted,
			TurnComplete: msg.ServerContent.TurnComplete,
		}
		if msg.ServerContent.ModelTurn != nil {
			res.ServerContent.ModelTurn = &genai.Content{
				Role: msg.ServerContent.ModelTurn.Role,
			}
			for _, p := range msg.ServerContent.ModelTurn.Parts {
				if p == nil {
					continue
				}
				part := &genai.Part{
					Text: p.Text,
				}
				if p.InlineData != nil {
					part.InlineData = &genai.Blob{
						MIMEType: p.InlineData.MimeType,
						Data:     p.InlineData.Data,
					}
				}
				res.ServerContent.ModelTurn.Parts = append(res.ServerContent.ModelTurn.Parts, part)
			}
		}
	}
	return res
}

func (c *GeminiLiveClient) processEventQueue() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case response := <-c.eventQueue:
			c.processServerResponse(response)
		}
	}
}

func (c *GeminiLiveClient) processServerResponse(response *genai.LiveServerMessage) {
	if response == nil {
		return
	}
	if response.ServerContent != nil {
		if response.ServerContent.Interrupted {
			c.HandleUserInterrupt()
		}
		if response.ServerContent.ModelTurn != nil {
			for _, part := range response.ServerContent.ModelTurn.Parts {
				if part.Text != "" {
					output := llms.AudioOutput{Type: llms.OutputTypeText, Text: part.Text}
					select {
					case c.textOutput <- output:
					default:
					}
				}
				if part.InlineData != nil {
					output := llms.AudioOutput{
						Type: llms.OutputTypeAudio,
						Data: base64.StdEncoding.EncodeToString(part.InlineData.Data),
					}
					select {
					case c.audioOutput <- output:
					default:
					}
				}
			}
		}
	}
}

func (c *GeminiLiveClient) heartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			apiLogUtils.LogGeneralInfo(c.conversationId + " [Heartbeat] Live session active")
		}
	}
}

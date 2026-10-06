package gemini

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

// Backend represents the targeted Gemini Live platform
type Backend string

const (
	BackendVertex   Backend = "vertex"
	BackendAIStudio Backend = "aistudio"
)

// Constants
const (
	DefaultAudioModel = "gemini-live-2.5-flash-native-audio"
	DefaultQueueSize  = 10000
)

// Default token configuration (Vertex AI minimum floor for ContextWindowCompression)
var (
	defaultTriggerTokens = int64(5000)
	defaultTargetTokens  = int64(2500)
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
	Model                    string                              `json:"model"`
	GenerationConfig         *bidiGenerationConfig               `json:"generationConfig,omitempty"`
	SystemInstruction        *bidiContent                        `json:"systemInstruction,omitempty"`
	Tools                    []*bidiTool                         `json:"tools,omitempty"`
	InputAudioTranscription  *bidiAudioTranscriptionConfig       `json:"inputAudioTranscription,omitempty"`
	OutputAudioTranscription *bidiAudioTranscriptionConfig       `json:"outputAudioTranscription,omitempty"`
	ContextWindowCompression *bidiContextWindowCompressionConfig `json:"contextWindowCompression,omitempty"`
	RealtimeInputConfig      *bidiRealtimeInputConfig            `json:"realtimeInputConfig,omitempty"`
}

type bidiRealtimeInputConfig struct {
	TurnCoverage               string                          `json:"turnCoverage,omitempty"`
	AutomaticActivityDetection *bidiAutomaticActivityDetection `json:"automaticActivityDetection,omitempty"`
}

type bidiAutomaticActivityDetection struct {
	Disabled                 bool   `json:"disabled,omitempty"`
	StartOfSpeechSensitivity string `json:"startOfSpeechSensitivity,omitempty"`
	EndOfSpeechSensitivity   string `json:"endOfSpeechSensitivity,omitempty"`
	PrefixPaddingMs          *int32 `json:"prefixPaddingMs,omitempty"`
	SilenceDurationMs        *int32 `json:"silenceDurationMs,omitempty"`
}

type bidiContextWindowCompressionConfig struct {
	TriggerTokens *int64             `json:"triggerTokens,omitempty"`
	SlidingWindow *bidiSlidingWindow `json:"slidingWindow,omitempty"`
}

type bidiSlidingWindow struct {
	TargetTokens *int64 `json:"targetTokens,omitempty"`
}

type bidiAudioTranscriptionConfig struct{}

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
	Behavior    string                 `json:"behavior,omitempty"`
}

type bidiServerMessage struct {
	SetupComplete *bidiSetupComplete `json:"setupComplete,omitempty"`
	ServerContent *bidiServerContent `json:"serverContent,omitempty"`
	ToolCall      *bidiToolCall      `json:"toolCall,omitempty"`
	UsageMetadata *bidiUsageMetadata `json:"usageMetadata,omitempty"`
}

type bidiSetupComplete struct {
	SessionID string `json:"sessionId,omitempty"`
}

type bidiServerContent struct {
	ModelTurn           *bidiContent       `json:"modelTurn,omitempty"`
	UserTurn            *bidiContent       `json:"userTurn,omitempty"`
	InputTranscription  *bidiTranscription `json:"inputTranscription,omitempty"`
	OutputTranscription *bidiTranscription `json:"outputTranscription,omitempty"`
	Interrupted         bool               `json:"interrupted,omitempty"`
	TurnComplete        bool               `json:"turnComplete,omitempty"`
	GenerationComplete  bool               `json:"generationComplete,omitempty"`
}

type bidiTranscription struct {
	Text     string `json:"text,omitempty"`
	Finished bool   `json:"finished,omitempty"`
}

type bidiUsageMetadata struct {
	PromptTokenCount           int                       `json:"promptTokenCount,omitempty"`
	CandidatesTokenCount       int                       `json:"candidatesTokenCount,omitempty"`
	ResponseTokenCount         int                       `json:"responseTokenCount,omitempty"`
	ThoughtsTokenCount         int                       `json:"thoughtsTokenCount,omitempty"`
	ToolUsePromptTokenCount    int                       `json:"toolUsePromptTokenCount,omitempty"`
	TotalTokenCount            int                       `json:"totalTokenCount,omitempty"`
	CachedContentTokenCount    int                       `json:"cachedContentTokenCount,omitempty"`
	PromptTokensDetails        []*bidiModalityTokenCount `json:"promptTokensDetails,omitempty"`
	CandidatesTokensDetails    []*bidiModalityTokenCount `json:"candidatesTokensDetails,omitempty"`
	ResponseTokensDetails      []*bidiModalityTokenCount `json:"responseTokensDetails,omitempty"`
	ToolUsePromptTokensDetails []*bidiModalityTokenCount `json:"toolUsePromptTokensDetails,omitempty"`
}

type bidiModalityTokenCount struct {
	Modality   string `json:"modality,omitempty"`
	TokenCount int    `json:"tokenCount,omitempty"`
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
	conn    *websocket.Conn
	model   string
	backend Backend

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
	eventQueue chan *bidiServerMessage

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
	pendingToolCalls    int32

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

	// Per-session logger
	sessionLogger *log.Logger
	logFile       *os.File
}

// NewGeminiLiveClient creates a new instance of GeminiLiveClient
func NewGeminiLiveClient(conversationId string, audioMode llms.AudioMode, pageConfig llms.PageConfig) (*GeminiLiveClient, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize per-session dedicated log file
	_ = os.MkdirAll("logs", 0750)
	cleanID := filepath.Base(conversationId)
	logPath := filepath.Join("logs", fmt.Sprintf("%s.log", cleanID))
	// #nosec G304 -- Log file path is scoped inside local logs directory
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	var sessionLogger *log.Logger
	if err == nil {
		sessionLogger = log.New(logFile, "", log.LstdFlags)
	} else {
		apiLogUtils.LogGeneralError("Failed to open session log file %s: %v", logPath, err)
	}

	backendStr := strings.ToLower(os.Getenv("GEMINI_BACKEND"))
	backend := BackendVertex
	if backendStr == string(BackendAIStudio) || backendStr == "ais" {
		backend = BackendAIStudio
	}

	geminiClient := &GeminiLiveClient{
		model:          DefaultAudioModel,
		backend:        backend,
		audioMode:      audioMode,
		tools:          make(map[string]tools.CustomTools),
		audioOutput:    make(chan llms.AudioOutput, DefaultQueueSize),
		textOutput:     make(chan llms.AudioOutput, DefaultQueueSize),
		ctx:            ctx,
		cancel:         cancel,
		connected:      false,
		eventQueue:     make(chan *bidiServerMessage, DefaultQueueSize),
		conversationId: conversationId,
		callbacks:      llms.Callbacks{},
		writeQueue:     make(chan interface{}, 200),
		MimeType:       "audio/pcm;rate=16000",
		jsonStream:     jsoniter.NewStream(jsoniter.ConfigCompatibleWithStandardLibrary, nil, 512),
		customModel:    pageConfig.IsCustomModel,
		brandName:      pageConfig.BrandName,
		sessionLogger:  sessionLogger,
		logFile:        logFile,
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
				Disabled:                 false,
				StartOfSpeechSensitivity: genai.StartSensitivityHigh,
				EndOfSpeechSensitivity:   genai.EndSensitivityLow,
				PrefixPaddingMs:          genai.Ptr(int32(300)),
				SilenceDurationMs:        genai.Ptr(int32(400)),
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

func (c *GeminiLiveClient) SetContextWindowCompression(triggerTokens, targetTokens int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sessionConfig != nil {
		c.sessionConfig.ContextWindowCompression = &genai.ContextWindowCompressionConfig{
			TriggerTokens: &triggerTokens,
			SlidingWindow: &genai.SlidingWindow{
				TargetTokens: &targetTokens,
			},
		}
	}
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

func (c *GeminiLiveClient) SetBackend(b Backend) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.backend = b
}

func (c *GeminiLiveClient) Backend() Backend {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.backend
}

func (c *GeminiLiveClient) setupAllTools(pageConfig llms.PageConfig) {
	allTools := make(map[string]tools.CustomTools, 16)

	if pageConfig.PageName == "RIDER_SUPPORT" || pageConfig.PageName == "" {
		syso := strings.ReplaceAll(llms.GetBaseInstructionPromptRiderSupport(), "BRAND_NAME", c.brandName) + pageConfig.Instruction

		c.sessionConfig.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{
				genai.NewPartFromText(syso),
			},
			Role: "user",
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
		Behavior:    genai.BehaviorBlocking,
	}
}

func (c *GeminiLiveClient) Connect() error {
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	var wsURL string
	headers := http.Header{}
	var projectID, location string

	if c.Backend() == BackendAIStudio {
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			if keyVal, ok := consul.ApiKeys.GeminiCredsVB["api_key"].(string); ok && keyVal != "" {
				apiKey = keyVal
			}
		}
		if apiKey == "" {
			for _, envPath := range []string{".env", "../.env", "../../.env"} {
				if data, err := os.ReadFile(envPath); err == nil {
					for _, line := range strings.Split(string(data), "\n") {
						line = strings.TrimSpace(line)
						if strings.HasPrefix(line, "GEMINI_API_KEY=") {
							apiKey = strings.Trim(strings.TrimPrefix(line, "GEMINI_API_KEY="), "\"' \r")
							break
						}
					}
				}
				if apiKey != "" {
					break
				}
			}
		}
		if apiKey == "" {
			c.LogError("GEMINI_API_KEY not found for AI Studio backend")
			c.cancel()
			return fmt.Errorf("GEMINI_API_KEY is required for AI Studio backend")
		}

		wsURL = "wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1alpha.GenerativeService.BidiGenerateContent"
		headers.Set("x-goog-api-key", apiKey)
	} else {
		projectID = os.Getenv("GCP_PROJECT_ID")
		if projectID == "" {
			if projVal, ok := consul.ApiKeys.GeminiCredsVB["project_id"].(string); ok && projVal != "" {
				projectID = projVal
			}
		}
		if projectID == "" {
			projectID = "mock-bard-pilot-project-id"
		}

		location = os.Getenv("GCP_LOCATION")
		if location == "" {
			location = "us-central1"
		}

		var tokenValue string
		if tokEnv := os.Getenv("GCP_ACCESS_TOKEN"); tokEnv != "" {
			tokenValue = tokEnv
		}

		// Try service account creds from consul if private key is not mock
		if tokenValue == "" {
			creds := consul.ApiKeys.GeminiCredsVB
			privKey, _ := creds["private_key"].(string)
			if privKey != "" && !strings.Contains(privKey, "MOCK") {
				credsBytes, _ := json.Marshal(creds)
				opts := credentials.DetectOptions{
					CredentialsJSON: credsBytes,
					Scopes:          []string{"https://www.googleapis.com/auth/cloud-platform"},
				}
				if credsFromFile, err := credentials.DetectDefault(&opts); err == nil {
					if tok, errTok := credsFromFile.Token(c.ctx); errTok == nil && tok != nil {
						tokenValue = tok.Value
					}
				}
			}
		}

		// Try standard Application Default Credentials (ADC)
		if tokenValue == "" {
			opts := credentials.DetectOptions{
				Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
			}
			if adcCreds, err := credentials.DetectDefault(&opts); err == nil {
				if tok, errTok := adcCreds.Token(c.ctx); errTok == nil && tok != nil {
					tokenValue = tok.Value
				}
			}
		}

		// Fallback to gcloud CLI access token
		if tokenValue == "" {
			if out, err := exec.CommandContext(c.ctx, "gcloud", "auth", "print-access-token").Output(); err == nil {
				tokenValue = strings.TrimSpace(string(out))
			}
		}

		if tokenValue == "" {
			// #nosec G101 -- Offline sandbox fallback token when Google Cloud credentials are unavailable
			tokenValue = "mock-bearer-token"
		}

		apiVer := "v1"
		if strings.Contains(c.model, "3.8") {
			apiVer = "v1beta1"
		}
		wsURL = fmt.Sprintf("wss://%s-aiplatform.googleapis.com/ws/google.cloud.aiplatform.%s.LlmBidiService/BidiGenerateContent", location, apiVer)
		headers.Set("Authorization", "Bearer "+tokenValue)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}

	conn, resp, err := dialer.DialContext(c.ctx, wsURL, headers)
	if resp != nil {
		for k, v := range resp.Header {
			c.LogInfo("Handshake Header: %s = %v", k, v)
		}
	}
	if err != nil {
		c.LogError("Failed to dial WebSocket %s: %v", wsURL, err)
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

	c.LogInfo("Successfully connected to Gemini Live via Barebones WebSocket (%s backend)", c.Backend())
	return nil
}

func (c *GeminiLiveClient) sendSetupPayload(projectID, location string) error {
	var fullModel string
	if c.Backend() == BackendAIStudio {
		if strings.HasPrefix(c.model, "models/") {
			fullModel = c.model
		} else if c.model == DefaultAudioModel {
			fullModel = "models/gemini-2.5-flash-native-audio-latest"
		} else {
			fullModel = "models/" + c.model
		}
	} else {
		cleanModel := strings.TrimPrefix(c.model, "models/")
		fullModel = fmt.Sprintf("projects/%s/locations/%s/publishers/google/models/%s", projectID, location, cleanModel)
	}

	var modalities []string
	for _, m := range c.sessionConfig.ResponseModalities {
		modalities = append(modalities, string(m))
	}
	if len(modalities) == 0 {
		modalities = []string{"AUDIO"}
	}

	setupMsg := bidiSetupPayload{
		Setup: bidiSetupConfig{
			Model: fullModel,
			GenerationConfig: &bidiGenerationConfig{
				ResponseModalities: modalities,
				Temperature:        c.sessionConfig.Temperature,
			},
			InputAudioTranscription:  &bidiAudioTranscriptionConfig{},
			OutputAudioTranscription: &bidiAudioTranscriptionConfig{},
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

	if c.sessionConfig.ContextWindowCompression != nil {
		setupMsg.Setup.ContextWindowCompression = &bidiContextWindowCompressionConfig{
			TriggerTokens: c.sessionConfig.ContextWindowCompression.TriggerTokens,
		}
		if c.sessionConfig.ContextWindowCompression.SlidingWindow != nil {
			setupMsg.Setup.ContextWindowCompression.SlidingWindow = &bidiSlidingWindow{
				TargetTokens: c.sessionConfig.ContextWindowCompression.SlidingWindow.TargetTokens,
			}
		}
	}

	if c.sessionConfig.RealtimeInputConfig != nil {
		ric := &bidiRealtimeInputConfig{
			TurnCoverage: string(c.sessionConfig.RealtimeInputConfig.TurnCoverage),
		}
		if aad := c.sessionConfig.RealtimeInputConfig.AutomaticActivityDetection; aad != nil {
			ric.AutomaticActivityDetection = &bidiAutomaticActivityDetection{
				Disabled:                 aad.Disabled,
				StartOfSpeechSensitivity: string(aad.StartOfSpeechSensitivity),
				EndOfSpeechSensitivity:   string(aad.EndOfSpeechSensitivity),
				PrefixPaddingMs:          aad.PrefixPaddingMs,
				SilenceDurationMs:        aad.SilenceDurationMs,
			}
		}
		setupMsg.Setup.RealtimeInputConfig = ric
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
						var params map[string]interface{}
						if tObj, ok := c.tools[fd.Name]; ok {
							params = tObj.GetParameters()
						}
						behavior := ""
						if c.Backend() == BackendVertex && fd.Behavior != "" {
							behavior = string(fd.Behavior)
						}
						fds = append(fds, &bidiFunctionDeclaration{
							Name:        fd.Name,
							Description: fd.Description,
							Parameters:  params,
							Behavior:    behavior,
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

func (c *GeminiLiveClient) LogInfo(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	apiLogUtils.LogGeneralInfo("%s %s", c.conversationId, msg)
	if c.sessionLogger != nil {
		c.sessionLogger.Printf("[INFO] %s", msg)
	}
}

func (c *GeminiLiveClient) LogError(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	apiLogUtils.LogGeneralError("%s %s", c.conversationId, msg)
	if c.sessionLogger != nil {
		c.sessionLogger.Printf("[ERROR] %s", msg)
	}
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
	if c.logFile != nil {
		_ = c.logFile.Close()
		c.logFile = nil
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

	// Use the `audio` field: `mediaChunks` is deprecated, and gemini-3.8-live on AI Studio
	// silently ignores audio sent via `mediaChunks` (no transcription, no response).
	msg := map[string]interface{}{
		"realtimeInput": map[string]interface{}{
			"audio": map[string]interface{}{
				"mimeType": c.MimeType,
				"data":     base64.StdEncoding.EncodeToString(audioData.ByteData),
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
				c.mu.Lock()
				isConn := c.connected
				c.connected = false
				c.mu.Unlock()
				if !isConn {
					return
				}
				c.LogError("WebSocket read error: %v", err)
				return
			}

			c.LogInfo("[RAW BIDI RECV]: %s", summarizeBidiMessage(messageBytes))

			var serverMsg bidiServerMessage
			if err := json.Unmarshal(messageBytes, &serverMsg); err != nil {
				c.LogError("Failed to unmarshal server message: %v", err)
				continue
			}

			select {
			case c.eventQueue <- &serverMsg:
			default:
				go c.processServerResponse(&serverMsg)
			}
		}
	}
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

func (c *GeminiLiveClient) processServerResponse(msg *bidiServerMessage) {
	if msg == nil {
		return
	}

	if msg.SetupComplete != nil {
		if msg.SetupComplete.SessionID != "" {
			c.LogInfo("Live Bidi session setup acknowledged with SessionID: %s", msg.SetupComplete.SessionID)
		} else {
			c.LogInfo("Live Bidi session setup acknowledged")
		}
	}

	// 1. Process Token Usage Metadata
	if msg.UsageMetadata != nil && c.callbacks.TokenCalculator != nil {
		var details []llms.TokenDetail
		for _, detail := range msg.UsageMetadata.PromptTokensDetails {
			if detail == nil {
				continue
			}
			modality := llms.TokenModalityText
			if strings.EqualFold(detail.Modality, "AUDIO") {
				modality = llms.TokenModalityAudio
			}
			details = append(details, llms.TokenDetail{
				TokenType:     llms.TokenTypeInput,
				TokenModality: modality,
				TokenCount:    detail.TokenCount,
				ModelName:     c.model,
			})
		}
		candDetails := msg.UsageMetadata.CandidatesTokensDetails
		if len(candDetails) == 0 && len(msg.UsageMetadata.ResponseTokensDetails) > 0 {
			candDetails = msg.UsageMetadata.ResponseTokensDetails
		}
		for _, detail := range candDetails {
			if detail == nil {
				continue
			}
			modality := llms.TokenModalityText
			if strings.EqualFold(detail.Modality, "AUDIO") {
				modality = llms.TokenModalityAudio
			}
			details = append(details, llms.TokenDetail{
				TokenType:     llms.TokenTypeOutput,
				TokenModality: modality,
				TokenCount:    detail.TokenCount,
				ModelName:     c.model,
			})
		}
		if msg.UsageMetadata.ThoughtsTokenCount > 0 {
			details = append(details, llms.TokenDetail{
				TokenType:     llms.TokenTypeOutput,
				TokenModality: llms.TokenModalityText,
				TokenCount:    msg.UsageMetadata.ThoughtsTokenCount,
				ModelName:     c.model,
			})
		}
		for _, detail := range msg.UsageMetadata.ToolUsePromptTokensDetails {
			if detail == nil {
				continue
			}
			modality := llms.TokenModalityText
			if strings.EqualFold(detail.Modality, "AUDIO") {
				modality = llms.TokenModalityAudio
			}
			details = append(details, llms.TokenDetail{
				TokenType:     llms.TokenTypeInput,
				TokenModality: modality,
				TokenCount:    detail.TokenCount,
				ModelName:     c.model,
			})
		}
		if msg.UsageMetadata.CachedContentTokenCount > 0 {
			details = append(details, llms.TokenDetail{
				TokenType:     llms.TokenTypeCached,
				TokenModality: llms.TokenModalityText,
				TokenCount:    msg.UsageMetadata.CachedContentTokenCount,
				ModelName:     c.model,
			})
		}
		// If both CandidatesTokensDetails and ResponseTokensDetails were empty but candidates were generated
		if len(candDetails) == 0 {
			candidateCount := msg.UsageMetadata.CandidatesTokenCount
			if candidateCount == 0 {
				candidateCount = msg.UsageMetadata.ResponseTokenCount
			}
			if candidateCount == 0 && msg.UsageMetadata.TotalTokenCount > msg.UsageMetadata.PromptTokenCount {
				candidateCount = msg.UsageMetadata.TotalTokenCount - msg.UsageMetadata.PromptTokenCount
			}
			if candidateCount > 0 {
				details = append(details, llms.TokenDetail{
					TokenType:     llms.TokenTypeOutput,
					TokenModality: llms.TokenModalityAudio,
					TokenCount:    candidateCount,
					ModelName:     c.model,
				})
			}
		}
		if len(details) == 0 {
			if msg.UsageMetadata.PromptTokenCount > 0 {
				details = append(details, llms.TokenDetail{
					TokenType:     llms.TokenTypeInput,
					TokenModality: llms.TokenModalityText,
					TokenCount:    msg.UsageMetadata.PromptTokenCount,
					ModelName:     c.model,
				})
			}
			if msg.UsageMetadata.CandidatesTokenCount > 0 {
				details = append(details, llms.TokenDetail{
					TokenType:     llms.TokenTypeOutput,
					TokenModality: llms.TokenModalityText,
					TokenCount:    msg.UsageMetadata.CandidatesTokenCount,
					ModelName:     c.model,
				})
			}
		}
		if len(details) > 0 {
			c.callbacks.TokenCalculator.AddTokens(details)
		}
	}

	// 2. Process Tool Calls (Increment pendingToolCalls before checking TurnComplete)
	if msg.ToolCall != nil && len(msg.ToolCall.FunctionCalls) > 0 {
		for _, fc := range msg.ToolCall.FunctionCalls {
			if fc == nil {
				continue
			}
			toolObj, exists := c.tools[fc.Name]
			if exists && c.callbacks.ToolExecutionProvider != nil {
				atomic.AddInt32(&c.pendingToolCalls, 1)
				argsBytes, _ := json.Marshal(fc.Args)
				go func(callID, name, argsStr string, t tools.CustomTools) {
					result, err := c.callbacks.ToolExecutionProvider.ExecuteTool(name, argsStr, t)
					// Allow any intermediate pre-tool TurnComplete message from Vertex to flush and be ignored
					time.Sleep(350 * time.Millisecond)
					atomic.AddInt32(&c.pendingToolCalls, -1)
					if err == nil && callID != "" {
						respMsg := map[string]interface{}{
							"toolResponse": map[string]interface{}{
								"functionResponses": []map[string]interface{}{
									{
										"id":       callID,
										"name":     name,
										"response": map[string]interface{}{"result": result},
									},
								},
							},
						}
						_ = c.sendEvent(respMsg)
					}
				}(fc.ID, fc.Name, string(argsBytes), toolObj)
			}
		}
	}

	// 3. Process Server Content (Model Turn, User Turn, Transcriptions, Interrupted, TurnComplete)
	if msg.ServerContent != nil {
		if msg.ServerContent.Interrupted {
			c.HandleUserInterrupt()
		}

		// Input Audio Transcription (User spoken text)
		if msg.ServerContent.InputTranscription != nil && msg.ServerContent.InputTranscription.Text != "" {
			if c.callbacks.ConversationProvider != nil {
				c.callbacks.ConversationProvider.AccumulateUserText(msg.ServerContent.InputTranscription.Text)
			}
		}

		// User Turn Transcription (if userTurn parts exist)
		if msg.ServerContent.UserTurn != nil && c.callbacks.ConversationProvider != nil {
			for _, part := range msg.ServerContent.UserTurn.Parts {
				if part != nil && part.Text != "" {
					c.callbacks.ConversationProvider.AccumulateUserText(part.Text)
				}
			}
		}

		// Output Audio Transcription (Assistant spoken text)
		if msg.ServerContent.OutputTranscription != nil && msg.ServerContent.OutputTranscription.Text != "" {
			if c.callbacks.ConversationProvider != nil {
				c.callbacks.ConversationProvider.AccumulateAssistantText(msg.ServerContent.OutputTranscription.Text)
			}
			output := llms.AudioOutput{Type: llms.OutputTypeText, Text: msg.ServerContent.OutputTranscription.Text}
			select {
			case c.textOutput <- output:
			default:
			}
		}

		// Model Turn Response
		if msg.ServerContent.ModelTurn != nil {
			for _, part := range msg.ServerContent.ModelTurn.Parts {
				if part == nil {
					continue
				}
				if part.Text != "" {
					if c.callbacks.ConversationProvider != nil {
						c.callbacks.ConversationProvider.AccumulateAssistantText(part.Text)
					}
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

		if msg.ServerContent.TurnComplete {
			if atomic.LoadInt32(&c.pendingToolCalls) == 0 {
				if c.callbacks.ConversationProvider != nil {
					c.callbacks.ConversationProvider.CompleteAssistantMessage()
					c.callbacks.ConversationProvider.CompleteUserMessage()
				}
			} else {
				c.LogInfo("Ignoring intermediate TurnComplete while %d tool call(s) pending", atomic.LoadInt32(&c.pendingToolCalls))
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
			c.LogInfo("[Heartbeat] Live session active")
		}
	}
}

func summarizeBidiMessage(rawJSON []byte) string {
	var rawMap map[string]interface{}
	if err := json.Unmarshal(rawJSON, &rawMap); err != nil {
		return string(rawJSON)
	}

	sanitizeMap(rawMap)
	out, err := json.Marshal(rawMap)
	if err != nil {
		return string(rawJSON)
	}
	return string(out)
}

func sanitizeMap(m map[string]interface{}) {
	for k, v := range m {
		if k == "data" {
			if str, ok := v.(string); ok && len(str) > 40 {
				m[k] = fmt.Sprintf("<base64 audio chunk, len: %d bytes>", len(str))
			}
		} else if subMap, ok := v.(map[string]interface{}); ok {
			sanitizeMap(subMap)
		} else if slice, ok := v.([]interface{}); ok {
			for _, item := range slice {
				if subMap, ok := item.(map[string]interface{}); ok {
					sanitizeMap(subMap)
				}
			}
		}
	}
}

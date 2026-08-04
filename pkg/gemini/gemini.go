package gemini

import (
	"RiderMate-Bot-Service/tools/postSales"
	"RiderMate-Bot-Service/tools/postSales/gemini"
	"RiderMate-Bot-Service/tools/searchTool"
	"RiderMate-Bot-Service/utils/apiLogUtils"
	"RiderMate-Bot-Service/utils/consul"
	"strings"

	"RiderMate-Bot-Service/internal/httprequest"
	"cloud.google.com/go/auth/credentials"

	// "RiderMate-Bot-Service/utils/consul"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"RiderMate-Bot-Service/llms"
	"RiderMate-Bot-Service/tools"

	jsoniter "github.com/json-iterator/go"
	"google.golang.org/genai"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary
var eventIDCounter uint64

// Constants
const (
	DefaultAudioModel = "models/gemini-live-2.5-flash-native-audio"
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

// Message types for writeQueue
type audioInputMessage struct {
	params genai.LiveSendRealtimeInputParameters
}

type clientContentMessage struct {
	input genai.LiveClientContentInput
}

type toolResponseMessage struct {
	input genai.LiveToolResponseInput
}

// GeminiLiveClient handles real-time communication with Gemini Live API
type GeminiLiveClient struct {
	id string
	// Client and session
	apiKey  string
	client  *genai.Client
	session *genai.Session
	model   string

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

	// Response coordination - queue response.create until current audio done
	pendingResponse   interface{} // Queued response event
	pendingResponseMu sync.Mutex  // Mutex for pending response

	// JSON stream pooling for better performance
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

	systemPrompt := strings.ReplaceAll(llms.GetBaseInstructionPromptPSGemini(), "BRAND_NAME", c.brandName)
	if isCustomModel {
		systemPrompt = llms.BaseInstructionPromptPSGeminiJustAudio
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

func (c *GeminiLiveClient) setupAllTools(pageConfig llms.PageConfig) {
	allTools := make(map[string]tools.CustomTools, 16)

	var (
		flightTool                = searchTool.SearchTool{}
		flightDetailsTool         = tools.FlightInformationTool{}
		nonReviewDiscountTool     = tools.DiscountTool{}
		baggageTool               = tools.BaggageTool{}
		cancellationTool          = tools.CancellationTool{}
		dateChangeTool            = tools.DateChangeTool{}
		visaTool                  = tools.VisaTool{}
		reviewDiscountTool        = tools.ReviewDiscountTool{}
		mealsTool                 = tools.MealsTool{}
		seatsTool                 = tools.SeatsTool{}
		insuranceTool             = tools.InsuranceTool{}
		reviewFlightItineraryTool = tools.ReviewFlightItineraryTool{}
		ancillariesTool           = tools.AncillariesTool{}
		paymentTool               = tools.PaymentTool{}
		addonTool                 = tools.AddonTool{}
	)

	if pageConfig.PageName == "POSTSALES" || pageConfig.PageName == "RIDER_SUPPORT" {
		syso := strings.ReplaceAll(llms.GetBaseInstructionPromptPSGemini(), "BRAND_NAME", c.brandName) + pageConfig.Instruction

		c.sessionConfig.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{
				genai.NewPartFromText(syso),
			},
			Role: "model",
		}

		cancelTool := gemini.BookingCancellationAgent{}
		etickettool := gemini.EticketTool{}
		checkinBoardingPassTool := gemini.WebcheckinTool{}
		nameChangeTool := gemini.NameCorrectionTool{}
		specialClaimTool := gemini.SpecialClaimTool{}
		trackerTool := gemini.ObservabilityTool{}
		dateChangePsTool := gemini.DateChangeAgent{}
		agentTool := gemini.AgentConnectTool{}
		enquiryTool := gemini.EnquiryTool{}
		psbaggageTool := gemini.BaggageTool{}
		addonPurchaesPsTool := gemini.AddonPurchaseAgent{}
		psFlightStatusTool := gemini.PSFlightStatusTool{}
		endCallTool := gemini.EndCallTool{}
		agentConnectTool := gemini.AgentConnectTool{}

		allTools[cancelTool.GetName()] = cancelTool
		allTools[etickettool.GetName()] = etickettool
		allTools[checkinBoardingPassTool.GetName()] = checkinBoardingPassTool
		allTools[nameChangeTool.GetName()] = nameChangeTool
		allTools[specialClaimTool.GetName()] = specialClaimTool
		allTools[trackerTool.GetName()] = trackerTool
		allTools[dateChangePsTool.GetName()] = dateChangePsTool
		allTools[agentTool.GetName()] = agentTool
		allTools[enquiryTool.GetName()] = enquiryTool
		allTools[psbaggageTool.GetName()] = psbaggageTool
		allTools[addonPurchaesPsTool.GetName()] = addonPurchaesPsTool
		allTools[psFlightStatusTool.GetName()] = psFlightStatusTool
		allTools[endCallTool.GetName()] = endCallTool
		allTools[agentConnectTool.GetName()] = agentConnectTool
	} else {
		allTools[flightTool.GetName()] = flightTool
		allTools[flightDetailsTool.GetName()] = flightDetailsTool
		allTools[nonReviewDiscountTool.GetName()] = nonReviewDiscountTool
		allTools[baggageTool.GetName()] = baggageTool
		allTools[cancellationTool.GetName()] = cancellationTool
		allTools[dateChangeTool.GetName()] = dateChangeTool
		allTools[visaTool.GetName()] = visaTool
		allTools[reviewDiscountTool.GetName()] = reviewDiscountTool
		allTools[mealsTool.GetName()] = mealsTool
		allTools[seatsTool.GetName()] = seatsTool
		allTools[insuranceTool.GetName()] = insuranceTool
		allTools[reviewFlightItineraryTool.GetName()] = reviewFlightItineraryTool
		allTools[ancillariesTool.GetName()] = ancillariesTool
		allTools[paymentTool.GetName()] = paymentTool
		allTools[addonTool.GetName()] = addonTool
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
	if c.connected {
		return nil
	}

	creds := consul.ApiKeys.GeminiCredsVB
	credsBytes, _ := json.Marshal(creds)
	options := credentials.DetectOptions{
		CredentialsJSON: credsBytes,
		Scopes:          []string{"https://www.googleapis.com/auth/cloud-platform"},
	}
	credsFromFile, err := credentials.DetectDefault(&options)
	if err != nil {
		fmt.Printf("Failed to create credentials: %v\n", err)
	}
	client, err := genai.NewClient(c.ctx,
		&genai.ClientConfig{
			Backend:     genai.BackendVertexAI,
			Project:     creds["project_id"].(string),
			Location:    "us-central1",
			Credentials: credsFromFile,
			HTTPOptions: genai.HTTPOptions{
				APIVersion: "v1beta1",
			},
			HTTPClient: &httprequest.Client,
		},
	)
	if err != nil {
		c.cancel()
		return fmt.Errorf("failed to create Gemini client: %w", err)
	}
	c.client = client

	session, err := c.client.Live.Connect(c.ctx, c.model, c.sessionConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to Gemini Live: %w", err)
	}

	c.session = session
	c.connected = true

	go c.receiveResponses()
	go c.processEventQueue()
	go c.handleWebSocketWrites()
	go c.heartbeat()

	apiLogUtils.LogGeneralInfo(c.conversationId + " Successfully connected to Gemini Live")
	return nil
}

func (c *GeminiLiveClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false
	if c.session != nil {
		c.session.Close()
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

	audioBlob := &genai.Blob{
		MIMEType: c.MimeType,
		Data:     audioData.ByteData,
	}

	msg := &audioInputMessage{
		params: genai.LiveSendRealtimeInputParameters{
			Audio: audioBlob,
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

	msg := &audioInputMessage{
		params: genai.LiveRealtimeInput{
			Text: text,
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
			if c.session != nil {
				switch v := event.(type) {
				case *audioInputMessage:
					_ = c.session.SendRealtimeInput(v.params)
				case *clientContentMessage:
					_ = c.session.SendClientContent(v.input)
				case *toolResponseMessage:
					_ = c.session.SendToolResponse(v.input)
				}
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
			if c.session == nil {
				return
			}
			response, err := c.session.Receive()
			if err != nil {
				if !c.connected {
					break
				}
				continue
			}

			select {
			case c.eventQueue <- response:
			default:
				go c.processServerResponse(response)
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

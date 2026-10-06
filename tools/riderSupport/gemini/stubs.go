package gemini

// Constants
const (
	AgentConnectToolName = "agent_connect_tool"
	EndCallToolName      = "end_call_tool"
)

// Tools structs implementing tools.CustomTools for RiderMate Delivery Voicebot
type OrderCancellationAgent struct{}
type OrderReceiptTool struct{}
type StoreArrivalTool struct{}
type ContactNoteTool struct{}
type SpecialClaimTool struct{}
type ObservabilityTool struct{}
type DateChangeAgent struct{}
type AgentConnectTool struct{}
type EnquiryTool struct{}
type BaggageTool struct{}
type AddonPurchaseAgent struct{}
type OrderStatusTool struct{}
type EndCallTool struct{}

// Interface compliance helpers
func (OrderCancellationAgent) GetName() string { return "OrderCancellationAgent" }
func (OrderReceiptTool) GetName() string       { return "OrderReceiptTool" }
func (StoreArrivalTool) GetName() string       { return "StoreArrivalTool" }
func (ContactNoteTool) GetName() string        { return "ContactNoteTool" }
func (SpecialClaimTool) GetName() string       { return "SpecialClaimTool" }
func (ObservabilityTool) GetName() string      { return "ObservabilityTool" }
func (DateChangeAgent) GetName() string        { return "DateChangeAgent" }
func (AgentConnectTool) GetName() string       { return "AgentConnectTool" }
func (EnquiryTool) GetName() string            { return "EnquiryTool" }
func (BaggageTool) GetName() string            { return "BaggageTool" }
func (AddonPurchaseAgent) GetName() string     { return "AddonPurchaseAgent" }
func (OrderStatusTool) GetName() string        { return "OrderStatusTool" }
func (EndCallTool) GetName() string            { return "EndCallTool" }

func (OrderCancellationAgent) GetDescription() string { return "Cancel order" }
func (OrderReceiptTool) GetDescription() string       { return "Get order receipt" }
func (StoreArrivalTool) GetDescription() string       { return "Log store arrival" }
func (ContactNoteTool) GetDescription() string        { return "Notify customer or log wait time" }
func (SpecialClaimTool) GetDescription() string       { return "File claim" }
func (ObservabilityTool) GetDescription() string      { return "Check app diagnostics" }
func (DateChangeAgent) GetDescription() string        { return "Extend delivery slot" }
func (AgentConnectTool) GetDescription() string       { return "Call customer or human support" }
func (EnquiryTool) GetDescription() string            { return "Check trip payout or bonus" }
func (BaggageTool) GetDescription() string            { return "Check bag rules" }
func (AddonPurchaseAgent) GetDescription() string     { return "Order rider gear" }
func (OrderStatusTool) GetDescription() string        { return "Get active or next order destination" }
func (EndCallTool) GetDescription() string            { return "End call" }

func (OrderCancellationAgent) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "OBJECT",
		"properties": map[string]interface{}{
			"orderId": map[string]interface{}{"type": "STRING"},
			"reason":  map[string]interface{}{"type": "STRING"},
		},
	}
}
func (OrderReceiptTool) GetParameters() map[string]interface{}  { return nil }
func (StoreArrivalTool) GetParameters() map[string]interface{}  { return nil }
func (ContactNoteTool) GetParameters() map[string]interface{}   { return nil }
func (SpecialClaimTool) GetParameters() map[string]interface{}  { return nil }
func (ObservabilityTool) GetParameters() map[string]interface{} { return nil }
func (DateChangeAgent) GetParameters() map[string]interface{}   { return nil }
func (AgentConnectTool) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "OBJECT",
		"properties": map[string]interface{}{
			"queue": map[string]interface{}{"type": "STRING"},
		},
	}
}
func (EnquiryTool) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "OBJECT",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{"type": "STRING"},
		},
	}
}
func (BaggageTool) GetParameters() map[string]interface{}        { return nil }
func (AddonPurchaseAgent) GetParameters() map[string]interface{} { return nil }
func (OrderStatusTool) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "OBJECT",
		"properties": map[string]interface{}{
			"orderId": map[string]interface{}{"type": "STRING"},
		},
	}
}
func (EndCallTool) GetParameters() map[string]interface{} { return nil }

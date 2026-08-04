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

func (OrderCancellationAgent) GetDescription() string {
	return "Cancel active delivery order due to rider emergency, vehicle breakdown, or store closure"
}
func (OrderReceiptTool) GetDescription() string {
	return "Retrieve delivery order receipt and itemized customer invoice"
}
func (StoreArrivalTool) GetDescription() string {
	return "Confirm rider arrival at merchant store or customer drop-off location"
}
func (ContactNoteTool) GetDescription() string {
	return "Update customer contact notes or address landmark"
}
func (SpecialClaimTool) GetDescription() string {
	return "Submit payout dispute, order damage report, or missing item claim"
}
func (ObservabilityTool) GetDescription() string {
	return "System observability, rider GPS tracking, and connection status"
}
func (DateChangeAgent) GetDescription() string {
	return "Request delivery time extension or reschedule customer slot"
}
func (AgentConnectTool) GetDescription() string {
	return "Transfer conversation to a human rider support representative"
}
func (EnquiryTool) GetDescription() string {
	return "Handle rider earnings, weekly payouts, and incentive bonus queries"
}
func (BaggageTool) GetDescription() string {
	return "Check thermal bag requirements, cargo capacity, and weight limits"
}
func (AddonPurchaseAgent) GetDescription() string {
	return "Order replacement rider gear, rain suits, or delivery helmets"
}
func (OrderStatusTool) GetDescription() string {
	return "Track active delivery order status and merchant preparation timer"
}
func (EndCallTool) GetDescription() string { return "Hang up or end rider voice session" }

func (OrderCancellationAgent) GetParameters() map[string]interface{} { return nil }
func (OrderReceiptTool) GetParameters() map[string]interface{}       { return nil }
func (StoreArrivalTool) GetParameters() map[string]interface{}       { return nil }
func (ContactNoteTool) GetParameters() map[string]interface{}        { return nil }
func (SpecialClaimTool) GetParameters() map[string]interface{}       { return nil }
func (ObservabilityTool) GetParameters() map[string]interface{}      { return nil }
func (DateChangeAgent) GetParameters() map[string]interface{}        { return nil }
func (AgentConnectTool) GetParameters() map[string]interface{}       { return nil }
func (EnquiryTool) GetParameters() map[string]interface{}            { return nil }
func (BaggageTool) GetParameters() map[string]interface{}            { return nil }
func (AddonPurchaseAgent) GetParameters() map[string]interface{}     { return nil }
func (OrderStatusTool) GetParameters() map[string]interface{}        { return nil }
func (EndCallTool) GetParameters() map[string]interface{}            { return nil }

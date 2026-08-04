package tools

// CustomTools defines the standard interface for application tools
type CustomTools interface {
	GetName() string
	GetDescription() string
	GetParameters() map[string]interface{}
}

// Base mock tool that implements CustomTools
type mockTool struct {
	Name        string
	Description string
	Params      map[string]interface{}
}

func (m mockTool) GetName() string {
	return m.Name
}

func (m mockTool) GetDescription() string {
	return m.Description
}

func (m mockTool) GetParameters() map[string]interface{} {
	return m.Params
}

// Concrete Tool definitions matching the imports

type DeliveryInformationTool struct{ mockTool }
type PayoutTool struct{ mockTool }
type ThermalBagTool struct{ mockTool }
type OrderCancellationTool struct{ mockTool }
type TimeExtensionTool struct{ mockTool }
type StoreWaitTimeTool struct{ mockTool }
type BonusIncentiveTool struct{ mockTool }
type CateringBoxTool struct{ mockTool }
type VehicleSupportTool struct{ mockTool }
type IncidentReportTool struct{ mockTool }
type RouteAssistanceTool struct{ mockTool }
type RiderGearTool struct{ mockTool }
type RiderPayoutTool struct{ mockTool }
type CustomerContactTool struct{ mockTool }

func (DeliveryInformationTool) GetName() string { return "DeliveryInformationTool" }
func (PayoutTool) GetName() string              { return "PayoutTool" }
func (ThermalBagTool) GetName() string          { return "ThermalBagTool" }
func (OrderCancellationTool) GetName() string   { return "OrderCancellationTool" }
func (TimeExtensionTool) GetName() string       { return "TimeExtensionTool" }
func (StoreWaitTimeTool) GetName() string       { return "StoreWaitTimeTool" }
func (BonusIncentiveTool) GetName() string      { return "BonusIncentiveTool" }
func (CateringBoxTool) GetName() string         { return "CateringBoxTool" }
func (VehicleSupportTool) GetName() string      { return "VehicleSupportTool" }
func (IncidentReportTool) GetName() string      { return "IncidentReportTool" }
func (RouteAssistanceTool) GetName() string     { return "RouteAssistanceTool" }
func (RiderGearTool) GetName() string           { return "RiderGearTool" }
func (RiderPayoutTool) GetName() string         { return "RiderPayoutTool" }
func (CustomerContactTool) GetName() string     { return "CustomerContactTool" }

// SelectTool returns the parsed tool based on a code string or placeholder
func SelectTool(code string) interface{} {
	return mockTool{
		Name:        "SelectedTool",
		Description: "Dynamically selected code execution tool",
	}
}

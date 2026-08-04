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

type FlightInformationTool struct{ mockTool }
type DiscountTool struct{ mockTool }
type BaggageTool struct{ mockTool }
type CancellationTool struct{ mockTool }
type DateChangeTool struct{ mockTool }
type VisaTool struct{ mockTool }
type ReviewDiscountTool struct{ mockTool }
type MealsTool struct{ mockTool }
type SeatsTool struct{ mockTool }
type InsuranceTool struct{ mockTool }
type ReviewFlightItineraryTool struct{ mockTool }
type AncillariesTool struct{ mockTool }
type PaymentTool struct{ mockTool }
type AddonTool struct{ mockTool }

func (FlightInformationTool) GetName() string     { return "DeliveryInformationTool" }
func (DiscountTool) GetName() string              { return "PayoutTool" }
func (BaggageTool) GetName() string               { return "ThermalBagTool" }
func (CancellationTool) GetName() string          { return "OrderCancellationTool" }
func (DateChangeTool) GetName() string            { return "TimeExtensionTool" }
func (VisaTool) GetName() string                  { return "StoreWaitTimeTool" }
func (ReviewDiscountTool) GetName() string        { return "BonusIncentiveTool" }
func (MealsTool) GetName() string                 { return "CateringBoxTool" }
func (SeatsTool) GetName() string                 { return "VehicleSupportTool" }
func (InsuranceTool) GetName() string             { return "IncidentReportTool" }
func (ReviewFlightItineraryTool) GetName() string { return "RouteAssistanceTool" }
func (AncillariesTool) GetName() string           { return "RiderGearTool" }
func (PaymentTool) GetName() string               { return "RiderPayoutTool" }
func (AddonTool) GetName() string                 { return "CustomerContactTool" }

// SelectTool returns the parsed tool based on a code string or placeholder
func SelectTool(code string) interface{} {
	return mockTool{
		Name:        "SelectedTool",
		Description: "Dynamically selected code execution tool",
	}
}

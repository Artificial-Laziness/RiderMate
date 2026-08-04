package searchTool

// SearchTool implements tools.CustomTools
type SearchTool struct{}

func (SearchTool) GetName() string {
	return "SearchTool"
}

func (SearchTool) GetDescription() string {
	return "Search nearby delivery orders and active assignments"
}

func (SearchTool) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"orderId":  "string",
			"location": "string",
			"status":   "string",
		},
		"required": []string{"orderId"},
	}
}

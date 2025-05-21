package entity

type Tool struct {
	Type     string   `json:"type"` // 固定值 "function"
	Function Function `json:"function"`
}

type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"` // JSON-Schema
}

package http

import "deepResearch/entity"

type DeepSeekRequestBody struct {
	Model    string             `json:"model,omitempty"`
	Messages []*DeepSeekMessage `json:"messages,omitempty"`
	Stream   bool               `json:"stream,omitempty"`

	Tools      []*entity.Tool `json:"tools,omitempty"`
	ToolChoice interface{}    `json:"tool_choice,omitempty"`
	//ResponseFormat *ResponseFormat `json:"response_format,omitempty"` // 做约束，deepseek貌似不支持，其他ai产品支持
}

//type ResponseFormat struct {
//	Type   string          `json:"type"`             // "json_object" 或 "json_schema"，json_object：返回json就行，json_schema：强约束返回内容
//	Schema json.RawMessage `json:"schema,omitempty"` // 只有 type == "json_schema" 才需要
//}

const (
	jsonObject = "json_object"
	jsonSchema = "json_schema"
)

// DeepSeekResponse 合并了正常响应和错误响应的字段
type DeepSeekResponse struct {
	ID      string           `json:"id"`      // 正常响应时返回
	Object  string           `json:"object"`  // 正常响应时返回
	Created int64            `json:"created"` // 正常响应时返回
	Model   string           `json:"model"`   // 正常响应时返回
	Choices []DeepSeekChoice `json:"choices"` // 正常响应时返回
	Usage   *DeepSeekUsage   `json:"usage"`   // 正常响应时返回
	Error   *DeepSeekError   `json:"error"`   // 异常响应时返回
}

type DeepSeekChoice struct {
	Index        int             `json:"index"`
	Message      DeepSeekMessage `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

type DeepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type DeepSeekUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type DeepSeekError struct {
	Message string      `json:"message"`
	Type    string      `json:"type"`
	Param   interface{} `json:"param"`
	Code    string      `json:"code"`
}

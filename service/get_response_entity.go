package service

// TrackerContext 用于跟踪tokens和行动
type TrackerContext struct {
	TokensUsed     int      `json:"tokensUsed"`
	Steps          int      `json:"steps"`
	VisitedURLs    []string `json:"visitedURLs"`
	ReadURLs       []string `json:"readURLs"`
	SearchQueries  []string `json:"searchQueries"`
	TotalTokens    int      `json:"totalTokens"`
	TokenBudget    int      `json:"tokenBudget"`
	StartTimestamp int64    `json:"startTimestamp"`
	EndTimestamp   int64    `json:"endTimestamp"`
}

// Step 表示一个推理步骤
type Step struct {
	Action      string      `json:"action"`
	Content     interface{} `json:"content"`
	Timestamp   int64       `json:"timestamp"`
	TokensUsed  int         `json:"tokensUsed"`
	TotalTokens int         `json:"totalTokens"`
}

// ResponseResult 包含查询响应结果
type ResponseResult struct {
	Action      string         `json:"action"`
	Answer      string         `json:"answer"`
	References  []string       `json:"references"`
	Context     TrackerContext `json:"context"`
	VisitedURLs []string       `json:"visitedURLs"`
	ReadURLs    []string       `json:"readURLs"`
	AllURLs     []string       `json:"allURLs"`
}

// WeightedURL 表示带权重的URL
type WeightedURL struct {
	URL   string  `json:"url"`
	Title string  `json:"title"`
	Score float64 `json:"score"`
}

// LLMClient 接口代表与LLM交互的客户端
type LLMClient interface {
	Complete(prompt string) (interface{}, error)
}

// SearchClient 接口代表与搜索API交互的客户端
type SearchClient interface {
	Search(query string) ([]WeightedURL, error)
	ReadURL(url string) (string, error)
}

package service

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// GetResponse 处理查询并返回结果
func GetResponse(
	question string,
	tokenBudget int,
	maxBadAttempts int,
	existingContext interface{},
	messages []interface{},
	numReturnedURLs int,
	noDirectAnswer bool,
	boostHostnames []string,
	badHostnames []string,
	onlyHostnames []string,
	maxRef int,
	minRelScore float64,
) (*ResponseResult, error) {

	// 初始化上下文和状态
	trackerContext := TrackerContext{
		TokensUsed:     0,
		Steps:          0,
		VisitedURLs:    []string{},
		ReadURLs:       []string{},
		SearchQueries:  []string{},
		TotalTokens:    0,
		TokenBudget:    tokenBudget,
		StartTimestamp: time.Now().Unix(),
	}

	allContext := []Step{}
	allKeywords := []string{}
	allQuestions := []string{question} // 初始问题
	allKnowledge := []string{}
	weightedURLs := []WeightedURL{}
	finalAnswer := ""
	badAttempts := 0

	// 特殊情况处理：如果是打招呼或闲聊，直接回答
	if isSimpleGreeting(question) {
		return &ResponseResult{
			Action:  "answer",
			Answer:  getGreetingResponse(question),
			Context: trackerContext,
		}, nil
	}

	// 初始化LLM客户端
	llmClient := &MockLLMClient{} // 实际应根据配置创建

	// 初始化搜索客户端
	searchClient := &MockSearchClient{} // 实际应根据配置创建

	// 主循环：反复尝试直到找到满意答案或达到最大尝试次数
	for trackerContext.Steps < maxBadAttempts && trackerContext.TokensUsed < tokenBudget {
		// 构建提示词
		prompt := buildPrompt(
			allContext,
			allQuestions,
			allKeywords,
			allKnowledge,
			weightedURLs,
		)

		// 获取当前步骤的动作
		llmResponse, err := llmClient.Complete(prompt)
		if err != nil {
			return nil, fmt.Errorf("LLM调用失败: %v", err)
		}

		// 将LLM响应转换为map
		currentStep := llmResponse.(map[string]interface{})

		// 更新token统计
		tokensUsed := estimateTokens(prompt) // 实际应计算真实tokens
		trackerContext.TokensUsed += tokensUsed
		trackerContext.TotalTokens = trackerContext.TokensUsed

		// 根据动作类型处理
		action := currentStep["action"].(string)

		step := Step{
			Action:      action,
			Content:     currentStep,
			Timestamp:   time.Now().Unix(),
			TokensUsed:  tokensUsed,
			TotalTokens: trackerContext.TotalTokens,
		}

		// 记录当前步骤
		allContext = append(allContext, step)
		trackerContext.Steps++

		switch action {
		case "search":
			// 执行搜索
			if searchRequests, ok := currentStep["searchRequests"].([]interface{}); ok {
				for _, req := range searchRequests {
					searchQuery := req.(string)
					trackerContext.SearchQueries = append(trackerContext.SearchQueries, searchQuery)

					searchResults, err := searchClient.Search(searchQuery)
					if err != nil {
						log.Printf("搜索失败: %v", err)
						continue
					}

					// 添加搜索结果到weightedURLs
					for _, result := range searchResults {
						// 检查URL是否已存在
						exists := false
						for _, existing := range weightedURLs {
							if existing.URL == result.URL {
								exists = true
								break
							}
						}

						if !exists {
							weightedURLs = append(weightedURLs, result)
						}
					}

					// 更新关键词
					extractKeywords(searchQuery, &allKeywords)
				}
			}

		case "visit":
			// 访问并读取URL内容
			if urlTargets, ok := currentStep["URLTargets"].([]interface{}); ok {
				for _, target := range urlTargets {
					url := target.(string)

					// 检查URL是否已访问
					visited := false
					for _, visitedURL := range trackerContext.VisitedURLs {
						if visitedURL == url {
							visited = true
							break
						}
					}

					if !visited {
						trackerContext.VisitedURLs = append(trackerContext.VisitedURLs, url)

						// 读取URL内容
						content, err := searchClient.ReadURL(url)
						if err != nil {
							log.Printf("读取URL失败: %v", err)
							continue
						}

						// 记录已读取的URL
						trackerContext.ReadURLs = append(trackerContext.ReadURLs, url)

						// 添加到知识库
						knowledge := fmt.Sprintf("Content from %s: %s", url, content)
						allKnowledge = append(allKnowledge, knowledge)
					}
				}
			}

		case "reflect":
			// 反思当前信息，提出新问题
			if reflectionQuestions, ok := currentStep["questions"].([]interface{}); ok {
				for _, q := range reflectionQuestions {
					question := q.(string)

					// 检查问题是否已存在
					exists := false
					for _, existing := range allQuestions {
						if existing == question {
							exists = true
							break
						}
					}

					if !exists {
						allQuestions = append(allQuestions, question)
					}
				}
			}

		case "answer":
			// 处理最终或中间答案
			answer := currentStep["answer"].(string)

			var references []string
			if refs, ok := currentStep["references"].([]interface{}); ok {
				for _, ref := range refs {
					references = append(references, ref.(string))
				}
			}

			// 评估答案质量
			isGoodAnswer := evaluateAnswer(answer, question, allKnowledge)

			if isGoodAnswer {
				finalAnswer = answer

				// 如果是原始问题的直接回答，结束循环
				if !noDirectAnswer && isDirectAnswerToOriginalQuestion(answer, question) {
					trackerContext.EndTimestamp = time.Now().Unix()

					return &ResponseResult{
						Action:      "answer",
						Answer:      finalAnswer,
						References:  references,
						Context:     trackerContext,
						VisitedURLs: trackerContext.VisitedURLs,
						ReadURLs:    trackerContext.ReadURLs,
						AllURLs:     extractAllURLs(weightedURLs),
					}, nil
				}
			} else {
				badAttempts++
				if badAttempts >= maxBadAttempts {
					// 达到最大失败尝试次数
					break
				}
			}
		}

		// 保存当前步骤的上下文（用于调试和重现）
		saveContextToFile(trackerContext, allContext)
	}

	// 设置结束时间
	trackerContext.EndTimestamp = time.Now().Unix()

	// 如果没有找到好的答案，使用最后一次尝试
	if finalAnswer == "" && len(allContext) > 0 {
		lastStep := allContext[len(allContext)-1]
		if lastStep.Action == "answer" {
			content := lastStep.Content.(map[string]interface{})
			finalAnswer = content["answer"].(string)
		} else {
			finalAnswer = "未能在给定的预算和尝试次数内找到满意答案。"
		}
	}

	// 构建参考资料列表
	var references []string
	for _, url := range trackerContext.ReadURLs {
		if len(references) < maxRef {
			references = append(references, url)
		}
	}

	return &ResponseResult{
		Action:      "answer",
		Answer:      finalAnswer,
		References:  references,
		Context:     trackerContext,
		VisitedURLs: trackerContext.VisitedURLs,
		ReadURLs:    trackerContext.ReadURLs,
		AllURLs:     extractAllURLs(weightedURLs),
	}, nil
}

// 辅助函数

// isSimpleGreeting 检查是否是简单问候
func isSimpleGreeting(question string) bool {
	greetings := []string{"hello", "hi", "hey", "你好", "早上好", "下午好", "晚上好"}
	questionLower := strings.ToLower(question)

	for _, greeting := range greetings {
		if strings.Contains(questionLower, greeting) {
			return true
		}
	}

	return false
}

// getGreetingResponse 获取问候回应
func getGreetingResponse(question string) string {
	return "你好！我是DeepResearch AI助手，有什么我可以帮您搜索或解答的问题吗？"
}

// buildPrompt 构建系统提示词
func buildPrompt(
	context []Step,
	allQuestions []string,
	allKeywords []string,
	knowledge []string,
	weightedURLs []WeightedURL,
) string {
	// 实际实现中应构建适合LLM的详细提示
	prompt := "系统：你是一个有帮助的AI助手，专注于深度搜索和推理。\n\n"

	if len(context) > 0 {
		prompt += "当前上下文：\n"
		// 仅包含最近的几个步骤以控制长度
		recentSteps := context
		if len(context) > 5 {
			recentSteps = context[len(context)-5:]
		}

		for _, step := range recentSteps {
			stepJSON, _ := json.Marshal(step)
			prompt += string(stepJSON) + "\n"
		}
	}

	if len(allQuestions) > 0 {
		prompt += "\n问题：\n"
		for _, q := range allQuestions {
			prompt += "- " + q + "\n"
		}
	}

	if len(knowledge) > 0 {
		prompt += "\n已获取知识：\n"
		// 仅包含最近的知识点以控制长度
		recentKnowledge := knowledge
		if len(knowledge) > 3 {
			recentKnowledge = knowledge[len(knowledge)-3:]
		}

		for _, k := range recentKnowledge {
			prompt += "- " + k + "\n"
		}
	}

	if len(weightedURLs) > 0 {
		prompt += "\n已发现的URL：\n"
		for _, url := range weightedURLs {
			prompt += fmt.Sprintf("- %s (分数: %.2f)\n", url.URL, url.Score)
		}
	}

	prompt += "\n现在，选择下一步行动（搜索、访问URL、反思或回答）："

	return prompt
}

// estimateTokens 估计提示词使用的token数量
func estimateTokens(text string) int {
	// 简化的token计算逻辑
	words := strings.Fields(text)
	return len(words)
}

// extractKeywords 从文本中提取关键词
func extractKeywords(text string, keywords *[]string) {
	// 简化实现，实际应使用NLP技术提取关键词
	words := strings.Fields(text)
	for _, word := range words {
		if len(word) > 4 && !contains(*keywords, word) {
			*keywords = append(*keywords, word)
		}
	}
}

// contains 检查slice是否包含特定元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// evaluateAnswer 评估答案质量
func evaluateAnswer(answer, question string, knowledge []string) bool {
	// 简化实现，实际应进行更复杂的评估
	// 检查答案是否包含足够信息且与问题相关
	if len(question) < 10 {
		return len(answer) > 50
	}
	return len(answer) > 50 && strings.Contains(strings.ToLower(answer), strings.ToLower(question[:10]))
}

// isDirectAnswerToOriginalQuestion 检查是否是对原始问题的直接回答
func isDirectAnswerToOriginalQuestion(answer, originalQuestion string) bool {
	// 简化实现
	return true
}

// saveContextToFile 保存上下文到文件（用于调试）
func saveContextToFile(context TrackerContext, allSteps []Step) {
	data := map[string]interface{}{
		"context": context,
		"steps":   allSteps,
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Printf("无法序列化上下文: %v", err)
		return
	}

	err = os.WriteFile("context.json", bytes, 0644)
	if err != nil {
		log.Printf("无法保存上下文: %v", err)
	}
}

// extractAllURLs 从加权URL列表中提取所有URL
func extractAllURLs(weightedURLs []WeightedURL) []string {
	var urls []string
	for _, wURL := range weightedURLs {
		urls = append(urls, wURL.URL)
	}
	return urls
}

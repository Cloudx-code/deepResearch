package service

import (
	"deepResearch/client/http"
	"deepResearch/common/consts"
	"deepResearch/common/utils"
	"deepResearch/entity"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Agent 代表深度搜索代理
type Agent struct {
	question       string
	tokenBudget    int64
	maxBadAttempts int64

	languageCode, languageStyle string
}

func NewAgent(question string, tokenBudget int64, maxBadAttempts int64) *Agent {
	return &Agent{
		question:       question,
		tokenBudget:    tokenBudget,
		maxBadAttempts: maxBadAttempts,
	}
}

func (a *Agent) GetResponse() string {
	step := 0
	totalStep := 0

	question := strings.TrimSpace(a.question)

	messages := []map[string]string{
		{
			"role":    "user",
			"content": question,
		},
	}

	return ""
}

func (a *Agent) setLanguage(question string) error {
	resp, err := http.NewDeepSeekTool().RunDeepSeek(consts.GetLanguagePrompt, question, consts.LanguageSchema)
	if err != nil {
		fmt.Printf("fail to RunDeepSeek,[setLanguage],err:%v", err)
		return err
	}
	if len(resp.Choices) == 0 {
		fmt.Printf("fail to check resp.Choices,[setLanguage],err:%v", err)
		return errors.New("len resp.Choices == 0")
	}
	contentStr, err := utils.ExtractJSONFromString(resp.Choices[0].Message.Content)
	if err != nil {
		fmt.Printf("fail to ExtractJSONFromString,[setLanguage],err:%v", err)
		return err
	}
	languageInfo := &entity.CheckLanguageInfo{}
	err = json.Unmarshal([]byte(contentStr), &languageInfo)
	if err != nil {
		fmt.Printf("fail to Unmarshal,[setLanguage],err:%v", err)
		return err
	}
	a.languageCode = languageInfo.LangCode
	a.languageStyle = languageInfo.LanguageStyle
	return nil
}

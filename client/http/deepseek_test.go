package http

import (
	"deepResearch/common/consts"
	"deepResearch/common/utils"
	"fmt"
	"testing"
)

func TestQueryDeepseek(t *testing.T) {
	//res, err := NewDeepSeekTool().RunDeepSeek("将用户的全部input翻译成英文", "我需要你只回复：你好",nil)
	res, err := NewDeepSeekTool().RunDeepSeek(consts.GetLanguagePrompt, "hello,who are you", consts.LanguageSchema)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(utils.Encode(res))
}

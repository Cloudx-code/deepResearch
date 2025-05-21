package utils

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExtractJSONFromString 在任意文本中提取首个合法 JSON（对象或数组）并返回原始文本片段。
func ExtractJSONFromString(content string) (string, error) {
	// ---------- 1. 剥离 Markdown 代码围栏 ----------
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content) // 再收尾一次

	// ---------- 2. 扫描 ----------
	inString, escaped := false, false
	depth := 0
	start := -1
	// 转为 []rune 可以正确处理 UTF-8，但下标不好对齐；这里直接用 []byte
	bs := []byte(content)

	for i := 0; i < len(bs); i++ {
		c := bs[i]

		// ---- 2.1 处理字符串内部状态 ----
		if inString {
			if escaped {
				escaped = false // 当前字符被转义，跳过特殊意义
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}

		// ---- 2.2 处理字符串开关 ----
		if c == '"' {
			inString = true
			continue
		}

		// ---- 2.3 处理括号深度 ----
		switch c {
		case '{', '[':
			if depth == 0 {
				start = i // 记录顶层 JSON 起点
			}
			depth++

		case '}', ']':
			if depth == 0 {
				// 不可能出现，跳过
				continue
			}
			depth--
			if depth == 0 && start != -1 {
				// 抓到一个顶层 JSON 片段
				raw := strings.TrimSpace(content[start : i+1])
				if json.Valid([]byte(raw)) { // 二次保障
					return raw, nil
				}
				// 若非法，继续向后搜索可能的下一个片段
				start = -1
			}
		}
	}

	return "", fmt.Errorf("未找到合法 JSON")
}

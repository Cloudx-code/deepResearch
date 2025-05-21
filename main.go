package main

import (
	"deepResearch/service"
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	// 解析命令行参数
	tokenBudget := flag.Int("budget", 100000, "Token预算")
	maxAttempts := flag.Int("attempts", 3, "最大尝试次数")
	flag.Parse()

	// 获取用户输入的查询
	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("请提供查询内容")
		os.Exit(1)
	}
	query := args[0]

	// 调用GetResponse函数执行查询，提供基本参数
	result, err := service.GetResponse(
		query,
		*tokenBudget,
		*maxAttempts,
		nil,   // existingContext
		nil,   // messages
		10,    // numReturnedURLs
		false, // noDirectAnswer
		nil,   // boostHostnames
		nil,   // badHostnames
		nil,   // onlyHostnames
		5,     // maxRef
		0.5,   // minRelScore
	)
	if err != nil {
		log.Fatalf("执行查询失败: %v", err)
	}

	// 输出结果
	if result.Action == "answer" {
		fmt.Println(result.Answer)

		// 如果有参考资料，则打印出来
		if len(result.References) > 0 {
			fmt.Println("\n参考资料:")
			for i, ref := range result.References {
				fmt.Printf("[%d] %s\n", i+1, ref)
			}
		}
	} else {
		fmt.Printf("未得到明确答案。最后的动作: %s\n", result.Action)
	}
}

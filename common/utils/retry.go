package utils

import (
	"context"
	"fmt"

	"time"
)

// Retry 函数：接收需要重试的操作函数（返回 error），重试次数，间隔时间
func Retry(ctx context.Context, attempts int, sleep time.Duration, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = fn() // 执行操作
		if err == nil {
			return nil // 成功时退出
		}
		fmt.Printf("Attempt %d failed, retrying in %v...\n", i+1, sleep)
		time.Sleep(sleep) // 等待重试
	}
	return fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}

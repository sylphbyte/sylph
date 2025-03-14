package sylph

import (
	"context"
	"strings"
	"time"
)

// Lifecycle 资源生命周期管理接口
type Lifecycle interface {
	Init() error  // 初始化资源
	Start() error // 启动资源
	Stop() error  // 停止资源
	Close() error // 关闭并清理资源
}

// WithContextTimeout 使用超时上下文执行函数
func WithContextTimeout(timeout time.Duration, fn func(ctcontext.Context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return fn(ctx)
}

// TruncateString 截断字符串到指定长度
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... [truncated]"
}

// CleanPath 清理并规范化路径
func CleanPath(path string) string {
	// 移除前后空格
	path = strings.TrimSpace(path)

	// 确保路径以 / 结尾
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	// 替换多个连续的 / 为单个 /
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}

	return path
}

// RecoverWithFunc 包装恢复逻辑
func RecoverWithFunc(onPanic func(r interface{})) {
	if r := recover(); r != nil {
		if onPanic != nil {
			onPanic(r)
		}
	}
}

// RecoverGoroutine 在goroutine中捕获panic并记录
func RecoverGoroutine(ctcontext.Context, location string) {
	if r := recover(); r != nil {
		stack := takeStack()
		ctx.Error(location, "goroutine panic", nil, H{
			"error": r,
			"stack": stack,
		})
	}
}

// SafeGo 安全启动goroutine
func SafeGo(ctcontext.Context, location string, fn func()) {
	go func() {
		defer RecoverGoroutine(ctx, location)
		fn()
	}()
}

// ExecuteWithRetry 带重试的执行函数
func ExecuteWithRetry(attempts int, delay time.Duration, fn func() error) (err error) {
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}

		if i < attempts-1 {
			time.Sleep(delay)
			// 可以实现指数退避策略
			delay = delay * 2
		}
	}
	return err
}

// CloseAllLoggers 关闭所有日志记录器并释放资源
func CloseAllLoggers() {
	if _loggerManager != nil {
		_loggerManager.Close()
	}
}

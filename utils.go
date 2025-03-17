package sylph

import (
	"context"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// 全局初始化优化
func init() {
	// 设置适当的GOMAXPROCS以避免超订阅
	cpus := runtime.NumCPU()
	runtime.GOMAXPROCS(cpus)

	// 调整GC参数，在高并发场景下平衡GC压力和内存使用
	// 默认为100，增加这个值会减少GC频率但增加内存使用
	debug.SetGCPercent(200)

	// 为了减少GC暂停时间，可以设置较小的并行度
	debug.SetMaxThreads(cpus * 2)
}

// 为常用的小对象添加对象池
var (
	mapPool = sync.Pool{
		New: func() interface{} {
			return make(map[string]interface{}, 8)
		},
	}

	// 获取一个预分配的map
	GetMap = func() map[string]interface{} {
		return mapPool.Get().(map[string]interface{})
	}

	// 归还map到池
	ReleaseMap = func(m map[string]interface{}) {
		for k := range m {
			delete(m, k)
		}
		mapPool.Put(m)
	}
)

// Lifecycle 资源生命周期管理接口
type Lifecycle interface {
	Init() error  // 初始化资源
	Start() error // 启动资源
	Stop() error  // 停止资源
	Close() error // 关闭并清理资源
}

// WithContextTimeout 使用超时上下文执行函数
func WithContextTimeout(timeout time.Duration, fn func(ctx context.Context) error) error {
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
func RecoverGoroutine(ctx Context, location string) {
	if r := recover(); r != nil {
		stack := takeStack()
		ctx.Error(location, "goroutine panic", nil, H{
			"error": r,
			"stack": stack,
		})
	}
}

// SafeGo 安全启动goroutine
//
//	func SafeGo(ctx Context, location string, fn func()) {
//		go func() {
//			defer RecoverGoroutine(ctx, location)
//			fn()
//		}()
//	}
//
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

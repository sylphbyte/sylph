package sylph

import (
	"context"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// init 全局初始化函数，优化系统性能
// 在包导入时自动执行，设置适当的系统参数以获得更好的性能
//
// 功能说明:
//   - 调整GOMAXPROCS以充分利用CPU资源
//   - 优化GC参数，减少GC频率，平衡内存使用和性能
//   - 设置GC并行度，减少GC暂停时间
//
// 逻辑说明:
//   - 将GOMAXPROCS设置为CPU核心数，避免上下文切换开销
//   - 增加GC百分比，降低GC触发频率，减轻GC压力
//   - 限制GC最大线程数，保证应用程序有足够资源
func init() {
	// 设置适当的GOMAXPROCS以避免超订阅
	// 将Go运行时使用的CPU数量设置为机器的逻辑CPU数量
	cpus := runtime.NumCPU()
	runtime.GOMAXPROCS(cpus)

	// 调整GC参数，在高并发场景下平衡GC压力和内存使用
	// 默认为100，增加这个值会减少GC频率但增加内存使用
	// 200表示当内存使用量达到上次GC后内存使用量的300%时触发GC
	debug.SetGCPercent(200)

	// 为了减少GC暂停时间，可以设置较小的并行度
	// 控制GC使用的最大线程数，这里设置为CPU数量的两倍
	debug.SetMaxThreads(cpus * 2)
}

// 为常用的小对象添加对象池以减少GC压力
var (
	// mapPool 是map[string]interface{}对象的池
	// 用于减少频繁创建和销毁map导致的GC压力
	mapPool = sync.Pool{
		New: func() interface{} {
			return make(map[string]interface{}, 8)
		},
	}

	// GetMap 获取一个预分配的map
	// 从对象池中获取一个map，避免频繁创建新map
	//
	// 返回:
	//   - map[string]interface{}: 一个可复用的map对象
	//
	// 使用示例:
	//
	//	m := sylph.GetMap()
	//	m["key"] = value
	//	// 使用完毕后归还
	//	sylph.ReleaseMap(m)
	GetMap = func() map[string]interface{} {
		return mapPool.Get().(map[string]interface{})
	}

	// ReleaseMap 归还map到对象池
	// 清空map的内容并将其放回对象池以便复用
	//
	// 参数:
	//   - m: 要归还的map对象
	//
	// 注意事项:
	//   - 在归还前会清空map中的所有键值对
	//   - 归还后不应再使用该map对象
	ReleaseMap = func(m map[string]interface{}) {
		for k := range m {
			delete(m, k)
		}
		mapPool.Put(m)
	}
)

// Lifecycle 资源生命周期管理接口
// 定义了资源从初始化到关闭的完整生命周期方法
//
// 功能说明:
//   - 提供标准化的资源生命周期管理接口
//   - 适用于需要完整生命周期管理的各类资源
//   - 实现清晰的资源状态转换：初始化->启动->停止->关闭
//
// 使用示例:
//
//	type Database struct {
//	    conn *sql.DB
//	}
//
//	func (db *Database) Init() error {
//	    // 初始化连接池配置
//	    return nil
//	}
//
//	func (db *Database) Start() error {
//	    var err error
//	    db.conn, err = sql.Open("postgres", connStr)
//	    return err
//	}
//
//	func (db *Database) Stop() error {
//	    // 停止接收新连接
//	    return nil
//	}
//
//	func (db *Database) Close() error {
//	    return db.conn.Close()
//	}
type Lifecycle interface {
	// Init 初始化资源
	// 在使用资源前进行必要的初始化工作
	//
	// 返回:
	//   - error: 初始化过程中的错误，成功则返回nil
	Init() error

	// Start 启动资源
	// 使资源开始工作，例如启动服务、建立连接等
	//
	// 返回:
	//   - error: 启动过程中的错误，成功则返回nil
	Start() error

	// Stop 停止资源
	// 使资源停止工作，但不释放资源
	//
	// 返回:
	//   - error: 停止过程中的错误，成功则返回nil
	Stop() error

	// Close 关闭并清理资源
	// 完全释放资源占用的所有资源
	//
	// 返回:
	//   - error: 关闭过程中的错误，成功则返回nil
	Close() error
}

// WithContextTimeout 使用超时上下文执行函数
// 创建一个带超时的上下文并执行指定函数，函数执行完成或超时后自动取消上下文
//
// 参数:
//   - timeout: 超时时间
//   - fn: 要执行的函数，接收上下文参数
//
// 返回:
//   - error: 函数执行的结果或超时错误
//
// 使用示例:
//
//	err := sylph.WithContextTimeout(5*time.Second, func(ctx context.Context) error {
//	    // 执行可能耗时的操作，如数据库查询或网络请求
//	    return db.QueryWithContext(ctx, query)
//	})
//
// 注意事项:
//   - 如果函数在超时前完成，上下文会被立即取消
//   - 如果函数在超时后仍在执行，应自行检查ctx.Done()并及时退出
//   - 此函数会在完成后自动调用cancel()，无需手动取消上下文
func WithContextTimeout(timeout time.Duration, fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return fn(ctx)
}

// TruncateString 截断字符串到指定长度
// 如果字符串长度超过指定长度，则截断并添加截断标记
//
// 参数:
//   - s: 要截断的字符串
//   - maxLen: 最大允许长度
//
// 返回:
//   - string: 截断后的字符串
//
// 使用示例:
//
//	truncated := sylph.TruncateString(longText, 100)
//	fmt.Println(truncated) // 输出不超过100个字符的字符串，如果被截断会加上标记
//
// 注意事项:
//   - 如果字符串长度小于或等于maxLen，将原样返回
//   - 截断标记"... [truncated]"会添加到截断点，不计入maxLen限制
//   - 此函数按字节处理，不考虑Unicode字符边界
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... [truncated]"
}

// CleanPath 清理并规范化路径
// 移除多余的空格和重复的路径分隔符，确保路径具有一致的格式
//
// 参数:
//   - path: 要清理的路径
//
// 返回:
//   - string: 规范化后的路径
//
// 使用示例:
//
//	cleaned := sylph.CleanPath("/api//users/")  // 返回 "/api/users/"
//	cleaned = sylph.CleanPath("  /api/users  ") // 返回 "/api/users/"
//
// 逻辑说明:
//   - 移除字符串前后的空白字符
//   - 确保路径以斜杠结尾
//   - 替换连续的多个斜杠为单个斜杠
//
// 注意事项:
//   - 不会解析相对路径如 "../"
//   - 总是确保路径以"/"结尾
//   - 不会修改协议前缀，如 "http://"会变成 "http:/"
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
// 捕获panic并通过提供的函数处理，通常用在defer语句中
//
// 参数:
//   - onPanic: 处理panic的函数，接收panic值作为参数
//
// 使用示例:
//
//	defer sylph.RecoverWithFunc(func(r interface{}) {
//	    log.Printf("Recovered from panic: %v", r)
//	    // 也可以在这里发送告警或记录详细日志
//	})
//
//	// 可能导致panic的代码
//	process(data)
//
// 注意事项:
//   - 如果没有发生panic，onPanic函数不会被调用
//   - 如果onPanic为nil，只会捕获panic但不执行任何处理
//   - 使用此函数可以防止panic传播导致程序崩溃
func RecoverWithFunc(onPanic func(r interface{})) {
	if r := recover(); r != nil {
		if onPanic != nil {
			onPanic(r)
		}
	}
}

// RecoverGoroutine 在goroutine中捕获panic并记录
// 捕获goroutine中的panic并使用上下文记录错误和堆栈信息
//
// 参数:
//   - ctx: 上下文，用于记录日志
//   - location: 位置信息，通常是goroutine的名称或标识
//
// 使用示例:
//
//	go func() {
//	    defer sylph.RecoverGoroutine(ctx, "worker.process")
//	    // 执行可能发生panic的代码
//	    processJob(job)
//	}()
//
// 逻辑说明:
//   - 捕获任何panic并防止goroutine崩溃
//   - 使用上下文记录panic详情和调用栈
//   - 错误日志包含panic值和完整的堆栈跟踪
//
// 注意事项:
//   - 仅应用于goroutine中，确保recover能够捕获panic
//   - 总是放在defer语句中并位于goroutine函数的最开始
//   - 不会重新抛出panic，允许goroutine安全终止
func RecoverGoroutine(ctx Context, location string) {
	if r := recover(); r != nil {
		stack := takeStack()
		ctx.Error(location, "goroutine panic", nil, map[string]interface{}{
			"error": r,
			"stack": stack,
		})
	}
}

// SafeGo 安全启动goroutine
// 包装goroutine以捕获可能的panic，防止程序崩溃
//
// 参数:
//   - ctx: 上下文，用于记录可能的panic
//   - location: 位置信息，用于标识goroutine
//   - fn: 要在goroutine中执行的函数
//
// 使用示例:
//
//	sylph.SafeGo(ctx, "worker.process", func() {
//	    // 执行可能发生panic的代码
//	    processComplexData(data)
//	})
//
// 注意: 实际实现在context.go中

// ExecuteWithRetry 带重试的执行函数
// 如果函数执行失败，会按照指定的尝试次数和延迟进行重试
//
// 参数:
//   - attempts: 最大尝试次数，包括第一次执行
//   - delay: 初始重试延迟时间
//   - fn: 要执行的函数
//
// 返回:
//   - error: 最后一次执行的错误，如果成功则为nil
//
// 逻辑说明:
//   - 首次立即执行，如果成功立即返回
//   - 失败后等待延迟时间再重试
//   - 每次重试失败后，延迟时间翻倍(指数退避)
//   - 最多重试到达指定的尝试次数
//
// 注意事项:
//   - 使用指数退避策略，每次重试后延迟时间会翻倍
//   - 只有当函数返回非nil错误时才会重试
//   - 返回的是最后一次执行的错误，如果有多次失败，前面的错误会被覆盖
//
// 使用示例:
//
//	err := sylph.ExecuteWithRetry(3, 100*time.Millisecond, func() error {
//	    // 执行可能暂时失败的操作，如网络请求
//	    return httpClient.Get(url)
//	})
//	if err != nil {
//	    // 所有重试都失败
//	    log.Printf("操作失败，最大重试次数已达到: %v", err)
//	}
func ExecuteWithRetry(attempts int, delay time.Duration, fn func() error) (err error) {
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}

		if i < attempts-1 { // 最后一次尝试后不需要等待
			time.Sleep(delay)
			delay *= 2 // 指数退避
		}
	}
	return err // 返回最后一次错误
}

// CloseAllLoggers 关闭所有日志记录器
// 在应用程序退出前安全地关闭所有日志记录器，确保日志被完全写入
//
// 功能说明:
//   - 通知并等待所有异步日志写入完成
//   - 关闭所有打开的日志文件和写入器
//   - 防止日志丢失和文件描述符泄漏
//
// 使用示例:
//
//	// 在主函数结束前调用
//	func main() {
//	    // 应用程序初始化和运行
//	    // ...
//
//	    // 应用程序结束前确保日志完全写入
//	    defer sylph.CloseAllLoggers()
//	}
//
// 注意事项:
//   - 应在应用程序退出前调用
//   - 调用后不应再进行日志记录
//   - 此函数会阻塞直到所有日志写入完成
func CloseAllLoggers() {
	// 先关闭标准日志记录器
	if _loggerManager != nil {
		_loggerManager.Close()
	}

	// 关闭所有自定义日志写入器
	if _logWriterManager != nil {
		_logWriterManager.Close()
	}
}

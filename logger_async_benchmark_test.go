package sylph

import (
	"fmt"
	"io"
	"strconv"
	"sync"
	"testing"
)

// NullWriter 实现一个无输出的writer
type NullWriter struct{}

func (w *NullWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

// 创建一个不输出的日志记录器用于基准测试
func createBenchLogger() *Logger {
	// 创建基础日志记录器
	logger := DefaultLogger("bench-test")

	// 将输出设置为io.Discard
	logger.entry.Out = io.Discard

	return logger
}

func BenchmarkAsyncLoggerConcurrent(b *testing.B) {
	// 创建基础日志记录器
	baseLogger := createBenchLogger()

	// 使用不同配置创建异步日志记录器
	asyncLogger := NewAsyncLogger(baseLogger, 10000,
		WithWorkers(4),     // 默认工作协程数
		WithFallback(true), // 启用降级
	)
	defer asyncLogger.Close()

	b.ResetTimer()

	// 不同并发级别的测试
	concurrencyLevels := []int{1, 2, 4, 8}

	for _, concurrency := range concurrencyLevels {
		b.Run("goroutines="+strconv.Itoa(concurrency), func(b *testing.B) {
			var wg sync.WaitGroup

			// 平均分配给每个goroutine的工作量
			logsPerGoroutine := b.N / concurrency
			if logsPerGoroutine < 1 {
				logsPerGoroutine = 1
			}

			b.ResetTimer()

			// 启动指定数量的goroutine
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()

					// 每个goroutine写入指定数量的日志
					for j := 0; j < logsPerGoroutine; j++ {
						msg := NewLoggerMessage()
						msg.Message = "bench log"
						asyncLogger.Info(msg)
					}
				}(i)
			}

			wg.Wait()
		})
	}
}

// 测试不同缓冲区大小对性能的影响
func BenchmarkAsyncLoggerBufferSize(b *testing.B) {
	baseLogger := createBenchLogger()
	bufferSizes := []int{100, 1000, 10000}

	for _, size := range bufferSizes {
		b.Run("buffer="+strconv.Itoa(size), func(b *testing.B) {
			asyncLogger := NewAsyncLogger(baseLogger, size)
			defer asyncLogger.Close()

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				msg := NewLoggerMessage()
				msg.Message = "buffer size test"
				asyncLogger.Info(msg)
			}
		})
	}
}

// 测试不同工作协程数对性能的影响
func BenchmarkAsyncLoggerWorkerCount(b *testing.B) {
	baseLogger := createBenchLogger()
	workerCounts := []int{1, 2, 4, 8}

	for _, count := range workerCounts {
		b.Run("workers="+strconv.Itoa(count), func(b *testing.B) {
			asyncLogger := NewAsyncLogger(baseLogger, 10000, WithWorkers(count))
			defer asyncLogger.Close()

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				msg := NewLoggerMessage()
				msg.Message = "worker count test"
				asyncLogger.Info(msg)
			}
		})
	}
}

// 测试降级与非降级模式的性能对比
func BenchmarkAsyncLoggerFallback(b *testing.B) {
	baseLogger := createBenchLogger()

	// 使用较小的缓冲区使降级更容易触发
	b.Run("with-fallback", func(b *testing.B) {
		asyncLogger := NewAsyncLogger(baseLogger, 100, WithFallback(true))
		defer asyncLogger.Close()

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			msg := NewLoggerMessage()
			msg.Message = "fallback test"
			asyncLogger.Info(msg)
		}

		// 输出丢弃的日志计数
		b.ReportMetric(float64(asyncLogger.GetDropCount()), "dropped_logs")
	})

	b.Run("no-fallback", func(b *testing.B) {
		asyncLogger := NewAsyncLogger(baseLogger, 100, WithFallback(false))
		defer asyncLogger.Close()

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			msg := NewLoggerMessage()
			msg.Message = "no fallback test"
			asyncLogger.Info(msg)
		}

		// 输出丢弃的日志计数
		b.ReportMetric(float64(asyncLogger.GetDropCount()), "dropped_logs")
	})
}

// BenchmarkAsyncLoggerHighLoad 测试高负载下异步日志性能
func BenchmarkAsyncLoggerHighLoad(b *testing.B) {
	// 创建基础日志记录器
	baseLogger := createBenchLogger()

	// 创建高负载配置
	highLoad := []struct {
		name       string
		bufferSize int
		workers    int
	}{
		{"small-buffer-few-workers", 1000, 2},
		{"small-buffer-many-workers", 1000, 8},
		{"large-buffer-few-workers", 50000, 2},
		{"large-buffer-many-workers", 50000, 8},
	}

	for _, cfg := range highLoad {
		b.Run(cfg.name, func(b *testing.B) {
			asyncLogger := NewAsyncLogger(baseLogger, cfg.bufferSize,
				WithWorkers(cfg.workers),
				WithFallback(true),
			)
			defer asyncLogger.Close()

			// 创建多个协程同时写入日志
			concurrency := 50
			messagesPerRoutine := b.N / concurrency
			if messagesPerRoutine < 1 {
				messagesPerRoutine = 1
			}

			b.ResetTimer()

			var wg sync.WaitGroup
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					for j := 0; j < messagesPerRoutine; j++ {
						msg := NewLoggerMessage()
						msg.Message = "high load test"
						msg.WithField("thread_id", id)
						msg.WithField("counter", j)
						asyncLogger.Info(msg)
					}
				}(i)
			}

			wg.Wait()

			// 报告丢弃的日志数量
			b.ReportMetric(float64(asyncLogger.GetDropCount()), "dropped_logs")
		})
	}
}

// BenchmarkAsyncLoggerRealWorld 模拟真实场景的日志记录
func BenchmarkAsyncLoggerRealWorld(b *testing.B) {
	baseLogger := createBenchLogger()

	// 不同的日志级别比例
	b.Run("realistic-mix", func(b *testing.B) {
		asyncLogger := NewAsyncLogger(baseLogger, 10000, WithWorkers(4))
		defer asyncLogger.Close()

		b.ResetTimer()

		// 模拟现实场景下的日志分布
		// 70% Info, 20% Debug, 5% Warn, 5% Error
		for i := 0; i < b.N; i++ {
			msg := NewLoggerMessage()
			msg.WithField("request_id", "req-"+strconv.Itoa(i%1000))
			msg.WithField("user_id", "user-"+strconv.Itoa(i%100))

			level := i % 100 // 使用模100来确定日志级别

			switch {
			case level < 70:
				msg.Message = "请求处理完成"
				asyncLogger.Info(msg)
			case level < 90:
				msg.Message = "处理详情"
				asyncLogger.Debug(msg)
			case level < 95:
				msg.Message = "性能警告"
				asyncLogger.Warn(msg)
			default:
				msg.Message = "请求处理失败"
				asyncLogger.Error(msg, fmt.Errorf("error-%d", i))
			}
		}
	})
}

// BenchmarkAsyncVsSyncLogger 对比同步和异步日志性能
func BenchmarkAsyncVsSyncLogger(b *testing.B) {
	baseLogger := createBenchLogger()
	asyncLogger := NewAsyncLogger(baseLogger, 10000, WithWorkers(4))
	defer asyncLogger.Close()

	b.Run("async", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			msg := NewLoggerMessage()
			msg.Message = "bench log"
			asyncLogger.Info(msg)
		}
	})

	b.Run("sync", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			msg := NewLoggerMessage()
			msg.Message = "bench log"
			baseLogger.Info(msg)
		}
	})
}

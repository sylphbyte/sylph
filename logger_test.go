package sylph

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewLogger(t *testing.T) {
	t.Run("should create logger with default config", func(t *testing.T) {
		logger := DefaultLogger("test")
		assert.NotNil(t, logger)
		assert.NotNil(t, logger.entry)
		assert.Equal(t, defaultLoggerConfig, logger.opt)
	})

	t.Run("should create logger with custom config", func(t *testing.T) {
		config := &LoggerConfig{
			Async: true,
		}
		logger := NewLogger("test", config)
		assert.NotNil(t, logger)
		assert.NotNil(t, logger.entry)
		assert.Equal(t, config, logger.opt)
	})
}

func TestLoggerMethods(t *testing.T) {
	logger := DefaultLogger("test")
	message := &LoggerMessage{
		Message: "test message",
	}

	tests := []struct {
		name   string
		method func(*LoggerMessage)
		level  logrus.Level
	}{
		{"Info", logger.Info, logrus.InfoLevel},
		{"Trace", logger.Trace, logrus.TraceLevel},
		{"Debug", logger.Debug, logrus.DebugLevel},
		{"Warn", logger.Warn, logrus.WarnLevel},
		{"Fatal", logger.Fatal, logrus.FatalLevel},
		{"Panic", logger.Panic, logrus.PanicLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.method(message)
			// TODO: Add assertions to verify log output
		})
	}
}

func TestLoggerError(t *testing.T) {
	logger := DefaultLogger("test")
	message := &LoggerMessage{
		Message: "test error",
	}
	err := assert.AnError

	logger.Error(message, err)
	assert.Equal(t, err.Error(), message.Error)
	// TODO: Add assertions to verify error log output
}

func TestLoggerAsync(t *testing.T) {
	config := &LoggerConfig{
		Async: true,
	}
	logger := NewLogger("test", config)
	message := &LoggerMessage{
		Message: "async test",
	}

	logger.Info(message)
	// TODO: Add assertions to verify async behavior
}

func TestLoggerRecover(t *testing.T) {
	// TODO: Add test case for recover mechanism
}

func TestSimplifiedLogger(t *testing.T) {
	// 创建日志器
	logger := DefaultLogger("test-logger")
	defer logger.Close()

	// 基本信息日志
	msg := NewLoggerMessage()
	msg.Message = "这是一条信息日志"
	msg.Data = map[string]interface{}{
		"user_id": 12345,
		"action":  "login",
		"time":    time.Now().Format(time.RFC3339),
	}
	logger.Info(msg)

	// 调试日志
	debugMsg := NewLoggerMessage()
	debugMsg.Message = "这是一条调试日志"
	debugMsg.Data = map[string]interface{}{
		"query":  "SELECT * FROM users",
		"params": []interface{}{1, "admin"},
	}
	logger.Debug(debugMsg)

	// 警告日志
	warnMsg := NewLoggerMessage()
	warnMsg.Message = "这是一条警告日志"
	warnMsg.Data = map[string]interface{}{
		"memory_usage": "85%",
		"threshold":    "80%",
	}
	logger.Warn(warnMsg)

	// 错误日志
	errMsg := NewLoggerMessage()
	errMsg.Message = "数据库连接失败"
	errMsg.Data = map[string]interface{}{
		"db_host": "127.0.0.1",
		"db_port": 3306,
	}
	testErr := errors.New("connection timeout")
	logger.Error(errMsg, testErr)

	// 并发日志测试
	for i := 0; i < 10; i++ {
		go func(index int) {
			concurrentMsg := NewLoggerMessage()
			concurrentMsg.Message = "并发日志测试"
			concurrentMsg.Data = map[string]interface{}{
				"goroutine_id": index,
				"timestamp":    time.Now().UnixNano(),
			}
			logger.Info(concurrentMsg)
		}(i)
	}

	// 等待所有异步日志完成
	time.Sleep(100 * time.Millisecond)

	t.Log("日志测试完成")
}

func TestLoggerCloseWait(t *testing.T) {
	logger := DefaultLogger("close-test")

	// 触发大量异步日志
	for i := 0; i < 100; i++ {
		msg := NewLoggerMessage()
		msg.Message = "测试关闭等待功能"
		msg.Data = map[string]interface{}{"index": i}
		logger.Info(msg)
	}

	// 测量关闭耗时
	start := time.Now()
	logger.Close()
	duration := time.Since(start)

	t.Logf("关闭日志记录器耗时: %v", duration)
}

func BenchmarkLoggerWithoutPool(b *testing.B) {
	logger := DefaultLogger("benchmark")
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := NewLoggerMessage()
		msg.Message = "性能测试日志"
		msg.Data = map[string]interface{}{
			"iteration": i,
			"timestamp": time.Now().UnixNano(),
		}
		logger.Info(msg)
	}
	b.StopTimer()

	// 等待所有异步日志完成
	logger.Close()
}

func TestSimplifiedLoggerAPI(t *testing.T) {
	logger := DefaultLogger("api-test")
	defer logger.Close()

	// 测试链式API
	t.Run("chain API", func(t *testing.T) {
		msg := NewLoggerMessage().
			WithField("user_id", 12345).
			WithField("action", "login").
			WithError(errors.New("test error")).
			WithLocation("auth.service").
			WithStack("fake stack trace")

		// 验证字段设置正确
		assert.Equal(t, "test error", msg.Error)
		assert.Equal(t, "auth.service", msg.Location)
		assert.Equal(t, "fake stack trace", msg.Stack)

		// 验证Data字段
		data, ok := msg.Data.(map[string]interface{})
		assert.True(t, ok, "Data should be a map")
		assert.Equal(t, 12345, data["user_id"])
		assert.Equal(t, "login", data["action"])

		// 记录日志（不应抛出异常）
		logger.Info(msg)
	})

	// 测试格式化API
	t.Run("formatting API", func(t *testing.T) {
		// 这些调用不应抛出异常
		logger.Infof("用户 %d 登录成功", 12345)
		logger.Debugf("正在处理 %s 请求", "GET /api/users")
		logger.Warnf("资源使用率: %.2f%%", 85.75)
		logger.Errorf(errors.New("database error"), "数据库查询失败: %s", "SELECT * FROM users")
	})

	// 测试上下文支持
	t.Run("context support", func(t *testing.T) {
		// 创建带取消的上下文
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 创建使用该上下文的日志记录器
		ctxLogger := logger.WithContext(ctx)

		// 记录日志（不应抛出异常）
		ctxLogger.Infof("使用上下文的日志")

		// 取消上下文
		cancel()

		// 即使上下文已取消，记录日志也应该安全
		ctxLogger.Infof("上下文已取消后的日志")

		// 验证接口转换
		_, ok := ctxLogger.(ILogger)
		assert.True(t, ok, "ContextLogger should implement ILogger")
	})
}

func TestBatchLogging(t *testing.T) {
	logger := DefaultLogger("batch-test")
	defer logger.Close()

	// 生成大量日志，测试性能和稳定性
	for i := 0; i < 100; i++ {
		msg := NewLoggerMessage().
			WithField("index", i).
			WithField("timestamp", time.Now().UnixNano())
		msg.Message = fmt.Sprintf("批量日志测试 #%d", i)

		logger.Info(msg)
	}

	// 等待日志处理完成
	time.Sleep(100 * time.Millisecond)
}

func TestAsyncLogger(t *testing.T) {
	// 创建基础日志记录器
	baseLogger := DefaultLogger("async-base")

	// 使用异步包装
	asyncLogger := NewAsyncLogger(baseLogger, 1000)
	defer asyncLogger.Close()

	// 测试基本日志记录
	t.Run("basic logging", func(t *testing.T) {
		// 这些调用不应抛出异常
		asyncLogger.Infof("异步日志测试")
		asyncLogger.Debugf("调试信息 #%d", 123)

		msg := NewLoggerMessage().
			WithField("component", "async-test").
			WithField("test_id", 456)
		msg.Message = "结构化异步日志"

		asyncLogger.Info(msg)
	})

	// 测试批量异步日志
	t.Run("batch async logging", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			asyncLogger.Infof("批量异步日志 #%d", i)
		}
	})

	// 测试上下文支持
	t.Run("async context support", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		ctxLogger := asyncLogger.WithContext(ctx)
		ctxLogger.Infof("带超时的异步日志")

		// 验证接口转换
		_, ok := ctxLogger.(ILogger)
		assert.True(t, ok, "Async context logger should implement ILogger")
	})

	// 等待异步日志处理完成
	time.Sleep(200 * time.Millisecond)
}

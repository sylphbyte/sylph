package sylph

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// LoggerMessage 测试
// =============================================================================

// TestNewLoggerMessage 测试创建日志消息
func TestNewLoggerMessage(t *testing.T) {
	msg := NewLoggerMessage()

	assert.NotNil(t, msg)
	assert.Nil(t, msg.Data)
	assert.Empty(t, msg.Error)
	assert.Empty(t, msg.Stack)
}

// TestLoggerMessageWithField 测试单字段添加
func TestLoggerMessageWithField(t *testing.T) {
	msg := NewLoggerMessage()

	// 测试单个字段
	msg.WithField("key1", "value1")
	data, ok := msg.Data.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "value1", data["key1"])

	// 测试链式调用
	msg.WithField("key2", 123).WithField("key3", true)
	assert.Equal(t, 123, data["key2"])
	assert.Equal(t, true, data["key3"])
}

// TestLoggerMessageWithFields 测试批量字段添加
func TestLoggerMessageWithFields(t *testing.T) {
	msg := NewLoggerMessage()

	fields := map[string]any{
		"user_id": 123,
		"action":  "login",
		"success": true,
	}

	msg.WithFields(fields)

	data, ok := msg.Data.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, 123, data["user_id"])
	assert.Equal(t, "login", data["action"])
	assert.Equal(t, true, data["success"])
}

// TestLoggerMessageWithFieldsNil 测试 nil fields
func TestLoggerMessageWithFieldsNil(t *testing.T) {
	msg := NewLoggerMessage()
	result := msg.WithFields(nil)

	assert.NotNil(t, result)
	assert.Nil(t, msg.Data)
}

// TestLoggerMessageWithError 测试错误添加
func TestLoggerMessageWithError(t *testing.T) {
	msg := NewLoggerMessage()
	testErr := errors.New("test error")

	msg.WithError(testErr)

	assert.Equal(t, "test error", msg.Error)
}

// TestLoggerMessageWithErrorNil 测试 nil 错误
func TestLoggerMessageWithErrorNil(t *testing.T) {
	msg := NewLoggerMessage()
	msg.WithError(nil)

	assert.Empty(t, msg.Error)
}

// TestLoggerMessageWithLocation 测试位置设置
func TestLoggerMessageWithLocation(t *testing.T) {
	msg := NewLoggerMessage()

	msg.WithLocation("TestFunction")

	assert.Equal(t, "TestFunction", msg.Location)
}

// TestLoggerMessageWithHeader 测试 Header 设置
func TestLoggerMessageWithHeader(t *testing.T) {
	msg := NewLoggerMessage()
	header := NewHeader(Endpoint("api"))

	msg.WithHeader(header)

	assert.NotNil(t, msg.Header)
	assert.Equal(t, Endpoint("api"), msg.Header.Endpoint())
}

// TestLoggerMessageWithStack 测试堆栈设置
func TestLoggerMessageWithStack(t *testing.T) {
	msg := NewLoggerMessage()
	stack := "stack trace line 1\nstack trace line 2"

	msg.WithStack(stack)

	assert.Equal(t, stack, msg.Stack)
}

// TestLoggerMessageChaining 测试链式调用
func TestLoggerMessageChaining(t *testing.T) {
	msg := NewLoggerMessage().
		WithField("user", "张三").
		WithLocation("TestFunc").
		WithError(errors.New("test")).
		WithStack("stack info")

	assert.NotNil(t, msg.Data)
	assert.Equal(t, "TestFunc", msg.Location)
	assert.Equal(t, "test", msg.Error)
	assert.Equal(t, "stack info", msg.Stack)
}

// TestLoggerMessageToLogrusFields 测试转换到 logrus 字段
func TestLoggerMessageToLogrusFields(t *testing.T) {
	msg := NewLoggerMessage()
	msg.WithField("key", "value")

	fields := msg.ToLogrusFields()

	assert.NotNil(t, fields)
	assert.Contains(t, fields, "_message")
}

// =============================================================================
// Logger 基础测试
// =============================================================================

// TestDefaultLoggerCreation 测试默认日志器创建
func TestDefaultLoggerCreation(t *testing.T) {
	logger := DefaultLogger("test-logger")

	assert.NotNil(t, logger)
	assert.NotNil(t, logger.entry)
	assert.Equal(t, "test-logger", logger.name)
	assert.False(t, logger.IsClosed())

	logger.Close()
}

// TestNewLoggerWithConfig 测试带配置创建
func TestNewLoggerWithConfig(t *testing.T) {
	config := &LoggerConfig{
		Async: false,
	}

	logger := NewLogger("custom-logger", config)

	assert.NotNil(t, logger)
	assert.Equal(t, config, logger.opt)
	assert.Equal(t, "custom-logger", logger.name)

	logger.Close()
}

// TestLoggerClose 测试关闭
func TestLoggerClose(t *testing.T) {
	logger := DefaultLogger("test-close")

	assert.False(t, logger.IsClosed())

	err := logger.Close()
	// Close 可能返回错误（如关闭stderr），这是预期行为
	assert.True(t, logger.IsClosed())

	// 重复关闭应该是安全的
	err = logger.Close()
	_ = err // 忽略重复关闭的错误
	assert.True(t, logger.IsClosed())
}

// TestLoggerWithContext 测试上下文设置
func TestLoggerWithContext(t *testing.T) {
	logger := DefaultLogger("test-context")
	defer logger.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	newLogger := logger.WithContext(ctx)
	assert.NotNil(t, newLogger)

	// 测试 nil context
	nilLogger := logger.WithContext(nil)
	assert.Equal(t, logger, nilLogger)
}

// =============================================================================
// Logger 结构化日志测试
// =============================================================================

// TestLoggerInfo 测试 Info 日志
func TestLoggerInfo(t *testing.T) {
	logger := DefaultLogger("test-info")
	defer logger.Close()

	msg := NewLoggerMessage().
		WithLocation("TestLoggerInfo").
		WithField("key", "value")
	msg.Message = "info message"

	assert.NotPanics(t, func() {
		logger.Info(msg)
	})
}

// TestLoggerTrace 测试 Trace 日志
func TestLoggerTrace(t *testing.T) {
	logger := DefaultLogger("test-trace")
	defer logger.Close()

	msg := NewLoggerMessage()
	msg.Message = "trace message"

	assert.NotPanics(t, func() {
		logger.Trace(msg)
	})
}

// TestLoggerDebug 测试 Debug 日志
func TestLoggerDebug(t *testing.T) {
	logger := DefaultLogger("test-debug")
	defer logger.Close()

	msg := NewLoggerMessage()
	msg.Message = "debug message"

	assert.NotPanics(t, func() {
		logger.Debug(msg)
	})
}

// TestLoggerWarn 测试 Warn 日志
func TestLoggerWarn(t *testing.T) {
	logger := DefaultLogger("test-warn")
	defer logger.Close()

	msg := NewLoggerMessage()
	msg.Message = "warn message"

	assert.NotPanics(t, func() {
		logger.Warn(msg)
	})
}

// TestLoggerErrorMethod 测试 Error 日志
func TestLoggerErrorMethod(t *testing.T) {
	logger := DefaultLogger("test-error")
	defer logger.Close()

	msg := NewLoggerMessage()
	msg.Message = "error occurred"
	testErr := errors.New("test error")

	assert.NotPanics(t, func() {
		logger.Error(msg, testErr)
	})

	// 验证错误已设置
	assert.Equal(t, "test error", msg.Error)
}

// TestLoggerErrorWithNil 测试 Error 带 nil
func TestLoggerErrorWithNil(t *testing.T) {
	logger := DefaultLogger("test-error-nil")
	defer logger.Close()

	msg := NewLoggerMessage()
	msg.Message = "no error"

	assert.NotPanics(t, func() {
		logger.Error(msg, nil)
	})
}

// =============================================================================
// Logger 格式化日志测试
// =============================================================================

// TestLoggerInfof 测试格式化 Info
func TestLoggerInfof(t *testing.T) {
	logger := DefaultLogger("test-infof")
	defer logger.Close()

	assert.NotPanics(t, func() {
		logger.Infof("User %s logged in", "张三")
	})
}

// TestLoggerTracef 测试格式化 Trace
func TestLoggerTracef(t *testing.T) {
	logger := DefaultLogger("test-tracef")
	defer logger.Close()

	assert.NotPanics(t, func() {
		logger.Tracef("Trace: %d items processed", 100)
	})
}

// TestLoggerDebugf 测试格式化 Debug
func TestLoggerDebugf(t *testing.T) {
	logger := DefaultLogger("test-debugf")
	defer logger.Close()

	assert.NotPanics(t, func() {
		logger.Debugf("Debug value: %v", map[string]int{"count": 42})
	})
}

// TestLoggerWarnf 测试格式化 Warn
func TestLoggerWarnf(t *testing.T) {
	logger := DefaultLogger("test-warnf")
	defer logger.Close()

	assert.NotPanics(t, func() {
		logger.Warnf("Warning: usage at %d%%", 85)
	})
}

// TestLoggerErrorf 测试格式化 Error
func TestLoggerErrorf(t *testing.T) {
	logger := DefaultLogger("test-errorf")
	defer logger.Close()

	testErr := errors.New("connection failed")

	assert.NotPanics(t, func() {
		logger.Errorf(testErr, "Failed to connect to %s", "database")
	})
}

// TestLoggerErrorfWithNilError 测试 Errorf 带 nil 错误
func TestLoggerErrorfWithNilError(t *testing.T) {
	logger := DefaultLogger("test-errorf-nil")
	defer logger.Close()

	assert.NotPanics(t, func() {
		logger.Errorf(nil, "No error occurred")
	})
}

// =============================================================================
// Logger 异步模式测试
// =============================================================================

// TestLoggerAsync 测试异步日志
func TestLoggerAsync(t *testing.T) {
	config := &LoggerConfig{
		Async: true,
	}

	logger := NewLogger("test-async", config)
	defer logger.Close()

	msg := NewLoggerMessage()
	msg.Message = "async test message"

	// 异步日志不应该阻塞
	start := time.Now()
	logger.Info(msg)
	duration := time.Since(start)

	// 异步应该很快返回（< 10ms）
	assert.Less(t, duration, 10*time.Millisecond)

	// 等待异步处理完成
	time.Sleep(100 * time.Millisecond)
}

// TestLoggerAsyncMultiple 测试多条异步日志
func TestLoggerAsyncMultiple(t *testing.T) {
	config := &LoggerConfig{
		Async: true,
	}

	logger := NewLogger("test-async-multiple", config)
	defer logger.Close()

	// 快速发送多条日志
	for i := 0; i < 10; i++ {
		msg := NewLoggerMessage()
		msg.Message = "message"
		msg.WithField("index", i)
		logger.Info(msg)
	}

	// 等待处理
	time.Sleep(100 * time.Millisecond)
}

// =============================================================================
// Logger 边界情况测试
// =============================================================================

// TestLoggerWithNilMessage 测试 nil 消息
func TestLoggerWithNilMessage(t *testing.T) {
	logger := DefaultLogger("test-nil")
	defer logger.Close()

	// Logger 不能处理 nil 消息，会 panic
	// 这是一个已知限制，实际使用中应避免传入 nil
	assert.Panics(t, func() {
		logger.Info(nil)
	})
}

// TestLoggerAfterClose 测试关闭后的行为
func TestLoggerAfterClose(t *testing.T) {
	logger := DefaultLogger("test-after-close")

	logger.Close()
	assert.True(t, logger.IsClosed())

	// 关闭后记录日志不应该崩溃
	msg := NewLoggerMessage()
	msg.Message = "after close"

	assert.NotPanics(t, func() {
		logger.Info(msg)
	})
}

// TestLoggerConcurrent 测试并发日志
func TestLoggerConcurrent(t *testing.T) {
	logger := DefaultLogger("test-concurrent")
	defer logger.Close()

	done := make(chan bool, 10)

	// 并发写入日志
	for i := 0; i < 10; i++ {
		go func(n int) {
			msg := NewLoggerMessage()
			msg.Message = "concurrent message"
			msg.WithField("goroutine", n)
			logger.Info(msg)
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// =============================================================================
// 性能基准测试
// =============================================================================

// BenchmarkLoggerInfo Info 日志性能
func BenchmarkLoggerInfo(b *testing.B) {
	logger := DefaultLogger("bench")
	defer logger.Close()

	msg := NewLoggerMessage()
	msg.Message = "benchmark message"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info(msg)
	}
}

// BenchmarkLoggerInfof 格式化日志性能
func BenchmarkLoggerInfof(b *testing.B) {
	logger := DefaultLogger("bench")
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Infof("Message %d", i)
	}
}

// BenchmarkLoggerAsync 异步日志性能
func BenchmarkLoggerAsync(b *testing.B) {
	config := &LoggerConfig{
		Async: true,
	}
	logger := NewLogger("bench-async", config)
	defer logger.Close()

	msg := NewLoggerMessage()
	msg.Message = "async benchmark"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info(msg)
	}
}

// BenchmarkNewLoggerMessage 创建消息性能
func BenchmarkNewLoggerMessage(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewLoggerMessage()
	}
}

// BenchmarkLoggerMessageWithFields 字段添加性能
func BenchmarkLoggerMessageWithFields(b *testing.B) {
	fields := map[string]any{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := NewLoggerMessage()
		msg.WithFields(fields)
	}
}

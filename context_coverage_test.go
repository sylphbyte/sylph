package sylph

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// 基础功能测试
// =============================================================================

// TestContextCreation 测试 Context 创建
func TestContextCreation(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/users")

	assert.NotNil(t, ctx)
	assert.NotEmpty(t, ctx.TakeHeader().TraceId())
	assert.Equal(t, "/api/users", ctx.TakeHeader().Path())
	assert.Equal(t, Endpoint("api"), ctx.TakeHeader().Endpoint())
}

// =============================================================================
// 数据存取测试 (DataContext)
// =============================================================================

// TestContextSet 测试 Set 方法
func TestContextSet(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	ctx.Set("string_key", "value")
	ctx.Set("int_key", 123)
	ctx.Set("bool_key", true)

	val, ok := ctx.Get("string_key")
	assert.True(t, ok)
	assert.Equal(t, "value", val)
}

// TestContextGet 测试 Get 方法
func TestContextGet(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	// 测试不存在的键
	val, ok := ctx.Get("non_existent")
	assert.False(t, ok)
	assert.Nil(t, val)

	// 测试存在的键
	ctx.Set("key", "value")
	val, ok = ctx.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "value", val)
}

// TestContextGetString 测试 GetString 方法
func TestContextGetString(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	// 测试存在的字符串
	ctx.Set("name", "张三")
	val, ok := ctx.GetString("name")
	assert.True(t, ok)
	assert.Equal(t, "张三", val)

	// 测试不存在的键
	_, ok = ctx.GetString("non_existent")
	assert.False(t, ok)
}

// TestContextGetInt 测试 GetInt 方法
func TestContextGetInt(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	// 测试存在的整数
	ctx.Set("age", 25)
	val, ok := ctx.GetInt("age")
	assert.True(t, ok)
	assert.Equal(t, 25, val)

	// 测试不存在的键
	_, ok = ctx.GetInt("non_existent")
	assert.False(t, ok)
}

// TestContextGetBool 测试 GetBool 方法
func TestContextGetBool(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	// 测试存在的布尔值
	ctx.Set("active", true)
	val, ok := ctx.GetBool("active")
	assert.True(t, ok)
	assert.True(t, val)

	// 测试不存在的键
	_, ok = ctx.GetBool("non_existent")
	assert.False(t, ok)
}

// TestContextMarkSet 测试 MarkSet 方法
func TestContextMarkSet(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	ctx.MarkSet("marked_key", "marked_value")

	val, ok := ctx.Get("marked_key")
	assert.True(t, ok)
	assert.Equal(t, "marked_value", val)
}

// =============================================================================
// Clone 测试
// =============================================================================

// TestContextClone 测试 Clone 方法
func TestContextClone(t *testing.T) {
	original := NewContext(Endpoint("api"), "/api/test")
	original.Set("shared", "data")

	cloned := original.Clone()

	// 验证不是同一个对象
	assert.NotSame(t, original, cloned)

	// 验证 TraceId 相同
	assert.Equal(t, original.TakeHeader().TraceId(), cloned.TakeHeader().TraceId())

	// 验证数据被复制
	val, ok := cloned.Get("shared")
	assert.True(t, ok)
	assert.Equal(t, "data", val)
}

// TestContextCloneIndependence 测试 Clone 独立性
func TestContextCloneIndependence(t *testing.T) {
	original := NewContext(Endpoint("api"), "/api/test")
	cloned := original.Clone()

	// 克隆后修改不影响原始
	cloned.Set("cloned_key", "cloned_value")
	_, ok := original.Get("cloned_key")
	assert.False(t, ok)

	// 原始修改不影响克隆
	original.Set("original_key", "original_value")
	_, ok = cloned.Get("original_key")
	assert.False(t, ok)
}

// =============================================================================
// Timeout 和 Cancel 测试
// =============================================================================

// TestContextWithTimeout 测试 WithTimeout 方法
func TestContextWithTimeout(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")
	defaultCtx, ok := ctx.(*DefaultContext)
	assert.True(t, ok)

	timeoutCtx, cancel := defaultCtx.WithTimeout(100 * time.Millisecond)
	defer cancel()

	// 等待超时
	<-timeoutCtx.Done()
	assert.Equal(t, context.DeadlineExceeded, timeoutCtx.Err())
}

// TestContextWithCancel 测试 WithCancel 方法
func TestContextWithCancel(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")
	defaultCtx, ok := ctx.(*DefaultContext)
	assert.True(t, ok)

	cancelCtx, cancel := defaultCtx.WithCancel()

	// 立即取消
	cancel()

	// 验证已取消
	<-cancelCtx.Done()
	assert.Equal(t, context.Canceled, cancelCtx.Err())
}

// TestContextWithDeadline 测试 WithDeadline 方法
func TestContextWithDeadline(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")
	defaultCtx, ok := ctx.(*DefaultContext)
	assert.True(t, ok)

	deadline := time.Now().Add(100 * time.Millisecond)
	deadlineCtx, cancel := defaultCtx.WithDeadline(deadline)
	defer cancel()

	// 验证截止时间
	dl, ok := deadlineCtx.Deadline()
	assert.True(t, ok)
	assert.True(t, dl.Equal(deadline) || dl.Before(deadline.Add(time.Millisecond)))

	// 等待超时
	<-deadlineCtx.Done()
	assert.Equal(t, context.DeadlineExceeded, deadlineCtx.Err())
}

// TestContextWithCancelCause 测试 WithCancelCause 方法
func TestContextWithCancelCause(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")
	defaultCtx, ok := ctx.(*DefaultContext)
	assert.True(t, ok)

	cancelCtx, cancel := defaultCtx.WithCancelCause()

	// 使用错误取消
	testErr := errors.New("test error")
	cancel(testErr)

	// 验证已取消
	<-cancelCtx.Done()
	assert.Equal(t, testErr, context.Cause(cancelCtx))
}

// =============================================================================
// Header 测试
// =============================================================================

// TestContextTakeHeader 测试 TakeHeader 方法
func TestContextTakeHeader(t *testing.T) {
	ctx := NewContext(Endpoint("web"), "/api/users/123")
	header := ctx.TakeHeader()

	assert.NotNil(t, header)
	assert.Equal(t, Endpoint("web"), header.Endpoint())
	assert.Equal(t, "/api/users/123", header.Path())
	assert.NotEmpty(t, header.TraceId())
}

// TestContextStoreHeader 测试 StoreHeader 方法
func TestContextStoreHeader(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")
	newHeader := NewHeader(Endpoint("web"))
	newHeader.StorePath("/new/path")

	ctx.StoreHeader(newHeader)

	header := ctx.TakeHeader()
	assert.Equal(t, Endpoint("web"), header.Endpoint())
	assert.Equal(t, "/new/path", header.Path())
}

// =============================================================================
// Mark 测试
// =============================================================================

// TestContextWithMark 测试 WithMark 方法
func TestContextWithMark(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	ctx.WithMark("priority", "urgent")

	marks := ctx.TakeMarks()
	assert.NotNil(t, marks)
}

// TestContextTakeMarks 测试 TakeMarks 方法
func TestContextTakeMarks(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	marks := ctx.TakeMarks()
	assert.NotNil(t, marks)
}

// =============================================================================
// WithValue 测试
// =============================================================================

// TestContextWithValue 测试 WithValue 方法
func TestContextWithValue(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")
	defaultCtx, ok := ctx.(*DefaultContext)
	assert.True(t, ok)

	key := "context_key"
	value := "context_value"

	valueCtx := defaultCtx.WithValue(key, value)

	// 验证值
	assert.Equal(t, value, valueCtx.Value(key))
}

// =============================================================================
// 标准 context.Context 接口测试
// =============================================================================

// TestContextDone 测试 Done 方法
func TestContextDone(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	// 默认不应该关闭
	select {
	case <-ctx.Done():
		t.Fatal("Should not be done initially")
	case <-time.After(10 * time.Millisecond):
		// 正常
	}
}

// TestContextErr 测试 Err 方法
func TestContextErr(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	// 初始应该返回 nil
	err := ctx.Err()
	assert.Nil(t, err)
}

// TestContextDeadline 测试 Deadline 方法
func TestContextDeadline(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	// 默认没有截止时间
	_, ok := ctx.Deadline()
	assert.False(t, ok)
}

// TestContextValue 测试 Value 方法
func TestContextValue(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	// 默认应该返回 nil
	val := ctx.Value("non_existent")
	assert.Nil(t, val)
}

// =============================================================================
// Logger 测试
// =============================================================================

// TestContextTakeLogger 测试 TakeLogger 方法
func TestContextTakeLogger(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	logger := ctx.TakeLogger()
	assert.NotNil(t, logger)
}

// TestContextInfo 测试 Info 方法
func TestContextInfo(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	assert.NotPanics(t, func() {
		ctx.Info("TestLocation", "test message", nil)
		ctx.Info("TestLocation", "test with data", map[string]any{"key": "value"})
	})
}

// TestContextDebug 测试 Debug 方法
func TestContextDebug(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	assert.NotPanics(t, func() {
		ctx.Debug("TestLocation", "debug message", nil)
	})
}

// TestContextWarn 测试 Warn 方法
func TestContextWarn(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	assert.NotPanics(t, func() {
		ctx.Warn("TestLocation", "warn message", nil)
	})
}

// TestContextTrace 测试 Trace 方法
func TestContextTrace(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	assert.NotPanics(t, func() {
		ctx.Trace("TestLocation", "trace message", nil)
	})
}

// TestContextError 测试 Error 方法
func TestContextError(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	testErr := errors.New("test error")
	assert.NotPanics(t, func() {
		ctx.Error("TestLocation", "error occurred", testErr, nil)
	})
}

// =============================================================================
// JWT 测试
// =============================================================================

// TestContextJwtClaim 测试 JWT 相关方法
func TestContextJwtClaim(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	// 初始应该为 nil
	claim := ctx.JwtClaim()
	assert.Nil(t, claim)
}

// =============================================================================
// 并发安全测试
// =============================================================================

// TestContextConcurrentSetGet 测试并发读写安全
func TestContextConcurrentSetGet(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")
	var wg sync.WaitGroup

	// 并发写入
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ctx.Set("key", n)
		}(i)
	}

	// 并发读取
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx.Get("key")
		}()
	}

	wg.Wait()
}

// TestContextConcurrentClone 测试并发 Clone 安全
func TestContextConcurrentClone(t *testing.T) {
	ctx := NewContext(Endpoint("api"), "/api/test")
	ctx.Set("shared", "value")

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			cloned := ctx.Clone()
			cloned.Set("data", n)
			cloned.Get("shared")
		}(i)
	}

	wg.Wait()
}

// =============================================================================
// 性能基准测试
// =============================================================================

// BenchmarkContextNew Context 创建性能
func BenchmarkContextNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewContext(Endpoint("api"), "/api/test")
	}
}

// BenchmarkContextSet Set 操作性能
func BenchmarkContextSet(b *testing.B) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Set("key", i)
	}
}

// BenchmarkContextGet Get 操作性能
func BenchmarkContextGet(b *testing.B) {
	ctx := NewContext(Endpoint("api"), "/api/test")
	ctx.Set("key", "value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Get("key")
	}
}

// BenchmarkContextClone Clone 操作性能
func BenchmarkContextClone(b *testing.B) {
	ctx := NewContext(Endpoint("api"), "/api/test")
	ctx.Set("key1", "value1")
	ctx.Set("key2", "value2")
	ctx.Set("key3", "value3")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ctx.Clone()
	}
}

// BenchmarkContextConcurrent 并发操作性能
func BenchmarkContextConcurrent(b *testing.B) {
	ctx := NewContext(Endpoint("api"), "/api/test")

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				ctx.Set("key", i)
			} else {
				ctx.Get("key")
			}
			i++
		}
	})
}

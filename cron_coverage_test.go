package sylph

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// CrontabMode 和 CrontabModeName 测试
// =============================================================================

// TestCrontabModeName 测试 CrontabMode 到名称的转换
func TestCrontabModeName(t *testing.T) {
	tests := []struct {
		mode     CrontabMode
		expected CrontabModeName
	}{
		{CrontabNormalMode, CrontabNormalName},
		{CrontabSkipMode, CrontabSkipName},
		{CrontabDelayMode, CrontabDelayName},
	}

	for _, tt := range tests {
		t.Run(string(tt.expected), func(t *testing.T) {
			name := tt.mode.Name()
			assert.Equal(t, tt.expected, name)
		})
	}
}

// TestCrontabModeNameString 测试 CrontabModeName.String()
func TestCrontabModeNameString(t *testing.T) {
	tests := []struct {
		name     CrontabModeName
		expected string
	}{
		{CrontabNormalName, "normal"},
		{CrontabSkipName, "skip"},
		{CrontabDelayName, "delay"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.name.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCrontabModeNameMode 测试 CrontabModeName 到 Mode 的转换
func TestCrontabModeNameMode(t *testing.T) {
	tests := []struct {
		name     CrontabModeName
		expected CrontabMode
	}{
		{CrontabNormalName, CrontabNormalMode},
		{CrontabSkipName, CrontabSkipMode},
		{CrontabDelayName, CrontabDelayMode},
	}

	for _, tt := range tests {
		t.Run(string(tt.name), func(t *testing.T) {
			mode := tt.name.Mode()
			assert.Equal(t, tt.expected, mode)
		})
	}
}

// TestCrontabModeNameModeInvalid 测试无效的模式名称会 panic
func TestCrontabModeNameModeInvalid(t *testing.T) {
	invalidName := CrontabModeName("invalid")

	assert.Panics(t, func() {
		invalidName.Mode()
	})
}

// TestCrontabModeNameValid 测试 Valid() 方法
func TestCrontabModeNameValid(t *testing.T) {
	tests := []struct {
		name     CrontabModeName
		expected bool
	}{
		{CrontabNormalName, true},
		{CrontabSkipName, true},
		{CrontabDelayName, true},
		{CrontabModeName("invalid"), false},
		{CrontabModeName(""), false},
		{CrontabModeName("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.name), func(t *testing.T) {
			result := tt.name.Valid()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCrontabModeMapping 测试模式映射关系
func TestCrontabModeMapping(t *testing.T) {
	// 验证模式到名称的映射
	assert.Equal(t, CrontabNormalName, crontabModeMapping[CrontabNormalMode])
	assert.Equal(t, CrontabSkipName, crontabModeMapping[CrontabSkipMode])
	assert.Equal(t, CrontabDelayName, crontabModeMapping[CrontabDelayMode])

	// 验证名称到模式的映射
	assert.Equal(t, CrontabNormalMode, crontabNameModeMapping[CrontabNormalName])
	assert.Equal(t, CrontabSkipMode, crontabNameModeMapping[CrontabSkipName])
	assert.Equal(t, CrontabDelayMode, crontabNameModeMapping[CrontabDelayName])
}

// TestCrontabModeRoundTrip 测试双向转换
func TestCrontabModeRoundTrip(t *testing.T) {
	modes := []CrontabMode{
		CrontabNormalMode,
		CrontabSkipMode,
		CrontabDelayMode,
	}

	for _, mode := range modes {
		t.Run(mode.Name().String(), func(t *testing.T) {
			// Mode -> Name -> Mode
			name := mode.Name()
			backToMode := name.Mode()
			assert.Equal(t, mode, backToMode)

			// Name -> Mode -> Name
			assert.Equal(t, name, backToMode.Name())
		})
	}
}

// =============================================================================
// CronServer 基础功能测试
// =============================================================================

// TestNewCronServer 测试创建 CronServer
func TestNewCronServer(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")
	configs := []TaskConfig{}

	server := NewCronServer(ctx, CrontabNormalMode, configs)

	assert.NotNil(t, server)
	assert.Equal(t, CrontabNormalMode, server.mode)
	assert.NotNil(t, server.tasks)
	assert.False(t, server.started)
}

// TestNewCronServerDifferentModes 测试不同模式创建
func TestNewCronServerDifferentModes(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")
	configs := []TaskConfig{}

	modes := []CrontabMode{
		CrontabNormalMode,
		CrontabSkipMode,
		CrontabDelayMode,
	}

	for _, mode := range modes {
		t.Run(mode.Name().String(), func(t *testing.T) {
			server := NewCronServer(ctx, mode, configs)
			assert.Equal(t, mode, server.mode)
		})
	}
}

// TestCronServerName 测试 Name() 方法
func TestCronServerName(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")
	server := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})

	name := server.Name()
	assert.Equal(t, "cron-server", name)
}

// TestCronServerRegister 测试任务注册
func TestCronServerRegister(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")
	server := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})

	// 创建 mock 任务
	called := false
	mockTask := func(c Context) error {
		called = true
		return nil
	}

	// 注册任务
	taskName := TaskName("test-task")
	server.Register(taskName, mockTask)

	// 验证任务已注册
	task, ok := server.receiveTask(taskName)
	assert.True(t, ok)
	assert.NotNil(t, task)

	// 验证任务可以执行
	err := task(ctx)
	assert.NoError(t, err)
	assert.True(t, called)
}

// TestCronServerRegisterMultiple 测试注册多个任务
func TestCronServerRegisterMultiple(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")
	server := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})

	// 注册多个任务
	count := 0
	for i := 0; i < 5; i++ {
		taskName := TaskName(string(rune('a' + i)))
		server.Register(taskName, func(c Context) error {
			count++
			return nil
		})
	}

	// 验证所有任务都已注册
	for i := 0; i < 5; i++ {
		taskName := TaskName(string(rune('a' + i)))
		_, ok := server.receiveTask(taskName)
		assert.True(t, ok, "Task %s should be registered", taskName)
	}
}

// TestCronServerReceiveTask 测试获取任务
func TestCronServerReceiveTask(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")
	server := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})

	taskName := TaskName("test")
	mockTask := func(c Context) error { return nil }

	// 注册前获取
	_, ok := server.receiveTask(taskName)
	assert.False(t, ok)

	// 注册后获取
	server.Register(taskName, mockTask)
	task, ok := server.receiveTask(taskName)
	assert.True(t, ok)
	assert.NotNil(t, task)
}

// TestCronServerLoadOptions 测试加载选项
func TestCronServerLoadOptions(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")
	server := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})

	// 初始选项为空
	assert.Empty(t, server.opts)

	// 加载选项（使用 nil 作为占位）
	server.LoadOptions(nil, nil)
	assert.Len(t, server.opts, 2)
}

// =============================================================================
// 选项构建测试
// =============================================================================

// TestCronServerLoadDefaultOption 测试默认选项
func TestCronServerLoadDefaultOption(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")
	server := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})

	opts := server.loadDefaultOption()

	// 应该有默认选项
	assert.NotEmpty(t, opts)
	// 默认有 2 个：WithSeconds 和 WithLogger
	assert.Len(t, opts, 2)
}

// TestCronServerModeOptions 测试不同模式的选项
func TestCronServerModeOptions(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")

	modes := []struct {
		mode          CrontabMode
		expectWrapper bool
	}{
		{CrontabNormalMode, false}, // Normal 模式没有 wrapper
		{CrontabSkipMode, true},    // Skip 模式有 wrapper
		{CrontabDelayMode, true},   // Delay 模式有 wrapper
	}

	for _, tt := range modes {
		t.Run(tt.mode.Name().String(), func(t *testing.T) {
			server := NewCronServer(ctx, tt.mode, []TaskConfig{})
			opts := server.modeOptions()

			if tt.expectWrapper {
				assert.Len(t, opts, 1, "应该有 1 个 wrapper")
			} else {
				assert.Empty(t, opts, "Normal 模式不应该有 wrapper")
			}
		})
	}
}

// TestCronServerCombinationOptions 测试组合选项
func TestCronServerCombinationOptions(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")

	t.Run("Normal mode", func(t *testing.T) {
		server := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})
		opts := server.combinationOptions()

		// 默认选项(2) + Normal 模式选项(0) = 2
		assert.Len(t, opts, 2)
	})

	t.Run("Skip mode", func(t *testing.T) {
		server := NewCronServer(ctx, CrontabSkipMode, []TaskConfig{})
		opts := server.combinationOptions()

		// 默认选项(2) + Skip 模式选项(1) = 3
		assert.Len(t, opts, 3)
	})

	t.Run("Delay mode", func(t *testing.T) {
		server := NewCronServer(ctx, CrontabDelayMode, []TaskConfig{})
		opts := server.combinationOptions()

		// 默认选项(2) + Delay 模式选项(1) = 3
		assert.Len(t, opts, 3)
	})
}

// =============================================================================
// Boot 和 Shutdown 测试
// =============================================================================

// TestCronServerBoot 测试启动
func TestCronServerBoot(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")
	server := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})

	// 注册一个简单任务
	server.Register("test", func(c Context) error { return nil })

	// 启动
	err := server.Boot()
	assert.NoError(t, err)
	assert.True(t, server.started)

	// 清理
	server.Shutdown()
}

// TestCronServerBootIdempotent 测试重复启动（幂等）
func TestCronServerBootIdempotent(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")
	server := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})

	server.Register("test", func(c Context) error { return nil })

	// 第一次启动
	err := server.Boot()
	assert.NoError(t, err)

	// 第二次启动应该也不报错（幂等）
	err = server.Boot()
	assert.NoError(t, err)

	// 清理
	server.Shutdown()
}

// TestCronServerShutdown 测试关闭
func TestCronServerShutdown(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")
	server := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})

	server.Register("test", func(c Context) error { return nil })
	server.Boot()

	// 关闭
	err := server.Shutdown()
	assert.NoError(t, err)
}

// TestCronServerBootShutdownCycle 测试启动关闭循环
func TestCronServerBootShutdownCycle(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")
	server := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})

	server.Register("test", func(c Context) error { return nil })

	// 启动
	err := server.Boot()
	assert.NoError(t, err)

	// 等待一小段时间
	time.Sleep(10 * time.Millisecond)

	// 关闭
	err = server.Shutdown()
	assert.NoError(t, err)
}

// =============================================================================
// 并发测试
// =============================================================================

// TestCronServerConcurrentRegister 测试并发注册
// 修复后：CronServer.Register 现在是并发安全的（使用 sync.RWMutex）
func TestCronServerConcurrentRegister(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")
	server := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})

	var wg sync.WaitGroup

	// 并发注册任务 - 修复后不会 panic
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			taskName := TaskName(string(rune('a' + n)))
			server.Register(taskName, func(c Context) error { return nil })
		}(i)
	}

	wg.Wait()

	// 验证所有任务都已注册
	count := 0
	for i := 0; i < 10; i++ {
		taskName := TaskName(string(rune('a' + i)))
		if _, ok := server.receiveTask(taskName); ok {
			count++
		}
	}

	// 所有 10 个任务都应该成功注册
	assert.Equal(t, 10, count, "所有任务都应该注册成功")
}

// =============================================================================
// TaskConfig 相关测试
// =============================================================================

// TestCronServerWithTaskConfigs 测试带配置创建
func TestCronServerWithTaskConfigs(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")

	configs := []TaskConfig{
		{
			Name: "task1",
			Spec: "*/5 * * * * *",
			Open: true,
			Desc: "test-task-1",
		},
		{
			Name: "task2",
			Spec: "*/10 * * * * *",
			Open: false,
			Desc: "test-task-2",
		},
	}

	server := NewCronServer(ctx, CrontabNormalMode, configs)

	assert.NotNil(t, server)
	assert.Equal(t, 2, len(server.taskConfigs))
}

// =============================================================================
// 边界情况测试
// =============================================================================

// TestCronServerEmptyConfigs 测试空配置
func TestCronServerEmptyConfigs(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")
	server := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})

	assert.NotNil(t, server)
	assert.Empty(t, server.taskConfigs)
}

// TestCronServerNilTaskHandler 测试注册 nil 任务
func TestCronServerNilTaskHandler(t *testing.T) {
	ctx := NewContext(Endpoint("test"), "/test")
	server := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})

	// 注册 nil 任务（虽然不推荐，但不应该 panic）
	assert.NotPanics(t, func() {
		server.Register("nil-task", nil)
	})

	// 获取任务
	task, ok := server.receiveTask("nil-task")
	assert.True(t, ok)
	assert.Nil(t, task)
}

// =============================================================================
// 性能基准测试
// =============================================================================

// BenchmarkCrontabModeNameConversion 模式名称转换性能
func BenchmarkCrontabModeNameConversion(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = CrontabNormalMode.Name()
	}
}

// BenchmarkCrontabModeValidation 模式验证性能
func BenchmarkCrontabModeValidation(b *testing.B) {
	name := CrontabNormalName
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = name.Valid()
	}
}

// BenchmarkCronServerRegister 任务注册性能
func BenchmarkCronServerRegister(b *testing.B) {
	ctx := NewContext(Endpoint("test"), "/test")
	server := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})
	handler := func(c Context) error { return nil }

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		taskName := TaskName(string(rune(i%26 + 'a')))
		server.Register(taskName, handler)
	}
}

// BenchmarkNewCronServer CronServer 创建性能
func BenchmarkNewCronServer(b *testing.B) {
	ctx := NewContext(Endpoint("test"), "/test")
	configs := []TaskConfig{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewCronServer(ctx, CrontabNormalMode, configs)
	}
}

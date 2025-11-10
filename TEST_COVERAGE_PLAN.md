# Sylph 框架测试覆盖率提升计划

## 📊 当前状态

**总覆盖率: 0.5%** ⚠️ 严重不足

### 现有测试文件
- ✅ `header_test.go` - Header 基础测试
- ✅ `logger_test.go` - Logger 基础测试
- ✅ `logger_async_benchmark_test.go` - 异步日志性能测试
- ✅ `project_test.go` - Project 生命周期测试
- ✅ `rocket_test.go` - RocketMQ 集成测试
- ✅ `server_manager_test.go` - 服务管理器测试
- ✅ `server_manager_integration_test.go` - 集成测试
- ✅ `storage_adapter_test.go` - 存储适配器测试
- ✅ `storage_test.go` - 存储配置测试

### 覆盖率目标

| 模块 | 当前 | 目标 | 优先级 |
|------|------|------|--------|
| Context | ~0% | 80% | 🔴 P0 |
| Logger | ~5% | 70% | 🟠 P1 |
| Storage | ~10% | 70% | 🟠 P1 |
| Project | ~15% | 60% | 🟡 P2 |
| Server Manager | ~10% | 60% | 🟡 P2 |
| Header | ~20% | 80% | 🟢 P3 |
| Utils | 0% | 50% | 🟡 P2 |
| **总计** | **0.5%** | **65%** | - |

---

## 🎯 测试策略

### 第一阶段：核心模块单元测试 (P0)

#### 1. Context 测试 (context_test.go)

**覆盖目标: 80%**

```go
// 需要测试的功能：
✅ 基本创建和克隆
✅ 数据存取 (Set/Get)
✅ 超时控制 (WithTimeout)
✅ 取消机制 (WithCancel)
✅ 日志绑定
✅ 并发安全
✅ Clone 的独立性
```

**测试用例:**
1. `TestNewContext` - 创建 Context
2. `TestContextDataOperations` - 数据存取
3. `TestContextClone` - 克隆功能
4. `TestContextTimeout` - 超时控制
5. `TestContextCancel` - 取消机制
6. `TestContextConcurrency` - 并发安全
7. `TestContextLogger` - 日志功能

#### 2. Logger 扩展测试 (logger_extended_test.go)

**覆盖目标: 70%**

```go
// 需要测试的功能：
✅ 异步批量写入
✅ 多端点隔离
✅ 日志轮转
✅ 格式化器
✅ 钩子机制
✅ 日志级别
✅ 错误处理
```

**测试用例:**
1. `TestLoggerAsync` - 异步日志
2. `TestLoggerBatch` - 批量写入
3. `TestLoggerEndpoints` - 端点隔离
4. `TestLoggerRotation` - 日志轮转
5. `TestLoggerFormatter` - 格式化
6. `TestLoggerHooks` - 钩子机制
7. `TestLoggerLevels` - 日志级别

---

### 第二阶段：存储和服务测试 (P1)

#### 3. Storage 扩展测试 (storage_extended_test.go)

**覆盖目标: 70%**

```go
// 需要测试的功能：
✅ MySQL 连接管理
✅ Redis 连接管理
✅ ES 连接管理
✅ 事务处理
✅ 健康检查
✅ 连接池
✅ 错误处理
```

**测试用例:**
1. `TestMysqlOperations` - MySQL 操作
2. `TestRedisOperations` - Redis 操作
3. `TestESOperations` - ES 操作
4. `TestTransaction` - 事务
5. `TestHealthCheck` - 健康检查
6. `TestConnectionPool` - 连接池
7. `TestStorageErrors` - 错误处理

#### 4. Server 测试 (server_test.go)

**覆盖目标: 60%**

```go
// 需要测试的功能：
✅ HTTP 服务启动/关闭
✅ Cron 任务调度
✅ RocketMQ 消息处理
✅ 路由注册
✅ 中间件
✅ 错误恢复
```

**测试用例:**
1. `TestGinServerLifecycle` - HTTP 生命周期
2. `TestCronScheduling` - Cron 调度
3. `TestRocketConsumer` - 消息消费
4. `TestRouteRegistration` - 路由注册
5. `TestMiddleware` - 中间件
6. `TestServerRecovery` - 错误恢复

---

### 第三阶段：集成和工具测试 (P2)

#### 5. 集成测试 (integration_test.go)

**覆盖目标: 50%**

```go
// 需要测试的场景：
✅ 完整请求链路
✅ 多服务协同
✅ 日志追踪
✅ 错误传播
✅ 优雅关闭
```

**测试用例:**
1. `TestFullRequestFlow` - 完整流程
2. `TestMultiServerCoordination` - 多服务
3. `TestTracing` - 链路追踪
4. `TestErrorPropagation` - 错误传播
5. `TestGracefulShutdown` - 优雅关闭

#### 6. Utils 测试 (utils_test.go)

**覆盖目标: 50%**

```go
// 需要测试的功能：
✅ SafeGo
✅ RecoverWithFunc
✅ ExecuteWithRetry
✅ TruncateString
✅ CleanPath
```

**测试用例:**
1. `TestSafeGo` - 安全 goroutine
2. `TestRecover` - 错误恢复
3. `TestRetry` - 重试机制
4. `TestStringUtils` - 字符串工具
5. `TestPathUtils` - 路径工具

---

## 📝 测试实施步骤

### Step 1: 创建 Context 测试 (最重要)

```bash
# 创建 context_extended_test.go
# 目标: 覆盖率从 0% -> 80%
```

### Step 2: 扩展 Logger 测试

```bash
# 扩展 logger_test.go
# 目标: 覆盖率从 5% -> 70%
```

### Step 3: 完善 Storage 测试

```bash
# 扩展 storage_test.go
# 目标: 覆盖率从 10% -> 70%
```

### Step 4: 添加集成测试

```bash
# 创建 integration_extended_test.go
# 目标: 覆盖率从 0% -> 50%
```

### Step 5: 工具函数测试

```bash
# 创建 utils_test.go
# 目标: 覆盖率从 0% -> 50%
```

---

## 🔧 测试工具和最佳实践

### 1. 使用 testify

```go
import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/suite"
)
```

### 2. 表格驱动测试

```go
func TestSomething(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {"case1", "input1", "expected1", false},
        {"case2", "input2", "expected2", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := DoSomething(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expected, result)
            }
        })
    }
}
```

### 3. Mock 和 Stub

```go
// 使用接口进行 mock
type MockStorage struct {
    mock.Mock
}

func (m *MockStorage) GetDB(name string) (*gorm.DB, error) {
    args := m.Called(name)
    return args.Get(0).(*gorm.DB), args.Error(1)
}
```

### 4. 并发测试

```go
func TestConcurrency(t *testing.T) {
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            // 测试并发操作
        }()
    }
    wg.Wait()
}
```

### 5. 基准测试

```go
func BenchmarkSomething(b *testing.B) {
    for i := 0; i < b.N; i++ {
        DoSomething()
    }
}
```

---

## 📈 覆盖率监控

### 生成覆盖率报告

```bash
# 运行测试并生成覆盖率
go test -coverprofile=coverage.out ./...

# 查看覆盖率
go tool cover -func=coverage.out

# 生成 HTML 报告
go tool cover -html=coverage.out -o coverage.html

# 按包查看
go test -coverprofile=coverage.out -coverpkg=./... ./...
```

### 持续监控

```bash
# 设置覆盖率目标
go test -cover -coverprofile=coverage.out ./... && \
go tool cover -func=coverage.out | grep total | \
awk '{if ($3+0 < 65) exit 1}'
```

---

## 🎯 里程碑

### 里程碑 1 (1周内)
- ✅ 创建 Context 完整测试
- ✅ 扩展 Logger 测试
- 🎯 目标覆盖率: 20%

### 里程碑 2 (2周内)
- ✅ 完善 Storage 测试
- ✅ 添加 Server 测试
- 🎯 目标覆盖率: 40%

### 里程碑 3 (3周内)
- ✅ 创建集成测试
- ✅ 添加 Utils 测试
- 🎯 目标覆盖率: 65%

---

## 💡 注意事项

1. **优先测试核心模块** - Context, Logger, Storage
2. **重点测试边界条件** - 空值、并发、超时
3. **测试错误处理** - 所有错误路径
4. **测试并发安全** - 数据竞争检测
5. **保持测试独立** - 不依赖外部服务
6. **使用 Mock** - 模拟外部依赖
7. **测试可读性** - 清晰的测试名称
8. **持续改进** - 定期review覆盖率

---

**让我们开始提升测试覆盖率！从 0.5% 到 65%！** 🚀


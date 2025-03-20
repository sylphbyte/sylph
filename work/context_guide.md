# Sylph 上下文操作指南

Sylph框架提供了强大的上下文功能，扩展了Go标准库中的`context`包。本指南详细介绍如何使用这些功能。

## 目录

- [基本概念](#基本概念)
- [核心接口](#核心接口)
- [DefaultContext方法](#defaultcontext方法)
- [包级函数](#包级函数)
- [最佳实践](#最佳实践)
- [高级用法](#高级用法)
- [性能优化](#性能优化)

## 基本概念

Sylph框架中的上下文系统围绕着`Context`接口展开，它继承了标准库的`context.Context`并扩展了日志、数据存储、请求头等功能。主要实现是`DefaultContext`结构体。

上下文在Sylph中主要用于：
- 请求链路追踪
- 传递请求相关信息
- 超时和取消控制
- 键值存储
- 日志记录

## 核心接口

`Context`接口继承了标准库的`context.Context`，并添加了以下主要功能：

```go
type Context interface {
    context.Context              // 标准库context接口
    LogContext                   // 日志功能
    DataContext                  // 数据存储功能
    TakeHeader() IHeader         // 获取请求头
    StoreHeader(header IHeader)  // 设置请求头
    WithMark(marks ...string)    // 添加标记
    TakeMarks() map[string]any   // 获取标记信息
    Clone() Context              // 创建上下文副本
    // 其他方法...
}
```

## DefaultContext方法

`DefaultContext`结构体提供了与标准库`context`相似的方法，但返回Sylph的`Context`类型：

### 基本方法

| 方法 | 说明 | 返回值 |
|------|------|--------|
| `WithTimeout(duration)` | 创建带超时的上下文 | `(Context, context.CancelFunc)` |
| `WithDeadline(time)` | 创建带截止时间的上下文 | `(Context, context.CancelFunc)` |
| `WithCancel()` | 创建可取消的上下文 | `(Context, context.CancelFunc)` |
| `WithCancelCause()` | 创建带取消原因的上下文 | `(Context, context.CancelCauseFunc)` |
| `WithValue(key, val)` | 创建带键值对的上下文 | `Context` |
| `WithMark(marks...)` | 为上下文添加标记 | 无返回值，就地修改 |
| `Cause()` | 获取上下文取消原因 | `error` |

### 使用示例

```go
// 创建超时上下文
ctx := NewDefaultContext("my-service", "/api/users")
timeoutCtx, cancel := ctx.WithTimeout(5 * time.Second)
defer cancel() // 记得释放资源

// 添加请求标记
ctx.WithMark("high-priority", "user-request")

// 添加值
userCtx := ctx.WithValue("user_id", "12345")

// 检查超时
select {
case <-timeoutCtx.Done():
    if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
        // 处理超时
    }
case <-someOperation():
    // 操作完成
}
```

## 包级函数

Sylph在包级别提供了一系列与标准库相似的函数，用于操作上下文：

| 函数 | 说明 | 对应标准库函数 |
|------|------|----------------|
| `Background()` | 创建背景上下文 | `context.Background()` |
| `TODO()` | 创建TODO上下文 | `context.TODO()` |
| `FromStdContext(ctx)` | 将标准库上下文转换为Sylph上下文 | 无 |
| `WithDeadline(parent, time)` | 创建带截止时间的上下文 | `context.WithDeadline()` |
| `WithTimeout(parent, duration)` | 创建带超时的上下文 | `context.WithTimeout()` |
| `WithCancel(parent)` | 创建可取消的上下文 | `context.WithCancel()` |
| `WithCancelCause(parent)` | 创建带取消原因的上下文 | `context.WithCancelCause()` |
| `WithValue(parent, key, val)` | 创建带键值对的上下文 | `context.WithValue()` |
| `Cause(ctx)` | 获取上下文取消原因 | `context.Cause()` |
| `WithMark(parent, marks...)` | 为上下文添加标记 | 无 |

### 包级函数使用示例

```go
// 创建基础上下文
ctx := sylph.Background()

// 带取消功能的上下文
cancelCtx, cancel := sylph.WithCancel(ctx)
defer cancel()

// 添加值和标记
valCtx := sylph.WithValue(cancelCtx, "transaction_id", "tx123")
markedCtx := sylph.WithMark(valCtx, "database-operation")

// 从标准库上下文转换
stdCtx := context.Background()
sylphCtx := sylph.FromStdContext(stdCtx)
```

## 最佳实践

### 1. 合理使用取消函数

始终使用`defer cancel()`确保资源释放：

```go
ctx, cancel := sylph.WithTimeout(parentCtx, 30*time.Second)
defer cancel()
```

### 2. 使用标记区分上下文

标记可以帮助识别不同的请求流程：

```go
// 对请求添加路由信息标记
ctx.WithMark("route:/users/create", "method:POST")
```

### 3. 使用Clone而非直接修改

需要修改上下文时，优先使用`Clone`创建副本：

```go
// 正确方式
newCtx := ctx.Clone()
newCtx.WithMark("priority:high")

// 危险方式（会影响原上下文）
ctx.WithMark("priority:high") // 可能影响其他使用ctx的代码
```

### 4. 避免存储过大的值

上下文不适合存储大型数据：

```go
// 不推荐
ctx.Set("user_data", hugeUserDataObject)

// 推荐
ctx.Set("user_id", userId)
// 然后在需要时获取完整数据
```

## 高级用法

### 取消原因传递

带原因的取消可以提供更多信息：

```go
ctx, cancel := ctx.WithCancelCause()

// 在其他goroutine中取消
go func() {
    time.Sleep(time.Second)
    cancel(errors.New("用户请求中断"))
}()

// 检查取消原因
<-ctx.Done()
err := ctx.Cause()
fmt.Println("操作被取消，原因:", err)
```

### 上下文链路追踪

结合标记和日志功能进行链路追踪：

```go
ctx.WithMark("trace_id", generateTraceId())
ctx.Debug("user.service", "处理用户请求", map[string]interface{}{
    "user_id": userId,
    "action": "login",
})
```

## 性能优化

1. **对象池复用**：`DefaultContext`使用对象池减少GC压力
2. **延迟初始化**：某些字段如事件系统采用懒加载
3. **预分配**：内部map采用预分配容量减少扩容
4. **原子操作**：关键字段使用原子操作避免锁竞争
5. **类型检查**：优先处理已知类型，避免多余转换

```go
// 池化上下文使用完毕后释放
defer func(dctx *DefaultContext) {
    dctx.Release()
}(ctx.(*DefaultContext))
```

---

通过本指南，您应该能够充分理解和利用Sylph框架的上下文功能，实现高效的请求处理和资源管理。 
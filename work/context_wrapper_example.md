# Context 包装函数使用示例

## 背景

为了实现对 Go 标准库 `context.Context` 的无缝替代，我们提供了一系列全局函数，用于创建和操作自定义 `Context` 类型。这些函数与标准库类似，但返回我们的自定义 `Context` 类型。

## 函数列表

| 自定义函数 | 对应标准库函数 | 说明 |
|------------|----------------|------|
| `Background()` | `context.Background()` | 创建一个空的背景上下文 |
| `TODO()` | `context.TODO()` | 创建一个TODO上下文 |
| `FromStdContext(ctx)` | - | 将标准库上下文转换为自定义上下文 |
| `CancelContext(parent)` | `context.WithCancel()` | 创建可取消的上下文 |
| `CauseContext(parent)` | `context.WithCancelCause()` | 创建带原因的可取消上下文 |
| `DeadlineContext(parent, time)` | `context.WithDeadline()` | 创建有截止时间的上下文 |
| `TimeoutContext(parent, duration)` | `context.WithTimeout()` | 创建有超时时间的上下文 |
| `ValueContext(parent, key, val)` | `context.WithValue()` | 创建带键值对的上下文 |
| `GetCause(ctx)` | `context.Cause()` | 获取上下文取消原因 |

## 使用示例

### 创建基础上下文

```go
// 创建背景上下文
ctx := sylph.Background()

// 创建TODO上下文
todoCtx := sylph.TODO()

// 从标准库上下文转换
stdCtx := context.Background()
customCtx := sylph.FromStdContext(stdCtx)
```

### 创建可取消的上下文

```go
// 使用DefaultContext的方法
ctx := sylph.NewDefaultContext("my-service", "request-path")
cancelCtx, cancel := ctx.WithTimeout(5 * time.Second)
defer cancel()

// 使用全局函数
ctx := sylph.Background()
cancelCtx, cancel := sylph.CancelContext(ctx)
defer cancel()

// 带超时的上下文
timeoutCtx, cancel := sylph.TimeoutContext(ctx, 10 * time.Second)
defer cancel()

// 带截止时间的上下文
deadline := time.Now().Add(1 * time.Hour)
deadlineCtx, cancel := sylph.DeadlineContext(ctx, deadline)
defer cancel()
```

### 带键值对的上下文

```go
// 添加值
ctx := sylph.Background()
ctx = sylph.ValueContext(ctx, "user_id", "12345")
ctx = sylph.ValueContext(ctx, "request_id", "abc-123")

// 使用DefaultContext的数据存储功能
ctx.Set("key1", "value1")
val, ok := ctx.Get("key1")
```

### 带取消原因的上下文

```go
ctx := sylph.Background()
ctx, cancel := sylph.CauseContext(ctx)

// 带原因取消
cancel(errors.New("operation timed out"))

// 获取取消原因
err := sylph.GetCause(ctx)
if err != nil {
    fmt.Printf("Context canceled because: %v\n", err)
}
```

## 与标准库互操作

自定义 `Context` 类型实现了标准库的 `context.Context` 接口，因此可以无缝传递给需要标准库上下文的函数：

```go
// 我们的自定义上下文
ctx := sylph.Background()

// 传递给标准库函数
req, err := http.NewRequestWithContext(ctx, "GET", "https://example.com", nil)

// 从标准库上下文转回自定义上下文
returnedCtx := sylph.FromStdContext(req.Context())
```

## 结论

通过这套上下文包装函数，我们可以在项目中使用自定义 `Context` 类型，同时保持与标准库 `context` 包的兼容性。这种方式为我们提供了更多的灵活性和功能扩展性，同时减少了类型转换的麻烦。 
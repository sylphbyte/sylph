# DefaultContext 方法使用示例

## 背景

`DefaultContext` 是我们框架中的核心上下文实现，它直接实现了 `Context` 接口，并提供了完整的上下文功能。现在我们为 `DefaultContext` 添加了与标准库 `context` 类似的方法，使其更易于使用。

## 新增方法列表

| 方法名 | 对应标准库函数 | 说明 |
|------------|----------------|------|
| `WithTimeout(duration)` | `context.WithTimeout()` | 创建有超时时间的上下文 |
| `WithDeadline(time)` | `context.WithDeadline()` | 创建有截止时间的上下文 |
| `WithValue(key, val)` | `context.WithValue()` | 创建带键值对的上下文 |
| `WithCancel()` | `context.WithCancel()` | 创建可取消的上下文 |
| `WithCancelCause()` | `context.WithCancelCause()` | 创建带取消原因的上下文 |
| `WithMark(marks...)` | - | 为上下文添加标记 |

## 使用示例

### 创建上下文

```go
// 创建基础上下文
ctx := sylph.NewDefaultContext("my-service", "request-path")
```

### 添加超时控制

```go
// 添加超时控制
timeoutCtx, cancel := ctx.WithTimeout(5 * time.Second)
defer cancel() // 记得取消以释放资源

// 使用
select {
case <-timeoutCtx.Done():
    if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
        fmt.Println("操作超时")
    }
case <-doSomething():
    fmt.Println("操作成功")
}
```

### 添加截止时间

```go
// 设置截止时间为10分钟后
deadline := time.Now().Add(10 * time.Minute)
deadlineCtx, cancel := ctx.WithDeadline(deadline)
defer cancel()

// 检查是否已过期
if deadline, ok := deadlineCtx.Deadline(); ok {
    fmt.Printf("截止时间: %v\n", deadline)
}
```

### 添加键值数据

```go
// 方式1: 使用WithValue方法 (类似标准库)
userCtx := ctx.WithValue("user_id", "12345")
requestCtx := userCtx.WithValue("request_id", "req-6789")

// 方式2: 使用Set方法 (更直接)
ctx.Set("user_id", "12345")
ctx.Set("request_id", "req-6789")

// 读取数据
if userId, ok := ctx.Value("user_id").(string); ok {
    fmt.Printf("用户ID: %s\n", userId)
}

// 也可以使用Get方法
if val, ok := ctx.Get("user_id"); ok {
    userId := val.(string)
    fmt.Printf("用户ID: %s\n", userId)
}
```

### 添加可取消控制

```go
// 创建可取消的上下文
cancelCtx, cancel := ctx.WithCancel()
defer cancel()

// 在其他goroutine中根据条件取消
go func() {
    time.Sleep(2 * time.Second)
    cancel() // 触发取消
}()

// 监听取消信号
select {
case <-cancelCtx.Done():
    fmt.Println("操作被取消")
    return
case <-doSomething():
    fmt.Println("操作成功完成")
}
```

### 带取消原因的上下文

```go
// 创建带取消原因的上下文
causeCtx, cancelWithCause := ctx.WithCancelCause()

// 在其他goroutine中取消并提供原因
go func() {
    time.Sleep(1 * time.Second)
    cancelWithCause(errors.New("操作被用户中断"))
}()

// 监听取消
select {
case <-causeCtx.Done():
    cause := context.Cause(causeCtx)
    fmt.Printf("操作被取消，原因: %v\n", cause)
    return
case <-doSomething():
    fmt.Println("操作成功完成")
}
```

### 添加标记

```go
// 使用WithMark添加标记
ctx.WithMark("important", "high-priority")

// 获取所有标记
marks := ctx.TakeMarks()
fmt.Printf("上下文标记: %v\n", marks)
```

## 最佳实践

1. **始终调用cancel函数**: 对于返回cancel函数的方法，确保在合适的时机调用它，通常使用defer
2. **传递上下文**: 将上下文传递给所有需要的函数，保持调用链的连贯性
3. **合理使用超时**: 为所有网络请求和IO操作设置合适的超时
4. **避免存储指针**: 在上下文中存储值而非指针，防止内存泄漏
5. **不修改原上下文**: 使用WithXXX方法创建新上下文，不要修改传入的上下文

## 与标准库context的区别

虽然API设计类似，但我们的`DefaultContext`有以下增强功能:

1. 额外的数据存储能力：通过`Get`/`Set`方法
2. 内置日志记录能力：通过`Info`, `Error`等方法
3. 请求头信息管理：通过`TakeHeader`和`StoreHeader`
4. 标记系统：通过`WithMark`和`TakeMarks`
5. 通知功能：通过`SendError`等方法
6. 对象池优化：更高效的内存使用 
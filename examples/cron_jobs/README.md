# 定时任务示例

这个示例展示了如何使用Sylph框架创建和管理定时任务。

## 功能特点

- 使用不同时间表达式的定时任务
- 任务执行过程中的日志记录
- 事件系统集成
- 动态更新任务执行频率
- 错误处理和告警机制
- HTTP接口查看任务执行结果

## 定时任务一览

1. **用户活跃度统计** - 每分钟执行一次，统计活跃用户数量
2. **订单统计** - 每5分钟执行一次，统计订单数据和收入
3. **数据归档** - 每天凌晨1点执行，执行数据归档操作 
4. **系统异常监控** - 每15分钟执行一次，检测系统错误率并在必要时触发告警
5. **配置刷新** - 动态频率(10秒或30秒)，演示如何动态更新任务执行频率

## 运行示例

确保已安装所有依赖：

```bash
go mod tidy
```

然后运行示例：

```bash
go run main.go
```

服务将在 http://localhost:8080 上启动。

## API测试

### 查看统计数据

```bash
curl http://localhost:8080/api/stats
```

响应示例：

```json
{
  "data": {
    "total_users": 1000,
    "active_users": 673,
    "new_users": 45,
    "total_orders": 450,
    "revenue": 28680.50,
    "last_updated_at": "2023-12-01T15:30:45Z"
  },
  "meta": {
    "updated_at": "2023-12-01T15:30:45Z"
  }
}
```

## 核心代码解析

### 创建定时任务服务器

```go
// 创建定时任务服务器
cronServer := sylph.NewCronServer()
```

### 添加定时任务

```go
// 添加每分钟执行一次的任务
cronServer.AddJob("0 * * * * *", "用户活跃度统计", func(ctx sylph.Context) {
    // 任务逻辑...
})
```

### 动态更新任务执行频率

```go
// 更新Cron表达式
if err := cronServer.UpdateJobCron(jobName, newCronExpr); err != nil {
    ctx.Error("cron.config", "更新任务执行频率失败", err, nil)
} else {
    cronExpr = newCronExpr
}
```

### 错误处理和事件发送

```go
if errorRate > 0.8 {
    // 模拟发现高错误率的情况
    err := fmt.Errorf("系统错误率过高: %.2f%%", errorRate*100)
    ctx.Error("cron.monitor", "系统异常监控警报", err, sylph.H{
        "error_rate": errorRate,
        "threshold":  0.8,
    })
    
    // 发送告警事件
    ctx.Emit("system.alert", sylph.H{
        "level":       "critical",
        "error_rate":  errorRate,
        "description": "系统错误率超过阈值",
    })
}
```

## Cron表达式说明

Sylph框架使用标准的Cron表达式格式，但额外支持秒级精度：

```
秒 分 时 日 月 周
```

例如：

- `0 * * * * *` - 每分钟的第0秒执行
- `0 */5 * * * *` - 每5分钟执行一次
- `0 0 1 * * *` - 每天凌晨1点执行
- `*/30 * * * * *` - 每30秒执行一次

## 最佳实践

1. **合理设置执行频率**：避免过于频繁的任务执行，以免系统负载过高
2. **任务隔离**：将不同类型的任务分开处理，避免单个任务阻塞其他任务
3. **错误处理**：为每个任务添加适当的错误处理机制
4. **日志记录**：记录任务执行的开始、结束和关键过程
5. **事件通知**：使用事件系统通知其他组件任务执行状态
6. **幂等性**：确保任务可以安全地重复执行，特别是在分布式环境中 
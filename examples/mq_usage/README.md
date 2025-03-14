# RocketMQ 集成示例

这个示例展示了如何使用Sylph框架集成RocketMQ进行消息的生产和消费。

## 功能特点

- RocketMQ生产者和消费者配置
- 消息发送和接收
- 事件系统集成
- 结构化日志记录
- 基于HTTP的API接口
- 优雅启动和关闭

## 前置条件

运行此示例需要一个可用的RocketMQ服务器。可以通过以下方式部署：

### 使用Docker运行RocketMQ

```bash
# 拉取并运行RocketMQ服务
docker run -d --name rmqnamesrv -p 9876:9876 apache/rocketmq:5.1.0 sh mqnamesrv
docker run -d --name rmqbroker -p 10909:10909 -p 10911:10911 --link rmqnamesrv:namesrv -e "NAMESRV_ADDR=namesrv:9876" apache/rocketmq:5.1.0 sh mqbroker
```

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

### 提交订单（生产消息）

```bash
curl -X POST http://localhost:8080/api/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user_123",
    "product_id": "product_456",
    "quantity": 2,
    "total_price": 199.99
  }'
```

成功后将返回：

```json
{
  "message": "订单已提交处理",
  "order_id": "ORD-20231201-123045-123"
}
```

提交后，服务会自动发送消息到RocketMQ，而注册的消费者会消费该消息并进行处理。

## 核心代码解析

### 创建RocketMQ服务器和生产者/消费者

```go
// 创建MQ服务器（RocketMQ）
mqServer := sylph.NewRocketMQServer(&sylph.RocketMQConfig{
    NameServer:   []string{"127.0.0.1:9876"}, // RocketMQ Name Server地址
    ClientIP:     "127.0.0.1",
    InstanceName: "sylph_example",
})

// 注册一个消息生产者
orderProducer := mqServer.CreateProducer(&sylph.ProducerConfig{
    Group:         "OrderProducerGroup",
    Topic:         "OrderTopic",
    Tag:           "order",
    RetryTimes:    3,
    SendTimeoutMS: 3000,
})

// 注册一个消息消费者
orderConsumer := mqServer.CreateConsumer(&sylph.ConsumerConfig{
    Group:         "OrderConsumerGroup",
    Topic:         "OrderTopic",
    Tag:           "order",
    InstanceName:  "order_consumer",
    ConsumerModel: sylph.ConsumerModelClustering,
    FromWhere:     sylph.ConsumeFromLastOffset,
})
```

### 注册消息处理函数

```go
// 注册消息处理函数
orderConsumer.RegisterHandler(func(ctx sylph.Context, msg []byte) error {
    ctx.Info("mq.consumer.order", "收到新订单消息", sylph.H{
        "msg_size": len(msg),
        "raw_data": string(msg),
    })

    // 解析消息
    var order OrderMessage
    if err := ctx.Json().Unmarshal(msg, &order); err != nil {
        return err
    }

    // 处理订单逻辑
    ctx.Info("mq.consumer.order", "订单处理完成", sylph.H{
        "order_id": order.OrderID,
    })

    // 触发订单处理完成事件
    ctx.Emit("order.processed", sylph.H{
        "order_id": order.OrderID,
        "status":   "processed",
    })

    return nil
})
```

### 发送消息到RocketMQ

```go
// 将订单转换为JSON
orderJSON, err := ctx.Json().Marshal(order)
if err != nil {
    // 错误处理...
}

// 通过RocketMQ发送消息
err = orderProducer.SendMessage(ctx, orderJSON)
if err != nil {
    // 错误处理...
}
```

## 最佳实践

1. **消息结构定义**：为每种消息类型定义清晰的结构体
2. **错误处理**：在消息处理过程中妥善处理错误
3. **消息幂等性**：确保消息处理具有幂等性，避免重复处理
4. **上下文传递**：在消息处理中正确使用Sylph上下文
5. **结构化日志**：记录详细的日志信息，便于问题排查
6. **事件驱动**：通过事件系统通知其他组件消息处理状态 
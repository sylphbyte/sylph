package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sylphbyte/sylph"
)

// 订单消息结构
type OrderMessage struct {
	OrderID    string    `json:"order_id"`
	UserID     string    `json:"user_id"`
	ProductID  string    `json:"product_id"`
	Quantity   int       `json:"quantity"`
	TotalPrice float64   `json:"total_price"`
	CreateTime time.Time `json:"create_time"`
}

func main() {
	// 创建项目管理器
	project := sylph.NewProject()

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

	// 注册消息处理函数
	orderConsumer.RegisterHandler(func(ctx sylph.Context, msg []byte) error {
		ctx.Info("mq.consumer.order", "收到新订单消息", sylph.H{
			"msg_size": len(msg),
			"raw_data": string(msg),
		})

		// 解析消息（这里简化处理，实际应用中需要进行JSON解析）
		var order OrderMessage
		if err := ctx.Json().Unmarshal(msg, &order); err != nil {
			ctx.Error("mq.consumer.order", "解析订单消息失败", err, sylph.H{
				"raw_data": string(msg),
			})
			return err
		}

		// 处理订单逻辑
		ctx.Info("mq.consumer.order", "订单处理完成", sylph.H{
			"order_id":    order.OrderID,
			"user_id":     order.UserID,
			"product_id":  order.ProductID,
			"quantity":    order.Quantity,
			"total_price": order.TotalPrice,
		})

		// 触发订单处理完成事件
		ctx.Emit("order.processed", sylph.H{
			"order_id": order.OrderID,
			"status":   "processed",
		})

		return nil
	})

	// 创建HTTP服务作为API入口
	httpServer := sylph.NewGinServer(":8080")

	// 注册路由
	httpServer.POST("/api/orders", func(c *sylph.GinContext) {
		ctx := sylph.FromGin(c.Context)

		// 接收订单数据
		var order OrderMessage
		if err := c.BindJSON(&order); err != nil {
			ctx.Error("api.orders", "请求数据无效", err, nil)
			c.JSON(400, sylph.H{"error": "无效的请求数据"})
			return
		}

		// 设置订单ID和创建时间
		order.OrderID = "ORD-" + time.Now().Format("20060102-150405-999")
		order.CreateTime = time.Now()

		// 将订单转换为JSON
		orderJSON, err := ctx.Json().Marshal(order)
		if err != nil {
			ctx.Error("api.orders", "序列化订单数据失败", err, sylph.H{
				"order": order,
			})
			c.JSON(500, sylph.H{"error": "内部服务器错误"})
			return
		}

		// 发送订单消息到MQ
		ctx.Info("api.orders", "开始发送订单消息", sylph.H{
			"order_id":    order.OrderID,
			"total_price": order.TotalPrice,
		})

		// 通过RocketMQ发送消息
		err = orderProducer.SendMessage(ctx, orderJSON)
		if err != nil {
			ctx.Error("api.orders", "发送订单消息失败", err, sylph.H{
				"order_id": order.OrderID,
			})
			c.JSON(500, sylph.H{"error": "消息发送失败"})
			return
		}

		ctx.Info("api.orders", "订单消息发送成功", sylph.H{
			"order_id": order.OrderID,
		})

		// 返回成功响应
		c.JSON(200, sylph.H{
			"message":  "订单已提交处理",
			"order_id": order.OrderID,
		})
	})

	// 添加健康检查接口
	httpServer.GET("/health", func(c *sylph.GinContext) {
		c.JSON(200, sylph.H{"status": "ok"})
	})

	// 挂载所有服务到项目
	project.Mounts(mqServer)
	project.Mounts(httpServer)

	// 启动项目
	if err := project.Boots(); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}

	log.Println("服务已启动，监听端口: 8080")
	log.Println("可以通过POST请求到 http://localhost:8080/api/orders 提交订单")

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务...")
	if err := project.Shutdowns(); err != nil {
		log.Fatalf("关闭服务失败: %v", err)
	}
	log.Println("服务已安全关闭")
}

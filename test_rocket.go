package sylph

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

// 加载RocketMQ配置
func loadRocketConfig(configPath string) (*RocketYaml, error) {
	// 读取YAML文件内容
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析YAML内容
	var config RocketYaml
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析YAML失败: %w", err)
	}

	return &config, nil
}

// 输出RocketMQ配置信息
func printRocketConfig(config *RocketYaml) {
	fmt.Println("===== RocketMQ 配置信息 =====")
	for name, instance := range config.RocketGroup {
		fmt.Printf("实例 [%s]:\n", name)
		fmt.Printf("  端点: %s\n", instance.Endpoint)
		fmt.Printf("  AccessKey: %s\n", maskString(instance.AccessKey))
		fmt.Printf("  SecretKey: %s\n", maskString(instance.SecretKey))

		fmt.Println("  主题列表:")
		for _, topic := range instance.Topics {
			fmt.Printf("    - %s (类型: %s, 标签: %s)\n",
				topic.Topic, topic.Kind.Name(),
				defaultIfEmpty(topic.Tags, "*"))
		}

		fmt.Println("  消费者列表:")
		for _, consumer := range instance.Consumers {
			fmt.Printf("    - 组: %s (并发数: %d, 等待: %ds)\n",
				consumer.Group, consumer.Num, consumer.Wait)
			fmt.Println("      订阅:")
			for _, sub := range consumer.Subscriptions {
				fmt.Printf("        - %s (标签: %s)\n",
					sub.Topic, defaultIfEmpty(sub.Tags, "*"))
			}
		}
	}
	fmt.Println("=============================")
}

// 发送测试消息
func sendTestMessage(producer IProducer) {
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		// 创建一个测试消息
		message := NewSendMessage(map[string]interface{}{
			"id":      "test-001",
			"content": "这是一条测试消息",
			"time":    time.Now().Format(time.RFC3339),
		})

		// 设置消息标签
		message.WithTag("test")

		// 添加自定义属性
		message.WithProperty("source", "test_program")
		message.WithProperty("version", "1.0")

		// 发送消息
		ctx := NewContext(Endpoint("test"), "test-message")
		ret := producer.Send(ctx, message)

		if err := ret.TakeError(); err != nil {
			fmt.Printf("发送消息失败: %v\n", err)
		} else {
			fmt.Println("消息发送成功!")
			receipts := ret.TakeReceipts()
			for i, receipt := range receipts {
				fmt.Printf("  回执 #%d: MessageID=%s\n", i+1, receipt.MessageID)
			}
		}
	}()

	// 等待发送完成
	wg.Wait()
}

// 等待程序终止信号
func waitForSignal() {
	fmt.Println("服务已启动，按 Ctrl+C 终止...")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

// 辅助函数：掩盖敏感字符串
func maskString(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + "****" + s[len(s)-2:]
}

// 辅助函数：默认值
func defaultIfEmpty(s, defaultValue string) string {
	if s == "" {
		return defaultValue
	}
	return s
}

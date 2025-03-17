package benchmark

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/sylphbyte/sylph"
)

// 运行所有基准测试示例
func RunAllExamples() {
	// 确保系统资源充分利用
	runtime.GOMAXPROCS(runtime.NumCPU())

	log.Println("======= 开始基准测试 =======")

	// 基本上下文操作基准测试
	RunContextBenchmark()

	// JSON处理基准测试
	RunJSONBenchmark()

	// 并发安全操作基准测试
	RunConcurrencyBenchmark()

	log.Println("======= 所有基准测试完成 =======")
}

// RunContextBenchmark 上下文操作基准测试
func RunContextBenchmark() {
	log.Println("开始上下文操作基准测试...")

	// 测试上下文创建
	createCtxResult, _ := RunBenchmark(func(ctx sylph.Context) error {
		// 基本的上下文获取和设置操作
		ctx.Set("testKey", "testValue")
		_, _ = ctx.Get("testKey")
		return nil
	}, BenchmarkOptions{
		Concurrency:      100,
		Iterations:       10000,
		WarmupIterations: 1000,
		Name:             "上下文创建与基本操作",
	})

	// 测试更复杂的上下文操作
	complexCtxResult, _ := RunBenchmark(func(ctx sylph.Context) error {
		// 模拟复杂的上下文操作场景
		ctx.Set("key1", "value1")
		ctx.Set("key2", map[string]interface{}{"nested": "value"})
		ctx.Set("key3", 42)

		_, _ = ctx.Get("key1")
		_, _ = ctx.Get("key2")
		_, _ = ctx.Get("key3")

		// 记录日志
		ctx.Debug("benchmark", "测试上下文日志", nil)

		return nil
	}, BenchmarkOptions{
		Concurrency:      100,
		Iterations:       5000,
		WarmupIterations: 500,
		Name:             "上下文复杂操作",
	})

	// 比较并打印结果
	fmt.Printf("\n上下文操作基准测试结果比较:\n")
	fmt.Printf("基本操作: %.2f RPS, 平均耗时: %v\n", createCtxResult.RPS, createCtxResult.AverageDuration)
	fmt.Printf("复杂操作: %.2f RPS, 平均耗时: %v\n", complexCtxResult.RPS, complexCtxResult.AverageDuration)
	fmt.Printf("性能差异: %.2f 倍\n\n", createCtxResult.RPS/complexCtxResult.RPS)
}

// RunJSONBenchmark JSON处理基准测试
func RunJSONBenchmark() {
	log.Println("开始JSON处理基准测试...")

	// 测试JSON序列化
	jsonSerializeResult, _ := RunBenchmark(func(ctx sylph.Context) error {
		// 模拟复杂的JSON数据
		data := map[string]interface{}{
			"id":        1234,
			"name":      "测试用户",
			"timestamp": time.Now().Unix(),
			"metadata": map[string]interface{}{
				"ip":      "127.0.0.1",
				"version": "1.0.0",
				"tags":    []string{"tag1", "tag2", "tag3"},
			},
			"isActive": true,
			"score":    98.6,
		}

		// 将数据保存到上下文
		ctx.Set("jsonData", data)

		return nil
	}, BenchmarkOptions{
		Concurrency:      50,
		Iterations:       5000,
		WarmupIterations: 500,
		Name:             "JSON数据处理",
	})

	fmt.Printf("\nJSON处理基准测试结果:\n")
	fmt.Printf("JSON操作: %.2f RPS, 平均耗时: %v\n", jsonSerializeResult.RPS, jsonSerializeResult.AverageDuration)
	fmt.Printf("P99耗时: %v\n\n", jsonSerializeResult.P99)
}

// RunConcurrencyBenchmark 并发安全操作基准测试
func RunConcurrencyBenchmark() {
	log.Println("开始并发安全操作基准测试...")

	// 测试SafeGo函数的性能
	safeGoResult, _ := RunBenchmark(func(ctx sylph.Context) error {
		// 使用SafeGo启动协程
		done := make(chan struct{})

		sylph.SafeGo(ctx, "benchmark.SafeGo", func() {
			// 模拟一些工作
			time.Sleep(time.Microsecond)
			close(done)
		})

		// 等待协程完成
		<-done
		return nil
	}, BenchmarkOptions{
		Concurrency:      200,
		Iterations:       1000,
		WarmupIterations: 100,
		Name:             "SafeGo函数性能",
	})

	fmt.Printf("\n并发安全操作基准测试结果:\n")
	fmt.Printf("SafeGo操作: %.2f RPS, 平均耗时: %v\n", safeGoResult.RPS, safeGoResult.AverageDuration)
	fmt.Printf("P99耗时: %v\n\n", safeGoResult.P99)
}

// RunRocketMQBenchmark RocketMQ操作基准测试
// 注意：此函数需要RocketMQ服务才能运行
func RunRocketMQBenchmark(instance sylph.RocketInstance, topic sylph.RocketTopic) {
	log.Println("开始RocketMQ操作基准测试...")

	// 创建生产者
	producer := sylph.NewRocketProducer(topic, instance)
	if err := producer.Boot(); err != nil {
		log.Printf("无法启动RocketMQ生产者: %v\n", err)
		return
	}

	// 测试消息发送
	sendResult, _ := RunBenchmark(func(ctx sylph.Context) error {
		// 创建测试消息
		message := sylph.NewSendMessage(map[string]interface{}{
			"id":        time.Now().UnixNano(),
			"content":   "测试消息",
			"timestamp": time.Now().Unix(),
		})

		// 设置标签
		message.WithTag(sylph.Tag("test"))

		// 发送消息
		ret := producer.Send(ctx, message)
		if ret.TakeError() != nil {
			return ret.TakeError()
		}

		return nil
	}, BenchmarkOptions{
		Concurrency:      10,  // RocketMQ通常不需要太高并发
		Iterations:       100, // 发送100条消息
		WarmupIterations: 10,
		Name:             "RocketMQ消息发送",
	})

	fmt.Printf("\nRocketMQ操作基准测试结果:\n")
	fmt.Printf("消息发送: %.2f RPS, 平均耗时: %v\n", sendResult.RPS, sendResult.AverageDuration)
	fmt.Printf("P99耗时: %v\n\n", sendResult.P99)
}

package sylph

import (
	"sync"
)

// event 独立事件系统
// 基于发布/订阅模式实现的事件处理系统，支持同步和异步事件触发
type event struct {
	subscribers sync.Map   // 事件订阅者映射表，键为事件名称，值为处理函数切片
	mu          sync.Mutex // 用于保护订阅操作的互斥锁
}

// EventHandler 定义事件处理函数类型
// 接收上下文对象和事件载荷作为参数
type EventHandler func(ctx Context, payload interface{})

// newEvent 创建新的事件系统
// 返回初始化的事件系统实例
func newEvent() *event {
	return &event{
		subscribers: sync.Map{},
	}
}

// On 订阅事件
// 将处理函数添加到指定事件的订阅列表中
// 参数:
//   - eventName: 事件名称
//   - handlers: 一个或多个事件处理函数
func (es *event) On(eventName string, handlers ...EventHandler) {
	es.mu.Lock()
	defer es.mu.Unlock()

	stored, _ := es.subscribers.LoadOrStore(eventName, []EventHandler{})
	if storedHandlers, ok := stored.([]EventHandler); ok {
		updated := append(storedHandlers, handlers...)
		es.subscribers.Store(eventName, updated)
	} else {
		// 处理意外类型（理论上不应发生）
		es.subscribers.Store(eventName, handlers)
	}
}

// Off 取消订阅特定事件
// 从订阅映射中移除指定事件的所有处理函数
// 参数:
//   - eventName: 要取消订阅的事件名称
func (es *event) Off(eventName string) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if _, ok := es.subscribers.Load(eventName); ok {
		es.subscribers.Delete(eventName)
	}
}

// Emit 同步触发事件
// 同步调用所有订阅了指定事件的处理函数
// 参数:
//   - ctx: 上下文对象
//   - eventName: 事件名称
//   - payload: 事件载荷
func (es *event) Emit(ctx Context, eventName string, payload interface{}) {
	if handlers, ok := es.subscribers.Load(eventName); ok {
		if storedHandlers, ok := handlers.([]EventHandler); ok {
			for _, h := range storedHandlers {
				h(ctx, payload)
			}
			// wg.Wait()
		}
	}
}

// AsyncEmit 异步触发事件并等待所有处理完成
// 异步调用所有订阅了指定事件的处理函数，并使用WaitGroup等待全部完成
// 参数:
//   - ctx: 上下文对象
//   - eventName: 事件名称
//   - payload: 事件载荷
func (es *event) AsyncEmit(ctx Context, eventName string, payload interface{}) {
	if handlers, ok := es.subscribers.Load(eventName); ok {
		if storedHandlers, ok := handlers.([]EventHandler); ok {
			var wg sync.WaitGroup
			for _, h := range storedHandlers {
				wg.Add(1)

				// 使用SafeGo启动goroutine
				handler := h // 创建副本避免闭包问题
				SafeGo(ctx, "x.event.AsyncEmit", func() {
					defer wg.Done()
					handler(ctx, payload)
				})
			}

			// 等待所有事件处理完成
			wg.Wait()
		}
	}
}

// AsyncEmitNoWait 异步触发事件但不等待完成
// 异步调用所有订阅了指定事件的处理函数，但不等待它们完成
// 参数:
//   - ctx: 上下文对象
//   - eventName: 事件名称
//   - payload: 事件载荷
func (es *event) AsyncEmitNoWait(ctx Context, eventName string, payload interface{}) {
	if handlers, ok := es.subscribers.Load(eventName); ok {
		if storedHandlers, ok := handlers.([]EventHandler); ok {
			for _, h := range storedHandlers {
				// 使用SafeGo启动goroutine
				handler := h // 创建副本避免闭包问题
				SafeGo(ctx, "x.event.AsyncEmitNoWait", func() {
					handler(ctx, payload)
				})
			}
		}
	}
}

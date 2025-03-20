package sylph

import (
	"context"
	"time"
)

// Background 创建一个新的背景上下文
//
// 返回:
//   - Context: 一个新的空白上下文，没有超时、取消或值
//
// 实现逻辑:
//   - 调用NewDefaultContext创建一个endpoint为"background"的新上下文
//   - 用作根上下文，为请求链的起点
func Background() Context {
	return NewContext("background", "")
}

// TODO 创建一个新的TODO上下文
//
// 返回:
//   - Context: 一个标记为"todo"的上下文，用于尚未确定上下文来源的场景
//
// 实现逻辑:
//   - 调用NewDefaultContext创建一个endpoint为"todo"的新上下文
//   - 用于表示代码中尚未确定上下文来源的位置
func TODO() Context {
	return NewContext("todo", "")
}

// FromStdContext 将标准库上下文转换为自定义上下文
//
// 参数:
//   - ctx: 标准库context.Context实例
//
// 返回:
//   - Context: 包装了标准库上下文的自定义上下文
//
// 实现逻辑:
//  1. 如果输入为nil，则返回nil
//  2. 如果输入已经是自定义Context接口类型，则直接返回
//  3. 否则创建一个新的DefaultContext，并设置其内部上下文为输入的上下文
//  4. 使标准库context可以在我们的系统中使用
func FromStdContext(ctx context.Context) Context {
	if ctx == nil {
		return nil
	}

	// 如果已经是自定义Context，直接返回
	if customCtx, ok := ctx.(Context); ok {
		return customCtx
	}

	// 创建一个新的默认上下文并替换内部context
	newCtx := NewContext("wrapper", "")
	if dc, ok := newCtx.(*DefaultContext); ok {
		dc.ctxInternal = ctx
	}

	return newCtx
}

// WithDeadline 创建带截止时间的上下文
//
// 参数:
//   - parent: 父上下文
//   - deadline: 截止时间点
//
// 返回:
//   - Context: 带截止时间的新上下文
//   - context.CancelFunc: 取消函数
//
// 实现逻辑:
//  1. 如果父上下文是DefaultContext类型，则直接调用其WithDeadline方法
//  2. 否则，使用标准库创建带截止时间的上下文，再包装为自定义上下文
//  3. 保持与标准库行为一致，但返回自定义Context类型
func WithDeadline(parent Context, deadline time.Time) (Context, context.CancelFunc) {
	if dctx, ok := parent.(*DefaultContext); ok {
		return dctx.WithDeadline(deadline)
	}

	// 回退到标准库
	stdCtx, cancel := context.WithDeadline(parent, deadline)
	return FromStdContext(stdCtx), cancel
}

// WithCancel 创建可取消的上下文
//
// 参数:
//   - parent: 父上下文
//
// 返回:
//   - Context: 可取消的新上下文
//   - context.CancelFunc: 取消函数
//
// 实现逻辑:
//  1. 如果父上下文是DefaultContext类型，则直接调用其WithCancel方法
//  2. 否则，使用标准库创建可取消上下文，再包装为自定义上下文
//  3. 保持与标准库行为一致，但返回自定义Context类型
func WithCancel(parent Context) (Context, context.CancelFunc) {
	if dctx, ok := parent.(*DefaultContext); ok {
		return dctx.WithCancel()
	}

	// 回退到标准库
	stdCtx, cancel := context.WithCancel(parent)
	return FromStdContext(stdCtx), cancel
}

// WithCancelCause 创建带取消原因的上下文
//
// 参数:
//   - parent: 父上下文
//
// 返回:
//   - Context: 带取消原因的新上下文
//   - context.CancelCauseFunc: 带原因的取消函数
//
// 实现逻辑:
//  1. 如果父上下文是DefaultContext类型，则直接调用其WithCancelCause方法
//  2. 否则，使用标准库创建带取消原因的上下文，再包装为自定义上下文
//  3. 保持与Go 1.20+标准库行为一致，但返回自定义Context类型
func WithCancelCause(parent Context) (Context, context.CancelCauseFunc) {
	if dctx, ok := parent.(*DefaultContext); ok {
		return dctx.WithCancelCause()
	}

	// 回退到标准库
	stdCtx, cancel := context.WithCancelCause(parent)
	return FromStdContext(stdCtx), cancel
}

// WithValue 创建带键值对的上下文
//
// 参数:
//   - parent: 父上下文
//   - key: 键名
//   - val: 键值
//
// 返回:
//   - Context: 带键值对的新上下文
//
// 实现逻辑:
//  1. 如果父上下文是DefaultContext类型，则直接调用其WithValue方法
//  2. 否则，使用标准库创建带键值的上下文，再包装为自定义上下文
//  3. 保持与标准库行为一致，但返回自定义Context类型
func WithValue(parent Context, key, val any) Context {
	if dctx, ok := parent.(*DefaultContext); ok {
		return dctx.WithValue(key, val)
	}

	// 回退到标准库
	stdCtx := context.WithValue(parent, key, val)
	return FromStdContext(stdCtx)
}

// Cause 获取上下文取消原因
//
// 参数:
//   - ctx: 要获取取消原因的上下文
//
// 返回:
//   - error: 上下文的取消原因，如果未取消或取消时未提供原因则为nil
//
// 实现逻辑:
//  1. 如果上下文是DefaultContext类型，则直接调用其Cause方法
//  2. 否则，调用标准库的context.Cause函数
//  3. 包装Go 1.20+的取消原因功能
func Cause(ctx Context) error {
	if dctx, ok := ctx.(*DefaultContext); ok {
		return dctx.Cause()
	}

	// 回退到标准库
	return context.Cause(ctx)
}

// WithMark 添加标记到上下文
//
// 参数:
//   - parent: 父上下文
//   - marks: 一个或多个标记字符串
//
// 返回:
//   - Context: 添加了标记的上下文（与输入相同，但已修改）
//
// 实现逻辑:
//  1. 直接调用父上下文的WithMark方法添加标记
//  2. 返回同一个上下文实例，而非创建新实例
//  3. 这样可以保持取消信号传递，避免断开与原上下文的关联
func WithMark(parent Context, marks ...string) Context {
	// 直接在原始上下文上添加标记，保持取消信号传递
	parent.WithMark(marks...)
	return parent
}

package sylph

import (
	cron "github.com/robfig/cron/v3"
	"github.com/sylphbyte/pr"
)

const (
	// CrontabNormalMode 默认normal模式
	// 允许多个相同任务并发执行
	CrontabNormalMode CrontabMode = iota + 1

	// CrontabSkipMode 跳过执行模式
	// 如果上一个任务还在执行，则跳过本次执行
	CrontabSkipMode

	// CrontabDelayMode 延迟执行模式
	// 如果上一个任务还在执行，则等待执行完成后再执行下一个
	CrontabDelayMode

	// CrontabNormalName 普通模式的名称
	CrontabNormalName CrontabModeName = "normal"

	// CrontabSkipName 跳过模式的名称
	CrontabSkipName CrontabModeName = "skip"

	// CrontabDelayName 延迟模式的名称
	CrontabDelayName CrontabModeName = "delay"
)

var (
	// crontabModeMapping 模式枚举到名称的映射
	// 用于从CrontabMode获取对应的CrontabModeName
	crontabModeMapping = map[CrontabMode]CrontabModeName{
		CrontabNormalMode: CrontabNormalName,
		CrontabSkipMode:   CrontabSkipName,
		CrontabDelayMode:  CrontabDelayName,
	}

	// crontabNameModeMapping 名称到模式枚举的映射
	// 用于从CrontabModeName获取对应的CrontabMode
	crontabNameModeMapping = map[CrontabModeName]CrontabMode{
		CrontabNormalName: CrontabNormalMode,
		CrontabSkipName:   CrontabSkipMode,
		CrontabDelayName:  CrontabDelayMode,
	}
)

// SwitchMode 模式开关配置
// 用于配置不同模式的启用状态
type SwitchMode map[CrontabModeName]bool

// CrontabModes CrontabMode的切片类型
// 用于配置多个计划任务模式
type CrontabModes []CrontabMode

// CrontabMode 计划任务执行模式枚举
// 定义了任务执行的行为方式
type CrontabMode int

// CrontabModeName 计划任务模式名称
// 模式的字符串表示形式，用于配置文件和日志
type CrontabModeName string

// String 获取模式名称的字符串表示
//
// 返回:
//   - string: 模式名称字符串
func (c CrontabModeName) String() string {
	return string(c)
}

// Mode 根据模式名称获取对应的模式枚举值
//
// 返回:
//   - CrontabMode: 模式枚举值
//
// 注意事项:
//   - 如果找不到对应的模式，将会panic
func (c CrontabModeName) Mode() CrontabMode {
	mode, ok := crontabNameModeMapping[c]
	if !ok {
		pr.Panic("not has mode name: %s", c)
	}

	return mode
}

// Valid 验证模式名称是否有效
//
// 返回:
//   - bool: 如果模式名称有效则返回true，否则返回false
func (c CrontabModeName) Valid() bool {
	_, ok := crontabNameModeMapping[c]
	return ok
}

// Name 获取模式对应的名称
//
// 返回:
//   - CrontabModeName: 模式名称
func (m CrontabMode) Name() CrontabModeName {
	return crontabModeMapping[m]
}

// CronRouteHandlers 计划任务路由处理器集合
// 存储任务路径到处理函数的映射
type CronRouteHandlers struct {
	Registry map[string]CrontabRouteFunc // 任务路径到处理函数的映射
}

// NewCronServer 创建一个新的计划任务服务器
//
// 参数:
//   - ctx: 上下文，用于日志记录和事件传递
//   - mode: 任务执行模式，控制任务执行的行为
//   - configs: 任务配置列表，定义要执行的任务
//
// 返回:
//   - *CronServer: 新创建的计划任务服务器实例
//
// 使用示例:
//
//	configs := []sylph.TaskConfig{
//	    {Name: MyTask{}, Spec: "0 * * * *", Open: true},
//	}
//	server := sylph.NewCronServer(ctx, sylph.CrontabSkipMode, configs)
//	server.Register(MyTask{}, handleMyTask)
func NewCronServer(ctx Context, mode CrontabMode, configs []TaskConfig) *CronServer {
	return &CronServer{
		ctx:         ctx,
		mode:        mode,
		logger:      newCronLogger(ctx),
		opts:        make([]cron.Option, 0),
		taskConfigs: configs,
		tasks:       make(map[TaskName]TaskHandler),
	}
}

// 确保CronServer实现了IServer接口
var _ IServer = (*CronServer)(nil)

// CronServer 计划任务服务器
// 管理和执行定时任务，支持多种执行模式
type CronServer struct {
	ctx    Context       // 上下文，用于日志记录和事件传递
	mode   CrontabMode   // 任务执行模式，如正常、跳过、延迟
	opts   []cron.Option // cron库的选项配置
	logger cron.Logger   // 日志记录器
	cron   *cron.Cron    // cron实例，用于管理定时任务

	taskConfigs []TaskConfig             // 任务配置列表
	tasks       map[TaskName]TaskHandler // 任务名称到处理函数的映射
	started     bool                     // 服务是否已启动
}

// Name 获取服务器名称
// 实现IServer接口的Name方法
//
// 返回:
//   - string: 服务器名称，固定为"cron-server"
func (c *CronServer) Name() string {
	return "cron-server"
}

// Register 注册任务处理函数
// 将任务名称与对应的处理函数关联
//
// 参数:
//   - name: 任务名称，必须实现TaskName接口
//   - task: 任务处理函数，类型为TaskHandler
//
// 使用示例:
//
//	server.Register(MyTask{}, func(ctx sylph.Context) error {
//	    // 处理任务逻辑
//	    return nil
//	})
func (c *CronServer) Register(name TaskName, task TaskHandler) {
	c.tasks[name] = task
}

// receiveTask 获取任务处理函数
// 根据任务名称查找对应的处理函数
//
// 参数:
//   - name: 任务名称
//
// 返回:
//   - task: 任务处理函数
//   - ok: 是否找到任务处理函数
func (c *CronServer) receiveTask(name TaskName) (task TaskHandler, ok bool) {
	task, ok = c.tasks[name]
	return
}

// loadDefaultOption 加载默认选项配置
// 设置cron库的基本选项，如启用秒级支持和日志记录器
//
// 返回:
//   - opts: cron库的选项配置列表
func (c *CronServer) loadDefaultOption() (opts []cron.Option) {
	return []cron.Option{
		cron.WithSeconds(),        // 启用秒级支持，允许秒级精度的定时任务
		cron.WithLogger(c.logger), // 使用自定义日志记录器
	}
}

// modeOptions 根据执行模式获取对应的选项配置
// 不同的执行模式对应不同的任务调度策略
//
// 返回:
//   - opts: 基于当前模式的cron选项配置
func (c *CronServer) modeOptions() (opts []cron.Option) {
	opts = make([]cron.Option, 0)

	var wrapper cron.JobWrapper
	switch c.mode {
	case CrontabSkipMode:
		// 如果上一个任务还在执行，则跳过本次执行
		wrapper = cron.SkipIfStillRunning(c.logger)
		break
	case CrontabDelayMode:
		// 如果上一个任务还在执行，则等待执行完成后再执行下一个
		wrapper = cron.DelayIfStillRunning(c.logger)
		break
	default:
		// 默认模式不需要额外的包装器
		return
	}

	return append(opts, cron.WithChain(wrapper))
}

// combinationOptions 组合所有选项配置
// 整合默认选项和模式特定选项
//
// 返回:
//   - []cron.Option: 组合后的完整选项配置
func (c *CronServer) combinationOptions() []cron.Option {
	return append(c.loadDefaultOption(), c.modeOptions()...)
}

// LoadOptions 加载额外的选项配置
// 允许外部添加自定义的cron选项
//
// 参数:
//   - opts: 要添加的cron选项配置
func (c *CronServer) LoadOptions(opts ...cron.Option) {
	c.opts = append(c.opts, opts...)
}

// bindSwitchedHandler 绑定已启用的任务处理函数
// 根据配置将任务处理函数添加到cron调度器
//
// 注意事项:
//   - 只有Open设置为true的任务才会被添加
//   - 如果任务未注册，会记录警告但不会中断启动
func (c *CronServer) bindSwitchedHandler() {

	for _, conf := range c.taskConfigs {
		if !conf.Open {
			// 跳过未启用的任务
			continue
		}

		// 检查任务是否已注册
		if _, ok := c.tasks[conf.Name]; !ok {
			c.ctx.Warn("server.CronServer.bindSwitchedHandler", "crontab task not setting", map[string]interface{}{
				"task": conf.Name.Name(),
			})

			pr.Warning("CronServer task %s not setting\n", conf.Name)
			continue
		}

		// 添加任务到cron调度器
		if _, err := c.cron.AddFunc(conf.Spec, c.takeRunHandler(conf.Name)); err != nil {
			pr.Panic("CronServer bindSwitchedHandler failed: %+v\n", err)
		}
	}
}

// takeRunHandler 创建任务执行函数
// 为任务创建执行包装函数，处理上下文和错误记录
//
// 参数:
//   - name: 任务名称
//
// 返回:
//   - func(): 可以被cron库调用的无参数函数
func (c *CronServer) takeRunHandler(name TaskName) func() {
	return func() {
		handler := c.tasks[name]

		pr.Red("run name: %s\n", name)
		// 克隆上下文，避免污染原始上下文
		ctx := c.ctx.Clone()

		// 记录任务名称到调试日志
		ctx.Set("task_name", name.Name())

		// 执行任务处理函数，记录错误
		if err := handler(ctx); err != nil {
			ctx.Error("server.CronServer.takeRunHandler", "cron task run failed", err, map[string]interface{}{
				"task": name.Name(),
			})
		}
	}
}

// Boot 启动服务器
// 实现IServer接口的Boot方法，初始化cron调度器并启动
//
// 返回:
//   - err: 启动过程中的错误，成功启动返回nil
//
// 注意事项:
//   - 如果服务器已启动，则不会重复启动
func (c *CronServer) Boot() (err error) {
	if c.started {
		// 防止重复启动
		return
	}

	c.started = true

	// 加载选项配置
	c.LoadOptions(
		c.combinationOptions()...,
	)
	// 创建cron实例
	c.cron = cron.New(c.opts...)

	// 绑定任务处理函数
	c.bindSwitchedHandler()
	// 启动cron调度器
	c.cron.Start()
	return
}

// Shutdown 关闭服务器
// 实现IServer接口的Shutdown方法，停止cron调度器
//
// 返回:
//   - error: 关闭过程中的错误，成功关闭返回nil
func (c *CronServer) Shutdown() error {
	c.cron.Stop()
	return nil
}

// cronLogger 自定义cron日志记录器
// 实现cron.Logger接口，将日志转发到Context
type cronLogger struct {
	ctx Context // 上下文，用于记录日志
}

// newCronLogger 创建新的cron日志记录器
// 将日志记录委托给Context实现
//
// 参数:
//   - ctx: 上下文，用于记录日志
//
// 返回:
//   - cron.Logger: 实现cron.Logger接口的日志记录器
func newCronLogger(ctx Context) cron.Logger {
	return &cronLogger{ctx: ctx}
}

// Info 记录信息级别日志
// 实现cron.Logger接口的Info方法
//
// 参数:
//   - msg: 日志消息
//   - values: 附加值，结构化记录
func (c *cronLogger) Info(msg string, values ...interface{}) {
	c.ctx.Info("server.cronLogger.Info", msg, map[string]interface{}{
		"data": values,
	})
}

// Error 记录错误级别日志
// 实现cron.Logger接口的Error方法
//
// 参数:
//   - err: 错误对象
//   - msg: 日志消息
//   - values: 附加值，结构化记录
func (c *cronLogger) Error(err error, msg string, values ...interface{}) {
	c.ctx.Error("server.cronLogger.Error", msg, err, map[string]interface{}{
		"data": values,
	})
}

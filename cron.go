package sylph

import (
	cron "github.com/robfig/cron/v3"
	"github.com/sylphbyte/pr"
)

const (
	// CrontabNormalMode 默认normal
	CrontabNormalMode CrontabMode = iota + 1

	// CrontabSkipMode 跳过执行
	CrontabSkipMode

	// CrontabDelayMode 延迟执行
	CrontabDelayMode

	CrontabNormalName CrontabModeName = "normal"

	CrontabSkipName CrontabModeName = "skip"

	CrontabDelayName CrontabModeName = "delay"
)

var (
	crontabModeMapping = map[CrontabMode]CrontabModeName{
		CrontabNormalMode: CrontabNormalName,
		CrontabSkipMode:   CrontabSkipName,
		CrontabDelayMode:  CrontabDelayName,
	}

	crontabNameModeMapping = map[CrontabModeName]CrontabMode{
		CrontabNormalName: CrontabNormalMode,
		CrontabSkipName:   CrontabSkipMode,
		CrontabDelayName:  CrontabDelayMode,
	}
)

type SwitchMode map[CrontabModeName]bool

type CrontabModes []CrontabMode

type CrontabMode int

type CrontabModeName string

func (c CrontabModeName) String() string {
	return string(c)
}
func (c CrontabModeName) Mode() CrontabMode {
	mode, ok := crontabNameModeMapping[c]
	if !ok {
		pr.Panic("not has mode name: %s", c)
	}

	return mode
}

func (c CrontabModeName) Valid() bool {
	_, ok := crontabNameModeMapping[c]
	return ok
}

func (m CrontabMode) Name() CrontabModeName {
	return crontabModeMapping[m]
}

type CronRouteHandlers struct {
	Registry map[string]CrontabRouteFunc
}

func NewCrontab(ctx Context, mode CrontabMode, configs []TaskConfig) *Crontab {
	return &Crontab{
		ctx:         ctx,
		mode:        mode,
		logger:      newCronLogger(ctx),
		opts:        make([]cron.Option, 0),
		taskConfigs: configs,
		tasks:       make(map[TaskName]TaskHandler),
	}
}

type Crontab struct {
	ctx    Context
	mode   CrontabMode
	opts   []cron.Option
	logger cron.Logger
	cron   *cron.Cron

	taskConfigs []TaskConfig
	tasks       map[TaskName]TaskHandler
	started     bool
}

func (c *Crontab) Register(name TaskName, task TaskHandler) {
	c.tasks[name] = task
}

func (c *Crontab) receiveTask(name TaskName) (task TaskHandler, ok bool) {
	task, ok = c.tasks[name]
	return
}

// 读取默认配置
func (c *Crontab) loadDefaultOption() (opts []cron.Option) {
	return []cron.Option{
		cron.WithSeconds(),
		cron.WithLogger(c.logger),
	}
}

func (c *Crontab) modeOptions() (opts []cron.Option) {
	opts = make([]cron.Option, 0)

	var wrapper cron.JobWrapper
	switch c.mode {
	case CrontabSkipMode:
		wrapper = cron.SkipIfStillRunning(c.logger)
		break
	case CrontabDelayMode:
		wrapper = cron.DelayIfStillRunning(c.logger)
		break
	default:
		return
	}

	return append(opts, cron.WithChain(wrapper))
}

func (c *Crontab) combinationOptions() []cron.Option {
	return append(c.loadDefaultOption(), c.modeOptions()...)
}

func (c *Crontab) LoadOptions(opts ...cron.Option) {
	c.opts = append(c.opts, opts...)
}

func (c *Crontab) bindSwitchedHandler() {

	for _, conf := range c.taskConfigs {
		if !conf.Open {
			continue
		}

		if _, ok := c.tasks[conf.Name]; !ok {
			c.ctx.Warn("server.Crontab.bindSwitchedHandler", "crontab task not setting", H{
				"task": conf.Name.Name(),
			})

			pr.Warning("Crontab task %s not setting\n", conf.Name)
			continue
		}

		//pr.Green("config: %+v\n", conf)
		if _, err := c.cron.AddFunc(conf.Spec, c.takeRunHandler(conf.Name)); err != nil {
			pr.Panic("Crontab bindSwitchedHandler failed: %+v\n", err)
		}
	}
}

func (c *Crontab) takeRunHandler(name TaskName) func() {
	return func() {
		handler := c.tasks[name]

		pr.Red("run name: %s\n", name)
		ctx := c.ctx.Clone()
		ctx.TakeHeader().StorePath(name.Name())
		ctx.TakeHeader().GenerateTraceId()

		if err := handler(ctx); err != nil {
			ctx.Error("server.Crontab.takeRunHandler", "cron task run failed", err, H{
				"task": name.Name(),
			})
		}
	}

}

func (c *Crontab) Boot() (err error) {
	if c.started {
		return
	}

	c.started = true

	c.LoadOptions(
		c.combinationOptions()...,
	)
	c.cron = cron.New(c.opts...)

	c.bindSwitchedHandler()
	c.cron.Start()
	return
}

func (c *Crontab) Shutdown() error {
	c.cron.Stop()
	return nil
}

type cronLogger struct {
	ctx Context
}

func newCronLogger(ctx Context) cron.Logger {
	return &cronLogger{ctx: ctx}
}

func (c *cronLogger) Info(msg string, values ...interface{}) {
	c.ctx.Info("server.cronLogger.Info", msg, map[string]interface{}{
		"data": values,
	})
}

func (c *cronLogger) Error(err error, msg string, values ...interface{}) {
	c.ctx.Error("server.cronLogger.Error", msg, err, map[string]interface{}{
		"data": values,
	})
}

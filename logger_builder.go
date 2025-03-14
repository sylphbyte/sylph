package sylph

import (
	"github.com/sirupsen/logrus"
	"os"
)

type LoggerBuilder struct {
	name      string
	opt       *LoggerConfig
	Formatter logrus.Formatter
	Hooks     logrus.LevelHooks
}

func NewLoggerBuilder(name string, opt *LoggerConfig) *LoggerBuilder {
	return &LoggerBuilder{
		name:  name,
		opt:   opt,
		Hooks: make(logrus.LevelHooks),
	}
}

func (l *LoggerBuilder) InjectHook(hook logrus.Hook) {
	l.Hooks.Add(hook)
}

func (l *LoggerBuilder) MakeHooks() {
	l.Hooks.Add(NewLoggerBufferHook(l.name, l.opt))
}

func (l *LoggerBuilder) Make() (entity *logrus.Logger) {
	l.MakeHooks()
	entity = &logrus.Logger{
		Out:       l.opt.stdout(),
		Formatter: &XLoggerFormatter{TimestampFormat: "2006-01-02 15:04:05.000"}, // ?
		Hooks:     l.Hooks,
		Level:     l.opt.level(),
		ExitFunc:  os.Exit,
	}

	return
}

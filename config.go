package sylph

import (
	"strings"

	"github.com/spf13/viper"
	"github.com/sylphbyte/pr"
)

var _ IConfigParser = (*Parser)(nil)

type IConfigParser interface {
	ParseFile(filepath string, conf any) (err error)
}

func NewParser(opt IOption) IConfigParser {
	return &Parser{
		opt: opt,
	}
}

type Parser struct {
	opt IOption
}

func (c Parser) ParseFile(filepath string, conf any) (err error) {
	pr.System("read config yaml: %s\n", filepath)

	viper.SetConfigName(filepath)
	c.envRewriteParse()
	c.withOption()

	if ignoreErr := viper.ReadInConfig(); ignoreErr != nil {
		pr.Error("read config %s failed: %v\n", filepath, ignoreErr)
	}

	if err = viper.Unmarshal(conf); err != nil {
		pr.Error("read config %s failed: %v\n", filepath, err)
	}

	return
}

func (c Parser) envRewriteParse() {
	if c.opt.IsEnvRewrite() {
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	}
}

func (c Parser) withOption() {
	configName := c.opt.TakeConfigName()
	if configName != "" {
		viper.SetConfigName(configName)
	}

	viper.SetConfigType(c.opt.TakeConfigType())
	for _, path := range c.opt.TakeConfigPaths() {
		viper.AddConfigPath(path) // 当前目录
	}
}

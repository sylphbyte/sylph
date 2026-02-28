package sylph

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
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

func (c Parser) ParseFile(filePath string, conf any) (err error) {
	printSystem("read config yaml: %s\n", filePath)

	// 如果需要保持 key 大小写，使用 yaml.v3 直接解析
	if c.opt.IsPreserveKeyCase() {
		return c.parseFileWithYaml(filePath, conf)
	}

	// 默认使用 viper 解析
	viper.SetConfigName(filePath)
	c.envRewriteParse()
	c.withOption()

	if ignoreErr := viper.ReadInConfig(); ignoreErr != nil {
		printError("read config %s failed: %v\n", filePath, ignoreErr)
	}

	if err = viper.Unmarshal(conf); err != nil {
		printError("read config %s failed: %v\n", filePath, err)
	}

	return
}

// parseFileWithYaml 使用 yaml.v3 直接解析，保持 map key 大小写
func (c Parser) parseFileWithYaml(filePath string, conf any) (err error) {
	// 查找配置文件
	var fullPath string
	for _, configPath := range c.opt.TakeConfigPaths() {
		candidate := filepath.Join(configPath, filePath)
		if !strings.HasSuffix(candidate, ".yaml") && !strings.HasSuffix(candidate, ".yml") {
			candidate += ".yaml"
		}
		if _, statErr := os.Stat(candidate); statErr == nil {
			fullPath = candidate
			break
		}
	}

	if fullPath == "" {
		printError("config file not found: %s\n", filePath)
		return errors.New("config file not found")
	}

	var data []byte
	if data, err = os.ReadFile(fullPath); err != nil {
		printError("read config %s failed: %v\n", fullPath, err)
		return errors.Wrap(err, "read config failed")
	}

	if err = yaml.Unmarshal(data, conf); err != nil {
		printError("parse config %s failed: %v\n", fullPath, err)
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

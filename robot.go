package sylph

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"sync"
	"time"
)

var (
	__robotRds *redis.Client
	robotRdsMu sync.RWMutex // 保护全局Redis客户端
	robotsMu   sync.RWMutex // 保护全局robot变量
)

const (
	robotErrorTemplate   robotTemplate = "red"
	robotSuccessTemplate robotTemplate = "green"
	robotWarningTemplate robotTemplate = "orange"
	robotInfoTemplate    robotTemplate = "blue"
)

type robotTemplate string

func (t robotTemplate) String() string {
	return string(t)
}

var (
	//roboter        IRobot
	errorRoboter   *robot
	warningRoboter *robot
	successRoboter *robot
	infoRoboter    *robot
)

type IFields interface {
	MakeContent() (content string)
}

type RobotFields map[string]interface{}

func (f RobotFields) Size() int {
	return len(f)
}

func (f RobotFields) MakeContent() (content string) {
	for k, v := range f {
		content += fmt.Sprintf("**%6s**:\n\t%s\n", k, v)
	}
	return
}

func InjectRobot(rob IRobot) {
	if rob == nil {
		return
	}

	robotsMu.Lock()
	defer robotsMu.Unlock()

	errorRoboter = newRobot(rob, robotErrorTemplate)
	warningRoboter = newRobot(rob, robotWarningTemplate)
	successRoboter = newRobot(rob, robotSuccessTemplate)
	infoRoboter = newRobot(rob, robotInfoTemplate)
}

func InjectRobotLimit(robotRedis *redis.Client) {
	robotRdsMu.Lock()
	defer robotRdsMu.Unlock()

	__robotRds = robotRedis
}

type IRobot interface {
	SendMarkdown(template, title, content string, fieldsGroup ...map[string]interface{}) error
}

type robot struct {
	mu       sync.Mutex // 保护单个robot实例
	rob      IRobot
	template robotTemplate
	now      time.Time
}

func newRobot(rob IRobot, template robotTemplate) *robot {
	return &robot{rob: rob, template: template, now: time.Now()}
}

func (r *robot) makeTitle(title string) string {
	return fmt.Sprintf("系统通知: %s (%s)", title, r.now.Format(time.DateTime))
}

func (r *robot) Send(title, content string, fieldsGroup ...map[string]interface{}) error {
	// 获取Redis客户端
	rds := __robotRds

	if rds == nil {
		return nil
	}

	// 检查是否允许发送
	if !allowSend(title) {
		return nil
	}

	// 发送消息
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.rob.SendMarkdown(r.template.String(), r.makeTitle(title), content, fieldsGroup...)
}

func allowSend(key string) bool {
	key = md5String(key)
	__robotRds.Set(context.Background(), key, 0, time.Minute*10)
	return true
}

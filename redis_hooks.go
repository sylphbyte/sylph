package sylph

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	_reset  = "\033[0m"
	_cyan   = "\033[36m"
	_yellow = "\033[33m"
	_green  = "\033[32m"
	_red    = "\033[31m"
)

// cmdRows 根据命令返回类型提取类似 GORM [rows:N] 的信息
func cmdRows(cmd redis.Cmder) string {
	switch v := cmd.(type) {
	case *redis.SliceCmd:
		return fmt.Sprintf("[rows:%d]", len(v.Val()))
	case *redis.MapStringStringCmd:
		return fmt.Sprintf("[rows:%d]", len(v.Val()))
	case *redis.StringSliceCmd:
		return fmt.Sprintf("[rows:%d]", len(v.Val()))
	case *redis.StringCmd:
		if v.Val() == "" {
			return "[rows:0]"
		}
		return "[rows:1]"
	case *redis.IntCmd:
		return fmt.Sprintf("[val:%d]", v.Val())
	case *redis.BoolCmd:
		return fmt.Sprintf("[val:%v]", v.Val())
	case *redis.StatusCmd:
		return fmt.Sprintf("[%s]", v.Val())
	default:
		return ""
	}
}

type RedisDebugHook struct{}

func (h RedisDebugHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h RedisDebugHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		ms := float64(time.Since(start).Microseconds()) / 1000.0

		costColor := _yellow
		if ms > 100 {
			costColor = _red
		}

		rows := cmdRows(cmd)
		if err != nil {
			log.Printf("%s[Redis]%s %s[%.3fms]%s %s%s%s cmd=%s%s%s args=%s%v%s %serr=%v%s",
				_cyan, _reset,
				costColor, ms, _reset,
				_yellow, rows, _reset,
				_green, cmd.Name(), _reset,
				_green, cmd.Args(), _reset,
				_red, err, _reset,
			)
		} else {
			log.Printf("%s[Redis]%s %s[%.3fms]%s %s%s%s cmd=%s%s%s args=%s%v%s",
				_cyan, _reset,
				costColor, ms, _reset,
				_yellow, rows, _reset,
				_green, cmd.Name(), _reset,
				_green, cmd.Args(), _reset,
			)
		}
		return err
	}
}

func (h RedisDebugHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		ms := float64(time.Since(start).Microseconds()) / 1000.0

		costColor := _yellow
		if ms > 100 {
			costColor = _red
		}

		for _, cmd := range cmds {
			rows := cmdRows(cmd)
			if cmd.Err() != nil {
				log.Printf("%s[Redis-Pipe]%s %s[%.3fms]%s %s%s%s cmd=%s%s%s args=%s%v%s %serr=%v%s",
					_cyan, _reset,
					costColor, ms, _reset,
					_yellow, rows, _reset,
					_green, cmd.Name(), _reset,
					_green, cmd.Args(), _reset,
					_red, cmd.Err(), _reset,
				)
			} else {
				log.Printf("%s[Redis-Pipe]%s %s[%.3fms]%s %s%s%s cmd=%s%s%s args=%s%v%s",
					_cyan, _reset,
					costColor, ms, _reset,
					_yellow, rows, _reset,
					_green, cmd.Name(), _reset,
					_green, cmd.Args(), _reset,
				)
			}
		}
		return err
	}
}

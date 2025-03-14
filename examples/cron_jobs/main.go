package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sylphbyte/sylph"
)

// 数据统计示例
type Statistics struct {
	TotalUsers    int       `json:"total_users"`
	ActiveUsers   int       `json:"active_users"`
	NewUsers      int       `json:"new_users"`
	TotalOrders   int       `json:"total_orders"`
	Revenue       float64   `json:"revenue"`
	LastUpdatedAt time.Time `json:"last_updated_at"`
}

// 模拟数据库中的统计数据
var stats = Statistics{
	TotalUsers:    1000,
	ActiveUsers:   650,
	NewUsers:      45,
	TotalOrders:   320,
	Revenue:       15680.50,
	LastUpdatedAt: time.Now().Add(-24 * time.Hour),
}

func main() {
	// 创建项目管理器
	project := sylph.NewProject()

	// 创建定时任务服务器
	cronServer := sylph.NewCronServer()

	// 添加每分钟执行一次的任务 - 用户活跃度统计
	cronServer.AddJob("0 * * * * *", "用户活跃度统计", func(ctx sylph.Context) {
		// 记录任务开始
		ctx.Info("cron.users.active", "开始统计用户活跃度", sylph.H{
			"start_time": time.Now().Format(time.RFC3339),
		})

		// 模拟业务逻辑 - 更新活跃用户数
		stats.ActiveUsers = 600 + (time.Now().Minute() % 100)
		stats.LastUpdatedAt = time.Now()

		// 记录执行结果
		ctx.Info("cron.users.active", "用户活跃度统计完成", sylph.H{
			"active_users": stats.ActiveUsers,
			"total_users":  stats.TotalUsers,
			"ratio":        fmt.Sprintf("%.2f%%", float64(stats.ActiveUsers)/float64(stats.TotalUsers)*100),
		})
	})

	// 添加每5分钟执行一次的任务 - 订单统计
	cronServer.AddJob("0 */5 * * * *", "订单统计", func(ctx sylph.Context) {
		ctx.Info("cron.orders.stats", "开始统计订单数据", sylph.H{
			"start_time": time.Now().Format(time.RFC3339),
		})

		// 模拟业务逻辑 - 更新订单数据
		newOrders := 10 + (time.Now().Minute() % 20)
		newRevenue := float64(newOrders) * 100.0

		stats.TotalOrders += newOrders
		stats.Revenue += newRevenue
		stats.LastUpdatedAt = time.Now()

		// 记录执行结果
		ctx.Info("cron.orders.stats", "订单统计完成", sylph.H{
			"total_orders":  stats.TotalOrders,
			"new_orders":    newOrders,
			"total_revenue": stats.Revenue,
			"new_revenue":   newRevenue,
		})

		// 发送事件通知
		ctx.AsyncEmit("stats.orders.updated", sylph.H{
			"total_orders": stats.TotalOrders,
			"revenue":      stats.Revenue,
		})
	})

	// 添加每天凌晨1点执行的任务 - 数据归档
	cronServer.AddJob("0 0 1 * * *", "数据归档", func(ctx sylph.Context) {
		ctx.Info("cron.archive", "开始数据归档", sylph.H{
			"date": time.Now().Format("2006-01-02"),
		})

		// 模拟耗时操作
		ctx.Info("cron.archive", "正在归档用户数据", nil)
		time.Sleep(1 * time.Second)

		ctx.Info("cron.archive", "正在归档订单数据", nil)
		time.Sleep(1 * time.Second)

		ctx.Info("cron.archive", "正在归档日志数据", nil)
		time.Sleep(1 * time.Second)

		// 记录执行结果
		ctx.Info("cron.archive", "数据归档完成", sylph.H{
			"status":      "success",
			"duration_ms": 3000,
			"date":        time.Now().Format("2006-01-02"),
		})
	})

	// 添加带错误处理的任务 - 异常监控
	cronServer.AddJob("0 */15 * * * *", "系统异常监控", func(ctx sylph.Context) {
		ctx.Info("cron.monitor", "开始系统异常监控", nil)

		// 模拟业务逻辑
		errorRate := float64(time.Now().Minute()%100) / 100.0

		if errorRate > 0.8 {
			// 模拟发现高错误率的情况
			err := fmt.Errorf("系统错误率过高: %.2f%%", errorRate*100)
			ctx.Error("cron.monitor", "系统异常监控警报", err, sylph.H{
				"error_rate": errorRate,
				"threshold":  0.8,
			})

			// 发送告警事件
			ctx.Emit("system.alert", sylph.H{
				"level":       "critical",
				"error_rate":  errorRate,
				"description": "系统错误率超过阈值",
			})
		} else {
			ctx.Info("cron.monitor", "系统运行正常", sylph.H{
				"error_rate": errorRate,
			})
		}
	})

	// 添加一个可以从配置动态更新的任务 - 模拟配置管理
	// 在实际应用中，可以通过配置中心或数据库更新cron表达式
	jobName := "配置刷新"
	cronExpr := "*/30 * * * * *" // 初始设置为每30秒执行一次

	cronServer.AddJob(cronExpr, jobName, func(ctx sylph.Context) {
		ctx.Info("cron.config", "开始刷新配置", nil)

		// 模拟配置刷新
		ctx.Info("cron.config", "配置刷新完成", sylph.H{
			"config_items": 15,
			"updated_at":   time.Now().Format(time.RFC3339),
		})

		// 模拟动态更新Cron表达式（实际应用中可以从数据库或配置中心获取）
		// 这里简单演示，每次执行都会修改自己的执行频率
		if time.Now().Second() < 30 {
			newCronExpr := "*/10 * * * * *" // 改为每10秒执行一次
			if cronExpr != newCronExpr {
				ctx.Info("cron.config", "更新任务执行频率", sylph.H{
					"job_name":      jobName,
					"old_cron_expr": cronExpr,
					"new_cron_expr": newCronExpr,
				})

				// 更新Cron表达式
				if err := cronServer.UpdateJobCron(jobName, newCronExpr); err != nil {
					ctx.Error("cron.config", "更新任务执行频率失败", err, nil)
				} else {
					cronExpr = newCronExpr
				}
			}
		} else {
			newCronExpr := "*/30 * * * * *" // 恢复为每30秒执行一次
			if cronExpr != newCronExpr {
				ctx.Info("cron.config", "更新任务执行频率", sylph.H{
					"job_name":      jobName,
					"old_cron_expr": cronExpr,
					"new_cron_expr": newCronExpr,
				})

				// 更新Cron表达式
				if err := cronServer.UpdateJobCron(jobName, newCronExpr); err != nil {
					ctx.Error("cron.config", "更新任务执行频率失败", err, nil)
				} else {
					cronExpr = newCronExpr
				}
			}
		}
	})

	// 创建HTTP服务用于查看统计数据
	httpServer := sylph.NewGinServer(":8080")

	// 添加统计数据查询接口
	httpServer.GET("/api/stats", func(c *sylph.GinContext) {
		ctx := sylph.FromGin(c.Context)
		ctx.Debug("api.stats", "查询统计数据", nil)

		// 返回最新统计数据
		c.JSON(200, sylph.H{
			"data": stats,
			"meta": sylph.H{
				"updated_at": stats.LastUpdatedAt,
			},
		})
	})

	// 添加健康检查接口
	httpServer.GET("/health", func(c *sylph.GinContext) {
		c.JSON(200, sylph.H{"status": "ok"})
	})

	// 挂载服务到项目
	project.Mounts(cronServer)
	project.Mounts(httpServer)

	// 启动项目
	if err := project.Boots(); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}

	log.Println("服务已启动，监听端口: 8080")
	log.Println("访问 http://localhost:8080/api/stats 查看统计数据")
	log.Println("定时任务已开始运行...")

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务...")
	if err := project.Shutdowns(); err != nil {
		log.Fatalf("关闭服务失败: %v", err)
	}
	log.Println("服务已安全关闭")
}

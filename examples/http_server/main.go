package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sylphbyte/sylph"
)

// 用户数据结构
type User struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	CreateAt time.Time `json:"create_at"`
}

// 模拟数据库
var userDB = map[string]User{
	"1": {ID: "1", Name: "张三", Email: "zhangsan@example.com", CreateAt: time.Now().Add(-24 * time.Hour)},
	"2": {ID: "2", Name: "李四", Email: "lisi@example.com", CreateAt: time.Now().Add(-48 * time.Hour)},
}

func main() {
	// 创建HTTP服务器
	server := sylph.NewGinServer(":8080")

	// 添加中间件
	server.Use(gin.Recovery())

	// 添加自定义中间件：请求日志记录
	server.Use(func(c *gin.Context) {
		// 获取开始时间
		startTime := time.Now()

		// 从Gin上下文获取Sylph上下文
		ctx := sylph.FromGin(c)

		// 记录请求开始
		ctx.Info("middleware.request", "收到请求", sylph.H{
			"method": c.Request.Method,
			"path":   c.Request.URL.Path,
			"ip":     c.ClientIP(),
		})

		// 处理请求
		c.Next()

		// 记录请求结束
		ctx.Info("middleware.response", "请求处理完成", sylph.H{
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
			"status":     c.Writer.Status(),
			"duration":   time.Since(startTime).String(),
			"durationMs": time.Since(startTime).Milliseconds(),
		})
	})

	// 注册路由
	setupRoutes(server)

	// 创建并启动项目
	project := sylph.NewProject()
	project.Mounts(server)

	// 添加一个示例的定时任务
	cronServer := sylph.NewCronServer()
	cronServer.AddJob("*/30 * * * * *", "活跃用户统计", func(ctx sylph.Context) {
		ctx.Info("cron.stats", "执行用户统计", sylph.H{
			"total_users": len(userDB),
			"time":        time.Now().Format(time.RFC3339),
		})
	})
	project.Mounts(cronServer)

	// 启动项目
	if err := project.Boots(); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}

	log.Println("服务已启动，监听端口: 8080")
	log.Println("访问: http://localhost:8080/api/users")

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

// 设置API路由
func setupRoutes(server sylph.GinServer) {
	// 健康检查
	server.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API路由组
	api := server.Group("/api")
	{
		// 用户相关接口
		users := api.Group("/users")
		{
			users.GET("", listUsers)
			users.GET("/:id", getUser)
			users.POST("", createUser)
			users.PUT("/:id", updateUser)
			users.DELETE("/:id", deleteUser)
		}
	}
}

// 获取用户列表
func listUsers(c *gin.Context) {
	ctx := sylph.FromGin(c)

	// 记录请求
	ctx.Debug("api.users.list", "获取用户列表", nil)

	// 转换map为切片
	users := make([]User, 0, len(userDB))
	for _, user := range userDB {
		users = append(users, user)
	}

	// 使用事件系统触发用户列表查询事件
	ctx.Emit("users.listed", sylph.H{
		"count": len(users),
	})

	// 返回用户列表
	c.JSON(200, gin.H{
		"data": users,
		"meta": gin.H{
			"total": len(users),
		},
	})
}

// 获取单个用户
func getUser(c *gin.Context) {
	ctx := sylph.FromGin(c)
	id := c.Param("id")

	// 记录请求
	ctx.Debug("api.users.get", "获取用户详情", sylph.H{
		"id": id,
	})

	// 查找用户
	user, exists := userDB[id]
	if !exists {
		ctx.Warn("api.users.get", "用户不存在", sylph.H{
			"id": id,
		})
		c.JSON(404, gin.H{"error": "用户不存在"})
		return
	}

	// 使用事件系统触发用户查询事件
	ctx.Emit("user.viewed", sylph.H{
		"userId": id,
	})

	// 返回用户信息
	c.JSON(200, user)
}

// 创建用户
func createUser(c *gin.Context) {
	ctx := sylph.FromGin(c)

	// 绑定请求数据
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		ctx.Error("api.users.create", "无效的请求数据", err, sylph.H{
			"requestBody": c.Request.Body,
		})
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	// 生成ID（简化处理）
	user.ID = time.Now().Format("20060102150405")
	user.CreateAt = time.Now()

	// 保存用户
	userDB[user.ID] = user

	// 使用事件系统触发用户创建事件
	ctx.AsyncEmit("user.created", sylph.H{
		"userId": user.ID,
		"name":   user.Name,
		"email":  user.Email,
	})

	// 记录创建成功
	ctx.Info("api.users.create", "用户创建成功", sylph.H{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
	})

	// 返回创建的用户
	c.JSON(201, user)
}

// 更新用户
func updateUser(c *gin.Context) {
	ctx := sylph.FromGin(c)
	id := c.Param("id")

	// 检查用户是否存在
	existingUser, exists := userDB[id]
	if !exists {
		ctx.Warn("api.users.update", "用户不存在", sylph.H{"id": id})
		c.JSON(404, gin.H{"error": "用户不存在"})
		return
	}

	// 绑定请求数据
	var updateData User
	if err := c.ShouldBindJSON(&updateData); err != nil {
		ctx.Error("api.users.update", "无效的请求数据", err, nil)
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	// 更新用户数据（保留ID和创建时间）
	updateData.ID = existingUser.ID
	updateData.CreateAt = existingUser.CreateAt
	userDB[id] = updateData

	// 使用事件系统触发用户更新事件
	ctx.AsyncEmit("user.updated", sylph.H{
		"userId": id,
		"name":   updateData.Name,
		"email":  updateData.Email,
	})

	// 记录更新成功
	ctx.Info("api.users.update", "用户更新成功", sylph.H{
		"id":    id,
		"name":  updateData.Name,
		"email": updateData.Email,
	})

	// 返回更新后的用户
	c.JSON(200, updateData)
}

// 删除用户
func deleteUser(c *gin.Context) {
	ctx := sylph.FromGin(c)
	id := c.Param("id")

	// 检查用户是否存在
	_, exists := userDB[id]
	if !exists {
		ctx.Warn("api.users.delete", "用户不存在", sylph.H{"id": id})
		c.JSON(404, gin.H{"error": "用户不存在"})
		return
	}

	// 删除用户
	delete(userDB, id)

	// 使用事件系统触发用户删除事件
	ctx.AsyncEmit("user.deleted", sylph.H{
		"userId": id,
	})

	// 记录删除成功
	ctx.Info("api.users.delete", "用户删除成功", sylph.H{"id": id})

	// 返回成功消息
	c.JSON(200, gin.H{"message": "用户已删除"})
}

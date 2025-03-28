package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/crypto/ssh"
)

func main() {
	// SSH服务器配置
	sshConfig := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			publicKeyFile("/Users/lifeng/.ssh/id_rsa"), // 使用私钥文件
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 生产环境中应使用ssh.FixedHostKey
		Timeout:         15 * time.Second,
	}

	// 连接SSH服务器
	sshClient, err := ssh.Dial("tcp", "123.57.2.204:22", sshConfig)
	if err != nil {
		log.Fatalf("无法连接SSH服务器: %v", err)
	}
	defer sshClient.Close()

	fmt.Println("成功连接SSH服务器")

	// Redis配置参数
	redisHost := "127.0.0.1" // 假设Redis服务在SSH服务器上
	redisPort := 6379
	redisPassword := "your_redis_password" // 设置实际的Redis密码

	// 创建Redis本地监听器
	redisLocalListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("无法创建Redis本地监听器: %v", err)
	}
	defer redisLocalListener.Close()

	redisLocalAddr := redisLocalListener.Addr().(*net.TCPAddr)

	// 启动Redis转发协程
	go func() {
		for {
			localConn, err := redisLocalListener.Accept()
			if err != nil {
				log.Printf("Redis本地连接接受错误: %v", err)
				return
			}

			// 创建到Redis的远程连接
			remoteConn, err := sshClient.Dial("tcp", fmt.Sprintf("%s:%d", redisHost, redisPort))
			if err != nil {
				log.Printf("无法连接远程Redis: %v", err)
				localConn.Close()
				continue
			}

			fmt.Printf("成功建立到Redis的连接\n")

			// 双向转发数据
			go copyConn(localConn, remoteConn)
			go copyConn(remoteConn, localConn)
		}
	}()

	fmt.Printf("建立Redis SSH隧道: 本地127.0.0.1:%d -> %s:%d\n", redisLocalAddr.Port, redisHost, redisPort)

	// 等待隧道建立
	time.Sleep(1 * time.Second)

	// 连接Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("127.0.0.1:%d", redisLocalAddr.Port),
		Password: redisPassword,
		DB:       0,
	})

	// 测试Redis连接
	ctx := context.Background()
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		fmt.Printf("Redis连接失败: %v\n", err)
	} else {
		fmt.Printf("Redis连接成功: %s\n", pong)

		// 测试Redis操作
		err = rdb.Set(ctx, "test_key", "Hello from SSH tunnel", 0).Err()
		if err != nil {
			fmt.Printf("Redis SET操作失败: %v\n", err)
		} else {
			val, err := rdb.Get(ctx, "test_key").Result()
			if err != nil {
				fmt.Printf("Redis GET操作失败: %v\n", err)
			} else {
				fmt.Printf("Redis GET结果: %s\n", val)
			}
		}
	}

	// 关闭Redis连接
	if err := rdb.Close(); err != nil {
		fmt.Printf("关闭Redis连接时发生错误: %v\n", err)
	}

	fmt.Println("测试完成，隧道将在10秒后关闭...")
	time.Sleep(10 * time.Second)
}

// 从私钥文件中加载公钥
func publicKeyFile(file string) ssh.AuthMethod {
	buffer, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("无法读取私钥文件 %s: %v", file, err)
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		log.Fatalf("无法解析私钥: %v", err)
	}
	return ssh.PublicKeys(key)
}

// 在两个连接之间复制数据
func copyConn(dst, src net.Conn) {
	defer dst.Close()
	defer src.Close()
	io.Copy(dst, src)
}

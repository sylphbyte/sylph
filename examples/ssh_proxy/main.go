package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

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

	// 直接尝试MySQL连接 - 从配置文件中获取
	mysqlHost := "pc-bp169o4hh8g4sq20b.rwlb.rds.aliyuncs.com"
	mysqlPort := 3306

	// 创建MySQL本地监听器
	mysqlLocalListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("无法创建MySQL本地监听器: %v", err)
	}
	defer mysqlLocalListener.Close()

	mysqlLocalAddr := mysqlLocalListener.Addr().(*net.TCPAddr)

	// 启动MySQL转发协程
	go func() {
		for {
			localConn, err := mysqlLocalListener.Accept()
			if err != nil {
				log.Printf("MySQL本地连接接受错误: %v", err)
				return
			}

			// 创建到MySQL的远程连接
			remoteConn, err := sshClient.Dial("tcp", fmt.Sprintf("%s:%d", mysqlHost, mysqlPort))
			if err != nil {
				log.Printf("无法连接远程MySQL: %v", err)
				localConn.Close()
				continue
			}

			fmt.Printf("成功建立到MySQL的连接\n")

			// 双向转发数据
			go copyConn(localConn, remoteConn)
			go copyConn(remoteConn, localConn)
		}
	}()

	fmt.Printf("建立MySQL SSH隧道: 本地127.0.0.1:%d -> %s:%d\n", mysqlLocalAddr.Port, mysqlHost, mysqlPort)
	fmt.Printf("可以使用以下命令连接MySQL:\n")
	fmt.Printf("mysql -h 127.0.0.1 -P %d -u wider_online -p\n", mysqlLocalAddr.Port)
	fmt.Printf("密码: 3uhpXdaR%%sn5\n")

	// 让隧道保持2分钟
	fmt.Println("隧道已建立，将在2分钟后关闭...")
	time.Sleep(2 * time.Minute)
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

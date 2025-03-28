package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
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

	// Elasticsearch配置参数
	esHost := "127.0.0.1" // 假设ES服务在SSH服务器上
	esPort := 9200
	esUsername := "elastic"
	esPassword := "your_es_password" // 设置实际的ES密码

	// 创建ES本地监听器
	esLocalListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("无法创建ES本地监听器: %v", err)
	}
	defer esLocalListener.Close()

	esLocalAddr := esLocalListener.Addr().(*net.TCPAddr)

	// 启动ES转发协程
	go func() {
		for {
			localConn, err := esLocalListener.Accept()
			if err != nil {
				log.Printf("ES本地连接接受错误: %v", err)
				return
			}

			// 创建到ES的远程连接
			remoteConn, err := sshClient.Dial("tcp", fmt.Sprintf("%s:%d", esHost, esPort))
			if err != nil {
				log.Printf("无法连接远程ES: %v", err)
				localConn.Close()
				continue
			}

			fmt.Printf("成功建立到ES的连接\n")

			// 双向转发数据
			go copyConn(localConn, remoteConn)
			go copyConn(remoteConn, localConn)
		}
	}()

	fmt.Printf("建立ES SSH隧道: 本地127.0.0.1:%d -> %s:%d\n", esLocalAddr.Port, esHost, esPort)

	// 等待隧道建立
	time.Sleep(1 * time.Second)

	// 连接Elasticsearch
	esConfig := elasticsearch.Config{
		Addresses: []string{fmt.Sprintf("http://127.0.0.1:%d", esLocalAddr.Port)},
		Username:  esUsername,
		Password:  esPassword,
	}

	es, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		fmt.Printf("创建ES客户端失败: %v\n", err)
		return
	}

	// 测试ES连接
	res, err := es.Info()
	if err != nil {
		fmt.Printf("ES连接失败: %v\n", err)
	} else {
		defer res.Body.Close()
		fmt.Println("ES连接成功, 服务器信息:")

		// 读取并显示响应
		buf := new(bytes.Buffer)
		buf.ReadFrom(res.Body)
		fmt.Println(buf.String())

		// 简单测试 - 索引一个文档
		docRes, err := es.Index(
			"test-index",
			strings.NewReader(`{"title":"SSH隧道测试","created":"`+time.Now().Format(time.RFC3339)+`"}`),
		)
		if err != nil {
			fmt.Printf("索引文档失败: %v\n", err)
		} else {
			defer docRes.Body.Close()
			buf = new(bytes.Buffer)
			buf.ReadFrom(docRes.Body)
			fmt.Printf("索引文档结果: %s\n", buf.String())
		}
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

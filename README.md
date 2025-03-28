# SSH保活测试工具

这个项目用于测试SSH连接的保活机制，主要针对MySQL、Redis和ES等数据库通过SSH隧道连接时的保活问题。

## 功能特性

1. SSH保活机制：每10秒自动发送SSH保活请求，防止连接因长时间不活动而断开
2. MySQL/Redis/ES连接测试：通过SSH隧道测试数据库连接
3. 配置文件支持：支持从YAML配置文件加载配置
4. 模拟存储管理器：提供模拟实现进行连接测试

## 测试用例

项目包含以下测试：

1. SSH保活测试：`tests/keepalive_test.go`
2. 存储连接测试：`tests/storage_connect_test.go`
3. 存储管理器测试：`tests/storage_manager_test.go`

## 使用方法

### 运行SSH保活测试

```bash
go test -v ./tests -run TestSSHKeepAlive
```

此测试将创建SSH连接并启动保活机制，持续发送保活请求60秒。

### 运行存储连接测试

```bash
go test -v ./tests -run TestStorageConnection
```

此测试将使用配置文件中的配置创建SSH代理，并测试MySQL、Redis和ES连接。

### 运行存储管理器测试

```bash
go test -v ./tests -run TestStorageManager
```

此测试使用模拟的存储管理器测试数据库操作。

## 配置文件

配置文件位于`etc/storage.yaml`，格式如下：

```yaml
# mysql 配置组
mysql_group:
  main:
    host: example.com
    port: 3306
    username: user
    password: password
    database: db_name
    proxy:
      type: ssh
      host: ssh_host
      port: 22
      username: ssh_user
      ssh_key: /path/to/key

# redis 配置组
redis_group:
  main:
    host: example.com
    port: 6379
    password: password
    proxy:
      type: ssh
      host: ssh_host
      port: 22
      username: ssh_user
      ssh_key: /path/to/key

# es 配置组
es_group:
  main:
    addresses: ["http://example.com:9200"]
    proxy:
      type: ssh
      host: ssh_host
      port: 22
      username: ssh_user
      ssh_key: /path/to/key
```

## 问题排查

如果测试失败，请检查：

1. SSH密钥文件是否存在且权限正确（建议使用`chmod 600`）
2. SSH服务器是否允许密钥认证
3. 数据库服务器是否能通过SSH服务器访问
4. 配置文件中的连接信息是否正确

## 开发计划

1. 添加更多的保活策略测试
2. 支持更多数据库类型
3. 添加性能测试
4. 改进错误处理和日志 
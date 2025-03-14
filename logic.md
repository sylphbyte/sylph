#### 日志封装还有要实现的功能

1. 创建一个基于logrus的二次封装 可以通过yaml去配置日志的相关配置
2. 要实现以下接口

```go
type ILogger interface {
WithRobot() ILogger
GiveRobot(robot Roboter) ILogger
Info(message string, data H)
Trace(message string, data H)
Debug(message string, data H)
Warn(message string, data H)
Fatal(message string, data H)
Panic(message string, data H)
Error(message string, err error, data H)
}

```

3. 日志输出格式是json 格式

```json
{
  // 这里是上下文信息
  "context": { 
    "service": "api",
    "module": "/open/fzjq/QD564/url/match",
    "trace_id": "1bfda5b0d908433ea49dafcf8f4aeec2",
    "phone_md5": "5a30bb8ffc023f69f7433e121bdf9a39",
    "channel": "QD564",
    "ip": "47.98.241.79"
  },
  "data": {
    "param": {
      "data": "/GZC/sbFSCIu0pGcze33XOSEZlXrav9yIGdWT1P5SwANsHjJ+JIB7fad/ZS+ZpaSak+ctce1qD3uOLVO+rjuvsv8A7m0YA9z3CzRBAVEfvhYHOBuQ2ZJ4UFRMl7xH8QLjs/H7Ta/YypjEQ4H9KFfRFdx6SEu+4Pi3vlWYRHffd7Zz7+MdOHvhXIFwJK5KjfgT2tHxzLaUwxkwksaUgQE84hyi9Yx0MjkEllUz7Kh+3Lun62A0KXS7VkiBl0sleE0aK4VPvTeMxHs2GFlooBxHR7fdtsG0YUcKNd3hrLk4rnQbeKIjY2fReOwjlYRgez9u/+pw8vwLBAM/UD1yqfRgjM7QE0+3RR+6Y8X3/fcFpYpa5RT8gjKIg4iFAyoG+lnAKTuPfp1f3axWsClF/dkwphnWWHDFHuxsjikbI/TUMF4cBbr+Q3kqh+l3de3MEzo3bS9c9cVVKOW0s44CVOwNMDMMGnvv6u+6N5hzGj1N/a8vFE0nplznjVmwwjlqe4YdIqDPbxl/5+APp9UtNjtiw==",
      "channelSign": "QD564"
    }
  },
  "level": "info",
  "msg": "request_input",
  "time": "2025-01-21T00:03:33+08:00"
}
```

4. 由于使用阿里云的nas 写入4k 日志 需要程序做buffer
5. 还要轮转切割
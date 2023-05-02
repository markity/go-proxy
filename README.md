### 搭建方法

在服务器上配置如下nat和ip转发(注意系统重启后需要再次设置):

```
> sudo sysctl -w net.ipv4.ip_forward=1
> sudo iptables -t nat -A POSTROUTING -s 10.8.0.0/16 ! -d 10.8.0.0/16 -j MASQUERADE
> sudo iptables -A FORWARD -s 10.8.0.0/16 -m state --state RELATED,ESTABLISHED -j ACCEPT
> sudo iptables -A FORWARD -d 10.8.0.0/16 -j ACCEPT
```

运行服务器:

```sh
> git clone https://github.com/markity/go-proxy.git
> cd ./go-proxy/server/
> sudo go run .
```

### 客户端连接服务器

首先编辑好`client/config.go`里面的ip配置, 然后运行下面的命令:

```
> cd ./go-proxy/client
> go build .
> sudo ./client -u markity -p 12345
```

> 上述-u参数表示username, -p表示password, markity和12345是默认的一个账号密码, 如有需要可以自行修改, 见后文

### 配置用户名密码

打开本项目的`server/user_table.go`, 自己照葫芦画瓢加几个用户就行了:

```go
func init() {
	UserMap["username1"] = "password1"
	UserMap["username2"] = "password2"
}
```

### 关于dns污染

客户端会编辑`/etc/resolv.conf`, 覆写掉原来的dns服务器而使用`client/config.go`配置的dns服务器。改成`1.1.1.1`和`8.8.8.8`。这样做的原因是经过我的测试, 默认配置的dns服务器无法正确解析google, youtube等国外被墙网站的域名, 国内的dns污染太严重, 乱解析一些国外域名。

退出客户端时, 将恢复原来的resolv.conf文件。

### 原理:

参见博客: https://markity.github.io/2023/04/29/vpn/
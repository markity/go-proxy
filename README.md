### 搭建方法

配置nat和ip转发:

```
sysctl -w net.ipv4.ip_forward=1
iptables -t nat -A POSTROUTING -s 10.8.0.0/16 ! -d 10.8.0.0/16 -m comment --comment 'vpndemo' -j MASQUERADE
iptables -A FORWARD -s 10.8.0.0/16 -m state --state RELATED,ESTABLISHED -j ACCEPT
iptables -A FORWARD -d 10.8.0.0/16 -j ACCEPT
```

运行服务器:

```sh
> git clone https://github.com/markity/go-proxy.git
> cd ./go-proxy/server/
> sudo go run .
```

### 客户端连接服务器

```
> cd ./go-proxy/client
> go build .
> sudo ./client -u markity -p 12345
```

### 配置用户名密码

打开本项目的server/user_table.go, 自己照葫芦画瓢加几个用户就行了:

```go
func init() {
	UserMap["username1"] = "password1"
	UserMap["username2"] = "password2"
}
```
### 说明:

没有考虑IPV6

### 客户端

0. 假设客户端的ip是182.169.22.1, 服务端的ip地址是179.22.94.11
1. 初始化dtls connection, 并连接上远端
2. 客户端主动发送自己的认证信息包, 等待服务端分配IP
3. 服务端主动发来分包, 里面包含了两个地址, LOCAL_IPV4(比如10.1.0.1), PEER_IPV4(10.1.0.2), 它们是从服务器的ip池里面取得的
4. 客户端配置好tun设备, 创建好tun0:

```sh
ip link set tun1 up mtu 1500
ip addr add 10.1.0.2/32 peer 10.1.0.1/32 dev tun0
```

5. 客户端设置路由, 如下:

```sh
// 得到默认网关
DEFAULT_GW=$(ip route|grep default|cut -d' ' -f3)
// VPN地址走默认网关
ip route add 179.22.94.11/32 via $DEFAULT_GW
// 其他地址走tun设备网关
ip route add 0.0.0.0/1 via 10.1.0.2
ip route add 128.0.0.0/1 via 10.1.0.2
```

这样配置后, VPN_IP的流量走默认网关, 其它IP走PEER_IPV4的虚拟网卡。如果tun设备异常终止, 我们上面的路由会自动清除。

解释下网关的设置: 如果我们给google的IP发包, 这个包会由tun接手(read tun设备), tun此时把包转发给dtls connection, 由于VPN_IPV4走默认网关, 因此包能正常发给VPN服务端。

当服务端发来消息, 通过connection拿到, 此时要write tun设备。

5. 之前已经设置了把外部发来的IP包路由到tun设备 因此客户端不断读tun设备, 写入dtls connection。之前已经设置, VPN发来的包经过默认路由, 可以进入应用程序。

### 服务端
0. 服务端从IP池里面分配一个GW_IPV4(比如10.1.0.1):
1. 初始化dtls listener, 开启循环监听
2. 监听到一个连接connection, 等待用户发来认证信息包, 这里要检查不允许一个用户重复登陆(做一个登陆状态表就行)
3. 认证完毕后, 可以拿到用户的IPV4地址比如182.169.22.1, 为客户端从IP池里面拿取一个一个IP地址(比如10.1.0.2), 创建一个tunx(比如tun0), 进行如下配置

```sh
link set tun0 up mtu 1500
ip addr add 10.1.0.1/32 peer 10.1.0.2/32
```

4. 向客户端发送10.1.0.1和10.1.0.2

5. 做一个路由:

```sh
ip link set tunx up mtu 1500
// 用户的ipv4地址路由到tun设备
ip addr add 182.169.22.1/32 dev $TUN_NAME
ip route add 10.1.0.2 via 10.1.0.1
```

5. 服务端不断读tun设备, 然后写入connection

效果就是, 客户端发来的ip包将被路由到
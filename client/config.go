package main

import "time"

// 服务端的ip
var ServerIP = "43.134.205.40"
var ServerPort = 8000

// 连接超时
var ConnetTimeout = time.Second * 3

// 读超时, 如果超过3秒没有从客户端-服务端连接中读到数据包, 那么就视为超时, 此时客户端关闭连接
var ReadTimeout = time.Second * 3

// 客户端发送心跳包的频率, 服务端读到心跳包后也会发送心跳包
// 因此心跳包的间隔应该较小
var HeartInterval = time.Second * 1

// 需要配置的DNS服务器, 会写入到/etc/resolv.conf
var DNSServerIPS = []string{"1.1.1.1", "8.8.8.8"}

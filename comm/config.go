package comm

import "time"

var ConnectTimeout = time.Second * 5
var HeartbeatInterval = time.Second * 1
var MaxLostHeartbeatN = 5

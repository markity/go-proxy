package comm

import (
	"encoding/json"

	"inet.af/netaddr"
)

type PacketType int

var IPDispatchPacketType PacketType = 1
var IPPacketPacketType PacketType = 2
var HeartPacketType PacketType = 3
var UnknownPacketType PacketType = 4

type Header struct {
	Type string `json:"type"`
}

// --

// 1
type IPDispatchPacket struct {
	Type      string `json:"type"`
	ForClient string `json:"ip1"`
	ForServer string `json:"ip2"`
}

func (m *IPDispatchPacket) Pack() []byte {
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return b
}

func NewIPDispatchPacket(forClient netaddr.IP, forServer netaddr.IP) *IPDispatchPacket {
	return &IPDispatchPacket{Type: "ip_dispatch", ForClient: forClient.String(), ForServer: forServer.String()}
}

// --

// 2
type IPPacketPacket struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

func (m *IPPacketPacket) Pack() []byte {
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return b
}

func NewIPPacketPacket(data string) *IPPacketPacket {
	return &IPPacketPacket{Type: "ip_packet", Data: data}
}

// --

// 3
type HeartPacket struct {
	Type string `json:"type"`
}

func (m *HeartPacket) Pack() []byte {
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return b
}

func NewHeartPacket() *HeartPacket {
	return &HeartPacket{Type: "heart"}
}

// --

func ParsePacket(b []byte) PacketType {
	var header Header
	if err := json.Unmarshal(b, &header); err != nil {
		return UnknownPacketType
	}

	switch header.Type {
	case "heart":
		return HeartPacketType
	case "ip_dispatch":
		return IPDispatchPacketType
	case "ip_packet":
		return IPPacketPacketType
	default:
		return UnknownPacketType
	}
}

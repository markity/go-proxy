package comm

import (
	"encoding/json"

	"inet.af/netaddr"
)

var MagicHeart = []byte{0xaa, 0xbb, 0xcc}

type PacketType int

const (
	IPDispatchPacketType = iota
	IPPacketPacketType
	HeartPacketType
	UnknownPacketType
)

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

func ParsePacket(b []byte) interface{} {
	var header Header
	if err := json.Unmarshal(b, &header); err != nil {
		return UnknownPacketType
	}

	switch header.Type {
	case "heart":
		return NewHeartPacket()
	case "ip_dispatch":
		var p IPDispatchPacket
		json.Unmarshal(b, &p)
		return &p
	case "ip_packet":
		var p IPPacketPacket
		json.Unmarshal(b, &p)
		return &p
	default:
		return nil
	}
}

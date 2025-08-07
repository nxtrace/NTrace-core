package util

import (
	"encoding/binary"
	"errors"
)

func GetIPHeaderLength(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, errors.New("received invalid IP header")
	}
	version := data[0] >> 4
	switch version {
	case 4:
		ihl := int(data[0] & 0x0F)
		if ihl < 5 {
			return 0, errors.New("invalid IPv4 header length")
		}
		return ihl * 4, nil
	case 6:
		return 40, nil
	default:
		return 0, errors.New("unknown IP version")
	}
}

func GetICMPResponsePayload(data []byte) ([]byte, error) {
	if len(data) < 1 {
		return nil, errors.New("received invalid IP header")
	}
	version := data[0] >> 4
	hdrLen, err := GetIPHeaderLength(data)
	if err != nil {
		return nil, err
	}
	switch version {
	case 4:
		if len(data) < hdrLen {
			return nil, errors.New("inner IPv4 header too short")
		}
		return data[hdrLen:], nil
	case 6:
		if len(data) < hdrLen {
			return nil, errors.New("inner IPv6 header too short")
		}
		// NextHeader 字段在偏移 6
		next := data[6]
		offset := hdrLen // 从基础头后面开始处理扩展头
		// 遍历并跳过所有常见的扩展头
		for {
			switch next {
			case 0, 43, 44, 50, 51, 60:
				// 扩展头第 2 字节是 hdrExtLen（单位 8 字节）
				if offset+1 >= len(data) {
					return nil, errors.New("IPv6 extension header too short")
				}
				hdrExtLen := int(data[offset+1])
				extLen := (hdrExtLen + 1) * 8
				offset += extLen
				if offset > len(data) {
					return nil, errors.New("IPv6 extension header overflow")
				}
				// 更新下一个头部类型
				next = data[offset]
			default:
				// 遇到非扩展头（例如 TCP=6, UDP=17, ICMPv6=58 等），跳出
				goto PAYLOAD
			}
		}
	PAYLOAD:
		// 从 offset 开始即为 UDP/TCP 报文
		return data[offset:], nil
	default:
		return nil, errors.New("unknown IP version")
	}
}

func GetTCPSeq(data []byte) (uint32, error) {
	if len(data) < 8 {
		return 0, errors.New("length of tcp header too short")
	}
	seqBytes := data[4:8]
	return binary.BigEndian.Uint32(seqBytes), nil
}

func GetUDPSeq(data []byte) (uint16, error) {
	if len(data) < 1 {
		return 0, errors.New("received invalid IP header")
	}
	hdrLen, err := GetIPHeaderLength(data)
	if err != nil {
		return 0, err
	}
	if len(data) < hdrLen {
		return 0, errors.New("inner IPv4 header too short")
	}
	seqBytes := data[4:6]
	return binary.BigEndian.Uint16(seqBytes), nil
}

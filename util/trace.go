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
		next := data[6]
		offset := hdrLen
		for {
			switch next {
			case 0, 43, 60: // HBH, Routing, DestOpts
				if offset+2 > len(data) {
					return nil, errors.New("IPv6 ext too short")
				}
				hdrExtLen := int(data[offset+1])
				extLen := (hdrExtLen + 1) * 8
				if offset+extLen > len(data) {
					return nil, errors.New("IPv6 ext overflow")
				}
				next = data[offset] // Next Header 是扩展头的第 0 字节
				offset += extLen
			case 44: // Fragment
				if offset+8 > len(data) {
					return nil, errors.New("IPv6 frag too short")
				}
				next = data[offset] // 第 0 字节仍是 Next Header
				offset += 8
			case 51: // AH
				if offset+2 > len(data) {
					return nil, errors.New("IPv6 AH too short")
				}
				ahLen := int(data[offset+1]) // 单位 4 字节，不含前 2 个 32-bit
				extLen := (ahLen + 2) * 4
				if offset+extLen > len(data) {
					return nil, errors.New("IPv6 AH overflow")
				}
				next = data[offset]
				offset += extLen
			case 50: // ESP
				return nil, errors.New("IPv6 ESP encountered; cannot locate upper-layer")
			default:
				// 到达上层（TCP=6, UDP=17, ICMPv6=58 等）
				if offset > len(data) {
					return nil, errors.New("IPv6 offset out of range")
				}
				return data[offset:], nil
			}
		}
	default:
		return nil, errors.New("unknown IP version")
	}
}

func GetICMPID(data []byte) (int, error) {
	if len(data) < 6 {
		return 0, errors.New("length of icmp header too short for ID")
	}
	seqBytes := data[4:6]
	return int(binary.BigEndian.Uint16(seqBytes)), nil
}

func GetICMPSeq(data []byte) (int, error) {
	if len(data) < 8 {
		return 0, errors.New("length of icmp header too short for seq")
	}
	seqBytes := data[6:8]
	return int(binary.BigEndian.Uint16(seqBytes)), nil
}

func GetTCPPorts(data []byte) (int, int, error) {
	if len(data) < 4 {
		return 0, 0, errors.New("length of tcp header too short for ports")
	}
	srcPort := int(binary.BigEndian.Uint16(data[0:2]))
	dstPort := int(binary.BigEndian.Uint16(data[2:4]))
	return srcPort, dstPort, nil
}

func GetTCPSeq(data []byte) (int, error) {
	if len(data) < 8 {
		return 0, errors.New("length of tcp header too short for seq")
	}
	seqBytes := data[4:8]
	return int(binary.BigEndian.Uint32(seqBytes)), nil
}

func GetUDPPorts(data []byte) (int, int, error) {
	if len(data) < 4 {
		return 0, 0, errors.New("length of udp header too short for ports")
	}
	srcPort := int(binary.BigEndian.Uint16(data[0:2]))
	dstPort := int(binary.BigEndian.Uint16(data[2:4]))
	return srcPort, dstPort, nil
}

func GetUDPSeq(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, errors.New("received invalid IPv4 header")
	}
	hdrLen, err := GetIPHeaderLength(data)
	if err != nil {
		return 0, err
	}
	if len(data) < hdrLen {
		return 0, errors.New("length of IPv4 header too short for seq")
	}
	seqBytes := data[4:6]
	return int(binary.BigEndian.Uint16(seqBytes)), nil
}

func GetUDPSeqv6(data []byte) (int, error) {
	if len(data) < 8 {
		return 0, errors.New("length of udp header too short for seq")
	}
	seqBytes := data[6:8]
	return int(binary.BigEndian.Uint16(seqBytes)), nil
}

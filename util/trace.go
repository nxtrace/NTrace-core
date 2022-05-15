package util

import (
	"encoding/binary"
	"errors"
)

func GetIPHeaderLength(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, errors.New("received invalid IP header")
	}
	return int((data[0] & 0x0F) * 4), nil
}

func GetICMPResponsePayload(data []byte) ([]byte, error) {
	length, err := GetIPHeaderLength(data)
	if err != nil {
		return nil, err
	}

	if len(data) < length {
		return nil, errors.New("length of packet too short")
	}

	return data[length:], nil
}

func GetUDPSrcPort(data []byte) uint16 {
	srcPortBytes := data[:2]
	srcPort := binary.BigEndian.Uint16(srcPortBytes)
	return srcPort
}

func GetTCPSeq(data []byte) uint32 {
	seqBytes := data[4:8]
	return binary.BigEndian.Uint32(seqBytes)
}

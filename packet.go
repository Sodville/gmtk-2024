package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"net"
	"time"
)

type PacketType uint8

type Packet struct {
	PacketType  PacketType
	HeaderSize  uint32
	MagicBytes  uint32
	Timestamp   uint64
	PayloadSize uint32
	TotalSize   uint32
}

type PacketData struct {
	Packet Packet
	Data   []byte
	Addr   net.UDPAddr
}

const MAGICBYTES = 73458339

type ReconcilliationData struct {
	Name string
}

type Event struct {
	Type EventType

	// these fields are considered unions and can be safely considered nil
	Enemies   []Enemy
	Level     LevelEnum
	Modifiers []Modifiers
}

type ServerStateData struct {
	LevelEnum LevelEnum
	Timestamp time.Time
}

type CoordinateData struct {
	X float32
	Y float32
}

const (
	PacketTypeMatchFind PacketType = iota + 1
	PacketTypeMatchHost
	PacketTypeMatchStart
	PacketTypeMatchConnect
	PacketTypeNegotiate
	PacketTypeKeepAlive
	PacketTypeDisconnect
	PacketTypeUpdateCurrentPlayer
	PacketTypeUpdatePlayers
	PacketTypeBulletStart
	PacketTypePlayerHit
	PacketTypeServerEvent
	PacketTypeClientToggleReady
	PacketTypeServerStateChanged
	PacketTypePlayerRoll
	PacketTypeModifiersUpdated
	PacketTypeModifierChosen
)

type NegotiationResponse struct {
	Addr net.UDPAddr
}

func ValidatePacket(packet Packet) error {
	if packet.TotalSize != packet.HeaderSize+packet.PayloadSize {
		return errors.New("packet has invalid sizes")
	}

	if packet.MagicBytes != MAGICBYTES {
		return errors.New("packet has invalid magic bytes")
	}

	return nil
}

func DeserializePacket(data []byte) (Packet, []byte, error) {
	var packet Packet

	buf := make([]byte, 2048)
	_ = copy(buf, data)

	r := bytes.NewReader(buf)

	err := binary.Read(r, binary.BigEndian, &packet.PacketType)
	if err != nil {
		fmt.Println("error during decoding of packet type", err)
		return packet, nil, err
	}

	err = binary.Read(r, binary.BigEndian, &packet.HeaderSize)
	if err != nil {
		fmt.Println("error during decoding of header size", err)
		return packet, nil, err
	}

	err = binary.Read(r, binary.BigEndian, &packet.MagicBytes)
	if err != nil {
		fmt.Println("error during decoding of magic bytes", err)
		return packet, nil, err
	}

	err = binary.Read(r, binary.BigEndian, &packet.Timestamp)
	if err != nil {
		fmt.Println("error during decoding of timestamp", err)
		return packet, nil, err
	}

	err = binary.Read(r, binary.BigEndian, &packet.PayloadSize)
	if err != nil {
		fmt.Println("error during decoding of paylaod size", err)
		return packet, nil, err
	}

	err = binary.Read(r, binary.BigEndian, &packet.TotalSize)
	if err != nil {
		fmt.Println("error during decoding total size", err)
		return packet, nil, err
	}

	err = ValidatePacket(packet)
	if err != nil {
		fmt.Println("error during packet validation", err)
		return packet, nil, err
	}

	rawData := buf[packet.HeaderSize:packet.TotalSize]
	return packet, rawData, nil
}

func SerializePacket(packet Packet, data interface{}) ([]byte, error) {
	var buf bytes.Buffer

	// setting metadata
	packet.HeaderSize = 17 + 8
	packet.MagicBytes = MAGICBYTES

	packet.Timestamp = uint64(time.Now().UTC().UnixMilli())

	binary.Write(&buf, binary.BigEndian, packet.PacketType)
	binary.Write(&buf, binary.BigEndian, packet.HeaderSize)
	binary.Write(&buf, binary.BigEndian, packet.MagicBytes)
	binary.Write(&buf, binary.BigEndian, packet.Timestamp)

	dataBytes, err := serializeData(data)
	if err != nil {
		return nil, err
	}
	packet.PayloadSize = uint32(len(dataBytes))
	packet.TotalSize = uint32(buf.Len()+8) + uint32(len(dataBytes))
	// adding the 8 bytes from totalsize and payloadsize values

	binary.Write(&buf, binary.BigEndian, packet.PayloadSize)
	binary.Write(&buf, binary.BigEndian, packet.TotalSize)

	// Append encoded data
	buf.Write(dataBytes)

	return buf.Bytes(), nil
}

func serializeData(data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

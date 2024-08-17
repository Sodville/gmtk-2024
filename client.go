package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const SERVERPORT = 8081
const MEDIATION_SERVERPORT = 8080

type Client struct {
	conn           *net.UDPConn
	host_addr      net.UDPAddr
	other_pos      CoordinateData
	packet_channel chan PacketData
	connections    []ConnectedPlayer
	is_connected   bool

	ID uint
}

func (c *Client) IsSelf(addr net.UDPAddr) bool {
	split_strings := strings.Split(c.conn.LocalAddr().String(), ":")

	port, _ := strconv.Atoi(split_strings[len(split_strings)-1])

	if addr.Port == port {
		return true
	}

	return false
}

func (c *Client) listen() {
	buf := make([]byte, 1024)
	for {
		n, addr, err := c.conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("error reading", err)
		}

		packet, data, err := DeserializePacket(buf[:n])
		if err != nil {
			fmt.Println("error reading", err)
		}

		packet_data := PacketData{packet, data, *addr}
		c.packet_channel <- packet_data
	}
}

func (c *Client) SendPosition(pos Position) {
	packet := Packet{}
	packet.PacketType = PacketTypePositition

	raw_data, err := SerializePacket(packet, pos)
	if err != nil {
		fmt.Println("error serializing coordinate packet", err)
	}

	c.conn.WriteToUDP(raw_data, &c.host_addr)
}

func (c *Client) RunLocalClient() {
	conn, err := net.ListenUDP("udp", nil)
	c.conn = conn
	if err != nil {
		fmt.Println("Error dialing UDP:", err)
		return
	}
	defer conn.Close()

	data := ReconcilliationData{"Hello, server!"}

	packet := Packet{}

	// we don't use PacketTypeMatchConnect here because we can skip that
	// step due to the presumption that we are already through the NAT
	// if we can recieve these packets
	packet.PacketType = PacketTypeNegotiate

	// we know the host addr because we are the host addr
	c.host_addr = net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: SERVERPORT}

	// we know that he is connected be cause he is us
	c.is_connected = true

	raw_data, _ := SerializePacket(packet, data)
	_, err = conn.WriteToUDP(raw_data, &c.host_addr)
	if err != nil {
		fmt.Println("Error sending data:", err)
		return
	}

	c.packet_channel = make(chan PacketData)

	go c.listen()

	for {
		c.HandlePacket()
	}
}

func (c *Client) HandlePacket() {
	select {
	case packet_data := <-c.packet_channel:
		dec := gob.NewDecoder(bytes.NewReader(packet_data.Data))
		switch packet_data.Packet.PacketType {
		case PacketTypeMatchConnect:
			err := dec.Decode(&c.host_addr)

			fmt.Println(packet_data.Packet, c.host_addr)

			packet := Packet{}
			packet.PacketType = PacketTypeNegotiate
			data := ReconcilliationData{"Hey other client!"}

			raw_data, err := SerializePacket(packet, data)
			if err != nil {
				fmt.Println("error serializing packet", err)
			}

			_, err = c.conn.WriteToUDP(raw_data, &c.host_addr)
			if err != nil {
				fmt.Println("something went wrong when reaching out to match", err)
			}

			c.is_connected = true
		case PacketTypeUpdatePlayers:
			err := dec.Decode(&c.connections)

			if err != nil {
				fmt.Println("somethign went wrong when upating connections", err)
			}

		case PacketTypeNegotiate:
			_ = dec.Decode(&c.ID)

			c.host_addr = packet_data.Addr
			fmt.Println(c.ID)
		}

	case <-time.After(5 * time.Second):
		packet := Packet{}
		packet.PacketType = PacketTypeKeepAlive
		data := ReconcilliationData{"keepalive"}

		serialized_packet, _ := SerializePacket(packet, data)

		_, err := c.conn.WriteToUDP(serialized_packet, &c.host_addr)
		if err != nil {
			fmt.Println("something went wrong when keeping alive", err)
		}
	}
}

func (c *Client) RunClient(server_ip string) {
	conn, err := net.ListenUDP("udp", nil)
	c.conn = conn
	if err != nil {
		fmt.Println("Error dialing UDP:", err)
		return
	}
	defer conn.Close()

	data := ReconcilliationData{"Hello, server!"}

	packet := Packet{}
	packet.PacketType = PacketTypeMatchFind

	// other addr is server address, and will later be routed to the other client
	c.host_addr = net.UDPAddr{IP: net.ParseIP(server_ip), Port: MEDIATION_SERVERPORT}

	raw_data, _ := SerializePacket(packet, data)
	_, err = conn.WriteToUDP(raw_data, &c.host_addr)
	if err != nil {
		fmt.Println("Error sending data:", err)
		return
	}

	c.packet_channel = make(chan PacketData)

	go c.listen()

	for {
		c.HandlePacket()
	}
}

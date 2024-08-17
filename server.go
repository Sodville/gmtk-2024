package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"time"
)

type Server struct {
	mediation_server net.UDPAddr
	conn             *net.UDPConn
	connections      map[*net.UDPAddr]net.UDPAddr
	packet_channel   chan PacketData
	started          bool
}

func (s *Server) listen() {
	buf := make([]byte, 1024)
	for {
		n, addr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("error reading", err)
		}

		packet, data, err := DeserializePacket(buf[:n])
		if err != nil {
			fmt.Println("error reading", err)
		}

		packet_data := PacketData{packet, data, *addr}
		s.packet_channel <- packet_data
	}
}

func (s *Server) Broadcast(packet_data PacketData) {
	for value, _ := range s.connections {
		raw_data, err := SerializePacket(packet_data.Packet, packet_data.Data)
		if err != nil {
			fmt.Println("error serializing packet in Broadcast", err)
		}

		s.conn.WriteToUDP(raw_data, value)
	}
}

func (s *Server) Host(mediation_server_ip string) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: SERVERPORT})
	s.conn = conn
	if err != nil {
		fmt.Println("Error dialing UDP:", err)
		return
	}
	defer conn.Close()

	data := ReconcilliationData{"Hello, server!"}

	packet := Packet{}
	packet.PacketType = PacketTypeMatchHost

	s.mediation_server = net.UDPAddr{IP: net.ParseIP(mediation_server_ip), Port: MEDIATION_SERVERPORT}

	raw_data, _ := SerializePacket(packet, data)
	_, err = conn.WriteToUDP(raw_data, &s.mediation_server)
	if err != nil {
		fmt.Println("Error sending data:", err)
		return
	}

	s.packet_channel = make(chan PacketData)

	s.connections = make(map[*net.UDPAddr]net.UDPAddr)

	go s.listen()

	go func() {
		for {
			time.Sleep(time.Second * 2)

			packet = Packet{}
			packet.PacketType = PacketTypeKeepAlive
			serialized_packet, _ := SerializePacket(packet, ReconcilliationData{"keepalive"})

			_, err = conn.WriteToUDP(serialized_packet, &s.mediation_server)
			if err != nil {
				fmt.Println("something went wrong when reaching out to match", err)
			}
			if s.started {
				return
			}
		}
	}()

	for {
		select {
		case packet_data := <-s.packet_channel:
			dec := gob.NewDecoder(bytes.NewReader(packet_data.Data))
			switch packet_data.Packet.PacketType {
			case PacketTypeMatchConnect:
				var new_connection net.UDPAddr
				err = dec.Decode(&new_connection)
				packet = Packet{}
				packet.PacketType = PacketTypeNegotiate
				data = ReconcilliationData{"Hey other client!"}

				raw_data, err := SerializePacket(packet, data)
				if err != nil {
					fmt.Println("error serializing packet", err)
				}

				_, err = conn.WriteToUDP(raw_data, &new_connection)
				if err != nil {
					fmt.Println("something went wrong when reaching out to match", err)
				}

				s.connections[&new_connection] = new_connection
				fmt.Println("got new connection")
				fmt.Println("connections: ", s.connections)

			case PacketTypeNegotiate:
				var inner_data ReconcilliationData
				err = dec.Decode(&inner_data)

				// if we get this packet there is a presumption that we have already
				// broken through the NAT address by sending a packet to said address.

				// therefore we can safely assume that the incomming packet is from the owner we want to connect with
				// and then we can set the owner of the packet to our desired target address to assert the case
				s.connections[&packet_data.Addr] = packet_data.Addr

				fmt.Println(packet_data.Packet, inner_data)
			}

		case <-time.After(5 * time.Second):
			packet = Packet{}
			packet.PacketType = PacketTypeKeepAlive
			data = ReconcilliationData{"keepalive"}

			serialized_packet, _ := SerializePacket(packet, data)

			_, err = conn.WriteToUDP(serialized_packet, &s.mediation_server)
			if err != nil {
				fmt.Println("something went wrong when reaching out to match", err)
			}
		}
	}
}

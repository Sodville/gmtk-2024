package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"maps"
	"net"
	"time"
)

type ConnectedPlayer struct {
	Addr     net.UDPAddr
	Position Position

	// currently does not work
	ID uint
}

type Server struct {
	mediation_server net.UDPAddr
	conn             *net.UDPConn
	connection_keys  []string
	connections      map[string]ConnectedPlayer
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

func (s *Server) Broadcast(packet Packet, data any) {
	connections := maps.Clone(s.connections)
	for _, value := range connections {
		raw_data, err := SerializePacket(packet, data)
		if err != nil {
			fmt.Println("error serializing packet in Broadcast", err)
		}

		s.conn.WriteToUDP(raw_data, &value.Addr)
	}
}

func (s *Server) AddConnection(key string, new_connection ConnectedPlayer) {
	s.connection_keys = append(s.connection_keys, key)
	s.connections[key] = new_connection
}

func (s *Server) Host(mediation_server_ip string) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: SERVERPORT})
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

	s.connections = make(map[string]ConnectedPlayer)

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

	go func() {
		for {
			time.Sleep(time.Millisecond * 20)

			packet = Packet{}
			packet.PacketType = PacketTypeUpdatePlayers

			connected_player_list := []ConnectedPlayer{}
			for _, key := range s.connection_keys {
				value := s.connections[key]
				connected_player_list = append(connected_player_list, value)
			}

			s.Broadcast(packet, connected_player_list)
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

				new_player := ConnectedPlayer{new_connection, Position{}, uint(len(s.connections)) + 1}
				s.AddConnection(new_connection.String(), new_player)

				packet = Packet{}
				packet.PacketType = PacketTypeNegotiate
				data := new_player.ID

				raw_data, err := SerializePacket(packet, data)
				if err != nil {
					fmt.Println("error serializing packet", err)
				}

				_, err = conn.WriteToUDP(raw_data, &new_connection)
				if err != nil {
					fmt.Println("something went wrong when reaching out to match", err)
				}

				fmt.Println("got new connection with id ", data)
				fmt.Println("connections: ", s.connections)

			case PacketTypeNegotiate:
				var inner_data ReconcilliationData
				err = dec.Decode(&inner_data)

				// if we get this packet there is a presumption that we have already
				// broken through the NAT address by sending a packet to said address.

				// therefore we can safely assume that the incomming packet is from the owner we want to connect with
				// and then we can set the owner of the packet to our desired target address to assert the case
				for _, key := range s.connection_keys {
					if packet_data.Addr.String() == key {
						break
					}
				}
				s.AddConnection(packet_data.Addr.String(), ConnectedPlayer{packet_data.Addr, Position{}, uint(len(s.connections)) + 1})
			case PacketTypePositition:
				var position Position
				err = dec.Decode(&position)
				if err != nil {
					fmt.Println("error decoding position: ", err)
					fmt.Println("packet: ", packet_data.Packet)
					fmt.Println("packet: ", packet_data.Data)
					continue
				}
				player := s.connections[packet_data.Addr.String()]
				player.Position = position
				s.connections[packet_data.Addr.String()] = player
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

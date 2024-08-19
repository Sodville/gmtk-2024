package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"time"
)

type Hosts struct {
	Keyword string
	Addr    *net.UDPAddr
	Time    int64
}

func timeoutStaleConnections(keyword_map *map[string]Hosts) {
	for key, value := range map[string]Hosts(*keyword_map) {
		if time.Now().UnixMilli()-value.Time > 7000 {
			fmt.Printf("%s user timed out using '%s' connection key\n", value.Addr, value.Keyword)
			delete(*keyword_map, key)
		}
	}
}

func RunMediationServer() {
	server_addr, err := net.ResolveUDPAddr("udp", ":8080")
	if err != nil {
		fmt.Println("Error resolving address:", err)
		return
	}

	var host_map map[string]Hosts
	host_map = make(map[string]Hosts)

	conn, err := net.ListenUDP("udp", server_addr)
	if err != nil {
		fmt.Println("Error listening:", err)
		return
	}

	fmt.Println("Listening")
	defer conn.Close()

	packet_channel := make(chan PacketData)

	go func() {
		for {
			timeoutStaleConnections(&host_map)
			time.Sleep(time.Second * 1)
		}
	}()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				fmt.Println("error reading", err)
			}

			packet, data, err := DeserializePacket(buf[:n])
			if err != nil {
				fmt.Println("error reading", err)
			}

			packet_data := PacketData{packet, data, *addr}
			packet_channel <- packet_data
		}
	}()

	for {
		select {
		case packet_data := <-packet_channel:
			dec := gob.NewDecoder(bytes.NewReader(packet_data.Data))
			switch packet_data.Packet.PacketType {
			case PacketTypeKeepAlive:
				// refreshing timeout
				for key, value := range host_map {
					if value.Addr.String() == packet_data.Addr.String() {
						value.Time = time.Now().UnixMilli()
						host_map[key] = value
					}
				}

			case PacketTypeMatchHost:
				var inner_data ReconcilliationData
				err := dec.Decode(&inner_data)
				if err != nil {
					fmt.Println("error during decoding", err)
				}

				// if already exists
				if host_map[inner_data.Name].Keyword != "" {
					break
				}

				host_map[inner_data.Name] = Hosts{inner_data.Name, &packet_data.Addr, time.Now().UnixMilli()}
				fmt.Println("added new host: ", inner_data)

			case PacketTypeMatchFind:
				var inner_data ReconcilliationData
				err := dec.Decode(&inner_data)
				if err != nil {
					fmt.Println("error during decoding", err)
				}

				// if already exists
				if host_map[inner_data.Name].Keyword != "" {
					fmt.Println("match found!")
					packet := Packet{}
					packet.PacketType = PacketTypeMatchConnect
					data := packet_data.Addr
					serialized_packet, err := SerializePacket(packet, data)
					if err != nil {
						fmt.Println("error during serialization", err)
					}

					host_addr := host_map[inner_data.Name].Addr
					_, err = conn.WriteToUDP(serialized_packet, host_addr)
					if err != nil {
						fmt.Println("error during sending packet", err)
					}

					data = *host_map[inner_data.Name].Addr
					serialized_packet, err = SerializePacket(packet, data)
					if err != nil {
						fmt.Println("error during serialization", err)
					}
					conn.WriteToUDP(serialized_packet, &packet_data.Addr)
				}
			case PacketTypeMatchStart:
				var inner_data ReconcilliationData
				err := dec.Decode(&inner_data)
				if err != nil {
					fmt.Println("error during decoding", err)
				}

				fmt.Printf("%s's server has started, and has been removed from eligible lobbies\n", packet_data.Addr.String())
				delete(host_map, inner_data.Name)
			}
		}
	}
}

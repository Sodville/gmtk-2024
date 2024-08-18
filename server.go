package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"math"
	"net"
	"sync"
	"time"
)

type ConnectedPlayer struct {
	Addr     net.UDPAddr
	Position Position

	// currently does not work
	ID uint
}

type HitInfo struct {
	Player ConnectedPlayer
	Damage int
}

type ServerEventType uint

const (
	ServerNewLevelEvent ServerEventType = iota + 1
)

type Server struct {
	mediation_server     net.UDPAddr
	conn                 *net.UDPConn
	connection_keys      []string
	connections          sync.Map
	packet_channel       chan PacketData
	started              bool
	bullets              []Bullet
	level                *Level
	packet_channel_mutex sync.Mutex
	event_channel        chan ServerEvent
}

func loadFromSyncMap[T any](key any, syncMap *sync.Map) (value T, ok bool) {
	anyValue, ok := syncMap.Load(key)
	if ok {
		value, ok := anyValue.(T)
		if ok {
			return value, true
		} else {
			log.Printf("loaded something that wasn't a %T from sync.Map!\n", value)
			return value, false
		}
	} else {
		log.Println("server tried to load non-present key from syncMap")
		return value, false
	}
}

func (s *Server) listen() {
	buf := make([]byte, 2048)
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
	for _, value := range s.connection_keys {
		raw_data, err := SerializePacket(packet, data)
		if err != nil {
			fmt.Println("error serializing packet in Broadcast", err)
		}

		player, ok := loadFromSyncMap[ConnectedPlayer](value, &s.connections)
		if ok {
			s.conn.WriteToUDP(raw_data, &player.Addr)
		}
	}
}

func (s *Server) ChangeLevel(levelType LevelEnum, when time.Time) {
	packet := Packet{}
	packet.PacketType = PacketTypeServerEvent

	new_event := ServerEvent{ServerStateData{levelType, when}, ServerNewLevelEvent}
	s.Broadcast(packet, new_event)

	s.event_channel <- new_event
}

func (s *Server) StartChangeLevel(levelType LevelEnum, when time.Time) {
	newLevel := Level{}
	LoadLevel(&newLevel, levelType)

	remaining := when.Sub(time.Now())

	time.Sleep(time.Duration(remaining))
	s.level = &newLevel
}

func (s *Server) HandleState() {
	for {
		select {
		case event_data := <-s.event_channel:
			switch event_data.Type {
			case ServerNewLevelEvent:
				go s.StartChangeLevel(event_data.State.LevelEnum, event_data.State.Timestamp)
			}
		}
	}
}

func (s *Server) Update() {
	bullets := []Bullet{}

	for _, bullet := range s.bullets {
		radians := bullet.Rotation
		x := math.Cos(radians)
		y := math.Sin(radians)

		bullet.Position.X += x * float64(bullet.Speed)
		bullet.Position.Y += y * float64(bullet.Speed)

		collision_object := s.level.CheckObjectCollision(bullet.Position)
		bullet.GracePeriod = max(0, bullet.GracePeriod-0.16)

		should_remove := false

		if bullet.GracePeriod == 0 {
			s.connections.Range(func(key, value any) bool {
				player, ok := value.(ConnectedPlayer)
				if ok {
					if bullet.Position.X < player.Position.X+TILE_SIZE &&
						bullet.Position.X+4 > player.Position.X && // 4 is width
						bullet.Position.Y < player.Position.Y+TILE_SIZE &&
						bullet.Position.Y+4 > player.Position.Y { // 4 is height
						packet := Packet{}
						packet.PacketType = PacketTypePlayerHit

						s.Broadcast(packet, HitInfo{player, 20}) // TODO: fix damage etc.
						should_remove = true
					}
				} else {
					log.Println("found something that wasn't a ConnectedPlayer iterating over sync.Map!")
				}
				// Iteration will stop if the function returns false for an element
				return true
			})
		}

		if collision_object != nil {
			should_remove = true
		}

		if !should_remove {
			bullets = append(bullets, bullet)
		}
	}
	s.bullets = bullets
}

func (s *Server) AddConnection(key string, new_connection ConnectedPlayer) {
	for _, value := range s.connection_keys {
		if key == value {
			return
		}
	}
	s.connection_keys = append(s.connection_keys, key)
	s.connections.Store(key, new_connection)
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
	s.event_channel = make(chan ServerEvent)

	s.connections = sync.Map{}

	go s.listen()
	go s.HandleState()

	go func() {
		for {
			time.Sleep(time.Second * 2)

			keepAlivePacket := Packet{}
			keepAlivePacket.PacketType = PacketTypeKeepAlive
			serialized_packet, _ := SerializePacket(keepAlivePacket, ReconcilliationData{"keepalive"})

			_, error := conn.WriteToUDP(serialized_packet, &s.mediation_server)
			if error != nil {
				fmt.Println("something went wrong when reaching out to match", error)
			}
			if s.started {
				return
			}
		}
	}()

	go func() {
		for {
			time.Sleep(time.Millisecond * SERVER_PLAYER_SYNC_DELAY_MS)

			updatePlayerPacket := Packet{}
			updatePlayerPacket.PacketType = PacketTypeUpdatePlayers

			connected_player_list := []ConnectedPlayer{}

			for _, key := range s.connection_keys {
				value, ok := loadFromSyncMap[ConnectedPlayer](key, &s.connections)
				if ok {
					connected_player_list = append(connected_player_list, value)
				}
			}

			s.Broadcast(updatePlayerPacket, connected_player_list)
		}
	}()

	for {
		select {
		case packet_data := <-s.packet_channel:
			s.packet_channel_mutex.Lock()
			dec := gob.NewDecoder(bytes.NewReader(packet_data.Data))
			switch packet_data.Packet.PacketType {
			case PacketTypeMatchConnect:
				var new_connection net.UDPAddr
				dec.Decode(&new_connection)

				// sync.Map (which is a struct) doesn't an equivalent method to len()
				new_player := ConnectedPlayer{new_connection, Position{}, uint(len(s.connection_keys)) + 1}
				s.AddConnection(new_connection.String(), new_player)

				negotiatePacket := Packet{}
				negotiatePacket.PacketType = PacketTypeNegotiate
				data := new_player.ID

				raw_data, error := SerializePacket(negotiatePacket, data)
				if error != nil {
					fmt.Println("error serializing packet", error)
				}

				_, error = conn.WriteToUDP(raw_data, &new_connection)
				if error != nil {
					fmt.Println("something went wrong when reaching out to match", error)
				}

				fmt.Println("got new connection with id ", data)
				fmt.Println("connections: ", &s.connections)

			case PacketTypeNegotiate:
				var inner_data ReconcilliationData
				dec.Decode(&inner_data)

				// if we get this packet there is a presumption that we have already
				// broken through the NAT address by sending a packet to said address.

				// therefore we can safely assume that the incomming packet is from the owner we want to connect with
				// and then we can set the owner of the packet to our desired target address to assert the case
				for _, key := range s.connection_keys {
					if packet_data.Addr.String() == key {
						break
					}
				}
				s.AddConnection(packet_data.Addr.String(), ConnectedPlayer{packet_data.Addr, Position{}, uint(len(s.connection_keys)) + 1})

			case PacketTypePositition:
				var position Position
				error := dec.Decode(&position)
				if error != nil {
					fmt.Println("error decoding position: ", error)
					fmt.Println("packet: ", packet_data.Packet)
					fmt.Println("packet: ", packet_data.Data)
					continue
				}
				player, ok := loadFromSyncMap[ConnectedPlayer](packet_data.Addr.String(), &s.connections)
				if ok {
					player.Position = position
					s.connections.Store(packet_data.Addr.String(), player)
				}

			case PacketTypeBulletStart:
				var bullet Bullet
				dec.Decode(&bullet)
				s.Broadcast(packet_data.Packet, bullet)

				bullet.GracePeriod = 1.5
				s.bullets = append(s.bullets, bullet)
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
		s.packet_channel_mutex.Unlock()
	}
}

package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"
)

type ConnectedPlayer struct {
	Addr           net.UDPAddr
	Position       Position
	Rotation       float64
	Weapon         WeaponType
	IsRolling      bool
	IsReady        bool
	TimeLastPacket uint64
	Life           int

	// currently does not work
	ID uint
}

type HitInfo struct {
	Player ConnectedPlayer
	Damage int
}
type ServerStateType uint

const (
	ServerStateWaitingRoom ServerStateType = iota + 1
	ServerStateStarting
	ServerStateShopping
	ServerStatePlaying
	ServerStateLevelCompleted
)

type EventType uint

const (
	NewLevelEvent EventType = iota + 1
	SpawnEnemiesEvent
	SpawnBoonEvent
	PrepareNewLevelEvent
)

type ServerStateContext struct {
	Time             time.Time
	Level            LevelEnum
	ModifiersOptions []Modifiers
	HasChosenOptions bool
}

type ServerState struct {
	State   ServerStateType
	Context ServerStateContext
}

type Server struct {
	mediation_server      net.UDPAddr
	conn                  *net.UDPConn
	connection_keys       []string
	connection_keys_mutex sync.RWMutex
	connections           sync.Map
	packet_channel        chan PacketData
	started               bool
	bullets               []Bullet
	bullets_mutex         sync.RWMutex
	level                 *Level
	State                 ServerState
	Enemies               []Enemy
	SpawnCooldown         float64
	Modifiers             Modifiers
	RemainingSpawnCycles  int
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

func (s *Server) CheckTimedOutPlayers() {
	s.connection_keys_mutex.RLock()
	newConnectionKeys := make([]string, 0)

	for _, conn := range s.connection_keys {
		player, ok := loadFromSyncMap[ConnectedPlayer](conn, &s.connections)
		if ok && int(player.TimeLastPacket)-int(time.Now().UnixMilli())+TIMEOUT_INTERVAL_MS > 0 {
			newConnectionKeys = append(newConnectionKeys, conn)
		}
	}
	s.connection_keys_mutex.RUnlock()

	s.connection_keys_mutex.Lock()
	s.connection_keys = newConnectionKeys
	s.connection_keys_mutex.Unlock()
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
	s.connection_keys_mutex.RLock()
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
	s.connection_keys_mutex.RUnlock()
}

func (s *Server) AllReady() bool {
	s.connection_keys_mutex.RLock()
	allReady := true
	for _, conn := range s.connection_keys {
		player, ok := loadFromSyncMap[ConnectedPlayer](conn, &s.connections)
		if ok {
			if !player.IsReady {
				allReady = false
			}
		}
	}
	s.connection_keys_mutex.RUnlock()

	return allReady

}

func (s *Server) getWaveDensity() int {
	return 1
}

func (s *Server) getNextLevel() LevelEnum {
	return LevelOne
}

func (s *Server) makeRandomModifiers() []Modifiers {

	additiveMod := Modifiers{}
	additiveMod.Monster = append(additiveMod.Monster, Modifier{ModifierCalcTypeAddi, ModifierTypeSpeed, 20.0})
	additiveMod.Player = append(additiveMod.Player, Modifier{ModifierCalcTypeAddi, ModifierTypeDamage, 20.0})

	multiMod := Modifiers{}
	multiMod.Monster = append(multiMod.Monster, Modifier{ModifierCalcTypeMulti, ModifierTypeDamage, 20.0})
	multiMod.Player = append(multiMod.Player, Modifier{ModifierCalcTypeMulti, ModifierTypeSpeed, 20.0})

	bothModifiers := []Modifiers{additiveMod, multiMod}
	return bothModifiers
}

func (s *Server) UpdateState() {
	if s.State.State == ServerStateWaitingRoom {
		if s.AllReady() {
			s.State.State = ServerStateStarting
			s.State.Context = ServerStateContext{}
			s.State.Context.Time = time.Now().Add(time.Second * 2)
		}
	} else if s.State.State == ServerStateStarting {
		if !s.AllReady() {
			s.State.State = ServerStateWaitingRoom
			s.State.Context = ServerStateContext{}
			s.State.Context.Level = LobbyLevel
		} else if s.State.Context.Time.Sub(time.Now()) <= 0 {
			s.State.State = ServerStatePlaying
			s.State.Context = ServerStateContext{}
			s.State.Context.Level = s.getNextLevel()

			s.SpawnCooldown = INITAL_SPAWN_COOLDOWN
			s.RemainingSpawnCycles = s.getWaveDensity()

			if !s.started {
				packet := Packet{}
				packet.PacketType = PacketTypeMatchStart

				data := ReconcilliationData{"Hello, server!"}
				raw_data, _ := SerializePacket(packet, data)
				_, err := s.conn.WriteToUDP(raw_data, &s.mediation_server)

				if err != nil {
					fmt.Println("error disconnecting from mediation server", err)
				}

				s.started = true
			}

			LoadLevel(s.level, s.State.Context.Level)
		}
	} else if s.State.State == ServerStatePlaying {
		log.Println(s.RemainingSpawnCycles, len(s.Enemies))
		if s.SpawnCooldown == 0 && s.RemainingSpawnCycles > 0 {
			s.RemainingSpawnCycles--
			s.StartSpawnMonsterEvent()
		} else if s.RemainingSpawnCycles <= 0 && len(s.Enemies) == 0 {
			s.State.State = ServerStateLevelCompleted

			s.State.Context = ServerStateContext{}
			s.State.Context.ModifiersOptions = s.makeRandomModifiers()
			s.State.Context.HasChosenOptions = false
		}
	} else if s.State.State == ServerStateLevelCompleted {
		if s.State.Context.HasChosenOptions {
			s.State.State = ServerStateStarting
			s.State.Context.Time = time.Now().Add(time.Second * 2)
		}
	}
}

func (s *Server) CheckState() {
	oldState := s.State.State
	s.UpdateState()

	if oldState != s.State.State {
		log.Println("server changes state to", s.State.State)
		packet := Packet{}
		packet.PacketType = PacketTypeServerStateChanged
		s.Broadcast(packet, s.State)
	}
}
func (s *Server) SetSpawnCooldown() float64 {
	r := rand.New(rand.NewSource(99))

	return float64(MINIMUM_SPAWN_COOLDOWN + r.Intn(MINIMUM_SPAWN_COOLDOWN))
}

func (s *Server) StartSpawnMonsterEvent() {
	totalWidth := s.level.Map.Width * TILE_SIZE
	totalHeight := s.level.Map.Height * TILE_SIZE

	r := rand.New(rand.NewSource(time.Now().Unix()))

	desiredX := r.Intn(totalWidth)
	desiredY := r.Intn(totalHeight)

	radius := 80
	EnemiesToSpawn := []Enemy{}
	for i := 0; i < r.Intn(MAX_SPAWN_COUNT); i++ {
		X := r.Intn(radius*2) - radius
		Y := r.Intn(radius*2) - radius

		// clamping inside arena
		x := float64(max(0, min(s.level.Map.Width * TILE_SIZE, X + desiredX)))
		y := float64(max(0, min(s.level.Map.Height * TILE_SIZE, Y + desiredY)))

		enemy := Enemy{
			CharacterZombie,
			Position{x, y},
			0,
			0,
			GetLifeForCharacter(CharacterZombie),
		}

		collision := s.level.CheckObjectCollision(enemy.Position)
		if collision == nil {
			EnemiesToSpawn = append(EnemiesToSpawn, enemy)
		}

	}

	event := Event{}
	event.Enemies = EnemiesToSpawn
	event.Type = SpawnEnemiesEvent

	packet := Packet{}
	packet.PacketType = PacketTypeServerEvent
	s.Broadcast(packet, event)

	s.Enemies = append(s.Enemies, EnemiesToSpawn...)
	s.SpawnCooldown = s.SetSpawnCooldown()
}

func (s *Server) Update() {
	bullets := []Bullet{}

	s.bullets_mutex.RLock()
	for _, bullet := range s.bullets {
		radians := bullet.Rotation
		x := math.Cos(radians)
		y := math.Sin(radians)

		bullet.Position.X += x * float64(bullet.Speed)
		bullet.Position.Y += y * float64(bullet.Speed)

		collision_object := s.level.CheckObjectCollision(bullet.Position)
		bullet.GracePeriod = max(0, bullet.GracePeriod-0.16)

		should_remove := false
		damage := GetWeaponDamage(bullet.WeaponType)

		// it's our bullet shooting enemies, pew pew
		if !bullet.HurtsPlayer {
			for key, enemy := range s.Enemies {
				if bullet.Position.X < enemy.Position.X+TILE_SIZE &&
					bullet.Position.X+4 > enemy.Position.X && // 4 is width
					bullet.Position.Y < enemy.Position.Y+TILE_SIZE &&
					bullet.Position.Y+4 > enemy.Position.Y { // 4 is height
					should_remove = true
					log.Println("hit enemy", enemy.Life)
					s.Enemies[key].Life = max(0, enemy.Life-damage)

				}
			}
		} else if bullet.GracePeriod == 0 {
			s.connections.Range(func(key, value any) bool {
				player, ok := value.(ConnectedPlayer)
				if ok {
					if bullet.Position.X < player.Position.X+TILE_SIZE &&
						bullet.Position.X+4 > player.Position.X && // 4 is width
						bullet.Position.Y < player.Position.Y+TILE_SIZE &&
						bullet.Position.Y+4 > player.Position.Y { // 4 is height
						if !player.IsRolling {
							packet := Packet{}
							packet.PacketType = PacketTypePlayerHit

							s.Broadcast(packet, HitInfo{player, damage}) // TODO: fix damage etc.
						}
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
	s.bullets_mutex.RUnlock()
	s.bullets_mutex.Lock()
	s.bullets = bullets
	s.bullets_mutex.Unlock()

	s.CheckState()

	s.SpawnCooldown = max(0, s.SpawnCooldown-0.16)

	enemies := []Enemy{}
	for key := range s.Enemies {
		enemy := s.Enemies[key]

		enemy.Update()

		if enemy.Life > 0 {
			enemies = append(enemies, s.Enemies[key])
		}
	}
	s.Enemies = enemies

	s.CheckTimedOutPlayers()
}

// Note that calls of this method should be protected by write-locking connection_keys_mutex
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

	s.connections = sync.Map{}

	s.State.State = ServerStateWaitingRoom

	go s.listen()

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

			s.connection_keys_mutex.RLock()
			for _, key := range s.connection_keys {
				value, ok := loadFromSyncMap[ConnectedPlayer](key, &s.connections)
				if ok {
					connected_player_list = append(connected_player_list, value)
				}
			}
			s.connection_keys_mutex.RUnlock()

			s.Broadcast(updatePlayerPacket, connected_player_list)
		}
	}()

	for {
		select {
		case packet_data := <-s.packet_channel:
			dec := gob.NewDecoder(bytes.NewReader(packet_data.Data))
			switch packet_data.Packet.PacketType {
			case PacketTypeMatchConnect:
				var new_connection net.UDPAddr
				dec.Decode(&new_connection)

				s.connection_keys_mutex.Lock()
				// sync.Map (which is a struct) doesn't have an equivalent method to len()
				new_player := ConnectedPlayer{
					new_connection,
					Position{},
					0,
					0,
					false,
					false,
					packet_data.Packet.Timestamp,
					PLAYER_LIFE,
					uint(len(s.connection_keys)) + 1,
				}
				s.AddConnection(new_connection.String(), new_player)
				s.connection_keys_mutex.Unlock()

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
				s.connection_keys_mutex.Lock()
				for _, key := range s.connection_keys {
					if packet_data.Addr.String() == key {
						break
					}
				}
				s.AddConnection(packet_data.Addr.String(), ConnectedPlayer{
					packet_data.Addr,
					Position{},
					0,
					0,
					false,
					false,
					packet_data.Packet.Timestamp,
					PLAYER_LIFE,
					uint(len(s.connection_keys)) + 1},
				)
				s.connection_keys_mutex.Unlock()

			case PacketTypeUpdateCurrentPlayer:
				var playerUpdate PlayerUpdateData
				decode_err := dec.Decode(&playerUpdate)
				if decode_err != nil {
					fmt.Println("error decoding player update: ", decode_err)
					continue
				}

				player, ok := loadFromSyncMap[ConnectedPlayer](packet_data.Addr.String(), &s.connections)
				if ok {
					player.Position = playerUpdate.Position
					player.Rotation = playerUpdate.Rotation
					player.Weapon = playerUpdate.Weapon
					player.IsRolling = playerUpdate.isRolling
					player.TimeLastPacket = packet_data.Packet.Timestamp

					s.connections.Store(packet_data.Addr.String(), player)
				}

			case PacketTypeClientToggleReady:
				if s.started {continue}
				player, ok := loadFromSyncMap[ConnectedPlayer](packet_data.Addr.String(), &s.connections)
				if ok {
					player.IsReady = !player.IsReady
				}
				s.connections.Store(packet_data.Addr.String(), player)

			case PacketTypePlayerHit:
				var hitInfo HitInfo
				dec.Decode(&hitInfo)
				s.Broadcast(packet_data.Packet, hitInfo)

			case PacketTypeModifierChosen:
				var modifiers Modifiers
				dec.Decode(&modifiers)

				s.State.Context.HasChosenOptions = true

				s.Modifiers.Add(modifiers)
				packet := Packet{}
				packet.PacketType = PacketTypeModifiersUpdated

				s.Broadcast(packet, s.Modifiers)

			case PacketTypeBulletStart:
				var bullet Bullet
				dec.Decode(&bullet)
				s.Broadcast(packet_data.Packet, bullet)

				bullet.GracePeriod = 1.5
				s.bullets_mutex.Lock()
				s.bullets = append(s.bullets, bullet)
				s.bullets_mutex.Unlock()

			case PacketTypePlayerRoll:
				player, ok := loadFromSyncMap[ConnectedPlayer](packet_data.Addr.String(), &s.connections)
				if ok {
					player.IsRolling = true
				}
				s.connections.Store(packet_data.Addr.String(), player)
			}
		}
	}
}

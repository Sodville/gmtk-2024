package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const SERVERPORT = 8081
const MEDIATION_SERVERPORT = 8080

type Client struct {
	conn                *net.UDPConn
	host_addr           net.UDPAddr
	packet_channel      chan PacketData
	player_states       map[string]PlayerState
	player_states_mutex sync.RWMutex
	bullets             []Bullet
	bullets_mutex       sync.RWMutex
	is_connected        bool
	event_channel       chan Event
	readyPlayersCount   uint
	playerCount         uint
	ServerState         ServerState
	PlayerLifePtr       *int
	Modifiers           *Modifiers

	ID uint
}

type Bullet struct {
	Position    Position
	Rotation    float64
	WeaponType  WeaponType
	Speed       float32
	GracePeriod float64
	HurtsPlayer bool
}

type PlayerState struct {
	Connection          ConnectedPlayer
	PreviousPos         Position
	PreviousRelativePos Position
	CurrentPos          Position
	MoveDuration        int
	FrameCount          uint
	RollDuration        float64
	RollSpeed           float64
}

type PlayerUpdateData struct {
	Position  Position
	Rotation  float64
	Weapon    WeaponType
	isRolling bool
	Life      int
}

func (c *Client) Self() *ConnectedPlayer {
	for _, player := range c.player_states {
		if c.IsSelf(player.Connection.Addr) {
			return &player.Connection
		}
	}

	return nil
}

func (c *Client) GetStateByAddr(addr string) *PlayerState {
	for _, player := range c.player_states {
		if player.Connection.Addr.String() == addr {
			return &player
		}
	}
	return nil
}

func (ps *PlayerState) GetInterpolatedPos() Position {
	// estimated frame count between packets
	// f := 1000.0 / SERVER_PLAYER_SYNC_DELAY_MS
	f := 6.0

	x := ps.PreviousPos.X + float64(ps.FrameCount)/f*(ps.CurrentPos.X-ps.PreviousPos.X)
	y := ps.PreviousPos.Y + float64(ps.FrameCount)/f*(ps.CurrentPos.Y-ps.PreviousPos.Y)

	return Position{x, y}
}

func (c *Client) ToggleReady() {
	packet := Packet{}
	packet.PacketType = PacketTypeClientToggleReady

	raw_data, err := SerializePacket(packet, Packet{}) // this second packet is dead
	if err != nil {
		fmt.Println("error serializing ready packet", err)
	}

	c.conn.WriteToUDP(raw_data, &c.host_addr)
}

func (c *Client) IsReady() bool {
	for _, player := range c.player_states {
		if c.IsSelf(player.Connection.Addr) {
			return player.Connection.IsReady
		}
	}

	fmt.Println("could not figure it out if we are ready")
	return false
}

func (c *Client) IsSelf(addr net.UDPAddr) bool {
	split_strings := strings.Split(c.conn.LocalAddr().String(), ":")

	port, _ := strconv.Atoi(split_strings[len(split_strings)-1])

	if addr.Port == port {
		return true
	}

	return false
}

func (c *Client) SendChosenModifiers(modifiers Modifiers) {
	packet := Packet{}
	packet.PacketType = PacketTypeModifierChosen

	raw_data, err := SerializePacket(packet, modifiers)
	if err != nil {
		fmt.Println("error serializing modifiers packet", err)
	}

	c.conn.WriteToUDP(raw_data, &c.host_addr)
}

func (c *Client) SendHit(hit HitInfo) {
	packet := Packet{}
	packet.PacketType = PacketTypePlayerHit

	raw_data, err := SerializePacket(packet, hit)
	if err != nil {
		fmt.Println("error serializing bullet packet", err)
	}

	c.conn.WriteToUDP(raw_data, &c.host_addr)
}

func (c *Client) SendShoot(bullet Bullet) {
	packet := Packet{}
	packet.PacketType = PacketTypeBulletStart

	raw_data, err := SerializePacket(packet, bullet)
	if err != nil {
		fmt.Println("error serializing bullet packet", err)
	}

	c.conn.WriteToUDP(raw_data, &c.host_addr)
}

func (c *Client) SendRoll() {
	packet := Packet{}
	packet.PacketType = PacketTypePlayerRoll

	raw_data, err := SerializePacket(packet, Packet{})
	if err != nil {
		fmt.Println("error serializing roll packet", err)
	}

	c.conn.WriteToUDP(raw_data, &c.host_addr)
}

func (c *Client) listen() {
	buf := make([]byte, 2048)
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

func (c *Client) SendPosition(pos Position, rotation float64, weapon WeaponType, isRolling bool, life int) {
	packet := Packet{}
	packet.PacketType = PacketTypeUpdateCurrentPlayer

	raw_data, err := SerializePacket(packet,
		PlayerUpdateData{
			pos,
			rotation,
			weapon,
			isRolling,
			life,
		})
	if err != nil {
		fmt.Println("error serializing coordinate packet", err)
	}

	c.conn.WriteToUDP(raw_data, &c.host_addr)
}

func (c *Client) HandleServerState(state ServerState) {
	if state.State == ServerStatePlaying {
		event := Event{}
		event.Type = NewLevelEvent
		event.Level = state.Context.Level

		go func() { c.event_channel <- event }()
	}

	if state.State == ServerStateStarting {
		event := Event{}
		event.Type = PrepareNewLevelEvent
		go func() { c.event_channel <- event }()
	}
	if state.State == ServerStateLevelCompleted {
		event := Event{}
		event.Type = SpawnBoonEvent
		event.Modifiers = state.Context.ModifiersOptions
		go func() { c.event_channel <- event }()
	}

	if state.State == ServerStateGameOver  {
		event := Event{}
		event.Type = GameOverEvent

		go func() { c.event_channel <- event }()
	}

	if state.State == ServerStateWaitingRoom {
		event := Event{}
		event.Type = NewLevelEvent
		event.Level = state.Context.Level

		*c.PlayerLifePtr = PLAYER_LIFE

		go func() { c.event_channel <- event }()
	}
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
	c.event_channel = make(chan Event)

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
		case PacketTypeBulletStart:
			var bullet Bullet
			err := dec.Decode(&bullet)

			if err != nil {
				fmt.Println("something went wrong decoding bullet", err)
			}
			c.bullets_mutex.Lock()
			c.bullets = append(c.bullets, bullet)
			c.bullets_mutex.Unlock()

		case PacketTypePlayerHit:
			var hitInfo HitInfo
			err := dec.Decode(&hitInfo)

			if c.IsSelf(hitInfo.Player.Addr) {
				// double check this
				*c.PlayerLifePtr -= hitInfo.Damage
			}
			state := c.GetStateByAddr(hitInfo.Player.Addr.String())
			if state.Connection.Life - hitInfo.Damage < 1 {
				go func () {
					event := Event{}
					event.Type = PlayerDiedEvent
					event.Player = state.Connection
					c.event_channel <- event }()
				}

			if err != nil {
				fmt.Println("something went wrong when decoding hit info", err)
			}

		case PacketTypeUpdatePlayers:
			var connections []ConnectedPlayer
			states := make(map[string]PlayerState)
			err := dec.Decode(&connections)

			if err != nil {
				fmt.Println("something went wrong when updating connections", err)
			}

			c.player_states_mutex.Lock()
			var readyPlayerCount uint = 0
			for _, pConn := range connections {
				id := pConn.Addr.String()
				ps, ok := c.player_states[id]
				if pConn.IsReady {
					readyPlayerCount++
				}
				if ok {
					ps.Connection = pConn
					ps.PreviousPos = ps.CurrentPos
					ps.CurrentPos = pConn.Position
					ps.FrameCount = 0
					ps.RollDuration = min(0, ps.RollDuration+ps.RollSpeed*0.085)
					states[id] = ps
				} else {
					states[id] = PlayerState{
						Connection:   pConn,
						MoveDuration: 0,
						FrameCount:   0,
						PreviousPos:  pConn.Position,
						CurrentPos:   pConn.Position,
					}
				}
			}
			c.readyPlayersCount = readyPlayerCount
			c.playerCount = uint(len(c.player_states))

			c.player_states = states
			c.player_states_mutex.Unlock()

		case PacketTypeNegotiate:
			_ = dec.Decode(&c.ID)

			c.host_addr = packet_data.Addr
			fmt.Println(c.ID)

		case PacketTypeServerStateChanged:
			var state ServerState
			_ = dec.Decode(&state)

			c.ServerState = state
			c.HandleServerState(c.ServerState)

		case PacketTypeModifiersUpdated:
			_ = dec.Decode(c.Modifiers)

		case PacketTypeServerEvent:
			var event Event
			_ = dec.Decode(&event)

			go func() { c.event_channel <- event }()

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

func (c *Client) CheckConnected() bool {
	endTime := time.Now().Add(time.Second * 2)
	for {
		if time.Now().After(endTime) {
			return false
		}

		if c.is_connected {
			return true
		}
	}
}

func (c *Client) RunClient(server_ip string, key string) {
	conn, err := net.ListenUDP("udp", nil)
	c.conn = conn
	if err != nil {
		fmt.Println("Error dialing UDP:", err)
		return
	}
	defer conn.Close()

	data := ReconcilliationData{key}

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
	c.event_channel = make(chan Event)

	go c.listen()

	for {
		c.HandlePacket()
	}
}

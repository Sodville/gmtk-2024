package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	SCREEN_WIDTH  = 320
	SCREEN_HEIGHT = 240
	TILE_SIZE     = 16
)

type Position struct {
	X, Y int
}

type Player struct {
	Position Position
	Speed    int
}

type Game struct {
	Player     Player
	Client     *Client
	FrameCount uint64
}

func CheckCollision(pos Position) bool {
	if pos.X < 1 || pos.X > SCREEN_WIDTH-TILE_SIZE {
		return true
	}
	if pos.Y < 1 || pos.Y > SCREEN_HEIGHT-TILE_SIZE {
		return true
	}
	return false
}

func (g *Game) Update() error {
	g.FrameCount++

	if g.Client.is_connected {
		g.Client.SendPosition(g.Player.Position)
	}

	g.Player.Update()
	if ebiten.IsKeyPressed(ebiten.KeyQ) {
		return ebiten.Termination
	}

	return nil
}

func (p *Player) Draw(screen *ebiten.Image) {
	vector.DrawFilledCircle(screen, float32(p.Position.X), float32(p.Position.Y), 15, color.White, false)
}

func (p *Player) Update() {
	player_pos := &p.Position
	init_player_pos := *player_pos

	if ebiten.IsKeyPressed(ebiten.KeyW) {
		player_pos.Y -= p.Speed
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		player_pos.Y += p.Speed
	}

	if CheckCollision(*player_pos) {
		player_pos.Y = init_player_pos.Y
	}

	if ebiten.IsKeyPressed(ebiten.KeyA) {
		player_pos.X -= p.Speed
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		player_pos.X += p.Speed
	}
	if CheckCollision(*player_pos) {
		player_pos.X = init_player_pos.X
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	tps := ebiten.ActualTPS()
	ebitenutil.DebugPrint(screen, fmt.Sprintf("TPS: %f", tps))

	g.Player.Draw(screen)

	for _, connection := range g.Client.connections {
		vector.DrawFilledCircle(screen, float32(connection.Position.X), float32(connection.Position.Y), 15, color.White, false)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return SCREEN_WIDTH, SCREEN_HEIGHT
}

func main() {
	is_server := flag.String("server", "y", "run server")
	is_host := flag.String("host", "n", "host")
	server_ip := flag.String("ip", "84.215.22.166", "ip")

	flag.Parse()

	if *is_server == "y" {
		RunMediationServer()
		return
	}

	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Hello, World!")

	client := Client{}
	if *is_host == "n" {
		go client.RunClient(*server_ip)

	} else {
		server := Server{}
		go server.Host(*server_ip)
		go client.RunLocalClient()
	}

	game := Game{Player: Player{Speed: TILE_SIZE, Position: Position{1, 1}}, Client: &client}

	if err := ebiten.RunGame(&game); err != nil {
		log.Fatal(err)
	}
}

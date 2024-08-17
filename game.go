package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"math"

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
	X, Y float64
}

type Delta struct {
	dX, dY float64
}

type Player struct {
	Position Position
	Speed    float64
}

type Game struct {
	Player     Player
	Client     *Client
	FrameCount uint64
}

func CheckCollisionX(pos *Position, delta *Delta) float64 {
	if pos.X+delta.dX < 1 {
		return 1
	}
	if pos.X+delta.dX > SCREEN_WIDTH-TILE_SIZE {
		return float64(SCREEN_WIDTH - TILE_SIZE)
	}
	return -1
}

func CheckCollisionY(pos *Position, delta *Delta) float64 {
	if pos.Y+delta.dY < 1 {
		return 1
	}
	if pos.Y+delta.dY > SCREEN_HEIGHT-TILE_SIZE {
		return float64(SCREEN_HEIGHT - TILE_SIZE)
	}
	return -1
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
	var delta Delta

	if ebiten.IsKeyPressed(ebiten.KeyW) {
		delta.dY -= p.Speed
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		delta.dY += p.Speed
	}

	if ebiten.IsKeyPressed(ebiten.KeyA) {
		delta.dX -= p.Speed
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		delta.dX += p.Speed
	}

	if delta.dX != 0 && delta.dY != 0 {
		factor := p.Speed / math.Sqrt(delta.dX*delta.dX+delta.dY*delta.dY)
		delta.dX *= factor
		delta.dY *= factor
	}

	collX := CheckCollisionX(&p.Position, &delta)
	if collX != -1 {
		p.Position.X = collX
	} else {
		p.Position.X += delta.dX
	}

	collY := CheckCollisionY(&p.Position, &delta)
	if collY != -1 {
		p.Position.Y = collY
	} else {
		p.Position.Y += delta.dY

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

package main

import (
	"flag"
	"fmt"
	"image"
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
	Chain    []Position
}

type Game struct {
	Player     Player
	Client     *Client
	FrameCount uint64
}

func CheckCollision(pos Position) bool {
	return false
}

func drawStackedSprites(screen *ebiten.Image, sprites []*ebiten.Image, rotation float64, x, y, offset int) {
	for i, sprite := range sprites {
		op := &ebiten.DrawImageOptions{}

		spriteWidth := sprite.Bounds().Size().X
		spriteHeight := sprite.Bounds().Size().Y
		op.GeoM.Translate(float64(-spriteWidth/2), float64(-spriteHeight/2)) // Center the sprite
		op.GeoM.Rotate(rotation / 3.14)
		op.GeoM.Translate(float64(spriteWidth/2), float64(spriteHeight/2)) // Re-adjust the center back
		op.GeoM.Translate(float64(x), float64(y+-i*offset))

		screen.DrawImage(sprite, op)
	}
}

func (g *Game) Update() error {
	g.FrameCount++
	//g.Client.SendPosition(CoordinateData{float32(g.Player.Position.X), float32(g.Player.Position.Y)})

	g.Player.Update()
	if ebiten.IsKeyPressed(ebiten.KeyQ) {
		return ebiten.Termination
	}

	//fmt.Println(g.Player.Position)

	return nil
}

func (p *Player) Draw(screen *ebiten.Image) {
	for _, chain := range p.Chain {
		vector.DrawFilledCircle(screen, float32(p.Position.X+chain.X), float32(p.Position.Y+chain.Y), 15, color.White, false)
	}
}

func (p *Player) Update() {
	player_pos := &p.Position
	init_player_pos := player_pos

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

}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return SCREEN_WIDTH, SCREEN_HEIGHT
}

func splitSpriteSheet(spriteSheet *ebiten.Image, spriteHeight int) []*ebiten.Image {
	var sprites []*ebiten.Image

	sheetWidth := spriteSheet.Bounds().Size().X
	sheetHeight := spriteSheet.Bounds().Size().Y

	for y := sheetHeight; y > 0; y -= spriteHeight {
		spriteRect := image.Rect(0, y, sheetWidth, y-spriteHeight)

		sprite := spriteSheet.SubImage(spriteRect).(*ebiten.Image)

		sprites = append(sprites, sprite)
	}

	return sprites
}

func main() {
	is_server := flag.String("server", "y", "run server")
	is_host := flag.String("host", "n", "host")
	server_ip := flag.String("ip", "192.168.0.48", "ip")

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

	chain := []Position{
		{0, 0},
		{10, 0},
		{20, 0},
	}
	game := Game{Player: Player{Speed: TILE_SIZE, Chain: chain}, Client: &client}

	if err := ebiten.RunGame(&game); err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"bytes"
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"log"
	"math"

	"github.com/lafriks/go-tiled"
	"github.com/lafriks/go-tiled/render"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	SCREEN_WIDTH  = 320
	SCREEN_HEIGHT = 240
	TILE_SIZE     = 16
	PLAYER_SPEED  = 2
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
	Sprite   *ebiten.Image
}

type Level struct {
	MapImage *ebiten.Image
	Map *tiled.Map
}

type Camera struct {
	Offset Position
}

type Game struct {
	Player     Player
	Client     *Client
	FrameCount uint64
	Level      *Level
	Camera     Camera
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

	if g.Client.is_connected && (g.FrameCount % 3 == 0) {
		g.Client.SendPosition(g.Player.Position)
	}

	g.Player.Update()

	camera_target_pos := Position{g.Player.Position.X - SCREEN_WIDTH / 2 , g.Player.Position.Y - SCREEN_HEIGHT / 2}
	g.Camera.Update(camera_target_pos)
	if ebiten.IsKeyPressed(ebiten.KeyQ) {
		return ebiten.Termination
	}
	return nil
}

func (p *Player) Draw(screen *ebiten.Image, camera Camera) {
	op := camera.GetCameraDrawOptions()

	op.GeoM.Translate(p.Position.X, p.Position.Y)

	screen.DrawImage(p.Sprite, op)
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

func (c *Camera) Update(target_pos Position) {
	coefficient := 20.0
	c.Offset.X += (target_pos.X - c.Offset.X) / coefficient
	c.Offset.Y += (target_pos.Y - c.Offset.Y) / coefficient
}

func (c *Camera) GetCameraDrawOptions() (*ebiten.DrawImageOptions) {
	op := ebiten.DrawImageOptions{}
	op.GeoM.Translate(-c.Offset.X, -c.Offset.Y)

	return &op
}

func (g *Game) Draw(screen *ebiten.Image) {

	screen.DrawImage(g.Level.MapImage, g.Camera.GetCameraDrawOptions())

	tps := ebiten.ActualTPS()
	ebitenutil.DebugPrint(screen, fmt.Sprintf("TPS: %f", tps))

	g.Player.Draw(screen, g.Camera)

	for _, connection := range g.Client.connections {
		if g.Client.IsSelf(connection.Addr) {
			continue
		}
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

	level := Level{}
	LoadLevel(&level)

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

	player_sprite, _, err := ebitenutil.NewImageFromFile("assets/Tiles/tile_0098.png")

	if err != nil {
		panic(err)
	}

	game := Game{Player: Player{Speed: PLAYER_SPEED, Position: Position{1, 1}, Sprite: player_sprite}, Client: &client, Level: &level }

	if err := ebiten.RunGame(&game); err != nil {
		log.Fatal(err)
	}
}

func LoadLevel(level *Level) {
	gameMap, err := tiled.LoadFile("assets/Tiled/sampleMap.tmx")

	if err != nil {
		panic(err)
	}

	mapRenderer, err := render.NewRenderer(gameMap)

	if err != nil {
		panic(err)
	}

	// render it to an in memory image
	err = mapRenderer.RenderVisibleLayers()

	if err != nil {
		panic(err)
	}

	var buff []byte
	buffer := bytes.NewBuffer(buff)

	mapRenderer.SaveAsPng(buffer)

	im, err := png.Decode(buffer)

	level.MapImage = ebiten.NewImageFromImage(im)
}

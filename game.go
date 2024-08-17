package main

import (
	"bytes"
	"flag"
	"fmt"
	"image/png"
	"log"
	"math"

	"github.com/lafriks/go-tiled"
	"github.com/lafriks/go-tiled/render"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	SCREEN_WIDTH  = 320
	SCREEN_HEIGHT = 240
	RENDER_WIDTH  = 640
	RENDER_HEIGHT = 480
	TILE_SIZE     = 16
	PLAYER_SPEED  = 2
	BULLET_SPEED  = 2.5
)

var BULLET_SPRITE *ebiten.Image = GetSpriteByID(115)

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
	MapImage   *ebiten.Image
	Map        *tiled.Map
	Collisions []*tiled.Object
	Spawn      *tiled.Object
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

func GetSpriteByID(ID int) *ebiten.Image {
	player_sprite, _, err := ebitenutil.NewImageFromFile(fmt.Sprintf("assets/Tiles/tile_%04d.png", ID))
	if err != nil {
		panic(err)
	}

	return player_sprite
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

func AbsoluteCursorPosition(camera *Camera) (int, int) {
	cursorX, cursorY := ebiten.CursorPosition()
	return cursorX+int(camera.Offset.X), cursorY+int(camera.Offset.Y)
}

func CalculateOrientationRads(camera *Camera, pos *Position) float64 {
	cursorX, cursorY := AbsoluteCursorPosition(camera)
	return math.Atan2(float64(cursorY)-pos.Y, float64(cursorX)-pos.X)
}

func CalculateOrientationAngle(camera *Camera, pos *Position) int {
	radians := CalculateOrientationRads(camera, pos)
	angle := radians * (180 / math.Pi)
	return int(angle+360) % 360
}

func (g *Game) Update() error {
	g.FrameCount++

	if g.Client.is_connected && (g.FrameCount%3 == 0) {
		g.Client.SendPosition(g.Player.Position)
	}

	g.Player.Update(g)

	camera_target_pos := Position{g.Player.Position.X - SCREEN_WIDTH/2, g.Player.Position.Y - SCREEN_HEIGHT/2}
	g.Camera.Update(camera_target_pos)
	if ebiten.IsKeyPressed(ebiten.KeyQ) {
		return ebiten.Termination
	}

	if ebiten.IsMouseButtonPressed(ebiten.MouseButton0) {
		current_pos := g.Player.Position
		rotation := CalculateOrientationRads(&g.Camera, &g.Player.Position)
		speed := float32(BULLET_SPEED)

		g.Client.SendShoot(Bullet{current_pos, rotation, speed})
	}
	
       for i, bullet := range g.Client.bullets {
               x := math.Cos(bullet.Rotation)
               y := math.Sin(bullet.Rotation)

               g.Client.bullets[i].Position.X += x * float64(bullet.Speed)
               g.Client.bullets[i].Position.Y += y * float64(bullet.Speed)
       }

	return nil

}

func (p *Player) Draw(screen *ebiten.Image, camera Camera) {
	op := camera.GetCameraDrawOptions()

	op.GeoM.Translate(p.Position.X, p.Position.Y)

	screen.DrawImage(p.Sprite, op)
}

func (p *Player) Update(game *Game) {
	player_pos := &p.Position

	if ebiten.IsKeyPressed(ebiten.KeyW) {
		player_pos.Y -= p.Speed
		collided_object := game.CheckObjectCollision(*player_pos)
		if collided_object != nil {
			player_pos.Y = collided_object.Y + collided_object.Height
		}
	}

	if ebiten.IsKeyPressed(ebiten.KeyS) {
		player_pos.Y += p.Speed
		collided_object := game.CheckObjectCollision(*player_pos)
		if collided_object != nil {
			player_pos.Y = collided_object.Y - TILE_SIZE
		}
	}

	if ebiten.IsKeyPressed(ebiten.KeyA) {
		player_pos.X -= p.Speed
		collided_object := game.CheckObjectCollision(*player_pos)
		if collided_object != nil {
			player_pos.X = collided_object.X + collided_object.Width
		}
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		player_pos.X += p.Speed
		collided_object := game.CheckObjectCollision(*player_pos)
		if collided_object != nil {
			player_pos.X = collided_object.X - TILE_SIZE
		}
	}

}

func (c *Camera) Update(target_pos Position) {
	coefficient := 20.0
	c.Offset.X += (target_pos.X - c.Offset.X) / coefficient
	c.Offset.Y += (target_pos.Y - c.Offset.Y) / coefficient
}

func (c *Camera) GetCameraDrawOptions() *ebiten.DrawImageOptions {
	op := ebiten.DrawImageOptions{}
	op.GeoM.Translate(-c.Offset.X, -c.Offset.Y)

	return &op
}

func (g *Game) CheckObjectCollision(position Position) *tiled.Object {
	for _, object := range g.Level.Collisions {
		if (object.X < position.X+TILE_SIZE &&
		object.X+object.Width > position.X &&
		object.Y < position.Y+TILE_SIZE &&
		object.Y+object.Height > position.Y) {
			return object
		}
	}

	return nil
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
		op := g.Camera.GetCameraDrawOptions()
		op.GeoM.Translate(connection.Position.X, connection.Position.Y)
		screen.DrawImage(g.Player.Sprite, op)
	}

	for _, bullet := range g.Client.bullets {
		op := g.Camera.GetCameraDrawOptions()
		op.GeoM.Translate(bullet.Position.X, bullet.Position.Y)
		screen.DrawImage(BULLET_SPRITE, op)
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

	ebiten.SetWindowSize(RENDER_WIDTH, RENDER_HEIGHT)
	ebiten.SetWindowTitle("Hello, World!")

	client := Client{}
	if *is_host == "n" {
		go client.RunClient(*server_ip)

	} else {
		server := Server{}
		go server.Host(*server_ip)
		go client.RunLocalClient()
	}

	player_sprite := GetSpriteByID(98) // PLAYER SPRITE

	game := Game{Player: Player{Speed: PLAYER_SPEED, Position: Position{1, 1}, Sprite: player_sprite}, Client: &client, Level: &level}

	if game.Level.Spawn != nil {
		game.Player.Position = Position{game.Level.Spawn.X, game.Level.Spawn.Y}
	}

	if err := ebiten.RunGame(&game); err != nil {
		log.Fatal(err)
	}
}

func LoadLevel(level *Level) {
	gameMap, err := tiled.LoadFile("assets/Tiled/sampleMap.tmx")
	level.Map = gameMap

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

	for _, object_group := range level.Map.ObjectGroups {
		if object_group.Name == "Collision" {
			level.Collisions = object_group.Objects
		}
	}

	for _, object_group := range level.Map.ObjectGroups {
		if object_group.Name == "Misc" {
			for _, object := range object_group.Objects {
				if object.Name == "player_spawn" {
					level.Spawn = object
				}
			}
		}
	}
}

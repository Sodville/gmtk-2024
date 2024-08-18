package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"math"
	"time"

	"github.com/lafriks/go-tiled"
	"github.com/lafriks/go-tiled/render"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	SCREEN_WIDTH                = 320
	SCREEN_HEIGHT               = 240
	RENDER_WIDTH                = 640
	RENDER_HEIGHT               = 480
	TILE_SIZE                   = 16
	PLAYER_SPEED                = 2
	BULLET_SPEED                = 2.5
	SERVER_PLAYER_SYNC_DELAY_MS = 50
)

type LevelEnum uint

const (
	LobbyLevel LevelEnum = iota
	LevelOne
	LevelCount
)

var emptyImage = ebiten.NewImage(3, 3)
var emptySubImage = emptyImage.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)

var BULLET_SPRITE *ebiten.Image = GetSpriteByID(115)

type Position struct {
	X, Y float64
}

type Delta struct {
	dX, dY float64
}

type Player struct {
	Position     Position
	Speed        float64
	Sprite       *ebiten.Image
	MoveDuration int
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
	Server     *Server
	FrameCount uint64
	Level      *Level
	Camera     Camera
	Sparks     []Spark

	event_handler_running bool
}

func GetSpriteByID(ID int) *ebiten.Image {
	player_sprite, _, err := ebitenutil.NewImageFromFile(fmt.Sprintf("assets/Tiles/tile_%04d.png", ID))
	if err != nil {
		panic(err)
	}

	return player_sprite
}

func AbsoluteCursorPosition(camera Camera) (int, int) {
	cursorX, cursorY := ebiten.CursorPosition()
	return cursorX + int(camera.Offset.X), cursorY + int(camera.Offset.Y)
}

func CalculateOrientationRads(camera Camera, pos Position) float64 {
	cursorX, cursorY := AbsoluteCursorPosition(camera)
	return math.Atan2(float64(cursorY)-pos.Y, float64(cursorX)-pos.X)
}

func CalculateOrientationAngle(camera Camera, pos Position) int {
	radians := CalculateOrientationRads(camera, pos)
	angle := radians * (180 / math.Pi)
	return int(angle+360) % 360
}

func (g *Game) Update() error {
	g.FrameCount++

	if g.Server != nil {
		g.Server.Update()
	}

	if g.event_handler_running == false {
		go g.HandleEvent()
	}

	if g.Client.is_connected && (g.FrameCount%3 == 0) {
		g.Client.SendPosition(g.Player.Position)
	}

	g.Player.Update(g)

	targetX := g.Player.Position.X - SCREEN_WIDTH/2
	targetX = max(0, targetX)
	targetX = min(float64(g.Level.Map.Width*TILE_SIZE-SCREEN_WIDTH), targetX)

	targetY := g.Player.Position.Y - SCREEN_HEIGHT/2
	targetY = max(0, targetY)
	targetY = min(float64(g.Level.Map.Height*TILE_SIZE-SCREEN_HEIGHT), targetY)

	camera_target_pos := Position{targetX, targetY}
	g.Camera.Update(camera_target_pos)
	if ebiten.IsKeyPressed(ebiten.KeyQ) {
		return ebiten.Termination
	}

	if ebiten.IsMouseButtonPressed(ebiten.MouseButton0) {
		current_pos := g.Player.Position
		rotation := CalculateOrientationRads(g.Camera, g.Player.GetCenter())
		speed := float32(BULLET_SPEED)

		g.Client.SendShoot(Bullet{current_pos, rotation, speed, 0})
	}

	for i, bullet := range g.Client.bullets {
		x := math.Cos(bullet.Rotation)
		y := math.Sin(bullet.Rotation)

		g.Client.bullets[i].Position.X += x * float64(bullet.Speed)
		g.Client.bullets[i].Position.Y += y * float64(bullet.Speed)
	}

	bullets := []Bullet{}
	for i, bullet := range g.Client.bullets {
		radians := bullet.Rotation
		x := math.Cos(radians)
		y := math.Sin(radians)

		bullet.Position.X += x * float64(bullet.Speed)
		bullet.Position.Y += y * float64(bullet.Speed)

		collision_object := g.Level.CheckObjectCollision(g.Client.bullets[i].Position)
		if collision_object != nil {
			// do cool
			g.Sparks = append(g.Sparks, Spark{2, bullet.Position, int(bullet.Rotation), 100, 2})
			//fmt.Println("added spark: ", g.Sparks)
		} else {
			bullets = append(bullets, bullet)
		}
	}
	g.Client.bullets = bullets

	states := make(map[string]PlayerState)
	for key, state := range g.Client.player_states {
		if g.Client.IsSelf(state.Connection.Addr) {
			states[key] = state
			continue
		}

		ps, _ := g.Client.player_states[key]
		currentRelativePosition := ps.GetInterpolatedPos()
		if state.CurrentPos.X != currentRelativePosition.X || state.CurrentPos.Y != currentRelativePosition.Y {
			ps.MoveDuration += 1
		} else {
			ps.MoveDuration = ps.MoveDuration % 30
			ps.MoveDuration = max(0, ps.MoveDuration-1)
		}

		ps.FrameCount++
		states[key] = ps
	}

	g.Client.player_states = states

	sparks := []Spark{}
	for _, spark := range g.Sparks {
		spark.Update()

		if spark.Lifetime != 0 {
			sparks = append(sparks, spark)
		}
	}

	g.Sparks = sparks

	return nil

}

func (p *Player) Draw(screen *ebiten.Image, camera Camera) {
	op := ebiten.DrawImageOptions{}
	if p.MoveDuration > 0 {
		op.GeoM.Translate(-8, -8)
		op.GeoM.Rotate(math.Sin(float64(p.MoveDuration/5)) * 0.2)
		op.GeoM.Translate(8, 8)
	}
	op.GeoM.Translate(p.Position.X, p.Position.Y)
	op.GeoM.Translate(-camera.Offset.X, -camera.Offset.Y)

	screen.DrawImage(p.Sprite, &op)
}

func (p *Player) Update(game *Game) {
	player_pos := &p.Position
	initial_pos := *player_pos

	if ebiten.IsKeyPressed(ebiten.KeyW) {
		player_pos.Y -= p.Speed
		collided_object := game.Level.CheckObjectCollision(*player_pos)
		if collided_object != nil {
			player_pos.Y = collided_object.Y + collided_object.Height
		}
	}

	if ebiten.IsKeyPressed(ebiten.KeyS) {
		player_pos.Y += p.Speed
		collided_object := game.Level.CheckObjectCollision(*player_pos)
		if collided_object != nil {
			player_pos.Y = collided_object.Y - TILE_SIZE
		}
	}

	if ebiten.IsKeyPressed(ebiten.KeyA) {
		player_pos.X -= p.Speed
		collided_object := game.Level.CheckObjectCollision(*player_pos)
		if collided_object != nil {
			player_pos.X = collided_object.X + collided_object.Width
		}
	}

	if ebiten.IsKeyPressed(ebiten.KeyD) {
		player_pos.X += p.Speed
		collided_object := game.Level.CheckObjectCollision(*player_pos)
		if collided_object != nil {
			player_pos.X = collided_object.X - TILE_SIZE
		}
	}

	if p.Position == initial_pos {
		p.MoveDuration = p.MoveDuration % 30
		p.MoveDuration = max(0, p.MoveDuration-1)
	} else {
		p.MoveDuration += 1
	}
}

func (p *Player) GetCenter() Position {
	return Position{
		p.Position.X + float64(p.Sprite.Bounds().Dx())/2,
		p.Position.Y + float64(p.Sprite.Bounds().Dy())/2,
	}
}

func (c *Camera) Update(target_pos Position) {
	coefficient := 10.0
	c.Offset.X += (target_pos.X - c.Offset.X) / coefficient
	c.Offset.Y += (target_pos.Y - c.Offset.Y) / coefficient
}

func (c *Camera) GetCameraDrawOptions() *ebiten.DrawImageOptions {
	op := ebiten.DrawImageOptions{}
	op.GeoM.Translate(-c.Offset.X, -c.Offset.Y)

	return &op
}

func (l *Level) CheckObjectCollision(position Position) *tiled.Object {
	for _, object := range l.Collisions {
		if object.X < position.X+TILE_SIZE &&
			object.X+object.Width > position.X &&
			object.Y < position.Y+TILE_SIZE &&
			object.Y+object.Height > position.Y {
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

	for _, state := range g.Client.player_states {
		if g.Client.IsSelf(state.Connection.Addr) {
			continue
		}
		op := ebiten.DrawImageOptions{}
		if state.MoveDuration > 0 {
			op.GeoM.Translate(-8, -8)
			op.GeoM.Rotate(math.Sin(float64(state.MoveDuration/5)) * 0.2)
			op.GeoM.Translate(8, 8)
		}

		RenderPos := state.GetInterpolatedPos()
		op.GeoM.Translate(RenderPos.X, RenderPos.Y)
		op.GeoM.Translate(-g.Camera.Offset.X, -g.Camera.Offset.Y)
		screen.DrawImage(g.Player.Sprite, &op)
	}

	for _, bullet := range g.Client.bullets {
		op := g.Camera.GetCameraDrawOptions()
		op.GeoM.Translate(bullet.Position.X, bullet.Position.Y)
		screen.DrawImage(BULLET_SPRITE, op)
	}

	for _, spark := range g.Sparks {
		spark.Draw(screen, &g.Camera)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return SCREEN_WIDTH, SCREEN_HEIGHT
}

func (g *Game) StartChangeLevel(levelType LevelEnum, when time.Time) {
	newLevel := Level{}
	LoadLevel(&newLevel, levelType)

	remaining := when.Sub(time.Now())

	time.Sleep(time.Duration(remaining))
	g.Level = &newLevel

	if g.Level.Spawn != nil {
		g.Player.Position = Position{g.Level.Spawn.X, g.Level.Spawn.Y}
	}
}

func (g *Game) HandleEvent() {
	g.event_handler_running = true
	for {
		select {
		case event_data := <-g.Client.event_channel:
			fmt.Println("handling event")
			switch event_data.Type {
			case ServerNewLevelEvent:
				go g.StartChangeLevel(event_data.State.LevelEnum, event_data.State.Timestamp)
			}
		}
	}
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
	LoadLevel(&level, LevelOne)

	ebiten.SetWindowSize(RENDER_WIDTH, RENDER_HEIGHT)
	ebiten.SetWindowTitle("Hello, World!")

	client := Client{}
	var server Server
	if *is_host == "n" {
		go client.RunClient(*server_ip)

	} else {
		server = Server{level: &level}
		go server.Host(*server_ip)
		go client.RunLocalClient()
	}

	player_sprite := GetSpriteByID(98) // PLAYER SPRITE

	game := Game{Player: Player{Speed: PLAYER_SPEED, Position: Position{1, 1}, Sprite: player_sprite}, Client: &client, Level: &level, Server: &server}

	if game.Level.Spawn != nil {
		game.Player.Position = Position{game.Level.Spawn.X, game.Level.Spawn.Y}
	}

	if err := ebiten.RunGame(&game); err != nil {
		log.Fatal(err)
	}
}

func LoadLevel(level *Level, levelType LevelEnum) {
	var gameMap *tiled.Map
	switch levelType {
	case LobbyLevel:
		_gameMap, err := tiled.LoadFile("assets/Tiled/sampleMap.tmx")
		gameMap = _gameMap

		if err != nil {
			panic(err)
		}
	case LevelCount:
		panic("do not use LEVEL COUNT as level")

	default:
		_gameMap, err := tiled.LoadFile(fmt.Sprintf("assets/Tiled/level_%d.tmx", levelType))
		gameMap = _gameMap

		if err != nil {
			panic(err)
		}
	}

	if gameMap == nil {
		panic("no gamemap sourced")
	}

	level.Map = gameMap

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

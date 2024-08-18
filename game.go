package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"math"

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
	SERVER_PLAYER_SYNC_DELAY_MS = 50
	TOGGLECOOLDOWN              = 30
	TIMEOUT_INTERVAL_MS         = 2500
)

var emptyImage = ebiten.NewImage(3, 3)
var emptySubImage = emptyImage.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)

type Game struct {
	Player     Player
	Client     *Client
	Server     *Server
	FrameCount uint64
	Level      *Level
	Camera     Camera
	Sparks     []Spark

	Debris []Bullet

	toggleCooldown        int
	event_handler_running bool
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
		g.Client.SendPosition(g.Player.Position, g.Player.Rotation, g.Player.Weapon, false) // TODO
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

	if ebiten.IsKeyPressed(ebiten.Key1) {
		g.Player.Weapon = WeaponBow
	}

	if ebiten.IsKeyPressed(ebiten.Key2) {
		g.Player.Weapon = WeaponRevolver
	}

	if ebiten.IsKeyPressed(ebiten.Key3) {
		g.Player.Weapon = WeaponGun
	}

	if ebiten.IsKeyPressed(ebiten.KeyR) && g.toggleCooldown == 0 {
		g.Client.ToggleReady()
		g.toggleCooldown = TOGGLECOOLDOWN
	}

	// we don't really care if it's frames or MS as it's not related to gameplay
	g.toggleCooldown = max(0, g.toggleCooldown-1)

	rotation := CalculateOrientationRads(g.Camera, g.Player.GetCenter())
	g.Player.Rotation = rotation

	if ebiten.IsMouseButtonPressed(ebiten.MouseButton0) && g.Player.ShootCooldown == 0 {
		current_pos := g.Player.Position

		g.Client.SendShoot(Bullet{
			current_pos,
			rotation,
			g.Player.Weapon,
			GetWeaponSpeed(g.Player.Weapon),
			0,
			GetWeaponFriendlyFire(g.Player.Weapon)},
		)
		g.Player.ShootCooldown = GetWeaponCooldown(g.Player.Weapon)
	}

	g.Player.ShootCooldown = max(0, g.Player.ShootCooldown-.16)

	g.Client.bullets_mutex.Lock()
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
			if bullet.WeaponType == WeaponBow {
				g.Debris = append(g.Debris, bullet)
			}
			//fmt.Println("added spark: ", g.Sparks)
		} else {
			bullets = append(bullets, bullet)
		}
	}
	g.Client.bullets = bullets
	g.Client.bullets_mutex.Unlock()

	states := make(map[string]PlayerState)
	g.Client.player_states_mutex.Lock()
	for key, state := range g.Client.player_states {
		if g.Client.IsSelf(state.Connection.Addr) {
			states[key] = state
			continue
		}

		ps := g.Client.player_states[key]
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
	g.Client.player_states_mutex.Unlock()

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

func (g *Game) Draw(screen *ebiten.Image) {

	screen.DrawImage(g.Level.MapImage, g.Camera.GetCameraDrawOptions())

	g.Player.Draw(screen, g.Camera)

	g.Client.player_states_mutex.RLock()
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

		distance := 8.

		op = ebiten.DrawImageOptions{}
		op.GeoM.Translate(-distance, -distance)

		if math.Pi*.5 < state.Connection.Rotation || state.Connection.Rotation < -math.Pi*.5 {
			op.GeoM.Scale(1, -1)
		}
		op.GeoM.Rotate(state.Connection.Rotation)

		op.GeoM.Translate(distance, distance)

		x := math.Cos(state.Connection.Rotation)
		y := math.Sin(state.Connection.Rotation)

		op.GeoM.Translate(x*distance, y*distance)

		op.GeoM.Translate(RenderPos.X, RenderPos.Y)
		op.GeoM.Translate(-g.Camera.Offset.X, -g.Camera.Offset.Y)

		screen.DrawImage(GetWeaponSprite(state.Connection.Weapon), &op)
	}
	g.Client.player_states_mutex.RUnlock()

	g.Client.bullets_mutex.RLock()
	for _, bullet := range g.Client.bullets {
		sprite := GetBulletSprite(bullet.WeaponType)

		width := sprite.Bounds().Dx()
		height := sprite.Bounds().Dy()

		op := ebiten.DrawImageOptions{}

		op.GeoM.Translate(-float64(width)/2, -float64(height)/2)
		op.GeoM.Rotate(bullet.Rotation + math.Pi*.5)
		op.GeoM.Translate(float64(width)/2, float64(height)/2)

		op.GeoM.Translate(bullet.Position.X, bullet.Position.Y)
		op.GeoM.Translate(-g.Camera.Offset.X, -g.Camera.Offset.Y)

		screen.DrawImage(sprite, &op)
	}

	for _, bullet := range g.Debris {
		sprite := GetBulletSprite(bullet.WeaponType)

		width := sprite.Bounds().Dx()
		height := sprite.Bounds().Dy()

		op := ebiten.DrawImageOptions{}

		op.GeoM.Translate(-float64(width)/2, -float64(height)/2)
		op.GeoM.Rotate(bullet.Rotation + math.Pi*.5)
		op.GeoM.Translate(float64(width)/2, float64(height)/2)

		op.GeoM.Translate(bullet.Position.X, bullet.Position.Y)
		op.GeoM.Translate(-g.Camera.Offset.X, -g.Camera.Offset.Y)

		screen.DrawImage(sprite, &op)
	}
	g.Client.bullets_mutex.RUnlock()

	for _, spark := range g.Sparks {
		spark.Draw(screen, &g.Camera)
	}

	ebitenutil.DebugPrint(screen, fmt.Sprintf("READY: %d/%d", g.Client.readyPlayersCount, g.Client.playerCount))
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return SCREEN_WIDTH, SCREEN_HEIGHT
}

func (g *Game) ChangeLevel(levelType LevelEnum) {
	newLevel := Level{}
	LoadLevel(&newLevel, levelType)

	g.Level = &newLevel

	// reseting on map change
	g.Debris = []Bullet{}
	g.Client.bullets = []Bullet{}

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
			case NewLevelEvent:
				g.ChangeLevel(event_data.Level)
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
	LoadLevel(&level, LobbyLevel)

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

	InitializeWeapons()

	player_sprite := GetSpriteByID(98) // PLAYER SPRITE

	game := Game{Player: Player{Speed: PLAYER_SPEED, Position: Position{1, 1}, Sprite: player_sprite, Weapon: WeaponBow}, Client: &client, Level: &level, Server: &server}

	if game.Level.Spawn != nil {
		game.Player.Position = Position{game.Level.Spawn.X, game.Level.Spawn.Y}
	}

	if err := ebiten.RunGame(&game); err != nil {
		log.Fatal(err)
	}
}

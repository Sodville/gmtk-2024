package main

import (
	"bytes"
	"flag"
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	SCREEN_WIDTH                = 320
	SCREEN_HEIGHT               = 240
	RENDER_WIDTH                = 640
	RENDER_HEIGHT               = 480
	TILE_SIZE                   = 16
	PLAYER_SPEED                = 2
	PLAYER_LIFE                 = 10
	ROLL_SPEED                  = 4
	SERVER_PLAYER_SYNC_DELAY_MS = 50
	TOGGLECOOLDOWN              = 30
	TIMEOUT_INTERVAL_MS         = 2500
	MAX_SPAWN_COUNT             = 12
	MINIMUM_SPAWN_COOLDOWN      = 30
	INITAL_SPAWN_COOLDOWN       = 60
	SPAWN_IDLE_TIME_FRAMES      = 60 * 2
	DEFAULT_GRACEPERIOD         = 6
	BOON_INTERACT_RANGE         = 33.0
)

var WHITE color.RGBA = color.RGBA{255, 255, 255, 255}
var BLACK color.RGBA = color.RGBA{0, 0, 0, 255}
var HOSTSMITHSPRITE = GetSpriteByID(86)
var JOINWIZARDSPRITE = GetSpriteByID(84)
var TOMBSPRITE = GetSpriteByID(64)

type TransitionState int

var fontFaceSource *text.GoTextFaceSource

const (
	TransitionStateNone TransitionState = iota
	TransitionStateStarted
	TransitionStateEnding
)

type Transition struct {
	Position Position
	Speed    float64
}

type Game struct {
	Player      Player
	Client      *Client
	Server      *Server
	FrameCount  uint64
	Level       *Level
	Camera      Camera
	Sparks      []Spark
	Enemies     []Enemy
	Debris      []Bullet
	Boons       []Boon
	Tombs       []Position
	LevelCount  int
	Modifiers   Modifiers
	JoinKey     string
	BigTextBuff string
	Healthbar   *Healthbar

	Transitions     []Transition
	TransitionState TransitionState
	TransitionWidth float64

	toggleCooldown        int
	event_handler_running bool
	isTypingJoinCode      bool
	ShouldCleanEnemies    bool
	isInWaitingRoom       bool
}

func (g *Game) Update() error {
	g.FrameCount++

	if g.Server != nil {
		g.Server.Update()
	}

	if g.event_handler_running == false && g.Client != nil {
		go g.HandleEvent()
	}

	if g.Client != nil && g.Client.is_connected && (g.FrameCount%3 == 0) {
		g.Client.SendPosition(
			g.Player.Position,
			g.Player.Rotation,
			g.Player.Weapon,
			g.Player.RollDuration > 0,
			g.Player.Life,
		) // TODO
	}

	if g.ShouldCleanEnemies {
		g.Enemies = []Enemy{}
		g.ShouldCleanEnemies = false
	}

	targetX := g.Player.Position.X - SCREEN_WIDTH/2
	targetX = max(0, targetX)
	targetX = min(float64(g.Level.Map.Width*TILE_SIZE-SCREEN_WIDTH), targetX)

	targetY := g.Player.Position.Y - SCREEN_HEIGHT/2
	targetY = max(0, targetY)
	targetY = min(float64(g.Level.Map.Height*TILE_SIZE-SCREEN_HEIGHT), targetY)

	camera_target_pos := Position{targetX, targetY}
	g.Camera.Update(camera_target_pos)
	if g.isTypingJoinCode {
		g.BigTextBuff = "join key: " + g.JoinKey
		keys := make([]ebiten.Key, 0)
		keys = inpututil.AppendJustPressedKeys(keys)
		for _, k := range keys {
			if len(k.String()) == 1 {
				g.JoinKey += k.String()
			} else if k == ebiten.KeyBackspace && len(g.JoinKey) > 0 {
				g.JoinKey = g.JoinKey[:len(g.JoinKey)-1]
			} else if k == ebiten.KeyEnter {
				g.isTypingJoinCode = false
				g.Join()
			} else if k == ebiten.KeyEscape {
				g.BigTextBuff = ""
				g.isTypingJoinCode = false
			}
		}
	} else {
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

		if ebiten.IsKeyPressed(ebiten.KeyR) && g.toggleCooldown == 0 && g.Client != nil {
			g.Client.ToggleReady()
			g.toggleCooldown = TOGGLECOOLDOWN
		}

		g.Player.Update(g)

		if ebiten.IsKeyPressed(ebiten.KeyE) && g.toggleCooldown == 0 && g.Client != nil {
			g.toggleCooldown = TOGGLECOOLDOWN
			for _, boon := range g.Boons {
				if g.Player.Position.Distance(boon.Position) < BOON_INTERACT_RANGE {
					g.Client.SendChosenModifiers(boon.Modifiers)
				}
			}
		}

	}

	for i := range g.Boons {
		boon := g.Boons[i]
		if g.FrameCount%2 == 0 {
			if g.Player.Position.Distance(boon.Position) < BOON_INTERACT_RANGE {
				boon.AnimationFrame = min(len(BOONSPRITES)-1, boon.AnimationFrame+1)
			} else {
				boon.AnimationFrame = max(0, boon.AnimationFrame-1)
			}
		}
		g.Boons[i] = boon
	}
	// we don't really care if it's frames or MS as it's not related to gameplay
	g.toggleCooldown = max(0, g.toggleCooldown-1)

	rotation := CalculateOrientationRads(g.Camera, g.Player.GetCenter())
	g.Player.Rotation = rotation

	if g.Client != nil {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButton0) && g.Player.ShootCooldown == 0 && g.Player.RollDuration == 0 && !g.Player.IsGhost() && g.Client != nil {
			current_pos := g.Player.Position
			speed := GetWeaponSpeed(g.Player.Weapon)
			speedMulti := g.Modifiers.GetModifiedPlayerValue(ModifierTypeBulletSpeed)

			speed *= float32(speedMulti)
			g.Client.SendShoot(Bullet{
				current_pos,
				rotation,
				g.Player.Weapon,
				speed,
				0,
				GetWeaponFriendlyFire(g.Player.Weapon)},
			)

			if WeaponHasSpark(g.Player.Weapon) {
				scale := g.Modifiers.GetModifiedPlayerValue(ModifierTypeDamage)
				current_pos.X += TILE_SIZE/2 + math.Cos(g.Player.Rotation)*TILE_SIZE
				current_pos.Y += TILE_SIZE/2 + math.Sin(g.Player.Rotation)*TILE_SIZE
				g.Sparks = append(g.Sparks, Spark{4, current_pos, g.Player.Rotation - .5, scale, speedMulti, WHITE})
				g.Sparks = append(g.Sparks, Spark{4, current_pos, g.Player.Rotation, scale, speedMulti, WHITE})
				g.Sparks = append(g.Sparks, Spark{4, current_pos, g.Player.Rotation + .5, scale, speedMulti, WHITE})
			}

			weaponCooldown := GetWeaponCooldown(g.Player.Weapon)
			weaponCooldown /= g.Modifiers.GetModifiedPlayerValue(ModifierTypeWeaponCooldown)
			g.Player.ShootCooldown = weaponCooldown
		}

		g.Player.ShootCooldown = max(0, g.Player.ShootCooldown-.16)

		if ebiten.IsKeyPressed(ebiten.KeySpace) && g.Player.RollCooldown == 0 && g.Client != nil {
			g.Client.SendRoll()
			g.Player.RollCooldown = 100
		}

		g.Player.RollCooldown = max(0, g.Player.RollCooldown-1)

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

			damage := GetWeaponDamage(bullet.WeaponType)
			damage *= g.Modifiers.GetModifiedPlayerValue(ModifierTypeDamage)
			hitEnemy := false

			if !bullet.HurtsPlayer {
				for key, enemy := range g.Enemies {
					if bullet.Position.X < enemy.Position.X+TILE_SIZE &&
						bullet.Position.X+4 > enemy.Position.X && // 4 is width
						bullet.Position.Y < enemy.Position.Y+TILE_SIZE &&
						bullet.Position.Y+4 > enemy.Position.Y { // 4 is height
						hitEnemy = true
						g.Enemies[key].Life = max(0, enemy.Life-int(damage))
						break
					}
				}
			}

			collision_object := g.Level.CheckObjectCollision(g.Client.bullets[i].Position)
			if collision_object != nil || hitEnemy {

				sparkPos := bullet.Position
				sparkPos.X += 4
				sparkPos.Y += 4

				if collision_object != nil {
					g.Sparks = append(g.Sparks, Spark{4, sparkPos, -bullet.Rotation - .5, 1, 1, color.RGBA{255, 255, 255, 255}})
					g.Sparks = append(g.Sparks, Spark{4, sparkPos, -bullet.Rotation, 1, 1, color.RGBA{192, 182, 200, 255}})
					g.Sparks = append(g.Sparks, Spark{4, sparkPos, -bullet.Rotation + .5, 1, 1, color.RGBA{255, 255, 255, 255}})
				} else if hitEnemy {
					redColor := color.RGBA{255, 28, 28, 255}
					g.Sparks = append(g.Sparks, Spark{4, sparkPos, bullet.Rotation - .5, 2, 2, redColor})
					g.Sparks = append(g.Sparks, Spark{4, sparkPos, bullet.Rotation, 2, 2, color.RGBA{192, 182, 200, 255}})
					g.Sparks = append(g.Sparks, Spark{4, sparkPos, bullet.Rotation + .5, 2, 2, redColor})

					for i := -.1; i < .1; i += .5 {
						redColor := color.RGBA{178, 28, 28, 255}
						g.Sparks = append(g.Sparks, Spark{5, sparkPos, bullet.Rotation + i*2.5, 1.2, 3, redColor})
					}

				}
				if bullet.WeaponType == WeaponBow && !hitEnemy {
					g.Debris = append(g.Debris, bullet)
				}
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

			currentRelativePosition := state.GetInterpolatedPos()
			if state.CurrentPos.X != currentRelativePosition.X || state.CurrentPos.Y != currentRelativePosition.Y {
				state.MoveDuration += 1
			} else {
				state.MoveDuration = state.MoveDuration % 30
				state.MoveDuration = max(0, state.MoveDuration-1)
			}

			state.FrameCount++
			states[key] = state
		}

		g.Client.player_states = states
		g.Client.player_states_mutex.Unlock()

	}
	sparks := []Spark{}
	for key := range g.Sparks {
		spark := g.Sparks[key]
		spark.Update()

		if spark.Lifetime != 0 {
			sparks = append(sparks, spark)
		}
	}
	g.Sparks = sparks

	enemies := []Enemy{}
	for key := range g.Enemies {
		target := g.Client.GetStateByAddr(g.Enemies[key].Target)

		if target != nil {
			g.Enemies[key].FindPath(target.CurrentPos, g.Level.ObstacleMatrix)
		} else {
			log.Println("enemy could not find target player")
		}

		g.Enemies[key].Update()
		if g.Enemies[key].Life > 0 {
			enemies = append(enemies, g.Enemies[key])
		}
	}
	g.Enemies = enemies

	if g.Level.HostSmith != nil {
		pos := Position{g.Level.HostSmith.X, g.Level.HostSmith.Y}
		distance := pos.Distance(g.Player.Position)

		if distance < BOON_INTERACT_RANGE && ebiten.IsKeyPressed(ebiten.KeyE) && g.Server == nil && g.toggleCooldown == 0 {
			g.toggleCooldown = TOGGLECOOLDOWN
			g.Host()
		}
	}

	if g.Level.JoinWizard != nil {
		pos := Position{g.Level.JoinWizard.X, g.Level.JoinWizard.Y}
		distance := pos.Distance(g.Player.Position)

		if distance < BOON_INTERACT_RANGE && ebiten.IsKeyPressed(ebiten.KeyE) && g.Server == nil && g.toggleCooldown == 0 {
			g.toggleCooldown = TOGGLECOOLDOWN
			g.isTypingJoinCode = true
		}
	}
	g.UpdateTransition()

	return nil

}

func (g *Game) Draw(screen *ebiten.Image) {

	screen.DrawImage(g.Level.MapImage, g.Camera.GetCameraDrawOptions())

	g.Player.Draw(screen, g.Camera)

	for _, enemy := range g.Enemies {
		enemy.Draw(screen, g.Camera, g)
	}

	for _, pos := range g.Tombs {
		op := g.Camera.GetCameraDrawOptions()
		op.GeoM.Translate(pos.X, pos.Y)
		screen.DrawImage(TOMBSPRITE, op)
	}

	if g.Client != nil {
		if g.isInWaitingRoom {
			if !g.Client.IsReady() {
				textOp := text.DrawOptions{}
				msg := "press 'r' to ready"
				fontSize := 12.
				textOp.GeoM.Translate(SCREEN_WIDTH/2, SCREEN_HEIGHT-fontSize*4)
				textOp.GeoM.Translate(-float64(len(msg)/2)*fontSize, fontSize)
				text.Draw(screen, msg, &text.GoTextFace{Source: fontFaceSource, Size: fontSize}, &textOp)

			}

			textOp := text.DrawOptions{}
			msg := fmt.Sprintf("READY: %d/%d", g.Client.readyPlayersCount, g.Client.playerCount)
			fontSize := 12.
			textOp.GeoM.Translate(SCREEN_WIDTH/2, SCREEN_HEIGHT-fontSize*3)
			textOp.GeoM.Translate(-float64(len(msg)/2)*fontSize, fontSize)
			text.Draw(screen, msg, &text.GoTextFace{Source: fontFaceSource, Size: fontSize}, &textOp)
		}

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

			if state.Connection.Life > 0 {
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

			} else {
				op.ColorScale.SetA(185)
				screen.DrawImage(GhostSprite, &op)
			}

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

	}
	for _, boon := range g.Boons {
		boon.Draw(screen, &g.Camera)
	}

	for _, spark := range g.Sparks {
		spark.Draw(screen, &g.Camera)
	}

	if g.TransitionState != TransitionStateNone {
		for i := range g.Transitions {
			transition := g.Transitions[i]
			vector.DrawFilledRect(screen, float32(transition.Position.X), float32(transition.Position.Y), float32(g.TransitionWidth), SCREEN_HEIGHT, BLACK, true)
		}

		op := text.DrawOptions{}

		fontSize := 18.
		op.GeoM.Translate(SCREEN_WIDTH/2, SCREEN_HEIGHT/2)
		levelString := fmt.Sprintf("Level %d", g.LevelCount)

		op.GeoM.Translate(-float64(len(levelString)/2)*fontSize, -fontSize/2)
		text.Draw(screen, levelString, &text.GoTextFace{Source: fontFaceSource, Size: fontSize}, &op)
	}

	if g.Level.HostSmith != nil {
		op := g.Camera.GetCameraDrawOptions()
		op.GeoM.Translate(g.Level.HostSmith.X, g.Level.HostSmith.Y)
		screen.DrawImage(HOSTSMITHSPRITE, op)

		pos := Position{g.Level.HostSmith.X, g.Level.HostSmith.Y}
		distance := pos.Distance(g.Player.Position)

		if distance < BOON_INTERACT_RANGE {
			textOp := text.DrawOptions{}
			textOp.GeoM = op.GeoM
			fontSize := 8.
			msg := "press 'e' to host"
			textOp.GeoM.Translate(-float64(len(msg)/2)*fontSize, -fontSize)
			text.Draw(screen, msg, &text.GoTextFace{Source: fontFaceSource, Size: fontSize}, &textOp)
		}
	}

	if g.Level.JoinWizard != nil {
		op := g.Camera.GetCameraDrawOptions()
		op.GeoM.Translate(g.Level.JoinWizard.X, g.Level.JoinWizard.Y)
		screen.DrawImage(JOINWIZARDSPRITE, op)

		pos := Position{g.Level.JoinWizard.X, g.Level.JoinWizard.Y}
		distance := pos.Distance(g.Player.Position)

		if distance < BOON_INTERACT_RANGE {
			textOp := text.DrawOptions{}
			textOp.GeoM = op.GeoM
			fontSize := 8.
			msg := "press 'e' to type join code"
			if g.isTypingJoinCode {
				msg = "'enter' to confirm"
			}
			textOp.GeoM.Translate(-float64(len(msg)/2)*fontSize, -fontSize)
			text.Draw(screen, msg, &text.GoTextFace{Source: fontFaceSource, Size: fontSize}, &textOp)
		}
	}

	textOp := text.DrawOptions{}
	textOp.GeoM.Translate(SCREEN_WIDTH/2, 0)
	fontSize := 16.
	textOp.GeoM.Translate(-float64(len(g.BigTextBuff)/2)*fontSize, fontSize+25)
	text.Draw(screen, g.BigTextBuff, &text.GoTextFace{Source: fontFaceSource, Size: fontSize}, &textOp)

	g.Healthbar.Draw(screen)
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

func (g *Game) StartLevelTransition() {
	n := rand.Intn(5) + 10

	g.TransitionState = TransitionStateStarted

	g.TransitionWidth = float64(SCREEN_WIDTH / n)
	for i := 0; i <= n; i++ {
		g.Transitions = append(g.Transitions, Transition{Position{float64(i) * g.TransitionWidth, SCREEN_HEIGHT}, 4. + rand.Float64()*10})
	}
}

func (g *Game) UpdateTransition() {
	if g.TransitionState == TransitionStateNone {
		return
	}

	transitions := []Transition{}
	for i := range g.Transitions {
		transition := g.Transitions[i]
		if g.TransitionState == TransitionStateStarted {
			transition.Position.Y = max(0, transition.Position.Y-transition.Speed)
		} else if g.TransitionState == TransitionStateEnding {
			transition.Position.Y -= transition.Speed
		}

		if transition.Position.Y > -SCREEN_HEIGHT {
			transitions = append(transitions, transition)
		}
	}

	if len(transitions) == 0 {
		g.TransitionState = TransitionStateNone
	}

	g.Transitions = transitions
}

func (g *Game) HandleEvent() {
	g.event_handler_running = true
	for {
		select {
		case event_data := <-g.Client.event_channel:
			fmt.Println("handling event")
			switch event_data.Type {
			case NewLevelEvent:
				g.ShouldCleanEnemies = true
				g.ChangeLevel(event_data.Level)
				g.TransitionState = TransitionStateEnding
				g.Tombs = []Position{}
				g.Boons = []Boon{}

				g.LevelCount++
				g.isInWaitingRoom = false
				if event_data.Level == LobbyLevel {
					g.LevelCount = 0
					g.isInWaitingRoom = true
				}
				g.BigTextBuff = ""
				l := g.Modifiers.GetModifiedPlayerValue(ModifierTypeLife)
				g.Player.Life = int(l * PLAYER_LIFE)
				g.Healthbar.MaxLife = g.Player.Life
			case SpawnEnemiesEvent:
				g.Enemies = append(g.Enemies, event_data.Enemies...)
			case PlayerDiedEvent:
				g.Tombs = append(g.Tombs, event_data.Player.Position)
			case SpawnBoonEvent:
				g.ShouldCleanEnemies = true
				for i, mod := range event_data.Modifiers {
					g.Boons = append(g.Boons, Boon{mod, g.Level.BoonSpawns[i], 0})
				}
			case PrepareNewLevelEvent:
				g.StartLevelTransition()
				// maybe make them do the cool
			case GameOverEvent:
				g.BigTextBuff = "GAME OVER"
			}
		}
	}
}

func (g *Game) Host() {
	LoadLevel(g.Level, LobbyLevel)
	server := Server{level: g.Level}
	g.Server = &server

	client := Client{}
	client.Modifiers = &g.Modifiers
	client.PlayerLifePtr = &g.Player.Life

	g.Player.Position = Position{g.Level.Spawn.X, g.Level.Spawn.Y}

	g.Client = &client

	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	result := make([]byte, 4)
	for i := range result {
		result[i] = letters[rand.Intn(len(letters))]
	}

	key := fmt.Sprintf("gmtk2024:%s", result)
	g.BigTextBuff = string(result)
	go server.Host("84.215.22.166", key)
	go client.RunLocalClient()

	g.isInWaitingRoom = true

}

func (g *Game) Join() {
	client := Client{}
	client.Modifiers = &g.Modifiers
	client.PlayerLifePtr = &g.Player.Life

	g.Client = &client
	go client.RunClient("84.215.22.166", fmt.Sprintf("gmtk2024:%s", g.JoinKey)) // this should be some buffer

	g.JoinKey = ""
	g.BigTextBuff = "connecting..."
	if !g.Client.CheckConnected() {
		g.BigTextBuff = "failed to connect"
		g.Client = nil
		return
	} else {
		g.BigTextBuff = ""
		LoadLevel(g.Level, LobbyLevel)
		g.isInWaitingRoom = true

		g.Player.Position = Position{g.Level.Spawn.X, g.Level.Spawn.Y}
	}
}

func main() {
	is_server := flag.String("server", "n", "run server")
	is_host := flag.String("host", "n", "host")

	flag.Parse()

	if *is_server == "y" {
		RunMediationServer()
		return
	}

	level := LoadPregameLevel()

	ebiten.SetWindowSize(RENDER_WIDTH, RENDER_HEIGHT)
	ebiten.SetWindowTitle("Hello, World!")

	b, err := os.ReadFile("assets/Fonts/PressStart2P-Regular.ttf")
	if err != nil {
		log.Fatal(err)
	}

	s, err := text.NewGoTextFaceSource(bytes.NewReader(b))
	if err != nil {
		log.Fatal(err)
	}

	fontFaceSource = s

	InitializeWeapons()
	InitializeCharacters()

	player_sprite := GetSpriteByID(98) // PLAYER SPRITE

	game := Game{
		Player: Player{
			Speed:     PLAYER_SPEED,
			RollSpeed: ROLL_SPEED,
			Position:  Position{1, 1},
			Sprite:    player_sprite,
			Weapon:    WeaponBow,
			Life:      PLAYER_LIFE,
		},
		Level: &level,
	}

	game.Healthbar = &Healthbar{
		MaxLife:       PLAYER_LIFE,
		PlayerLifePtr: &game.Player.Life,
		X:             SCREEN_WIDTH/2 - 160/2, // half of screen width minus half of bar width
		Y:             10,
		Width:         160,
		Height:        10,
		BorderWidth:   2,
		BorderColor:   color.RGBA{0, 0, 0, 255},
		FillColor:     color.RGBA{100, 190, 50, 255},
	}

	if *is_host == "y" {
		game.Host()
	}

	if game.Level.Spawn != nil {
		game.Player.Position = Position{game.Level.Spawn.X, game.Level.Spawn.Y}
	}

	if err := ebiten.RunGame(&game); err != nil {
		log.Fatal(err)
	}
}

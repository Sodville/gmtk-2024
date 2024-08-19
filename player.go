package main

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

type Player struct {
	Position     Position
	Speed        float64
	Sprite       *ebiten.Image
	MoveDuration int
	Weapon       WeaponType
	Rotation     float64
	RollSpeed    float64
	RollDuration float64
	RollCooldown float64
	Invulnerable bool
	GracePeriod  float64
	Life         int

	ShootCooldown float64
}

func (p *Player) Draw(screen *ebiten.Image, camera Camera) {
	op := ebiten.DrawImageOptions{}
	if p.RollDuration != 0 {
		op.GeoM.Translate(-8, -8)
		op.GeoM.Rotate(p.RollDuration)
		op.GeoM.Translate(8, 8)
	} else if p.MoveDuration > 0 {
		op.GeoM.Translate(-8, -8)
		op.GeoM.Rotate(math.Sin(float64(p.MoveDuration/5)) * 0.2)
		op.GeoM.Translate(8, 8)
	}
	op.GeoM.Translate(p.Position.X, p.Position.Y)
	op.GeoM.Translate(-camera.Offset.X, -camera.Offset.Y)

	screen.DrawImage(p.Sprite, &op)
	DrawWeapon(screen, camera, p.Weapon, *p)
}

func (p *Player) Update(game *Game) {
	player_pos := &p.Position
	initial_pos := *player_pos

	var speed float64
	if ebiten.IsKeyPressed(ebiten.KeySpace) && p.RollCooldown == 0 {
		current_pos := initial_pos
		current_pos.Y += TILE_SIZE
		direction := math.Pi

		if ebiten.IsKeyPressed(ebiten.KeyD) {
			p.RollDuration = math.Pi * -2
		} else {
			p.RollDuration = math.Pi * 2
			current_pos.X += TILE_SIZE
			direction = 0
		}
		game.Sparks = append(game.Sparks, Spark{ 2, current_pos, direction - .16, 3, 1.5, WHITE})
		game.Sparks = append(game.Sparks, Spark{ 2, current_pos, direction + .16, 3, 1.5, WHITE})

		p.Invulnerable = true
	}

	if p.RollDuration > 0 {
		speed = p.RollSpeed
		p.RollDuration = max(0, p.RollDuration-p.RollSpeed*0.085)
	} else if p.RollDuration < 0 {
		speed = p.RollSpeed
		p.RollDuration = min(0, p.RollDuration+p.RollSpeed*0.085)
	} else {
		speed = p.Speed
		// TODO: should be server decided probs
		p.Invulnerable = false
	}

	if ebiten.IsKeyPressed(ebiten.KeyW) {
		player_pos.Y -= speed
		collided_object := game.Level.CheckObjectCollision(*player_pos)
		if collided_object != nil {
			player_pos.Y = collided_object.Y + collided_object.Height
		}
	}

	if ebiten.IsKeyPressed(ebiten.KeyS) {
		player_pos.Y += speed
		collided_object := game.Level.CheckObjectCollision(*player_pos)
		if collided_object != nil {
			player_pos.Y = collided_object.Y - TILE_SIZE
		}
	}

	if ebiten.IsKeyPressed(ebiten.KeyA) {
		player_pos.X -= speed
		collided_object := game.Level.CheckObjectCollision(*player_pos)
		if collided_object != nil {
			player_pos.X = collided_object.X + collided_object.Width
		}
	}

	if ebiten.IsKeyPressed(ebiten.KeyD) {
		player_pos.X += speed
		collided_object := game.Level.CheckObjectCollision(*player_pos)
		if collided_object != nil {
			player_pos.X = collided_object.X - TILE_SIZE
		}
	}

	p.Life -= game.Client.PendingDamageTaken
	game.Client.PendingDamageTaken = 0

	// "Cooldown" animation when player stops moving
	if p.Position == initial_pos {
		p.MoveDuration = p.MoveDuration % 30
		p.MoveDuration = max(0, p.MoveDuration-1)
	} else {
		p.MoveDuration += 1
	}

	p.GracePeriod = max(0, p.GracePeriod - .16)
	if p.GracePeriod == 0 {
		for _, enemy := range game.Enemies {
			if enemy.Position.X < p.Position.X + TILE_SIZE &&
			enemy.Position.X + TILE_SIZE > p.Position.X &&
			enemy.Position.Y < p.Position.Y + TILE_SIZE &&
			enemy.Position.Y + TILE_SIZE > p.Position.Y {
				game.Client.SendHit(HitInfo{*game.Client.Self(), GetCharacterDamage(enemy.Type)})
				p.GracePeriod = DEFAULT_GRACEPERIOD
			}
		}
	}
}

func (p *Player) GetCenter() Position {
	return Position{
		p.Position.X + float64(p.Sprite.Bounds().Dx())/2,
		p.Position.Y + float64(p.Sprite.Bounds().Dy())/2,
	}
}

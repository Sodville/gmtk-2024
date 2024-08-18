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

	ShootCooldown float64
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
	DrawWeapon(screen, camera, p.Weapon, *p)
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

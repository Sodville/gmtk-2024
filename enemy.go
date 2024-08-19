package main

import (
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type CharacterType uint

const (
	CharacterZombie CharacterType = iota + 1
	CharacterSpawnSign
	CharacterCount
)

var CharacterImageMap map[CharacterType]*ebiten.Image = make(map[CharacterType]*ebiten.Image)

func InitializeCharacters() {
	for i := 1; i < int(CharacterCount); i++ {
		switch i {
		case int(CharacterCount):
			image, _, err := ebitenutil.NewImageFromFile("assets/Characters/character_1.png")
			if err != nil {
				panic(err)
			}
			CharacterImageMap[CharacterType(i)] = image
		default:
			image, _, err := ebitenutil.NewImageFromFile(fmt.Sprintf("assets/Characters/character_%d.png", i))
			if err != nil {
				panic(err)
}
			CharacterImageMap[CharacterType(i)] = image
		}
	}
}

func GetLifeForCharacter(character CharacterType) int {
	switch character {
		case CharacterZombie:
			return 13
		default:
			return 10
	}
}

func GetCharacterDamage(character CharacterType) int {
	switch character {
		case CharacterZombie:
			return 2
		default:
			return 2
	}
}

type Enemy struct {
	Type CharacterType
	Position Position
	MoveDuration int
	Lifetime uint
	Life int
}

func (e *Enemy) Draw(screen *ebiten.Image, camera Camera) {
	if e.Lifetime >= SPAWN_IDLE_TIME_FRAMES {
		op := ebiten.DrawImageOptions{}
		if e.MoveDuration > 0 {
			op.GeoM.Translate(-8, -8)
			op.GeoM.Rotate(math.Sin(float64(e.MoveDuration/5)) * 0.2)
			op.GeoM.Translate(8, 8)
		}
		op.GeoM.Translate(e.Position.X, e.Position.Y)
		op.GeoM.Translate(-camera.Offset.X, -camera.Offset.Y)

		screen.DrawImage(CharacterImageMap[e.Type], &op)
	} else {
		op := ebiten.DrawImageOptions{}
		op.GeoM.Translate(e.Position.X, e.Position.Y)
		op.GeoM.Translate(-camera.Offset.X, -camera.Offset.Y)

		screen.DrawImage(CharacterImageMap[CharacterSpawnSign], &op)
	}
}

func (e *Enemy) Update() {
	e.Lifetime++

	if e.Lifetime <= SPAWN_IDLE_TIME_FRAMES {
		return
	}

	initial_pos := e.Position

	if e.Position == initial_pos {
		e.MoveDuration = e.MoveDuration % 30
		e.MoveDuration = max(0, e.MoveDuration-1)
	} else {
		e.MoveDuration += 1
	}
}

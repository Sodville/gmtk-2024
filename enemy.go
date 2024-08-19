package main

import (
	"fmt"
	"image/color"
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

// we are cheating here and introducing game to the render because we can't introduce it for the update
// because the server uses the update method to compute enemies
// we could perhaps just nil it on the server but nah
func (e *Enemy) Draw(screen *ebiten.Image, camera Camera, game *Game) {
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

	if e.Lifetime == SPAWN_IDLE_TIME_FRAMES {
		currentPos := e.Position
		currentPos.X += TILE_SIZE / 2
		currentPos.Y += TILE_SIZE / 2

		for i := 0; i < 7; i++ {
			color := color.RGBA{49, 19, 29, 100}
			game.Sparks = append(game.Sparks, Spark{ 6, currentPos, float64(i) - .16, .5, .5, color})
			game.Sparks = append(game.Sparks, Spark{ 6, currentPos, float64(i) + .16, .5, .5, color})

			color.R = 149
			color.G = 119
			color.B = 129
			game.Sparks = append(game.Sparks, Spark{ 8, currentPos, float64(i) - .26, .5, .75, color})
			game.Sparks = append(game.Sparks, Spark{ 8, currentPos, float64(i) + .26, .5, .75, color})
		}
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

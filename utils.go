package main

import (
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type Position struct {
	X, Y float64
}

type Delta struct {
	dX, dY float64
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

func GetSpriteByID(ID int) *ebiten.Image {
	player_sprite, _, err := ebitenutil.NewImageFromFile(fmt.Sprintf("assets/Tiles/tile_%04d.png", ID))
	if err != nil {
		panic(err)
	}

	return player_sprite
}

package main

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type Position struct {
	X, Y float64
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

func (p *Position) Distance(other Position) float64 {
	xDelta := p.X - other.X
	yDelta := p.Y - other.Y

	return math.Sqrt(xDelta*xDelta + yDelta*yDelta)
}

func drawTextWithStroke(dst *ebiten.Image, str string, face text.Face, textColor, strokeColor color.Color, strokeWidth int, textOp *text.DrawOptions) {
	for dy := -strokeWidth; dy <= strokeWidth; dy++ {
		for dx := -strokeWidth; dx <= strokeWidth; dx++ {
			if dx*dx+dy*dy >= strokeWidth*strokeWidth {
				continue
			}
			textOp.GeoM.Translate(float64(dx), float64(dy))
			textOp.ColorScale.Reset()
			textOp.ColorScale.ScaleWithColor(strokeColor)
			text.Draw(dst, str, face, textOp)
			textOp.GeoM.Translate(-float64(dx), -float64(dy))
		}
	}
	textOp.ColorScale.Reset()
	textOp.ColorScale.ScaleWithColor(textColor)
	text.Draw(dst, str, face, textOp)
}

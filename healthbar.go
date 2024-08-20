package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Healthbar struct {
	MaxLife       int
	PlayerLifePtr *int
	X, Y          float32
	Width         float32
	Height        float32
	BorderWidth   float32
	BorderColor   color.Color
	FillColor     color.Color
}

func (h *Healthbar) Draw(screen *ebiten.Image) {
	fillWidth := max(0, float32(*h.PlayerLifePtr)*h.Width/float32(h.MaxLife))
	vector.DrawFilledRect(screen, h.X, h.Y, fillWidth, h.Height, h.FillColor, false)

	vector.DrawFilledRect(screen, h.X, h.Y-h.BorderWidth, h.Width-h.BorderWidth, h.BorderWidth, h.BorderColor, false)
	vector.DrawFilledRect(screen, h.X+h.Width-h.BorderWidth, h.Y-h.BorderWidth, h.BorderWidth, h.Height+h.BorderWidth, h.BorderColor, false)
	vector.DrawFilledRect(screen, h.X-h.BorderWidth, h.Y+h.Height-h.BorderWidth, h.Width, h.BorderWidth, h.BorderColor, false)
	vector.DrawFilledRect(screen, h.X-h.BorderWidth, h.Y-h.BorderWidth, h.BorderWidth, h.Height+h.BorderWidth, h.BorderColor, false)
}

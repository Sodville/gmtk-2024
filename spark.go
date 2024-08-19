package main

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var polygonImage *ebiten.Image = ebiten.NewImage(1, 1)

type Spark struct {
	Lifetime float64
	Position Position
	Angle    float64
	Scale    float64
	Force    float64
	Color    color.RGBA
}

func (s *Spark) calculateMovement() (float64, float64) {
	x := math.Cos(s.Angle) * s.Lifetime * s.Force * .1
	y := math.Sin(s.Angle) * s.Lifetime * s.Force * .1

	return x, y
}

func (s *Spark) Update() {
	x, y := s.calculateMovement()

	s.Position.X += x
	s.Position.Y += y

	s.Lifetime = max(0, s.Lifetime-.16)
}

func (s *Spark) Draw(screen *ebiten.Image, camera *Camera) {
	angle := s.Angle
	points := []Position{
		{
			X: s.Position.X - camera.Offset.X + math.Cos(angle)*s.Lifetime*s.Scale,
			Y: s.Position.Y - camera.Offset.Y + math.Sin(angle)*s.Lifetime*s.Scale,
		},
		{
			X: s.Position.X - camera.Offset.X + math.Cos(angle+math.Pi/2)*s.Lifetime*s.Scale*0.3,
			Y: s.Position.Y - camera.Offset.Y + math.Sin(angle+math.Pi/2)*s.Lifetime*s.Scale*0.3,
		},
		{
			X: s.Position.X - camera.Offset.X - math.Cos(angle)*s.Lifetime*s.Scale*3.5,
			Y: s.Position.Y - camera.Offset.Y - math.Sin(angle)*s.Lifetime*s.Scale*3.5,
		},
		{
			X: s.Position.X - camera.Offset.X + math.Cos(angle-math.Pi/2)*s.Lifetime*s.Scale*0.3,
			Y: s.Position.Y - camera.Offset.Y - math.Sin(angle+math.Pi/2)*s.Lifetime*s.Scale*0.3,
		},
	}

	path := vector.Path{}
	path.MoveTo(float32(points[0].X), float32(points[0].Y))
	for _, p := range points[1:] {
		path.LineTo(float32(p.X), float32(p.Y))
	}
	path.Close()

	vs, is := path.AppendVerticesAndIndicesForFilling(nil, nil)
	polygonImage.Fill(s.Color)

	screen.DrawTriangles(vs, is, polygonImage, &ebiten.DrawTrianglesOptions{})

}

package main

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Spark struct {
	Lifetime float64
	Position Position
	Angle    int
	Scale    float64
	Force    float64
}

func (s *Spark) calculateMovement() (float64, float64) {
	x := math.Cos(float64(s.Angle) * s.Lifetime * s.Force)
	y := math.Sin(float64(s.Angle) * s.Lifetime * s.Force)

	return x, y
}

func (s *Spark) Update() {
	x, y := s.calculateMovement()

	s.Position.X += x
	s.Position.Y += y

	s.Lifetime = max(0, s.Lifetime-.16)
}

func (s *Spark) Draw(screen *ebiten.Image, camera *Camera) {
	points := []Position{
		{
			X: s.Position.X + math.Cos(float64(s.Angle))*s.Lifetime*s.Scale,
			Y: s.Position.Y + camera.Offset.Y + math.Sin(float64(s.Angle))*s.Lifetime*s.Scale,
		},
		{
			X: s.Position.X + camera.Offset.X + math.Cos(float64(s.Angle)+math.Pi/2)*s.Lifetime*s.Scale*0.3,
			Y: s.Position.Y + camera.Offset.Y + math.Sin(float64(s.Angle)+math.Pi/2)*s.Lifetime*s.Scale*0.3,
		},
		{
			X: s.Position.X + camera.Offset.X - math.Cos(float64(s.Angle))*s.Lifetime*s.Scale*3.5,
			Y: s.Position.Y + camera.Offset.Y - math.Sin(float64(s.Angle))*s.Lifetime*s.Scale*3.5,
		},
		{
			X: s.Position.X + camera.Offset.X + math.Cos(float64(s.Angle)-math.Pi/2)*s.Lifetime*s.Scale*0.3,
			Y: s.Position.Y + camera.Offset.Y - math.Sin(float64(s.Angle)+math.Pi/2)*s.Lifetime*s.Scale*0.3,
		},
	}

	path := vector.Path{}
	path.MoveTo(float32(points[0].X), float32(points[0].Y))
	for _, p := range points[1:] {
		path.LineTo(float32(p.X), float32(p.Y))
	}
	path.Close()

	vs, is := path.AppendVerticesAndIndicesForFilling(nil, nil)
	for i := range vs {
		vs[i].SrcX = 1
		vs[i].SrcY = 1
		vs[i].ColorR = 1
		vs[i].ColorG = 1
		vs[i].ColorB = 1
		vs[i].ColorA = 1
	}

	screen.DrawTriangles(vs, is, emptySubImage, &ebiten.DrawTrianglesOptions{})

}

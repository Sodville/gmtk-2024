package main

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type ModifierCalcType int
type ModifierType int

const (
	ModifierCalcTypeMulti ModifierCalcType = iota
	ModifierCalcTypeAddi
)

const (
	ModifierTypeSpeed ModifierType = iota
	ModifierTypeDamage
	ModifierTypeFireRate
)

type Modifier struct {
	CalcType ModifierCalcType
	Type     ModifierType
	Value    float64
}

func (m *Modifier) GetString(prefix string) string {
	var r string
	moreOrIncreased := "more"
	if m.CalcType == ModifierCalcTypeAddi {
		moreOrIncreased = "increased"
	}

	switch m.Type {
	case ModifierTypeDamage:
		r = "%s gains %d% %s damage"
	default:
		r = "%s gains %d% %s ..."
	}

	return fmt.Sprintf(r, prefix, m.Value/100, moreOrIncreased)
}

type Modifiers struct {
	Monster []Modifier
	Player  []Modifier
}

func getModifiedValue(valueType ModifierType, modifiers []Modifier) float64 {
	base := 0.0
	multi := make([]float64, 0)
	for _, m := range modifiers {
		if m.Type == valueType {
			if m.CalcType == ModifierCalcTypeMulti {
				multi = append(multi, m.Value)
			} else {
				base += m.Value
			}
		}
	}

	for _, n := range multi {
		base *= n
	}

	return base
}

func (m *Modifiers) GetModifiedMonsterValue(valueType ModifierType) float64 {
	return getModifiedValue(valueType, m.Monster)
}

func (m *Modifiers) GetModifiedPlayerValue(valueType ModifierType) float64 {
	return getModifiedValue(valueType, m.Player)
}

func (m *Modifiers) Add(newModifiers Modifiers) {
	m.Monster = append(m.Monster, newModifiers.Monster...)
	m.Player = append(m.Player, newModifiers.Player...)
}

type Boon struct {
	Modifiers Modifiers
	Position  Position
}

func (b *Boon) Draw(screen *ebiten.Image, camera *Camera) {
	x := b.Position.X - camera.Offset.X
	y := b.Position.Y - camera.Offset.Y
	vector.DrawFilledRect(screen, float32(x), float32(y), 16, 16, WHITE, true)
}

func (b *Boon) Update(screen *ebiten.Image) {

}

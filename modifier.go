package main

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type ModifierCalcType int
type ModifierType int

var BOONSPRITES = []*ebiten.Image{GetSpriteByID(89), GetSpriteByID(90), GetSpriteByID(91)}

const (
	ModifierCalcTypeMulti ModifierCalcType = iota
	ModifierCalcTypeAddi
)

const (
	ModifierTypeSpeed ModifierType = iota
	ModifierTypeDamage
	ModifierTypeWeaponCooldown
	ModifierTypeBulletSpeed
	ModifierTypeLife

	ModifierTypeCount
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
	case ModifierTypeWeaponCooldown:
		r = "%s gain %.0f%% %s fire rate"
	case ModifierTypeLife:
		r = "%s gain %.0f%% %s life"
	case ModifierTypeBulletSpeed:
		r = "%s gain %.0f%% %s bullet speed"
	case ModifierTypeSpeed:
		r = "%s gain %.0f%% %s move speed"
	case ModifierTypeDamage:
		r = "%s gain %.0f%% %s damage"
	default:
		r = "%s gain %.0f%% %s ..."
	}

	return fmt.Sprintf(r, prefix, m.Value*100, moreOrIncreased)
}

type Modifiers struct {
	Monster []Modifier
	Player  []Modifier
}

func getModifiedValue(valueType ModifierType, modifiers []Modifier) float64 {
	base := 1.0
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
		base *= 1 + n
	}

	return base
}

func (m *Modifiers) getTotalModifiedValue() float64 {
	base := 1.0
	multi := make([]float64, 0)
	for _, m := range m.Player {
		if m.CalcType == ModifierCalcTypeMulti {
			multi = append(multi, m.Value)
		} else {
			base += m.Value
		}
	}

	for _, m := range m.Monster {
		if m.CalcType == ModifierCalcTypeMulti {
			multi = append(multi, m.Value)
		} else {
			base += m.Value
		}
	}

	for _, n := range multi {
		base *= 1 + n
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
	Modifiers      Modifiers
	Position       Position
	AnimationFrame int
}

func (b *Boon) Draw(screen *ebiten.Image, camera *Camera) {
	op := camera.GetCameraDrawOptions()
	op.GeoM.Translate(b.Position.X, b.Position.Y)
	screen.DrawImage(BOONSPRITES[b.AnimationFrame], op)

	if b.AnimationFrame > 0 {
		textOp := text.DrawOptions{}
		textOp.GeoM = op.GeoM
		fontSize := 8.

		playerString := b.Modifiers.Player[0].GetString("Players")
		monsterString := b.Modifiers.Monster[0].GetString("Monsters")
		textOp.ColorScale.ScaleWithColor(color.RGBA{20, 140, 20, 255})

		textOp.GeoM.Translate(-float64(len(playerString)/2)*fontSize, -fontSize*4)
		text.Draw(screen, playerString, &text.GoTextFace{Source: fontFaceSource, Size: fontSize}, &textOp)

		textOp = text.DrawOptions{}
		textOp.GeoM = op.GeoM
		textOp.ColorScale.ScaleWithColor(color.RGBA{200, 20, 20, 255})

		textOp.GeoM.Translate(-float64(len(monsterString)/2)*fontSize, -fontSize*2)
		text.Draw(screen, monsterString, &text.GoTextFace{Source: fontFaceSource, Size: fontSize}, &textOp)

		textOp = text.DrawOptions{}
		textOp.GeoM = op.GeoM
		info := "press 'e' to choose"

		textOp.GeoM.Translate(-float64(len(info)/2)*fontSize, -fontSize)
		text.Draw(screen, info, &text.GoTextFace{Source: fontFaceSource, Size: fontSize}, &textOp)
	}
}

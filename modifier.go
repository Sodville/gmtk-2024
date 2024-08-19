package main

import "fmt"

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
	Type ModifierType
	Value float64
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

	return fmt.Sprintf(r, prefix, m.Value / 100, moreOrIncreased)
}

type Modifiers struct {
	Monster []Modifier
	Player []Modifier
}

func getModifiedValue(valueType ModifierType, modifiers[] Modifier) float64 {
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

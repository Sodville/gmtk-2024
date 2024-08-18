package main

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type WeaponType uint

const (
	WeaponBow WeaponType = iota + 1
	WeaponRevolver
	WeaponGun
	WeaponCount
)

var WeaponImageMap map[WeaponType]*ebiten.Image = make(map[WeaponType]*ebiten.Image)
var BulletImageMap map[WeaponType]*ebiten.Image = make(map[WeaponType]*ebiten.Image)

func GetWeaponSprite(weapon WeaponType) *ebiten.Image{
	if weapon >= WeaponCount || weapon <= 0 {
		weapon = WeaponBow
	}

	return WeaponImageMap[weapon]
}

func GetBulletSprite(weapon WeaponType) *ebiten.Image{
	if weapon >= WeaponCount || weapon <= 0 {
		weapon = WeaponBow
	}

	return BulletImageMap[weapon]
}

func GetWeaponCooldown(weapon WeaponType) float64 {
	switch weapon {
	case WeaponBow:
		return 2.25
	default:
		return 2
	}
}

func GetWeaponDamage(weapon WeaponType) int {
	switch weapon {
	case WeaponBow:
		return 3
	default:
		return 2
	}
}

func GetWeaponSpeed(weapon WeaponType) float32 {
	switch weapon {
	case WeaponBow:
		return 3
	default:
		return 2
	}
}

func GetWeaponFriendlyFire(weapon WeaponType) bool {
	switch weapon {
	default:
		return false
	}
}

func InitializeWeapons() {
	for i := 1; i < int(WeaponCount); i ++ {
		switch i {
		case int(WeaponCount):
			image, _, err := ebitenutil.NewImageFromFile("assets/Weapons/weapon_1.png")
			if err != nil {
				panic(err)
			}
			WeaponImageMap[WeaponType(i)] = image
		default:
			image, _, err := ebitenutil.NewImageFromFile(fmt.Sprintf("assets/Weapons/weapon_%d.png", i))
			if err != nil {
				panic(err)
			}
			WeaponImageMap[WeaponType(i)] = image
		}


		switch i {
		case int(WeaponCount):
			image, _, err := ebitenutil.NewImageFromFile("assets/Bullets/bullet_1.png")
			if err != nil {
				panic(err)
			}
			BulletImageMap[WeaponType(i)] = image
		case int(WeaponGun):
			fallthrough
		case int(WeaponRevolver):
			image, _, err := ebitenutil.NewImageFromFile(fmt.Sprintf("assets/Bullets/bullet_%d.png", WeaponRevolver))
			if err != nil {
				panic(err)
			}
			BulletImageMap[WeaponType(i)] = image
		default:
			image, _, err := ebitenutil.NewImageFromFile(fmt.Sprintf("assets/Bullets/bullet_%d.png", i))
			if err != nil {
				panic(err)
			}
			BulletImageMap[WeaponType(i)] = image
		}
	}
}

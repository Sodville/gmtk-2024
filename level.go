package main

import (
	"bytes"
	"fmt"
	"image/png"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lafriks/go-tiled"
	"github.com/lafriks/go-tiled/render"
)

type LevelEnum uint

const (
	LobbyLevel LevelEnum = iota
	LevelOne
	LevelCount
)

type Level struct {
	MapImage   *ebiten.Image
	Map        *tiled.Map
	Collisions []*tiled.Object
	Spawn      *tiled.Object
}

func (l *Level) CheckObjectCollision(position Position) *tiled.Object {
	for _, object := range l.Collisions {
		if object.X < position.X+TILE_SIZE &&
			object.X+object.Width > position.X &&
			object.Y < position.Y+TILE_SIZE &&
			object.Y+object.Height > position.Y {
			return object
		}
	}

	return nil
}

func LoadLevel(level *Level, levelType LevelEnum) {
	var gameMap *tiled.Map
	switch levelType {
	case LobbyLevel:
		_gameMap, err := tiled.LoadFile("assets/Tiled/sampleMap.tmx")
		gameMap = _gameMap

		if err != nil {
			panic(err)
		}
	case LevelCount:
		panic("do not use LEVEL COUNT as level")

	default:
		_gameMap, err := tiled.LoadFile(fmt.Sprintf("assets/Tiled/level_%d.tmx", levelType))
		gameMap = _gameMap

		if err != nil {
			panic(err)
		}
	}

	if gameMap == nil {
		panic("no gamemap sourced")
	}

	level.Map = gameMap

	mapRenderer, err := render.NewRenderer(gameMap)

	if err != nil {
		panic(err)
	}

	// render it to an in memory image
	err = mapRenderer.RenderVisibleLayers()

	if err != nil {
		panic(err)
	}

	var buff []byte
	buffer := bytes.NewBuffer(buff)

	mapRenderer.SaveAsPng(buffer)

	im, err := png.Decode(buffer)

	level.MapImage = ebiten.NewImageFromImage(im)

	for _, object_group := range level.Map.ObjectGroups {
		if object_group.Name == "Collision" {
			level.Collisions = object_group.Objects
		}
	}

	for _, object_group := range level.Map.ObjectGroups {
		if object_group.Name == "Misc" {
			for _, object := range object_group.Objects {
				if object.Name == "player_spawn" {
					level.Spawn = object
				}
			}
		}
	}
}

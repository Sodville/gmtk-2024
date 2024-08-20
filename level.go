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
	LevelTwo
	LevelCount
)

type Level struct {
	MapImage       *ebiten.Image
	Map            *tiled.Map
	Collisions     []*tiled.Object
	Spawn          *tiled.Object
	BoonSpawns     []Position
	ObstacleMatrix [][]bool
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

// Returns 2D boolean array where true indicates
// obstacles/collision used for path finding
//
// To get a specific point index as array[row][column] or array[y][x]
func (l *Level) generateObstacleMatrix() [][]bool {
	boolArray := make([][]bool, l.Map.Height)
	for yTile := 0; yTile < l.Map.Height; yTile++ {
		y := yTile*l.Map.TileHeight + l.Map.TileHeight/2 // want the coord in the middle of the tile
		boolArray[yTile] = make([]bool, l.Map.Width)

		for xTile := 0; xTile < l.Map.Width; xTile++ {
			x := xTile*l.Map.TileWidth + l.Map.TileWidth/2

			objPointer := l.CheckObjectCollision(Position{float64(x), float64(y)})
			boolArray[yTile][xTile] = objPointer != nil
		}
	}

	return boolArray
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

	level.ObstacleMatrix = level.generateObstacleMatrix()

	for _, object_group := range level.Map.ObjectGroups {
		if object_group.Name == "Misc" {
			for _, object := range object_group.Objects {
				if object.Name == "player_spawn" {
					level.Spawn = object
				}
				if object.Name == "boon_spawn" {
					level.BoonSpawns = append(level.BoonSpawns, Position{object.X, object.Y})
				}
			}
		}
	}

	if levelType != LobbyLevel && len(level.BoonSpawns) < 2 {
		panic("2 boon spawns are REQUIRED")
	}
}

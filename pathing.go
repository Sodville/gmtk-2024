package main

import (
	"container/heap"
	"fmt"
	"log"
	"math"
)

type IntPosition struct {
	x, y int
}

type Node struct {
	intPos  IntPosition
	gCost   int
	hCost   int
	Parent  *Node
	HeapIdx int
}

func (n *Node) fCost() int {
	return n.gCost + n.hCost
}

type PriorityQueue []*Node

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].fCost() < pq[j].fCost()
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].HeapIdx = i
	pq[j].HeapIdx = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	node := x.(*Node)
	node.HeapIdx = n
	*pq = append(*pq, node)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	node := old[n-1]
	old[n-1] = nil // avoid memory leak
	node.HeapIdx = -1
	*pq = old[0 : n-1]
	return node
}

func manhattanDistance(a, b IntPosition) int {
	return int(math.Abs(float64(a.x-b.x)) + math.Abs(float64(a.y-b.y)))
}

func getNeighbors(p IntPosition, grid [][]bool) []IntPosition {
	neighbors := []IntPosition{}
	directions := []IntPosition{{0, 1}, {1, 0}, {0, -1}, {-1, 0}}
	for _, d := range directions {
		newP := IntPosition{p.x + d.x, p.y + d.y}
		if newP.x >= 0 && newP.x < len(grid[0]) && newP.y >= 0 && newP.y < len(grid) && !grid[newP.y][newP.x] {
			neighbors = append(neighbors, newP)
		}
	}
	return neighbors
}

func astar(start, goal IntPosition, grid [][]bool) []IntPosition {
	openList := make(PriorityQueue, 0)
	heap.Init(&openList)

	startNode := &Node{intPos: start, gCost: 0, hCost: manhattanDistance(start, goal)}
	heap.Push(&openList, startNode)

	closedSet := make(map[IntPosition]bool)
	nodeMap := make(map[IntPosition]*Node)
	nodeMap[start] = startNode

	for openList.Len() > 0 {
		current := heap.Pop(&openList).(*Node)

		if current.intPos == goal {
			path := []IntPosition{}
			for current != nil {
				path = append([]IntPosition{current.intPos}, path...)
				current = current.Parent
			}
			return path
		}

		closedSet[current.intPos] = true

		for _, neighbor := range getNeighbors(current.intPos, grid) {
			if closedSet[neighbor] {
				continue
			}

			gCost := current.gCost + 1

			neighborNode, exists := nodeMap[neighbor]
			if !exists {
				neighborNode = &Node{intPos: neighbor, gCost: gCost, hCost: manhattanDistance(neighbor, goal), Parent: current}
				nodeMap[neighbor] = neighborNode
				heap.Push(&openList, neighborNode)
			} else if gCost < neighborNode.gCost {
				neighborNode.gCost = gCost
				neighborNode.Parent = current
				heap.Fix(&openList, neighborNode.HeapIdx)
			}
		}
	}

	return nil
}

func FindPath(start, goal Position, obstacles [][]bool) []Position {
	// Cast the coordinates to ints to simplify working with "tile space"
	// and then translate them to the nearest tile center point
	// First we find the closest corner between tiles
	// and then we offset to the center
	startTile := IntPosition{int(start.X) / 16, int(start.Y) / 16}
	goalTile := IntPosition{int(goal.X) / 16, int(goal.Y) / 16}

	intPath := astar(startTile, goalTile, obstacles)
	if intPath == nil {
		log.Println("A* couldn't find a valid path to a player")
		return nil
	}
	path := make([]Position, 0)
	for _, val := range intPath {
		path = append(path, Position{float64(val.x), float64(val.y)})
	}
	return path
}

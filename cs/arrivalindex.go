package cs

// Military arrival checks only inspect reports within three light-years.
// Populate neighbouring cells in report order, then retain the exact distance
// predicate. This is a conservative filter, not a change to combat geometry.
const arrivalCellSize = 64

func arrivalCellCoordinate(x int) int {
	if x < 0 {
		return (x+1)/arrivalCellSize - 1
	}
	return x / arrivalCellSize
}

func arrivalCellAt(position Vector) Vector {
	return Vector{arrivalCellCoordinate(position.X), arrivalCellCoordinate(position.Y)}
}

func (index *routeThreatIndex) buildArrivalCells() {
	index.arrivalCells = make(map[Vector][]int)
	for i, threat := range index.threats {
		cell := arrivalCellAt(threat.next)
		for x := cell.X - 1; x <= cell.X+1; x++ {
			for y := cell.Y - 1; y <= cell.Y+1; y++ {
				key := Vector{x, y}
				index.arrivalCells[key] = append(index.arrivalCells[key], i)
			}
		}
	}
}

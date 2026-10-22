package layout

type Direction string

const (
	DirectionRight Direction = "right"
	DirectionDown  Direction = "down"
)

type SplitInstruction struct {
	FromLogical int
	Direction   Direction
	NewLogical  int
}

const MaxPerRow = 3

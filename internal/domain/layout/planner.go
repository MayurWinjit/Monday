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

func Plan(n int) []SplitInstruction {
	if n <= 1 {
		return []SplitInstruction{}
	}

	rows := chunk(n, MaxPerRow)
	instructions := make([]SplitInstruction, 0, n-1)

	logical := 0
	prevRowFirst := 0
	for rowIdx, rowLen := range rows {
		var rowFirst int
		if rowIdx == 0 {
			rowFirst = logical
		} else {
			logical++
			rowFirst = logical
			instructions = append(instructions, SplitInstruction{
				FromLogical: prevRowFirst,
				Direction:   DirectionDown,
				NewLogical:  rowFirst,
			})
		}

		for i := 1; i < rowLen; i++ {
			logical++
			instructions = append(instructions, SplitInstruction{
				FromLogical: logical - 1,
				Direction:   DirectionRight,
				NewLogical:  logical,
			})
		}
		prevRowFirst = rowFirst
	}

	return instructions
}

func chunk(n, maxPerRow int) []int {
	rows := make([]int, 0, (n+maxPerRow-1)/maxPerRow)
	remaining := n
	for remaining > 0 {
		rowLen := remaining
		if rowLen > maxPerRow {
			rowLen = maxPerRow
		}
		rows = append(rows, rowLen)
		remaining -= rowLen
	}
	return rows
}

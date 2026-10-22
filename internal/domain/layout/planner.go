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

// Plan returns an ordered list of SplitInstruction describing how to split
// n panes out of a single starting pane (logical index 0, which already
// exists and needs no instruction) into a row-based grid with at most
// MaxPerRow panes per row - three agents become one row of three, five
// become a row of three and a row of two, and so on. It produces exactly
// n-1 instructions for n>=1 (zero for n<=1).
//
// Algorithm: chunk n into row sizes of at most MaxPerRow. The first pane of
// row 0 is logical 0 (already exists). For every subsequent row, emit one
// Down split from the first pane of the previous row to create the first
// pane of the new row, then emit len(row)-1 Right splits walking left to
// right within that row.
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

// chunk splits n items into row sizes of at most maxPerRow, e.g.
// chunk(5, 3) -> [3, 2], chunk(3, 3) -> [3], chunk(7, 3) -> [3, 3, 1].
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

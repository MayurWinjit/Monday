package layout

import "testing"

func TestChunk(t *testing.T) {
	tests := []struct {
		n    int
		want []int
	}{
		{3, []int{3}},
		{4, []int{3, 1}},
		{5, []int{3, 2}},
		{6, []int{3, 3}},
		{7, []int{3, 3, 1}},
		{1, []int{1}},
		{2, []int{2}},
	}
	for _, tt := range tests {
		got := chunk(tt.n, MaxPerRow)
		if len(got) != len(tt.want) {
			t.Fatalf("chunk(%d): expected %v, got %v", tt.n, tt.want, got)
		}
		for i := range tt.want {
			if got[i] != tt.want[i] {
				t.Fatalf("chunk(%d): expected %v, got %v", tt.n, tt.want, got)
			}
		}
	}
}

func TestPlan_DocumentedExamples(t *testing.T) {
	tests := []struct {
		n        int
		wantRows []int
	}{
		{3, []int{3}},
		{4, []int{3, 1}},
		{5, []int{3, 2}},
		{6, []int{3, 3}},
		{7, []int{3, 3, 1}},
	}

	for _, tt := range tests {
		instructions := Plan(tt.n)

		if len(instructions) != tt.n-1 {
			t.Fatalf("Plan(%d): expected %d instructions, got %d: %+v", tt.n, tt.n-1, len(instructions), instructions)
		}

		rows := simulateRows(tt.n, instructions)
		if len(rows) != len(tt.wantRows) {
			t.Fatalf("Plan(%d): expected row sizes %v, got %v", tt.n, tt.wantRows, rowSizes(rows))
		}
		for i, want := range tt.wantRows {
			if len(rows[i]) != want {
				t.Fatalf("Plan(%d): expected row sizes %v, got %v", tt.n, tt.wantRows, rowSizes(rows))
			}
		}
	}
}

func TestPlan_Invariants(t *testing.T) {
	for n := 1; n <= 12; n++ {
		instructions := Plan(n)

		if len(instructions) != max(0, n-1) {
			t.Fatalf("Plan(%d): expected %d instructions, got %d", n, max(0, n-1), len(instructions))
		}

		introduced := map[int]bool{0: true}
		seenNew := map[int]bool{}
		for _, instr := range instructions {
			if !introduced[instr.FromLogical] {
				t.Fatalf("Plan(%d): instruction %+v splits from a pane not yet introduced", n, instr)
			}
			if seenNew[instr.NewLogical] {
				t.Fatalf("Plan(%d): logical index %d produced twice", n, instr.NewLogical)
			}
			if instr.Direction != DirectionRight && instr.Direction != DirectionDown {
				t.Fatalf("Plan(%d): invalid direction %q", n, instr.Direction)
			}
			seenNew[instr.NewLogical] = true
			introduced[instr.NewLogical] = true
		}

		if len(introduced) != n {
			t.Fatalf("Plan(%d): expected %d distinct logical panes, got %d", n, n, len(introduced))
		}
	}
}

func TestPlan_NoPanesForZeroOrOne(t *testing.T) {
	if got := Plan(0); len(got) != 0 {
		t.Fatalf("Plan(0): expected no instructions, got %+v", got)
	}
	if got := Plan(1); len(got) != 0 {
		t.Fatalf("Plan(1): expected no instructions, got %+v", got)
	}
}

func simulateRows(n int, instructions []SplitInstruction) [][]int {
	rows := [][]int{{0}}
	rowOf := map[int]int{0: 0}

	for _, instr := range instructions {
		switch instr.Direction {
		case DirectionDown:
			rows = append(rows, []int{instr.NewLogical})
			rowOf[instr.NewLogical] = len(rows) - 1
		case DirectionRight:
			r := rowOf[instr.FromLogical]
			rows[r] = append(rows[r], instr.NewLogical)
			rowOf[instr.NewLogical] = r
		}
	}
	return rows
}

func rowSizes(rows [][]int) []int {
	sizes := make([]int, len(rows))
	for i, r := range rows {
		sizes[i] = len(r)
	}
	return sizes
}

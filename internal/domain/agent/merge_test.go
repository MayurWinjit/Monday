package agent

import "testing"

func TestMerge(t *testing.T) {
	tests := []struct {
		name    string
		global  []Agent
		project []Agent
		want    []string // expected names in order
		wantSrc map[string]AgentSource
	}{
		{
			name:    "no overlap, project appended",
			global:  []Agent{{Name: "architect"}, {Name: "reviewer"}},
			project: []Agent{{Name: "developer"}},
			want:    []string{"architect", "reviewer", "developer"},
		},
		{
			name:    "project overrides global on name clash",
			global:  []Agent{{Name: "architect", Description: "global version"}},
			project: []Agent{{Name: "architect", Description: "project version"}},
			want:    []string{"architect"},
		},
		{
			name:    "empty global",
			global:  nil,
			project: []Agent{{Name: "developer"}},
			want:    []string{"developer"},
		},
		{
			name:    "empty project",
			global:  []Agent{{Name: "architect"}},
			project: nil,
			want:    []string{"architect"},
		},
		{
			name:    "both empty",
			global:  nil,
			project: nil,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Merge(tt.global, tt.project)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d agents, got %d: %+v", len(tt.want), len(got), got)
			}
			for i, name := range tt.want {
				if got[i].Name != name {
					t.Fatalf("expected position %d to be %q, got %q", i, name, got[i].Name)
				}
			}
		})
	}
}

func TestMerge_ProjectOverrideKeepsGlobalPosition(t *testing.T) {
	global := []Agent{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	project := []Agent{{Name: "b", Description: "overridden"}}

	got := Merge(global, project)
	if len(got) != 3 {
		t.Fatalf("expected 3 agents, got %d: %+v", len(got), got)
	}
	if got[0].Name != "a" || got[1].Name != "b" || got[2].Name != "c" {
		t.Fatalf("expected order a,b,c, got %+v", got)
	}
	if got[1].Description != "overridden" {
		t.Fatalf("expected overridden description, got %+v", got[1])
	}
}

func TestMerge_DoesNotMutateInputs(t *testing.T) {
	global := []Agent{{Name: "a", Description: "orig"}}
	project := []Agent{{Name: "a", Description: "new"}}

	_ = Merge(global, project)

	if global[0].Description != "orig" {
		t.Fatalf("Merge must not mutate its global input, got %+v", global[0])
	}
}

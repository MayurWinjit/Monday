package pipeline

import (
	"testing"

	"github.com/MayurWinjit/monday/internal/domain/agent"
)

func TestValidate(t *testing.T) {
	catalog := []agent.Agent{{Name: "architect"}, {Name: "developer"}, {Name: "reviewer"}}

	tests := []struct {
		name    string
		p       Pipeline
		catalog []agent.Agent
		wantErr bool
	}{
		{
			name:    "valid single agent",
			p:       Pipeline{Name: "solo", Agents: []string{"architect"}},
			catalog: catalog,
			wantErr: false,
		},
		{
			name:    "valid multiple agents with repeat",
			p:       Pipeline{Name: "trio", Agents: []string{"architect", "developer", "developer", "reviewer"}},
			catalog: catalog,
			wantErr: false,
		},
		{
			name:    "empty name",
			p:       Pipeline{Name: "", Agents: []string{"architect"}},
			catalog: catalog,
			wantErr: true,
		},
		{
			name:    "whitespace-only name",
			p:       Pipeline{Name: "   ", Agents: []string{"architect"}},
			catalog: catalog,
			wantErr: true,
		},
		{
			name:    "untrimmed name",
			p:       Pipeline{Name: " trio", Agents: []string{"architect"}},
			catalog: catalog,
			wantErr: true,
		},
		{
			name:    "no agents",
			p:       Pipeline{Name: "empty", Agents: nil},
			catalog: catalog,
			wantErr: true,
		},
		{
			name:    "unknown agent",
			p:       Pipeline{Name: "trio", Agents: []string{"architect", "ghostwriter"}},
			catalog: catalog,
			wantErr: true,
		},
		{
			name:    "empty catalog rejects any agent",
			p:       Pipeline{Name: "trio", Agents: []string{"architect"}},
			catalog: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.p, tt.catalog)
			if tt.wantErr && err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

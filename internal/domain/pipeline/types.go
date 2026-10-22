package pipeline

import "time"

type Pipeline struct {
	Name      string    `json:"name"`
	Agents    []string  `json:"agents"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

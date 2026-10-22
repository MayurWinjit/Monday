package interaction

type Question struct {
	ID          string   `json:"id"`
	Question    string   `json:"question"`
	Description string   `json:"description,omitempty"`
	Options     []string `json:"options,omitempty"`
	Answer      *string  `json:"answer,omitempty"`
}

type InteractionFile struct {
	Agent     string     `json:"agent"`
	Questions []Question `json:"questions"`
}

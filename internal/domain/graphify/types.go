package graphify

// GraphRef identifies one saved graphify-out folder under ~/.monday/graphs,
// by the label the user picks it by (the directory name) and its absolute
// path on disk.
type GraphRef struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

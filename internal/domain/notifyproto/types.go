package notifyproto

type NotifyType string

const (
	NotifyQuestion NotifyType = "QUES"
	NotifyDone     NotifyType = "DONE"
)

type NotifyMessage struct {
	Type      NotifyType `json:"type"`
	SessionID string     `json:"session_id"`
	PaneID    string     `json:"pane_id"`
}

type NotifyAck struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

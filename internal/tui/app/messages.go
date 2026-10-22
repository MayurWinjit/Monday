package app

// This file holds tea.Msg types shared across more than one screen.
// Screen-local messages (e.g. a single screen's own result/tick messages)
// live inside that screen's own package instead - SwitchScreenMsg in
// model.go is the only message every screen needs to know about, so it is
// the only one defined so far. Later steps add to this file only when a
// message genuinely crosses screen boundaries.

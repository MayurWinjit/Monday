package session

import (
	"testing"
	"time"

	"github.com/MayurWinjit/monday/internal/domain/history"
	"github.com/MayurWinjit/monday/internal/domain/interaction"
	"github.com/MayurWinjit/monday/internal/domain/notifyproto"
)

func TestNotifyRouter_RoutesDone(t *testing.T) {
	backend := newFakeBackend()
	sess := baseSessionForHandlers()
	backend.sessions[sess.ID] = sess
	backend.histories[sess.ID] = history.History{Requirement: "x"}

	svc, _, _ := newTestService(&fakeHerdr{}, backend, time.Now())
	router := NewNotifyRouter(svc)

	updated, file, err := router.Route(notifyproto.NotifyMessage{Type: notifyproto.NotifyDone, SessionID: "s1", PaneID: "w1:p2"})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if file != nil {
		t.Fatalf("expected nil interaction file for a DONE message, got %+v", file)
	}
	if updated.Steps[0].Status != StepDone {
		t.Fatalf("expected step 0 done, got %s", updated.Steps[0].Status)
	}
}

func TestNotifyRouter_RoutesQuestion(t *testing.T) {
	backend := newFakeBackend()
	sess := baseSessionForHandlers()
	sess.Steps[1].Status = StepRunning
	backend.sessions[sess.ID] = sess
	backend.interactions["s1/developer"] = interaction.InteractionFile{
		Agent:     "developer",
		Questions: []interaction.Question{{ID: "q1", Question: "Which approach?"}},
	}

	svc, _, _ := newTestService(&fakeHerdr{}, backend, time.Now())
	router := NewNotifyRouter(svc)

	updated, file, err := router.Route(notifyproto.NotifyMessage{Type: notifyproto.NotifyQuestion, SessionID: "s1", PaneID: "w1:p3"})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if file == nil || len(file.Questions) != 1 {
		t.Fatalf("expected the developer interactions file, got %+v", file)
	}
	if updated.Steps[1].Status != StepNeedsInput {
		t.Fatalf("expected step 1 needs_input, got %s", updated.Steps[1].Status)
	}
}

func TestNotifyRouter_UnknownTypeErrors(t *testing.T) {
	backend := newFakeBackend()
	backend.sessions["s1"] = baseSessionForHandlers()
	svc, _, _ := newTestService(&fakeHerdr{}, backend, time.Now())
	router := NewNotifyRouter(svc)

	_, _, err := router.Route(notifyproto.NotifyMessage{Type: "BOGUS", SessionID: "s1", PaneID: "w1:p2"})
	if err == nil {
		t.Fatalf("expected an error for an unknown notify type")
	}
}

package pipeline

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/onelastcommit/noctra/internal/source"
)

type progressSource struct {
	fetchSource
	mu      sync.Mutex
	calls   []string
	markErr error
}

func (s *progressSource) record(call string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, call)
}

func (s *progressSource) MarkInProgress(context.Context, source.Ticket) error {
	s.record("mark")
	return s.markErr
}

func (s *progressSource) ReleaseInProgress(ctx context.Context, _ source.Ticket) error {
	if ctx.Err() != nil {
		s.record("release-on-dead-context")
		return ctx.Err()
	}
	s.record("release")
	return nil
}

func (s *progressSource) BackToTrigger(context.Context, source.Ticket, string) error {
	s.record("back")
	return nil
}

func (s *progressSource) MarkReady(context.Context, source.Ticket, source.ReadyInfo) error {
	s.record("ready")
	return nil
}

func progressPipeline(src *progressSource) *Pipeline {
	return &Pipeline{sources: []source.TicketSource{src}}
}

var progressTicket = source.Ticket{Source: "linear", Identifier: "ENG-1"}

func TestInProgress_UnsettledRunIsReleased(t *testing.T) {
	src := &progressSource{fetchSource: fetchSource{name: "linear"}}
	p := progressPipeline(src)

	func() {
		p.claimInProgress(context.Background(), progressTicket)
		defer p.releaseInProgress(context.Background(), progressTicket)
	}()

	if got := strings.Join(src.calls, ","); got != "mark,release" {
		t.Fatalf("calls = %s; want mark,release", got)
	}
}

func TestInProgress_BackToTriggerReleasesOnce(t *testing.T) {
	src := &progressSource{fetchSource: fetchSource{name: "linear"}}
	p := progressPipeline(src)

	func() {
		p.claimInProgress(context.Background(), progressTicket)
		defer p.releaseInProgress(context.Background(), progressTicket)
		p.ticketBackToTrigger(context.Background(), progressTicket, "blocked")
	}()

	if got := strings.Join(src.calls, ","); got != "mark,release,back" {
		t.Fatalf("calls = %s; want mark,release,back", got)
	}
}

func TestInProgress_SettledRunIsNotReleased(t *testing.T) {
	src := &progressSource{fetchSource: fetchSource{name: "linear"}}
	p := progressPipeline(src)

	func() {
		p.claimInProgress(context.Background(), progressTicket)
		defer p.releaseInProgress(context.Background(), progressTicket)
		p.settleInProgress(progressTicket.Identifier)
	}()

	if got := strings.Join(src.calls, ","); got != "mark" {
		t.Fatalf("calls = %s; want only mark", got)
	}
}

func TestInProgress_FailedMarkIsNotReleased(t *testing.T) {
	src := &progressSource{fetchSource: fetchSource{name: "linear"}, markErr: errors.New("no transition")}
	p := progressPipeline(src)

	func() {
		p.claimInProgress(context.Background(), progressTicket)
		defer p.releaseInProgress(context.Background(), progressTicket)
		p.ticketBackToTrigger(context.Background(), progressTicket, "blocked")
	}()

	if got := strings.Join(src.calls, ","); got != "mark,back" {
		t.Fatalf("calls = %s; want mark,back", got)
	}
}

func TestInProgress_ReleaseSurvivesCancelledContext(t *testing.T) {
	src := &progressSource{fetchSource: fetchSource{name: "linear"}}
	p := progressPipeline(src)
	ctx, cancel := context.WithCancel(context.Background())

	p.claimInProgress(ctx, progressTicket)
	cancel()
	p.releaseInProgress(ctx, progressTicket)

	if got := strings.Join(src.calls, ","); got != "mark,release" {
		t.Fatalf("calls = %s; want mark,release", got)
	}
}

package telegram

import (
	"context"
	"strings"
	"testing"
	"time"
)

type scriptedConv struct {
	prompts []string
	got     []string
}

func (c *scriptedConv) Answer(_ context.Context, text string) (string, bool) {
	c.got = append(c.got, text)
	if len(c.got) >= len(c.prompts) {
		return "done", true
	}
	return c.prompts[len(c.got)], false
}

func registerFlow(d *Dispatcher, conv *scriptedConv) {
	d.RegisterConversation("flow", "a flow", func(_ context.Context, args string) (string, Conversation) {
		if args != "" {
			reply, done := conv.Answer(context.Background(), args)
			if done {
				return reply, nil
			}
			return reply, conv
		}
		return conv.prompts[0], conv
	})
}

func TestConversation_RoutesPlainMessagesUntilDone(t *testing.T) {
	d := NewDispatcher()
	conv := &scriptedConv{prompts: []string{"repo?", "project?", "branch?"}}
	registerFlow(d, conv)

	if reply := d.Dispatch(context.Background(), "/flow"); reply != "repo?" {
		t.Fatalf("start = %q, want %q", reply, "repo?")
	}
	if reply := d.Dispatch(context.Background(), "owner/name"); reply != "project?" {
		t.Fatalf("first answer = %q, want %q", reply, "project?")
	}
	if reply := d.Dispatch(context.Background(), "Noctra"); reply != "branch?" {
		t.Fatalf("second answer = %q, want %q", reply, "branch?")
	}
	if reply := d.Dispatch(context.Background(), "main"); reply != "done" {
		t.Fatalf("final answer = %q, want %q", reply, "done")
	}

	want := []string{"owner/name", "Noctra", "main"}
	for i, w := range want {
		if conv.got[i] != w {
			t.Errorf("answer %d = %q, want %q", i, conv.got[i], w)
		}
	}

	if reply := d.Dispatch(context.Background(), "stray"); !strings.Contains(reply, "Unknown command") {
		t.Errorf("after completion = %q, want the unknown-command reply", reply)
	}
}

func TestConversation_ArgsConsumedAsFirstAnswer(t *testing.T) {
	d := NewDispatcher()
	conv := &scriptedConv{prompts: []string{"repo?", "project?"}}
	registerFlow(d, conv)

	if reply := d.Dispatch(context.Background(), "/flow owner/name"); reply != "project?" {
		t.Fatalf("start with args = %q, want %q", reply, "project?")
	}
	if len(conv.got) != 1 || conv.got[0] != "owner/name" {
		t.Fatalf("conversation got %v, want the args as the first answer", conv.got)
	}
}

func TestConversation_CancelStopsIt(t *testing.T) {
	d := NewDispatcher()
	conv := &scriptedConv{prompts: []string{"repo?", "project?"}}
	registerFlow(d, conv)

	d.Dispatch(context.Background(), "/flow")
	if reply := d.Dispatch(context.Background(), "/cancel"); !strings.Contains(reply, "Cancelled") {
		t.Fatalf("/cancel = %q, want a cancellation", reply)
	}
	if reply := d.Dispatch(context.Background(), "owner/name"); !strings.Contains(reply, "Unknown command") {
		t.Errorf("after cancel = %q, want the flow to be gone", reply)
	}
	if len(conv.got) != 0 {
		t.Errorf("cancelled flow still consumed %v", conv.got)
	}
}

func TestCancel_WithNothingPending(t *testing.T) {
	d := NewDispatcher()
	if reply := d.Dispatch(context.Background(), "/cancel"); !strings.Contains(reply, "Nothing to cancel") {
		t.Errorf("/cancel with no flow = %q", reply)
	}
}

func TestConversation_OtherCommandInterruptsIt(t *testing.T) {
	d := NewDispatcher()
	conv := &scriptedConv{prompts: []string{"repo?", "project?"}}
	registerFlow(d, conv)

	d.Dispatch(context.Background(), "/flow")
	reply := d.Dispatch(context.Background(), "/ping")
	if !strings.Contains(reply, "Abandoned") {
		t.Errorf("interrupting reply = %q, want it to mention the abandoned flow", reply)
	}
	if !strings.Contains(reply, "pong") {
		t.Errorf("interrupting reply = %q, want the command to still run", reply)
	}
	if len(conv.got) != 0 {
		t.Errorf("interrupted flow still consumed %v", conv.got)
	}
}

func TestConversation_ExpiresAfterTTL(t *testing.T) {
	now := time.Now()
	d := NewDispatcher()
	d.now = func() time.Time { return now }

	conv := &scriptedConv{prompts: []string{"repo?", "project?"}}
	registerFlow(d, conv)
	d.Dispatch(context.Background(), "/flow")

	now = now.Add(sessionTTL + time.Second)
	reply := d.Dispatch(context.Background(), "owner/name")
	if !strings.Contains(reply, "timed out") {
		t.Errorf("expired flow = %q, want a timeout notice", reply)
	}
	if len(conv.got) != 0 {
		t.Errorf("expired flow still consumed %v", conv.got)
	}
}

func TestConversation_TTLRefreshesOnEachAnswer(t *testing.T) {
	now := time.Now()
	d := NewDispatcher()
	d.now = func() time.Time { return now }

	conv := &scriptedConv{prompts: []string{"repo?", "project?", "branch?"}}
	registerFlow(d, conv)
	d.Dispatch(context.Background(), "/flow")

	for i := range 2 {
		now = now.Add(sessionTTL - time.Minute)
		if reply := d.Dispatch(context.Background(), "answer"); strings.Contains(reply, "timed out") {
			t.Fatalf("answer %d timed out despite arriving inside the TTL", i)
		}
	}
}

func TestHelp_ListsConversationCommands(t *testing.T) {
	d := NewDispatcher()
	d.RegisterConversation("addrepo", "Add a repository", func(_ context.Context, _ string) (string, Conversation) {
		return "", nil
	})

	reply := d.Dispatch(context.Background(), "/help")
	if !strings.Contains(reply, "/addrepo") {
		t.Errorf("help = %q, want it to list /addrepo", reply)
	}
	if !strings.Contains(reply, "/cancel") {
		t.Errorf("help = %q, want it to list /cancel", reply)
	}
}

func TestConversation_NilConversationStartsNoFlow(t *testing.T) {
	d := NewDispatcher()
	d.RegisterConversation("flow", "a flow", func(_ context.Context, _ string) (string, Conversation) {
		return "bad input", nil
	})

	if reply := d.Dispatch(context.Background(), "/flow"); reply != "bad input" {
		t.Fatalf("start = %q", reply)
	}
	if reply := d.Dispatch(context.Background(), "anything"); !strings.Contains(reply, "Unknown command") {
		t.Errorf("plain text = %q, want no flow to be pending", reply)
	}
}

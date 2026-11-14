package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/onelastcommit/noctra/internal/config"
	"github.com/onelastcommit/noctra/internal/linear"
	"github.com/onelastcommit/noctra/internal/source"
)

const releaseInProgressTimeout = 30 * time.Second

func (p *Pipeline) claimInProgress(ctx context.Context, ticket source.Ticket) {
	if err := p.ticketSource(ticket).MarkInProgress(ctx, ticket); err != nil {
		slog.Warn("could not mark ticket in progress", "id", ticket.Identifier, "source", ticket.Source, "err", err)
		return
	}
	p.mu.Lock()
	if p.inProgress == nil {
		p.inProgress = map[string]struct{}{}
	}
	p.inProgress[ticket.Identifier] = struct{}{}
	p.mu.Unlock()
}

func (p *Pipeline) settleInProgress(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, claimed := p.inProgress[id]
	delete(p.inProgress, id)
	return claimed
}

func (p *Pipeline) releaseInProgress(ctx context.Context, ticket source.Ticket) {
	if !p.settleInProgress(ticket.Identifier) {
		return
	}
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), releaseInProgressTimeout)
	defer cancel()
	if err := p.ticketSource(ticket).ReleaseInProgress(releaseCtx, ticket); err != nil {
		slog.Warn("could not move ticket out of in progress", "id", ticket.Identifier, "source", ticket.Source, "err", err)
	}
}

func inProgressSummary(cfg *config.Config, states linear.StateIDs) string {
	var parts []string
	for _, name := range cfg.TicketSources {
		switch name {
		case "linear":
			if states.InProgress == "" {
				parts = append(parts, fmt.Sprintf("Linear %q not found, tickets stay put", cfg.InProgressState))
			} else {
				parts = append(parts, fmt.Sprintf("Linear %q", cfg.InProgressState))
			}
		case "jira":
			parts = append(parts, fmt.Sprintf("Jira %q", cfg.JiraInProgressStatus))
		case "github":
			parts = append(parts, fmt.Sprintf("GitHub label %q", cfg.GitHubInProgressLabel))
		}
	}
	return strings.Join(parts, ", ")
}

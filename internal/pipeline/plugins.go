package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/onelastcommit/noctra/internal/plugins"
)

func (p *Pipeline) installPlugins(ctx context.Context) {
	wanted, err := plugins.Resolve(p.cfg.PluginPacks, p.cfg.PluginsExtra)
	if err != nil {
		slog.Warn("agent plugins disabled", "err", err)
		return
	}
	if len(wanted) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, plugins.SetupTimeout)
	defer cancel()
	wanted, unmet := plugins.CheckRequirements(ctx, wanted)
	for _, u := range unmet {
		slog.Warn("agent skill left out; its runtime dependency is missing", "skill", u.Plugin+"/"+u.Skill, "needs", u.Requirement.Name, "err", u.Err, "fix", u.Requirement.Hint)
	}
	p.pluginSkipped = plugins.SkillNames(unmet)
	installed, errs := plugins.InstallAll(ctx, p.cfg.PluginsDir, wanted)
	for _, err := range errs {
		slog.Warn("agent plugin unavailable; runs continue without it", "err", err)
	}
	p.plugins = installed
	p.pluginFailures = len(errs)
}

func (p *Pipeline) pluginDirs() []string {
	dirs := make([]string, 0, len(p.plugins))
	for _, inst := range p.plugins {
		dirs = append(dirs, inst.Dir)
	}
	return dirs
}

func pluginSummary(packs []string, installed []plugins.Installed, failures int, skipped []string) string {
	if !plugins.Enabled(packs) && len(installed) == 0 && failures == 0 && len(skipped) == 0 {
		return "Disabled"
	}
	skills := 0
	var unvetted []string
	for _, inst := range installed {
		skills += len(inst.Skills)
		if !inst.Vetted {
			unvetted = append(unvetted, inst.Name)
		}
	}
	summary := fmt.Sprintf("%d plugins, %d skills", len(installed), skills)
	if plugins.Enabled(packs) {
		summary = strings.Join(packs, ", ") + " (" + summary + ")"
	}
	if len(unvetted) > 0 {
		summary += " + unvetted: " + strings.Join(unvetted, ", ")
	}
	if len(skipped) > 0 {
		summary += "; left out for missing dependencies: " + strings.Join(skipped, ", ")
	}
	if failures > 0 {
		summary += fmt.Sprintf("; %d failed to fetch", failures)
	}
	return summary
}

package setup

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/onelastcommit/noctra/internal/config"
	"github.com/onelastcommit/noctra/internal/plugins"
)

const pluginSetupTimeout = 3 * time.Minute

func setUpPlugins(scriptDir string) {
	cfg, err := config.Load(scriptDir)
	if err != nil {
		fmt.Printf("⚠️  Could not reload config to set up agent plugins: %v\n", err)
		return
	}
	wanted, err := plugins.Resolve(cfg.PluginPacks, cfg.PluginsExtra)
	if err != nil {
		fmt.Printf("⚠️  Agent plugins not set up: %v\n", err)
		return
	}
	if len(wanted) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), pluginSetupTimeout)
	defer cancel()
	installPluginSet(ctx, os.Stdout, cfg.PluginsDir, wanted)
}

func installPluginSet(ctx context.Context, out io.Writer, root string, wanted []plugins.Plugin) (installed, failed int) {
	fmt.Fprintf(out, "\n─── Agent plugins ───\nSetting up %d plugins in %s\n", len(wanted), root)
	skills := 0
	for _, p := range wanted {
		inst, err := plugins.Install(ctx, root, p)
		if err != nil {
			failed++
			fmt.Fprintf(out, "  ⚠️  %-18s %v\n", p.Name, err)
			continue
		}
		installed++
		skills += len(inst.Skills)
		label := ""
		if !inst.Vetted {
			label = " (unvetted)"
		}
		fmt.Fprintf(out, "  ✓ %-18s %d skills%s\n", p.Name, len(inst.Skills), label)
	}
	fmt.Fprintf(out, "✅ %d plugins ready, %d skills — every agent run picks them up.\n", installed, skills)
	if failed > 0 {
		fmt.Fprintf(out, "   %d failed; Noctra retries them on every start and runs without them meanwhile.\n", failed)
	}
	fmt.Fprintln(out)
	return installed, failed
}

package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"fusion-platform.io/fusion-runner/internal/runner"
)

const asciiArt = `
    ____           _
   / __/_  _______(_)___  ____       _      _____  ____ __   _____
  / /_/ / / / ___/ / __ \/ __ \_____| | /| / / _ \/ __ '/ | / / _ \
 / __/ /_/ (__  ) / /_/ / / / /_____/ |/ |/ /  __/ /_/ /| |/ /  __/
/_/  \__,_/____/_/\____/_/ /_/      |__/|__/\___/\__,_/ |___/\___/
`

func main() {
	cfg := runner.LoadConfig()
	slog.SetDefault(runner.NewLogger(cfg))

	// Printed before dispatch so the provided metadata is always visible in
	// the log, even when cfg.RunnerType itself is what's wrong.
	printStartupBanner(cfg)
	slog.Info("runner starting", "entrypoint", cfg.Entrypoint, "mountPath", cfg.MountPath, "port", cfg.Port)

	r, err := runner.New(cfg)
	if err != nil {
		slog.Error("unsupported runner type", "error", err)
		os.Exit(1)
	}

	if err := r.Setup(); err != nil {
		slog.Error("setup failed", "error", err)
		os.Exit(1)
	}

	if err := r.Exec(); err != nil {
		slog.Error("exec failed", "error", err)
		os.Exit(1)
	}
}

func printStartupBanner(cfg runner.Config) {
	fmt.Print(asciiArt)
	fmt.Println("─────────────────────────────────────────────────────────────────────")
	fmt.Printf("  Started: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Println("─────────────────────────────────────────────────────────────────────")
	printField("Artifact",      cfg.Artifact)
	printField("Tag",           cfg.Tag)
	printField("Version",       cfg.Version)
	printField("Namespace",     cfg.Namespace)
	printField("Runner type",   cfg.RunnerType)
	printField("Entrypoint",    cfg.Entrypoint)
	printField("Port",          cfg.Port)
	printField("Mount path",    cfg.MountPath)
	printField("Maintainer",    cfg.Maintainer)
	printField("Builder image", cfg.BuilderImage)
	printField("Ingress path",  cfg.IngressPath)
	fmt.Println("─────────────────────────────────────────────────────────────────────")
}

func printField(label, value string) {
	if value != "" {
		fmt.Printf("  %-16s %s\n", label+":", value)
	}
}

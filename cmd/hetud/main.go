package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/claude"
	"github.com/3toInf/hetu/internal/config"
	"github.com/3toInf/hetu/internal/daemon"
	"github.com/3toInf/hetu/internal/logx"
	"github.com/3toInf/hetu/internal/policy"
	"github.com/3toInf/hetu/internal/project"
	"github.com/3toInf/hetu/internal/store"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{Use: "hetud"}
	serve := &cobra.Command{Use: "serve", RunE: serveRun}
	root.AddCommand(serve)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serveRun(cmd *cobra.Command, _ []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := config.EnsureDataDir(); err != nil {
		return err
	}

	// Wire the daemon logger (file + stderr) before anything logs.
	lg, logF, err := logx.Setup(logx.Options{Path: config.LogPath(), Level: logx.LevelFromEnv(), Stderr: true})
	if err != nil {
		return err
	}
	defer func() {
		if logF != nil {
			logF.Close()
		}
	}()
	slog.SetDefault(lg)

	// Single-instance guard: refuse to start if another hetud is already
	// running, so a second `hetud serve` can't steal the unix socket and orphan
	// the live process. The lock is held until serveRun returns.
	lock, err := daemon.AcquireInstanceLock(config.LockPath())
	if err != nil {
		return err
	}
	defer lock.Close()

	// Ensure socket directory exists
	socketPath := config.SocketPath()
	socketDir := filepath.Dir(socketPath)
	if err := os.MkdirAll(socketDir, 0o700); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	st, err := store.Open(ctx, config.DBPath())
	if err != nil {
		return err
	}
	defer st.Close()

	claudeAgent := claude.NewAgent(claude.Options{
		SocketPath:   socketPath,
		SettingsPath: config.ClaudeSettingsPath(),
	})
	resolver := project.NewResolver(st)
	agents := map[string]agent.Agent{"claude": claudeAgent}

	// Seed the approval-rules file on first run, then build a loader watching it.
	rulesPath := config.RulesPath()
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		if err := policy.Save(rulesPath, policy.DefaultRules()); err != nil {
			return fmt.Errorf("seed rules: %w", err)
		}
	}
	mgr := daemon.NewSessionManagerOpts(st, resolver, agents, daemon.ManagerOptions{
		Policy: policy.NewLoader(rulesPath),
		Logger: lg,
	})
	sch := daemon.NewDiscoveryScheduler(st, resolver, map[string]agent.DiscoverySource{"claude": claudeAgent.DiscoverySource()})

	_ = sch.Run(ctx) // initial discovery
	srv := daemon.NewServerWithLogger(st, mgr, sch, lg)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ctx, socketPath) }()

	select {
	case <-ctx.Done():
		// graceful: stop driven sessions
	case err := <-errCh:
		return err
	}
	_ = mgr.Close()
	srv.Shutdown(ctx)
	return nil
}

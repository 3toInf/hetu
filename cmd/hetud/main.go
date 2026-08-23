package main

import (
	"fmt"
	"log/slog"
	"net/http"
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

// Web-server flags, bound in newServeCmd. Package-level so serveRun can read
// them; the values are only ever written by cobra during flag parsing.
var (
	webAddr string // HTTP listen addr ("" = config.WebAddr() unless --no-web)
	webDir  string // serve frontend from this disk dir ("" = embedded dist)
	noWeb   bool   // do not start the HTTP server
)

func main() {
	root := &cobra.Command{Use: "hetud"}
	root.AddCommand(newServeCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newServeCmd builds the `hetud serve` command. Extracted from main() so tests
// can construct it and parse its flags without executing the daemon.
func newServeCmd() *cobra.Command {
	c := &cobra.Command{Use: "serve", RunE: serveRun}
	c.Flags().StringVar(&webAddr, "web-addr", "", "HTTP listen addr (default HETU_WEB_ADDR or 127.0.0.1:19191); empty with --no-web = off")
	c.Flags().StringVar(&webDir, "web-dir", "", "serve frontend from this disk dir instead of the embedded dist")
	c.Flags().BoolVar(&noWeb, "no-web", false, "do not start the HTTP server")
	return c
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

	// HTTP web server. Effective addr: --web-addr wins; else config.WebAddr()
	// (HETU_WEB_ADDR or default) unless --no-web turns the server off.
	addr := webAddr
	if addr == "" && !noWeb {
		addr = config.WebAddr()
	}
	if webDir != "" {
		if fi, err := os.Stat(webDir); err != nil || !fi.IsDir() {
			return fmt.Errorf("web-dir %s is not a directory", webDir)
		}
	}
	// hs is assigned before the goroutine so the ctx.Done branch below can
	// reach it to shut the HTTP server down alongside the unix-socket one.
	var hs *http.Server
	if addr != "" {
		ws := daemon.NewWebServer(srv, addr, webDir)
		hs = &http.Server{Addr: addr, Handler: ws.Handler()}
		go func() { errCh <- hs.ListenAndServe() }()
	}

	select {
	case <-ctx.Done():
		// graceful: stop driven sessions and both servers
		if hs != nil {
			hs.Shutdown(ctx)
		}
	case err := <-errCh:
		return err
	}
	_ = mgr.Close()
	srv.Shutdown(ctx)
	return nil
}

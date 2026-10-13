package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var groupJIDPattern = regexp.MustCompile(`^\d{5,25}(?:-\d{5,20})?@g\.us$`)

// Config holds infrastructure values only. Everything else is runtime Settings, edited in /admin.
type Config struct {
	ListenAddr string
	StatePath  string
	WebDir     string
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func loadConfig() Config {
	return Config{
		ListenAddr: envOr("LISTEN_ADDR", ":8080"),
		StatePath:  envOr("STATE_PATH", "data/state.json"),
		WebDir:     envOr("WEB_DIR", "web/dist"),
	}
}

// newApp builds an App with its request-independent state. Stores are opened by the caller.
func newApp(cfg Config, client *http.Client) *App {
	return &App{cfg: cfg, client: client, now: time.Now, sessions: newSessionStore(), limiter: newAttemptLimiter(), status: &runtimeStatus{}}
}

func main() {
	envFile := flag.String("env", ".env", "optional legacy configuration file, read once to seed settings")
	preview := flag.Bool("preview", false, "print a fresh English agenda; never send")
	discover := flag.Bool("discover", false, "query the configured WhatsApp group; never send")
	health := flag.Bool("healthcheck", false, "check the local HTTP service")
	flag.Parse()
	cfg := loadConfig()
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	ctx := context.Background()
	if *health {
		address := cfg.ListenAddr
		if strings.HasPrefix(address, ":") {
			address = "127.0.0.1" + address
		}
		r, err := client.Get("http://" + address + "/healthz")
		if err != nil {
			os.Exit(1)
		}
		r.Body.Close()
		if r.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}
	a := newApp(cfg, client)
	dataDir := filepath.Dir(cfg.StatePath)
	var err error
	a.settings, err = openSettings(dataDir, func() Settings { return seedSettings(*envFile) })
	if err != nil {
		log.Fatal(err)
	}
	if *preview {
		p, err := a.preview(ctx, a.settings.current())
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(p.Text)
		return
	}
	if *discover {
		g, err := a.discover(ctx, a.settings.current())
		if err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(os.Stdout).Encode(g)
		return
	}
	a.store, err = openStore(cfg.StatePath)
	if err != nil {
		log.Fatal(err)
	}
	a.admin, err = openAdmin(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	a.audit, err = openAudit(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &http.Server{Addr: cfg.ListenAddr, Handler: a.handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 45 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	go a.runWorker(ctx)
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	s := a.settings.current()
	log.Printf("NBCC API listening on %s; send_enabled=%t; poll_interval=%s", cfg.ListenAddr, s.SendEnabled, s.pollInterval())
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var groupJIDPattern = regexp.MustCompile(`^\d{5,25}(?:-\d{5,20})?@g\.us$`)

type Config struct {
	FeedURL, APIKey, InstanceID, GroupJID, GroupName, AdminToken, WebhookSecret, ListenAddr, StatePath string
	SendEnabled                                                                                        bool
	PollInterval                                                                                       time.Duration
}

func loadConfig(path string) (Config, error) {
	values := map[string]string{}
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				return Config{}, errors.New("invalid .env line")
			}
			values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), "\"'")
		}
		if err = scanner.Err(); err != nil {
			return Config{}, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	get := func(key, fallback string) string {
		if v, ok := os.LookupEnv(key); ok {
			return v
		}
		if v, ok := values[key]; ok {
			return v
		}
		return fallback
	}
	c := Config{FeedURL: get("NBCC_FEED_URL", ""), APIKey: get("GRAPNEL_API_KEY", ""), InstanceID: get("GRAPNEL_INSTANCE_ID", ""), GroupJID: get("GRAPNEL_GROUP_JID", ""), GroupName: get("GRAPNEL_GROUP_NAME", "NBCC Study Group"), AdminToken: get("ADMIN_TOKEN", ""), WebhookSecret: get("WEBHOOK_SECRET", ""), ListenAddr: get("LISTEN_ADDR", ":8080"), StatePath: get("STATE_PATH", "data/state.json")}
	c.SendEnabled, err = strconv.ParseBool(get("SEND_ENABLED", "false"))
	if err != nil {
		return c, errors.New("SEND_ENABLED must be true or false")
	}
	c.PollInterval, err = time.ParseDuration(get("POLL_INTERVAL", "5m"))
	if err != nil || c.PollInterval < time.Minute {
		return c, errors.New("POLL_INTERVAL must be at least 1m")
	}
	u, err := url.Parse(c.FeedURL)
	if err != nil || u.Scheme != "https" || u.Host != "elearning.nbcc.ca" || u.Path != "/d2l/le/calendar/feed/user/feed.ics" || u.Query().Get("token") == "" || u.User != nil || u.Fragment != "" {
		return c, errors.New("NBCC_FEED_URL must be the private HTTPS NBCC calendar feed")
	}
	if len(c.AdminToken) < 32 || len(c.WebhookSecret) < 32 {
		return c, errors.New("ADMIN_TOKEN and WEBHOOK_SECRET need at least 32 characters")
	}
	if c.InstanceID != "" && !uuidPattern.MatchString(c.InstanceID) {
		return c, errors.New("invalid GRAPNEL_INSTANCE_ID")
	}
	if c.GroupJID != "" && !groupJIDPattern.MatchString(c.GroupJID) {
		return c, errors.New("invalid GRAPNEL_GROUP_JID")
	}
	if c.SendEnabled && (c.APIKey == "" || c.InstanceID == "" || c.GroupJID == "") {
		return c, errors.New("sending requires API key, instance and exact group JID")
	}
	return c, nil
}

func main() {
	envFile := flag.String("env", ".env", "local configuration file")
	preview := flag.Bool("preview", false, "print a fresh English agenda; never send")
	discover := flag.Bool("discover", false, "query the configured WhatsApp group; never send")
	register := flag.String("register-webhook", "", "register a public HTTPS callback; does not send WhatsApp messages")
	health := flag.Bool("healthcheck", false, "check the local HTTP service")
	flag.Parse()
	cfg, err := loadConfig(*envFile)
	if err != nil {
		log.Fatal(err)
	}
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	a := &App{cfg: cfg, client: client, now: time.Now}
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
	if *preview {
		p, err := a.preview(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(p.Text)
		return
	}
	if *discover {
		g, err := a.discover(ctx)
		if err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(os.Stdout).Encode(g)
		return
	}
	if *register != "" {
		if cfg.APIKey == "" || cfg.InstanceID == "" || cfg.GroupJID == "" {
			log.Fatal("configure the exact instance and group before registering a webhook")
		}
		if err = a.registerWebhook(ctx, *register); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Webhook registered. SEND_ENABLED is unchanged.")
		return
	}
	a.store, err = openStore(cfg.StatePath)
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
	log.Printf("NBCC API listening on %s; send_enabled=%t; poll_interval=%s", cfg.ListenAddr, cfg.SendEnabled, cfg.PollInterval)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/t-kiattisak/kibana-rca-agent/internal/analyzer"
	"github.com/t-kiattisak/kibana-rca-agent/internal/esclient"
	"github.com/t-kiattisak/kibana-rca-agent/internal/notifier"
	"github.com/t-kiattisak/kibana-rca-agent/internal/sanitizer"
)

type Config struct {
	ElasticURL     string
	ElasticIndex   string
	ElasticUser    string
	ElasticPass    string
	GeminiKey      string
	GeminiModel    string
	SlackWebhook   string
	DiscordWebhook string
	PollInterval   time.Duration
	TimeWindow     time.Duration
	ErrorThreshold int
}

func loadConfig() Config {
	loadDotEnv()
	pollSec, _ := strconv.Atoi(getEnv("POLL_INTERVAL_SECONDS", "15"))
	timeWinMin, _ := strconv.Atoi(getEnv("TIME_WINDOW_MINUTES", "1"))
	threshold, _ := strconv.Atoi(getEnv("ERROR_THRESHOLD_COUNT", "3"))

	return Config{
		ElasticURL:     getEnv("ELASTICSEARCH_URL", "http://localhost:9200"),
		ElasticIndex:   getEnv("ELASTICSEARCH_INDEX", "logs-app-*"),
		ElasticUser:    getEnv("ELASTICSEARCH_USERNAME", ""),
		ElasticPass:    getEnv("ELASTICSEARCH_PASSWORD", ""),
		GeminiKey:      getEnv("GEMINI_API_KEY", ""),
		GeminiModel:    getEnv("GEMINI_MODEL", "gemini-2.5-flash"),
		SlackWebhook:   getEnv("SLACK_WEBHOOK_URL", ""),
		DiscordWebhook: getEnv("DISCORD_WEBHOOK_URL", ""),
		PollInterval:   time.Duration(pollSec) * time.Second,
		TimeWindow:     time.Duration(timeWinMin) * time.Minute,
		ErrorThreshold: threshold,
	}
}

func main() {
	cfg := loadConfig()

	log.Println("🚀 Starting Kibana AI RCA Agent...")
	log.Printf("• Elasticsearch URL : %s (Index: %s)", cfg.ElasticURL, cfg.ElasticIndex)
	log.Printf("• Error Threshold   : %d errors in %v", cfg.ErrorThreshold, cfg.TimeWindow)
	log.Printf("• Poll Interval     : %v", cfg.PollInterval)
	log.Printf("• Gemini Model      : %s", cfg.GeminiModel)
	if cfg.DiscordWebhook != "" {
		log.Println("• Discord Alerts    : Enabled")
	}

	if cfg.GeminiKey == "" {
		log.Println("⚠️  WARNING: GEMINI_API_KEY is not set. LLM analysis will be skipped unless configured in environment.")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	es := esclient.New(cfg.ElasticURL, cfg.ElasticIndex, cfg.ElasticUser, cfg.ElasticPass)
	notify := notifier.New(cfg.SlackWebhook, cfg.DiscordWebhook)

	var aiAnalyzer *analyzer.Analyzer
	if cfg.GeminiKey != "" {
		var err error
		aiAnalyzer, err = analyzer.New(ctx, cfg.GeminiKey, cfg.GeminiModel)
		if err != nil {
			log.Fatalf("Failed to initialize Gemini analyzer: %v", err)
		}
		log.Println("✅ Gemini AI Analyzer initialized successfully.")
	}

	// Cooldown tracker to prevent alert storms / duplicate LLM calls
	var lastIncidentTime time.Time
	cooldownDuration := 3 * time.Minute

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("👀 Agent is now watching Elasticsearch for incidents...")

	for {
		select {
		case <-sigChan:
			log.Println("Shutting down RCA Agent...")
			return

		case <-ticker.C:
			// Check if cooldown period is active
			if time.Since(lastIncidentTime) < cooldownDuration {
				continue
			}

			triggered, count, err := es.CheckErrorSpike(ctx, cfg.TimeWindow, cfg.ErrorThreshold)
			if err != nil {
				log.Printf("Error checking Elasticsearch: %v", err)
				continue
			}

			if !triggered {
				continue
			}

			now := time.Now().UTC()
			lastIncidentTime = now
			log.Printf("🚨 INCIDENT DETECTED! Error spike of %d errors found in the last %v", count, cfg.TimeWindow)

			// 1. Fetch Correlated Context Logs
			incident, err := es.FetchCorrelatedLogs(ctx, now, 25)
			if err != nil {
				log.Printf("Failed to fetch correlated logs: %v", err)
				continue
			}

			// 2. Data Sanitization / Masking
			sanitizer.SanitizeIncident(incident)

			// 3. Multi-Agent War Room Analysis (with Knowledge Docs & Token Tracker)
			if aiAnalyzer == nil {
				log.Println("ℹ️  GEMINI_API_KEY not configured. Displaying raw incident context:")
				fmt.Printf("Incident on Service: %s, Total Errors: %d\n", incident.ServiceName, incident.TotalErrors)
				continue
			}

			// Load organizational knowledge docs (ADRs, Business rules, Runbooks)
			knowledgeDocs := analyzer.LoadKnowledgeDocs("docs/knowledge")
			log.Printf("📚 Loaded %d organizational knowledge docs for Multi-Agent War Room", len(knowledgeDocs))

			log.Println("🧠 Convening Modular Multi-Agent War Room (Tech Lead, SRE, Product Lead)...")
			warRoom, err := aiAnalyzer.RunModularWarRoom(ctx, incident, knowledgeDocs)
			if err != nil {
				log.Printf("War Room analysis failed: %v", err)
				continue
			}

			// 4. Dispatch Notifications with Token Breakdown
			notify.PrintWarRoomConsole(incident, warRoom)
			if err := notify.SendWarRoomDiscord(incident, warRoom); err != nil {
				log.Printf("Failed to dispatch War Room Discord notification: %v", err)
			}
		}
	}
}

func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			v = strings.Trim(v, `"'`)
			if os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}


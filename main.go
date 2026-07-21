package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"codebuddy-router/internal/config"
	"codebuddy-router/internal/router"
	"codebuddy-router/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

func main() {
	cfg := config.Load()

	// ─── Interactive Setup (terminal only) ──────────────────────────
	if term.IsTerminal(int(os.Stdin.Fd())) {
		// Loop: setup → validate → retry on failure
		for {
			if len(cfg.APIKeys) == 0 || cfg.SetupMode {
				interactiveSetup(cfg)
			}

			if os.Getenv("SKIP_VALID") != "" || len(cfg.APIKeys) == 0 {
				break
			}

			fmt.Print("\n🔍 Validating CodeBuddy API key... ")
			ok, msg := testKey(cfg.APIKeys[0])
			if ok {
				fmt.Printf("✅ %s\n", msg)
				break
			}
			fmt.Printf("❌ %s\n", msg)
			fmt.Println("\n⚠️  Key validation failed. Press Enter to re-enter keys, or type 'q' to quit.")
			input, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			if strings.TrimSpace(input) == "q" {
				os.Exit(1)
			}
			cfg.APIKeys = nil
		}
	} else if len(cfg.APIKeys) == 0 {
		// Headless — no terminal to prompt
		fmt.Println("\n❌ No API keys configured.")
		fmt.Println("   Set API_KEYS=ck_... in .env and restart.")
		os.Exit(1)
	}

	// ─── Transport ──────────────────────────────────────────────────
	transport, err := router.DetermineTransport(cfg.Proxy)
	if err != nil {
		fmt.Printf("❌ Proxy error: %v\n", err)
		os.Exit(1)
	}

	pool := router.NewKeyPool(cfg.APIKeys)
	events := make(chan router.LogEntry, 1000)
	srv := router.NewServer(cfg, pool, transport, events)

	// ─── Start server ───────────────────────────────────────────────
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Server: %v\n", err)
			os.Exit(1)
		}
	}()

	// ─── Banner ─────────────────────────────────────────────────────
	printBanner(cfg)

	// ─── TUI or headless ────────────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	if term.IsTerminal(int(os.Stdout.Fd())) {
		p := tea.NewProgram(tui.New(pool, events, cfg), tea.WithAltScreen())
		go func() {
			<-sigCh
			p.Quit()
		}()
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		}
	} else {
		fmt.Println("📝 Headless mode")
		go func() {
			for entry := range events {
				fmt.Printf("[%s] %d %-22s %v\n",
					entry.Time.Format("15:04:05"),
					entry.StatusCode,
					entry.Model,
					entry.Latency.Truncate(time.Millisecond),
				)
			}
		}()
		<-sigCh
	}

	fmt.Println("\n👋 Shutting down...")
}

// ─── Interactive Setup ────────────────────────────────────────────

func interactiveSetup(cfg *config.Config) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  🚀 CodeBuddy CLI Gateway — Setup")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("Enter your CodeBuddy API key(s) (ck_...)")
	fmt.Println("Press Enter on an empty line when done.")

	var keys []string
	for {
		fmt.Print("\nKey: ")
		key, _ := reader.ReadString('\n')
		key = strings.TrimSpace(key)

		if key == "" {
			if len(keys) == 0 {
				fmt.Println("   ⚠️  At least one key is required!")
				continue
			}
			break
		}

		// Validate before accepting
		fmt.Print("   Checking... ")
		ok, msg := testKey(key)
		if !ok {
			fmt.Printf("❌ %s\n", msg)
			fmt.Println("   Try again or press Enter to skip this key.")
			continue
		}
		fmt.Printf("✅ %s\n", msg)
		keys = append(keys, key)
		fmt.Printf("   Key #%d saved.\n", len(keys))
	}

	// Proxy
	fmt.Println("\n🌐 Proxy (optional)")
	fmt.Println("   Format: http://host:port, socks5://user:pass@host:port")
	fmt.Print("   Press Enter to skip: ")
	proxy, _ := reader.ReadString('\n')
	proxy = strings.TrimSpace(proxy)

	// Port
	fmt.Printf("\n📡 Port [%s]: ", cfg.Port)
	portInput, _ := reader.ReadString('\n')
	portInput = strings.TrimSpace(portInput)
	if portInput != "" {
		cfg.Port = portInput
	}

	cfg.APIKeys = keys
	cfg.Proxy = proxy
	config.SaveToEnv(cfg)

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  ✅ Setup complete! (.env saved)")
	fmt.Printf("  🔑 Router Key: %s\n", cfg.RouterKey)
	fmt.Println("  Use this key to authenticate CLI tools.")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
}

// ─── Key Validation ──────────────────────────────────────────────

func testKey(apiKey string) (bool, string) {
	client := &http.Client{Timeout: 15 * time.Second}

	payload := `{"model":"claude-haiku-4.5","messages":[{"role":"system","content":"hi"},{"role":"user","content":"hi"}],"stream":true,"max_tokens":5}`
	req, err := http.NewRequest("POST", "https://www.codebuddy.ai/v2/chat/completions", strings.NewReader(payload))
	if err != nil {
		return false, fmt.Sprintf("request error: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "CLI/2.105.2 CodeBuddy/2.105.2")
	req.Header.Set("X-Product", "SaaS")
	req.Header.Set("X-App", "cli")
	req.Header.Set("X-IDE-Type", "CLI")
	req.Header.Set("X-IDE-Name", "CLI")
	req.Header.Set("X-IDE-Version", "2.105.2")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("X-Domain", "www.codebuddy.ai")

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Sprintf("connection error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, "key valid, CodeBuddy reachable"
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return false, "invalid key (401/403)"
	}
	if resp.StatusCode == 429 {
		return false, "rate limited (429) — try again later"
	}
	return false, fmt.Sprintf("unexpected HTTP %d", resp.StatusCode)
}

// ─── Banner ──────────────────────────────────────────────────────

func printBanner(cfg *config.Config) {
	w := 54
	r := strings.Repeat

	fmt.Println()
	fmt.Println(r("━", w))
	fmt.Println("  🚀  CodeBuddy CLI Gateway v1.0")
	fmt.Println(r("━", w))
	fmt.Printf("  📡  Base URL : http://localhost:%s/v1\n", cfg.Port)
	fmt.Printf("  📡  Chat     : http://localhost:%s/v1/chat/completions\n", cfg.Port)
	fmt.Printf("  📡  Models   : http://localhost:%s/v1/models\n", cfg.Port)
	fmt.Printf("  📡  Health   : http://localhost:%s/health\n", cfg.Port)
	fmt.Println(r("─", w))
	fmt.Printf("  🔑  Router Key : %s\n", cfg.RouterKey)
	fmt.Printf("  🔑  API Keys   : %d loaded\n", len(cfg.APIKeys))
	fmt.Printf("  🧠  Models     : %d available\n", len(router.Models))
	if cfg.Proxy != "" {
		fmt.Printf("  🌐  Proxy      : %s\n", cfg.Proxy)
	} else {
		fmt.Printf("  🌐  Proxy      : direct\n")
	}
	fmt.Println(r("━", w))
	fmt.Println("  💡 Connect your CLI:")
	fmt.Printf("     URL:     http://localhost:%s/v1\n", cfg.Port)
	fmt.Printf("     API Key: %s\n", cfg.RouterKey)
	fmt.Println(r("━", w))
	fmt.Println()
}

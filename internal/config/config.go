package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port      string
	APIKeys   []string
	RouterKey string // deterministic per-machine
	Proxy     string
	SetupMode bool
}

func Load() *Config {
	godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "2069"
	}

	keysRaw := os.Getenv("API_KEYS")
	var keys []string
	for _, k := range strings.Split(keysRaw, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			keys = append(keys, k)
		}
	}

	setupMode := os.Getenv("SETUP") == "1" || os.Getenv("SETUP") == "true"

	return &Config{
		Port:      port,
		APIKeys:   keys,
		RouterKey: MachineRouterKey(),
		Proxy:     os.Getenv("PROXY"),
		SetupMode: setupMode,
	}
}

// MachineRouterKey generates a deterministic router key based on
// hostname + machine-id. Same machine always gets the same key.
// Different machines get different keys.
func MachineRouterKey() string {
	hostname, _ := os.Hostname()

	machineID := ""
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		machineID = strings.TrimSpace(string(data))
	} else if data, err := os.ReadFile("/var/lib/dbus/machine-id"); err == nil {
		machineID = strings.TrimSpace(string(data))
	}

	// Fallback: generate stable ID from /proc/sys/kernel/random/boot_id (Linux)
	if machineID == "" {
		if data, err := os.ReadFile("/proc/sys/kernel/random/boot_id"); err == nil {
			machineID = strings.TrimSpace(string(data))
		}
	}

	// Final fallback: use hostname + time-based salt (stable per boot)
	if machineID == "" {
		machineID = "fallback-" + hostname
	}

	salt := "codebuddy-gateway-v1"
	raw := fmt.Sprintf("%s|%s|%s", hostname, machineID, salt)
	hash := sha256.Sum256([]byte(raw))
	return "rtr_" + hex.EncodeToString(hash[:24])
}

func GenerateKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func SaveToEnv(cfg *Config) error {
	lines := []string{
		fmt.Sprintf("PORT=%s", cfg.Port),
		fmt.Sprintf("API_KEYS=%s", strings.Join(cfg.APIKeys, ",")),
		fmt.Sprintf("PROXY=%s", cfg.Proxy),
	}
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(".env", []byte(content), 0600)
}

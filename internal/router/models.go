package router

type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MaxInput    int    `json:"max_input_tokens"`
	MaxOutput   int    `json:"max_output_tokens"`
	Multimodal  bool   `json:"multimodal"`
}

var Models = map[string]Model{
	// ─── Claude ─────────────────────────────────────────────────
	"claude-opus-4.7-1m":     {"claude-opus-4.7-1m", "Claude Opus 4.7 1M", 1000000, 24000, true},
	"claude-opus-4.6":        {"claude-opus-4.6", "Claude Opus 4.6", 176000, 24000, true},
	"claude-sonnet-4.6":      {"claude-sonnet-4.6", "Claude Sonnet 4.6", 176000, 24000, true},
	"claude-haiku-4.5":       {"claude-haiku-4.5", "Claude Haiku 4.5", 176000, 24000, true},

	// ─── GPT ────────────────────────────────────────────────────
	"gpt-5.6-luna":           {"gpt-5.6-luna", "GPT-5.6 Luna", 1000000, 128000, true},
	"gpt-5.6-sol":            {"gpt-5.6-sol", "GPT-5.6 Sol", 272000, 128000, true},
	"gpt-5.6-terra":          {"gpt-5.6-terra", "GPT-5.6 Terra", 1000000, 128000, true},
	"gpt-5.5":                {"gpt-5.5", "GPT-5.5", 1000000, 72000, true},
	"gpt-5.4":                {"gpt-5.4", "GPT-5.4", 272000, 128000, true},
	"gpt-5.3-codex":          {"gpt-5.3-codex", "GPT-5.3 Codex", 272000, 128000, true},
	"gpt-5.2-codex":          {"gpt-5.2-codex", "GPT-5.2 Codex", 272000, 128000, true},
	"gpt-5.2":                {"gpt-5.2", "GPT-5.2", 272000, 128000, true},
	"gpt-5.1-codex-max":      {"gpt-5.1-codex-max", "GPT-5.1 Codex Max", 272000, 128000, true},
	"gpt-5.1-codex-mini":     {"gpt-5.1-codex-mini", "GPT-5.1 Codex Mini", 272000, 128000, true},
	"gpt-5.1-codex":          {"gpt-5.1-codex", "GPT-5.1 Codex", 272000, 128000, true},
	"gpt-5.1":                {"gpt-5.1", "GPT-5.1", 272000, 128000, true},
	"gpt-5-codex":            {"gpt-5-codex", "GPT-5 Codex", 272000, 64000, true},
	"gpt-4.1":                {"gpt-4.1", "GPT-4.1", 1047576, 32768, true},

	// ─── Gemini ─────────────────────────────────────────────────
	"gemini-3.5-flash":       {"gemini-3.5-flash", "Gemini 3.5 Flash", 1000000, 65536, true},
	"gemini-3.1-pro":         {"gemini-3.1-pro", "Gemini 3.1 Pro", 400000, 64000, true},
	"gemini-3.1-flash-lite":  {"gemini-3.1-flash-lite", "Gemini 3.1 Flash Lite", 200000, 65536, true},
	"gemini-3.0-pro":         {"gemini-3.0-pro", "Gemini 3.0 Pro", 400000, 64000, true},
	"gemini-3.0-flash":       {"gemini-3.0-flash", "Gemini 3.0 Flash", 400000, 64000, true},
	"gemini-2.5-pro":         {"gemini-2.5-pro", "Gemini 2.5 Pro", 400000, 64000, true},
	"gemini-2.5-flash":       {"gemini-2.5-flash", "Gemini 2.5 Flash", 400000, 64000, true},

	// ─── GLM ────────────────────────────────────────────────────
	"glm-5.2":       {"glm-5.2", "GLM 5.2", 200000, 48000, false},
	"glm-5.1":       {"glm-5.1", "GLM 5.1", 200000, 48000, false},
	"glm-5v-turbo":  {"glm-5v-turbo", "GLM 5V Turbo", 200000, 48000, true},
	"glm-5.0":       {"glm-5.0", "GLM 5.0", 200000, 48000, false},
	"glm-4.6":       {"glm-4.6", "GLM 4.6", 200000, 48000, false},

	// ─── Kimi ───────────────────────────────────────────────────
	"kimi-k2.6": {"kimi-k2.6", "Kimi K2.6", 128000, 8192, false},
	"kimi-k2.5": {"kimi-k2.5", "Kimi K2.5", 128000, 8192, false},
	"kimi-k3":   {"kimi-k3", "Kimi K3", 128000, 8192, false},

	// ─── DeepSeek ───────────────────────────────────────────────
	"deepseek-v3.2": {"deepseek-v3.2", "DeepSeek V3.2", 128000, 8192, false},

	// ─── MiniMax ────────────────────────────────────────────────
	"minimax-m3": {"minimax-m3", "MiniMax M3", 128000, 8192, false},

	// ─── Reasoning ──────────────────────────────────────────────
	"o4-mini": {"o4-mini", "o4-mini", 128000, 32000, true},
}

func IsValidModel(id string) bool {
	_, ok := Models[id]
	return ok
}

func StripPrefix(model string) string {
	if len(model) > 3 && model[:3] == "cb/" {
		return model[3:]
	}
	return model
}

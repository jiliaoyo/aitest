package config

import (
	"bufio"
	"fmt"
	"math"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	AIAPIStyleChatCompletions = "chat_completions"
	AIAPIStyleResponses       = "responses"
)

// Config 汇总启动配置；在进程启动时一次性读取并校验。
type Config struct {
	AppEnv            string // dev | prod
	HTTPAddr          string
	PublicOrigin      string
	TrustedProxyCIDRs []netip.Prefix
	DatabaseURL       string
	SessionTTL        time.Duration
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	UploadDir         string
	UploadMaxBytes    int64
	RunWorker         bool
	WorkerID          string
	WorkerConcurrency int

	AIBaseURL              string
	AIAPIKey               string
	AIModel                string
	AIAPIStyle             string
	AITimeout              time.Duration
	AIGenerationDailyLimit int
	AIGenerationCallBudget int
	// 费用按美元/百万 token 配置，并在每次调用审计时固化，避免模型价格变化污染历史统计。
	AIInputPricePerMillion  float64
	AIOutputPricePerMillion float64
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}
	appEnv := getenv("APP_ENV", "dev")
	c := Config{
		AppEnv:       appEnv,
		HTTPAddr:     getenv("HTTP_ADDR", ":8080"),
		PublicOrigin: getenv("PUBLIC_ORIGIN", "http://localhost:5173"),
		DatabaseURL:  getenv("DATABASE_URL", "postgres://postgres@localhost:5432/ai_shuati_dev?sslmode=disable"),
		UploadDir:    getenv("UPLOAD_DIR", "./data/uploads"),
		RunWorker:    getenv("RUN_WORKER", strconv.FormatBool(appEnv == "dev")) == "true",
		WorkerID:     getenv("WORKER_ID", "worker-1"),
		AIBaseURL:    os.Getenv("AI_BASE_URL"),
		AIAPIKey:     os.Getenv("AI_API_KEY"),
		AIModel:      getenv("AI_MODEL", ""),
		AIAPIStyle:   getenv("AI_API_STYLE", AIAPIStyleChatCompletions),
	}
	trustedProxyCIDRs, err := parseTrustedProxyCIDRs(os.Getenv("TRUSTED_PROXY_CIDRS"))
	if err != nil {
		return c, err
	}
	c.TrustedProxyCIDRs = trustedProxyCIDRs
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("缺少必填环境变量 DATABASE_URL")
	}

	ttl, err := time.ParseDuration(getenv("SESSION_TTL", "720h"))
	if err != nil {
		return c, fmt.Errorf("SESSION_TTL 无效: %w", err)
	}
	c.SessionTTL = ttl
	accessTTL, err := time.ParseDuration(getenv("ACCESS_TOKEN_TTL", "15m"))
	if err != nil {
		return c, fmt.Errorf("ACCESS_TOKEN_TTL 无效: %w", err)
	}
	if accessTTL <= 0 {
		return c, fmt.Errorf("ACCESS_TOKEN_TTL 必须大于 0")
	}
	c.AccessTokenTTL = accessTTL
	refreshTTL, err := time.ParseDuration(getenv("REFRESH_TOKEN_TTL", "720h"))
	if err != nil {
		return c, fmt.Errorf("REFRESH_TOKEN_TTL 无效: %w", err)
	}
	if refreshTTL <= 0 {
		return c, fmt.Errorf("REFRESH_TOKEN_TTL 必须大于 0")
	}
	c.RefreshTokenTTL = refreshTTL

	maxBytes, err := strconv.ParseInt(getenv("UPLOAD_MAX_BYTES", "10485760"), 10, 64)
	if err != nil {
		return c, fmt.Errorf("UPLOAD_MAX_BYTES 无效: %w", err)
	}
	c.UploadMaxBytes = maxBytes

	concurrency, err := strconv.Atoi(getenv("WORKER_CONCURRENCY", "2"))
	if err != nil || concurrency < 1 {
		return c, fmt.Errorf("WORKER_CONCURRENCY 无效")
	}
	c.WorkerConcurrency = concurrency

	aiTimeout, err := time.ParseDuration(getenv("AI_TIMEOUT", "300s"))
	if err != nil {
		return c, fmt.Errorf("AI_TIMEOUT 无效: %w", err)
	}
	c.AITimeout = aiTimeout
	dailyGenerationLimit, err := strconv.Atoi(getenv("AI_GENERATION_DAILY_LIMIT", "10"))
	if err != nil || dailyGenerationLimit < 0 {
		return c, fmt.Errorf("AI_GENERATION_DAILY_LIMIT 无效: 请输入非负整数")
	}
	c.AIGenerationDailyLimit = dailyGenerationLimit
	generationCallBudget, err := strconv.Atoi(getenv("AI_GENERATION_CALL_BUDGET", "6"))
	if err != nil || generationCallBudget < 1 {
		return c, fmt.Errorf("AI_GENERATION_CALL_BUDGET 无效: 请输入正整数")
	}
	c.AIGenerationCallBudget = generationCallBudget
	inputPrice, err := parsePrice("AI_INPUT_PRICE_PER_MILLION")
	if err != nil {
		return c, err
	}
	outputPrice, err := parsePrice("AI_OUTPUT_PRICE_PER_MILLION")
	if err != nil {
		return c, err
	}
	c.AIInputPricePerMillion = inputPrice
	c.AIOutputPricePerMillion = outputPrice
	if c.AIAPIStyle != AIAPIStyleChatCompletions && c.AIAPIStyle != AIAPIStyleResponses {
		return c, fmt.Errorf("AI_API_STYLE 必须是 %s 或 %s", AIAPIStyleChatCompletions, AIAPIStyleResponses)
	}

	if c.AppEnv != "dev" && c.AppEnv != "prod" {
		return c, fmt.Errorf("APP_ENV 必须是 dev 或 prod")
	}
	if c.RunWorker && c.WorkerID == "" {
		return c, fmt.Errorf("RUN_WORKER=true 时必须提供 WORKER_ID")
	}
	return c, nil
}

func (c Config) SecureCookie() bool { return c.AppEnv == "prod" }

func (c Config) AIConfigured() bool { return c.AIBaseURL != "" && c.AIAPIKey != "" && c.AIModel != "" }

func parseTrustedProxyCIDRs(raw string) ([]netip.Prefix, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	prefixes := make([]netip.Prefix, 0)
	for _, value := range strings.Split(raw, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXY_CIDRS 无效: %w", err)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parsePrice(key string) (float64, error) {
	value, err := strconv.ParseFloat(getenv(key, "0"), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0, fmt.Errorf("%s 无效: 请输入非负数字", key)
	}
	return value, nil
}

func loadDotEnv() error {
	path := ".env"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = "backend/.env"
	}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"')) {
			value = value[1 : len(value)-1]
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("设置 %s 失败: %w", key, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	return nil
}

package config

import (
	"bufio"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 汇总启动配置；在进程启动时一次性读取并校验。
type Config struct {
	AppEnv            string // dev | prod
	HTTPAddr          string
	PublicOrigin      string
	TrustedProxyCIDRs []netip.Prefix
	DatabaseURL       string
	SessionTTL        time.Duration
	UploadDir         string
	UploadMaxBytes    int64
	RunWorker         bool
	WorkerID          string
	WorkerConcurrency int

	AIBaseURL string
	AIAPIKey  string
	AIModel   string
	AITimeout time.Duration
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

	aiTimeout, err := time.ParseDuration(getenv("AI_TIMEOUT", "60s"))
	if err != nil {
		return c, fmt.Errorf("AI_TIMEOUT 无效: %w", err)
	}
	c.AITimeout = aiTimeout

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

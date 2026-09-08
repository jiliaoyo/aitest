package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/aishuati/backend/internal/ai"
)

func main() {
	limit := flag.Int("limit", 0, "最多回放的固定样本数；0 表示全部")
	maxCalls := flag.Int("max-calls", 0, "最多回放调用数；0 表示不额外限制")
	flag.Parse()
	report, err := ai.RunOfflineEvalWithLimit(*limit, *maxCalls)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

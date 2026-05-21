package remote

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type RemoteRedisConfig struct {
	Addr     string `json:"addr"`
	Password string `json:"password"`
}

func FetchRedisConfig(url string) (*RemoteRedisConfig, error) {
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github returned non-OK status: %d", resp.StatusCode)
	}

	var cfg RemoteRedisConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

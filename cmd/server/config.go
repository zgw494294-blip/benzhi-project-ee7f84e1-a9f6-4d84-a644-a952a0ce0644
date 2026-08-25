package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	addr             string
	database         string
	selfcheck        bool
	selfcheckTimeout time.Duration
}

func parseConfig() (config, error) {
	defaultAddr := "127.0.0.1:19081"
	if raw := strings.TrimSpace(os.Getenv("PORT")); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil || port < 1 || port > 65535 {
			return config{}, fmt.Errorf("PORT 必须是 1 至 65535 的端口号")
		}
		defaultAddr = net.JoinHostPort("127.0.0.1", raw)
	}
	var cfg config
	flag.StringVar(&cfg.addr, "addr", defaultAddr, "HTTP 监听地址")
	flag.StringVar(&cfg.database, "db", "calibration.db", "SQLite 数据库路径或 DSN")
	flag.BoolVar(&cfg.selfcheck, "selfcheck", false, "运行真实 HTTP 全流程自检后退出")
	flag.DurationVar(&cfg.selfcheckTimeout, "selfcheck-timeout", 8*time.Second, "selfcheck 总超时时间")
	flag.Parse()
	if err := validateAddress(cfg.addr); err != nil {
		return config{}, err
	}
	if cfg.selfcheckTimeout <= 0 {
		return config{}, fmt.Errorf("selfcheck-timeout 必须大于零")
	}
	if cfg.selfcheck && cfg.database == "calibration.db" {
		cfg.database = "file:selfcheck?mode=memory&cache=shared"
	}
	return cfg, nil
}

func validateAddress(addr string) error {
	host, rawPort, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("addr 必须包含主机和端口: %w", err)
	}
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("addr 不得省略主机")
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("addr 端口无效")
	}
	return nil
}

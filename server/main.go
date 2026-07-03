package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// Config 后端启动配置。用户可编辑 config.json 自行修改。
type Config struct {
	Addr      string `json:"addr"`      // 监听地址，默认 127.0.0.1:9898
	StaticDir string `json:"staticDir"` // 前端静态资源目录，默认 ./web
}

func defaultConfig() Config {
	return Config{
		Addr:      "127.0.0.1:9898",
		StaticDir: "./web",
	}
}

// loadConfig 读取配置文件；文件不存在时使用默认值。
// 环境变量 ADDR / STATIC_DIR 可覆盖配置文件中的值。
func loadConfig(path string) Config {
	cfg := defaultConfig()

	b, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(b, &cfg); err != nil {
			log.Printf("配置文件解析失败，使用默认值: %v", err)
			cfg = defaultConfig()
		}
	} else {
		log.Printf("未找到配置文件 %s，使用默认配置", path)
	}

	// 环境变量覆盖（保留向后兼容）
	if v := os.Getenv("ADDR"); v != "" {
		cfg.Addr = v
	}
	if v := os.Getenv("STATIC_DIR"); v != "" {
		cfg.StaticDir = v
	}
	return cfg
}

func main() {
	configPath := flag.String("c", "./config.json", "配置文件路径")
	flag.Parse()

	cfg := loadConfig(*configPath)

	hub := newHub()
	go hub.run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWS(hub, w, r)
	})

	abs, _ := filepath.Abs(cfg.StaticDir)
	log.Printf("静态资源目录: %s", abs)

	fs := http.FileServer(http.Dir(cfg.StaticDir))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// SPA 回退：不存在的路径返回 index.html
		p := filepath.Join(cfg.StaticDir, filepath.Clean(r.URL.Path))
		if _, err := os.Stat(p); os.IsNotExist(err) {
			http.ServeFile(w, r, filepath.Join(cfg.StaticDir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})

	log.Printf("家庭棋牌室服务启动: http://%s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

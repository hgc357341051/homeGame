package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	hub := newHub()
	go hub.run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWS(hub, w, r)
	})

	// 静态资源：前端构建产物（默认 ./web，可通过 STATIC_DIR 覆盖）
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "./web"
	}
	abs, _ := filepath.Abs(staticDir)
	log.Printf("静态资源目录: %s", abs)

	fs := http.FileServer(http.Dir(staticDir))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// SPA 回退：不存在的路径返回 index.html
		p := filepath.Join(staticDir, filepath.Clean(r.URL.Path))
		if _, err := os.Stat(p); os.IsNotExist(err) {
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("家庭棋牌室服务启动: http://localhost%s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

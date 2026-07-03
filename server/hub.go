package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 30 * time.Second
	maxMessageSize = 8192
)

// Client 代表一个 WebSocket 连接对应的玩家
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	playerID string
	name     string
	room     *Room
	mu       sync.Mutex
}

// Hub 管理所有连接
type Hub struct {
	clients    map[string]*Client
	rm         *RoomManager
	register   chan *Client
	unregister chan *Client
	mu         sync.Mutex
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		rm:         newRoomManager(),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
	}
}

func (h *Hub) run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c.playerID] = c
			h.mu.Unlock()
		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c.playerID]; ok {
				delete(h.clients, c.playerID)
			}
			h.mu.Unlock()
			if c.room != nil {
				c.room.handleDisconnect(c)
			}
			close(c.send)
		}
	}
}

func (c *Client) sendMsg(m Message) {
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	select {
	case c.send <- b:
	default:
		// 缓冲满，丢弃
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			c.sendMsg(Message{Type: "error", Data: ActionData{"msg": "消息格式错误"}})
			continue
		}
		data, _ := msg.Data.(map[string]interface{})
		if data == nil {
			data = ActionData{}
		}
		c.hub.rm.handleAction(c, msg.Type, data)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWS 处理 WebSocket 升级
func serveWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade err:", err)
		return
	}
	client := &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 256),
		playerID: hub.genUniqueID(),
	}
	hub.register <- client
	go client.writePump()
	client.readPump()
}

func (c *Client) emitError(msg string) {
	c.sendMsg(Message{Type: "error", Data: ActionData{"msg": msg}})
}

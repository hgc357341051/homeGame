package main

// 统一消息结构：{type, data}
type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

// 客户端动作统一解析载体
type ActionData map[string]interface{}

// Event 为后端产生的事件。Target==-1 广播全员，否则发给指定 seat。
type Event struct {
	Type   string      // 消息 type
	Data   interface{} // 消息 data
	Target int         // -1=广播，>=0=仅发给该 seat
}

// 公开出牌信息
type PlayInfo struct {
	Player string `json:"player"` // playerID
	Seat   int    `json:"seat"`
	Cards  []Card `json:"cards"`
	Pass   bool   `json:"pass,omitempty"`
}

// 房间状态中的座位（公开视角，绝不含手牌）
type SeatView struct {
	Seat         int    `json:"seat"`
	PlayerID     string `json:"playerId"`
	Name         string `json:"name"`
	Avatar       string `json:"avatar"`
	Chips        int    `json:"chips"`
	Ready        bool   `json:"ready"`
	CardCount    int    `json:"cardCount"`
	IsLandlord   bool   `json:"isLandlord,omitempty"`
	IsDealer     bool   `json:"isDealer,omitempty"`
	IsFolded     bool   `json:"isFolded,omitempty"`
	IsLooked     bool   `json:"isLooked,omitempty"`
	IsOwner      bool   `json:"isOwner,omitempty"`
	Online       bool   `json:"online"`
	Offline      bool   `json:"offline,omitempty"`              // 掉线保留座位中（可被夺回/超时释放）
	OfflineLeft  int    `json:"offlineLeft,omitempty"`          // 掉线座位剩余秒数（超时释放）
	CurrentBet   int    `json:"currentBet,omitempty"`
	HasNiu       bool   `json:"hasNiu,omitempty"`
	NiuValue     int    `json:"niuValue,omitempty"`
	NiuName      string `json:"niuName,omitempty"`
	SettledDelta int    `json:"settledDelta,omitempty"`
	LookedIndices []bool `json:"lookedIndices,omitempty"` // 蒙牌模式：已查看的牌索引
	IsRevealed   bool   `json:"isRevealed"`               // 蒙牌模式：是否已开牌
	RevealedCards []Card `json:"revealedCards,omitempty"`  // 开牌后该座位全部牌（所有人可见）
}

// 广播的房间状态（不含任何手牌；个人手牌由 deal 单独下发）
type RoomStateView struct {
	Code        string         `json:"code"`
	Game        string         `json:"game"`
	HostID      string         `json:"hostId"`
	Phase       string         `json:"phase"` // waiting/playing/settled
	Seats       []SeatView     `json:"seats"`
	MySeat      int            `json:"mySeat"` // -1 表示旁观
	PublicArea  PublicAreaView `json:"publicArea"`
	MinPlayers  int            `json:"minPlayers"`
	MaxPlayers  int            `json:"maxPlayers"`
	GameLabel   string         `json:"gameLabel"`
	BlindMode   bool           `json:"blindMode,omitempty"`
}

type PublicAreaView struct {
	LastPlay      *PlayInfo `json:"lastPlay,omitempty"`
	LastPlays     []PlayInfo `json:"lastPlays,omitempty"` // 各 seat 最近出的牌(牛牛同时展示)
	BottomCards   []Card    `json:"bottomCards,omitempty"`
	Pot           int       `json:"pot,omitempty"`
	CurrentSeat   int       `json:"currentSeat,omitempty"`
	BaseBet       int       `json:"baseBet,omitempty"`
	CurrentBet    int       `json:"currentBet,omitempty"`
	LookedCount   int       `json:"lookedCount,omitempty"`
	ActiveCount   int       `json:"activeCount,omitempty"`
	Phase         string    `json:"phase,omitempty"` // 子阶段：callLandlord/betting/compare/settle 等
	Message       string    `json:"message,omitempty"`
	DealerSeat    int       `json:"dealerSeat,omitempty"`
	WinnerSeat    int       `json:"winnerSeat,omitempty"`
}

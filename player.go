package main

import (
	"fmt"
)

type UserStats struct {
	TotalScore int64 `json:"total_score"`
	RankedScore int64 `json:"ranked_score"`
	Performance int32 `json:"pp"`
	Playcount int32 `json:"playcount"`
	Rank int32 `json:"rank"`
	Accuracy float32 `json:"accuracy"`
	MaxCombo int16 `json:"max_combo"`
}

type Player struct {
	ID int32
	Username, SafeUsername, Password, Token string
	Stats []UserStats

	Privileges, Country, Action, Gamemode byte
	ActionText, ActionHash string
	ActionMods, ActionBeatmap int32

	Channels []*Channel
	Spectators []*Player
	Spectating *Player
	Match *MultiplayerLobby

	Ping int64
	Bot, Relax bool
	FakeName string

	Queue Packetstream
}

func (p *Player) IsAdmin() bool {
	return (p.Privileges & 16) != 0
}

func (p *Player) SetRelax(v bool, x bool) {
	if p.Gamemode == 3 {
		p.SetRelax(false, false)
	}
	if p.Relax == v || p.Action == 12 {
		return
	}
	p.Relax = v

	if x {
		Broadcast(11, StatsPacketInterface(p)...)
	}
}

var modes = [...]string{ "std", "taiko", "ctb", "mania" }
var squery = [...]string {
	"SELECT ranked_score_std, total_score_std, playcount_std, avg_accuracy_std/100, pp_std",
	"SELECT ranked_score_taiko, total_score_taiko, playcount_taiko, avg_accuracy_taiko/100, pp_taiko",
	"SELECT ranked_score_ctb, total_score_ctb, playcount_ctb, avg_accuracy_ctb/100, pp_ctb",
	"SELECT ranked_score_mania, total_score_mania, playcount_mania, avg_accuracy_mania/100, pp_mania",
}
func (p *Player) GetStatsFixed(mode byte) {
	table := "users"
	dbmode := mode
	if mode > 3 {
		table = "rx"
		dbmode -= 4
	}
	err := db.QueryRow(squery[dbmode]+" FROM "+table+"_stats WHERE id = ? LIMIT 1", p.ID).Scan(
		&p.Stats[mode].RankedScore, &p.Stats[mode].TotalScore, &p.Stats[mode].Playcount,
		&p.Stats[mode].Accuracy, &p.Stats[mode].Performance)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	_ = db.QueryRow("SELECT COUNT(id) FROM " + table + "_stats WHERE pp_"+modes[dbmode]+" >= ?", p.Stats[mode].Performance).Scan(&p.Stats[mode].Rank)
	_ = db.QueryRow("SELECT combo FROM scores WHERE userid = ? AND status = 3 AND mode = ? ORDER BY combo DESC LIMIT 1", p.ID, dbmode).Scan(&p.Stats[mode].MaxCombo)
}

func (p *Player) GetStats() {
	for i := 0; i < 7; i++ {
		p.GetStatsFixed(byte(i))
	}
}

func (p *Player) GetFriends() []int32 {
	f := make([]int32, 0)
	rows, err := db.Query("SELECT user2 FROM users_relationships WHERE user1 = ?", p.ID)
	if err != nil {
		return f
	}
	defer rows.Close()
	for rows.Next() {
		var id int32
		err = rows.Scan(&id)
		f = append(f, id)
	}
	return f
}

func FindPlayer(token string) *Player {
	for i := 0; i < len(Players); i++ {
		if Players[i].Token == token {
			return Players[i]
		}
	}
	return nil
}

func FindPlayerById(id int32) *Player {
	for i := 0; i < len(Players); i++ {
		if Players[i].ID == id {
			return Players[i]
		}
	}
	return nil
}

func FindPlayerByUsername(name string) *Player {
	for i := 0; i < len(Players); i++ {
		if Players[i].Username == name {
			return Players[i]
		}
	}
	return nil
}

func FindPlayerByUsernameSafe(name string) *Player {
	for i := 0; i < len(Players); i++ {
		if Players[i].SafeUsername == name {
			return Players[i]
		}
	}
	return nil
}

func RemovePlayer(p *Player) {
	fmt.Println(p.Username + " logged out.")

	for i := 0; i < len(Players); i++ {
		if Players[i] == p {
			Players[i] = Players[len(Players)-1]
			Players[len(Players)-1] = nil
			Players = Players[:len(Players)-1]
		}
		if len(Players) > i {
			Players[i].Queue.WritePacket(12, p.ID, int32(0))
		}
	}

	for i := 0; i < len(p.Channels); i++ {
		p.Channels[i].PlayerLeave(p)
	}

	if p.Spectating != nil {
		StopSpectatingEvent(p)
	}
	if p.Match != nil {
		MatchLeaveEvent(p)
	}
}

func Broadcast(id int16, v ... interface{}) {
	for i := 0; i < len(Players); i++ {
		if !Players[i].Bot {
			Players[i].Queue.WritePacket(id, v...)
		}
	}
}
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unsafe"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

// Events (Suffix "Event", Multiplayer Events Prefix "Match")
func ChannelJoinEvent(p *Player, Data []byte) {
	s := ReadStringNPos(Data)
	if s == "" || s[0] != '#' {
		return
	}
	if c := FindChannel(s); c.PlayerJoin(p) {
		p.Queue.WritePacket(64, s)
		fmt.Printf("%s joined %s[%d]\n", p.Username, s, c.Users())
		for i := 0; i < len(Players); i++ {
			if Players[i].Bot || (c.AdminRead && !p.IsAdmin()) {
				continue
			}
			Players[i].Queue.WritePacket(65, c.Name, c.Topic, c.Users())
		}
		return
	}
	p.Queue.WritePacket(66, s)
}

func ChannelLeaveEvent(p *Player, Data []byte) {
	s := ReadStringNPos(Data)
	if s == "" || s[0] != '#' {
		return
	}
	if s == "#highlight" || s == "#userlog" {
		return
	}
	c := FindChannel(s)
	if c == nil {
		return
	}
	c.PlayerLeave(p)
	for i := 0; i < len(Players); i++ {
		if Players[i].Bot || (c.AdminRead && !p.IsAdmin()) {
			continue
		}
		Players[i].Queue.WritePacket(65, c.Name, c.Topic, c.Users())
	}
}

func StatusUpdateEvent(p *Player, Data []byte) {
	s := Stream{Data:Data, Pos: 1}
	p.Action = Data[0]
	p.ActionText = s.ReadString()
	p.ActionHash = s.ReadString()
	p.ActionMods = s.ReadInt32()
	p.Gamemode = s.Data[s.Pos]
	p.SetRelax((p.ActionMods & 128) != 0, false)
	s.Pos++
	p.ActionBeatmap = s.ReadInt32()
	Broadcast(11, StatsPacketInterface(p)...) // HOW
}

func MultiStatsUpdateRequestEvent(p *Player, data []byte) {
	if len(data) < 2 {
		return
	}
	c := *(*int16)(unsafe.Pointer(&data[0]))
	if len(data) < (2 + int(c) * 4) {
		return
	}
	for i := int16(0); i < c; i++ {
		t := FindPlayerById(*(*int32)(unsafe.Pointer(&data[2 + (i * 4)])))
		if t == nil {
			continue
		}
		StatsPacket(&p.Queue, t)
	}
}

func StatsUpdateRequestEvent(p *Player) {
	gm := p.Gamemode
	if p.Relax {
		gm += 4
	}
	if gm > 6 {
		gm = 0
	}
	p.GetStatsFixed(gm)
	StatsPacket(&p.Queue, p)
}

// TODO: Replace Multiplayer chat with actual channels.
// TODO: when above done, make commands return the response as string
func IrcMessageEvent(p *Player, data []byte) {
	s := Stream{Data:data}
	if data[0] == 0 {
		s.Pos += 2
	} else {
		s.ReadString()
	}

	content := s.ReadString()
	target := s.ReadString()

	if target[0] != '#' {
		return
	}

	// TODO: handle commands
	if strings.HasPrefix(content, cmdPrefix) {
		args := strings.Split(content[len(cmdPrefix):], " ")
		command := args[0]
		args = args[1:]
		cmd := FindCommand(command)
		if cmd != nil && (p.Privileges & byte(cmd.Elevation)) != 0  {
			cmd.Exec(p, target, args)
		}
	}

	if target == "#spectator" {
		if p.Spectating == nil && len(p.Spectators) != 0 {
			for i := 0; i < len(p.Spectators); i++ {
				p.Spectators[i].Queue.WritePacket(7, p.Username, content, "#spectator", p.ID)
			}
		} else if p.Spectating != nil {
			for i := 0; i < len(p.Spectating.Spectators); i++ {
				if p.Spectating.Spectators[i] != p {
					p.Spectating.Spectators[i].Queue.WritePacket(7, p.Username, content, "#spectator", p.ID)
				}
			}
		}
		return
	}

	if target == "#multiplayer" {
		if p.Match == nil {
			return
		}

		for i := 0; i < len(p.Match.Players); i++ {
			if p.Match.Players[i] != p {
				p.Match.Players[i].Queue.WritePacket(7, p.Username, content, "#multiplayer", p.ID)
			}
		}
		return
	}

	c := FindChannel(target)
	if c != nil {
		c.Message(p, content)
	}
}

func StartSpectatingEvent(p *Player, data []byte) {
	if len(data) != 4 {
		return
	}

	if p.Spectating != nil {
		StopSpectatingEvent(p)
	}

	t := FindPlayerById(*(*int32)(unsafe.Pointer(&data[0])))
	if t == nil || t.Bot {
		return
	}

	p.Spectating = t
	t.Spectators = append(t.Spectators, p)

	t.Queue.WritePacket(13, p.ID)
	for i := 0; i < len(t.Spectators); i++ {
		t.Spectators[i].Queue.WritePacket(42, p.ID)
	}
}

func StopSpectatingEvent(p *Player) {
	if p.Spectating == nil {
		return
	}

	for i := 0; i < len(p.Spectating.Spectators); i++ {
		if p.Spectating.Spectators[i] == p {
			p.Spectating.Spectators[i] = p.Spectating.Spectators[len(p.Spectating.Spectators)-1]
			p.Spectating.Spectators[len(p.Spectating.Spectators)-1] = nil
			p.Spectating.Spectators = p.Spectating.Spectators[:len(p.Spectating.Spectators)-1]
		}
	}

	if len(p.Spectating.Spectators) == 0 {
		p.Spectating.Queue.WritePacket(66, "#spectator")
	}
	p.Spectating.Queue.WritePacket(14, p.ID)
	for i := 0; i < len(p.Spectating.Spectators); i++ {
		p.Spectating.Spectators[i].Queue.WritePacket(42, p.ID)
	}
	p.Spectating = nil
	p.Queue.WritePacket(66, "#spectator")
}

func SpectateFramesEvent(p *Player, data []byte) {
	if p.Spectators == nil || len(p.Spectators) == 0 {
		return
	}

	for i := 0; i < len(p.Spectating.Spectators); i++ {
		p.Spectating.Spectators[i].Queue.WritePacket(15, data)
	}
}

func JoinLobby(p *Player) {
	Lobby = append(Lobby, p)
	for i := 0; i < len(Matches); i++ {
		p.Queue.WritePacket(26, Matches[i].MatchData())
	}
}

func LeaveLobby(p *Player) {
	for i := 0; i < len(Lobby); i++ {
		if Lobby[i] == p {
			Lobby[i] = Lobby[len(Lobby)-1]
			Lobby[len(Lobby)-1] = nil
			Lobby = Lobby[:len(Lobby)-1]
		}
	}
}

func MatchCreateEvent(p *Player, data []byte) {
	m := CreateMatch()
	m.ReadMatch(data)
	m.Host = p.ID
	m.Creator = m.Host
	fmt.Println(p.Username, "created a new MultiplayerLobby", m.Name)
	if !m.AddPlayer(p, m.Password) {
		p.Queue.WritePacket(37)
		return
	}

	mdata := m.MatchData()
	p.Queue.WritePacket(26, mdata)
	p.Queue.WritePacket(36, mdata)
	p.Queue.WritePacket(64, "#multiplayer")
	p.Queue.WritePacket(65, "#multiplayer", "Multiplayer Game Channel", 1)

	for i := 0; i < len(Lobby); i++ {
		Lobby[i].Queue.WritePacket(26, mdata)
	}
}

func MatchChangePasswordEvent(player *Player, bytes []byte) {
	if player.Match.Host != player.ID || len(bytes) < 12 { // 12 = 8 + 2 null strings
		return
	}
	s := Stream{Data:bytes, Pos:8}
	_ = s.ReadString()
	newPassword := s.ReadString()
	player.Match.Password = newPassword
	for i := 0; i < len(player.Match.Players); i++ {
		player.Match.Players[i].Queue.WritePacket(91, newPassword)
	}
}

func MatchFinishedEvent(p *Player) {
	if p.Match == nil {
		return
	}

	slot := p.Match.GetPlayerSlotIndex(p)
	if slot == -1 {
		p.Match = nil
		return
	}
	if p.Match.Slots[slot].Status != 32 {
		return
	}

	p.Match.Slots[slot].Completed = true
	if p.Match.CheckFinished() {
		p.Match.MatchRunning = false
		for i := 0; i < 16; i++ {
			if p.Match.Slots[i].Status == 32 && p.Match.Slots[i].User != nil  {
				p.Match.Slots[i].User.Queue.WritePacket(58)
			}
		}
	}
}

func MatchSkipEvent(p *Player) {
	if p.Match == nil {
		return
	}

	slot := p.Match.GetPlayerSlotIndex(p)
	if slot == -1 {
		p.Match = nil
		return
	}
	if p.Match.Slots[slot].Status != 32 {
		return
	}

	p.Match.Slots[slot].Skipped = true
	for i := 0; i < 16; i++ {
		if p.Match.Slots[i].Status == 32 && p.Match.Slots[i].User != nil  {
			p.Match.Slots[i].User.Queue.WritePacket(81, slot)
		}
	}
	if p.Match.CheckSkip() {
		for i := 0; i < 16; i++ {
			if p.Match.Slots[i].Status == 32 && p.Match.Slots[i].User != nil  {
				p.Match.Slots[i].User.Queue.WritePacket(61)
			}
		}
	}
}

func MatchFailedEvent(p *Player) {
	if p.Match == nil {
		return
	}

	slot := p.Match.GetPlayerSlotIndex(p)
	if slot == -1 {
		p.Match = nil
		return
	}
	if p.Match.Slots[slot].Status != 32 {
		return
	}
	for i := 0; i < 16; i++ {
		if p.Match.Slots[i].Status == 32 && p.Match.Slots[i].User != nil  {
			p.Match.Slots[i].User.Queue.WritePacket(57, slot)
		}
	}
}

func MatchLoadedEvent(p *Player) {
	if p.Match == nil {
		return
	}

	slot := p.Match.GetPlayerSlot(p)
	if slot == nil {
		p.Match = nil
		return
	}
	slot.Loaded = true
	if p.Match.CheckLoaded() {
		for i := 0; i < 16; i++ {
			if p.Match.Slots[i].Status == 32 && p.Match.Slots[i].User != nil  {
				p.Match.Slots[i].User.Queue.WritePacket(53)
			}
		}
	}
}

func MatchStartEvent(p *Player, bytes []byte) {
	if p.Match == nil || p.Match.Host != p.ID {
		return
	}

	p.Match.ReadMatch(bytes)
	p.Match.MatchRunning = true
	for i := 0; i < 16; i++ {
		if p.Match.Slots[i].User != nil && p.Match.Slots[i].Status == 8 {
			p.Match.Slots[i].Status = 32
			p.Match.Slots[i].User.Queue.WritePacket(46, bytes)
		}
	}
	p.Match.Update()
}

func PrivateMessage(p *Player, data []byte) {
	s := Stream{Data:data}
	if data[0] == 0 {
		s.Pos += 2
	} else {
		s.ReadString()
	}

	content := s.ReadString()
	target := s.ReadString()
	if target[0] == '#' {
		IrcMessageEvent(p, data)
		return
	}

	if target == botName {
		if !strings.HasPrefix(content, cmdPrefix) {
			return
		}
		target = p.Username
		// TODO: process commands
		return
	}

	t := FindPlayerByUsername(target)
	if t == nil {
		return
	}
	t.Queue.WritePacket(7, p.Username, content, target, p.ID)
}

func MatchModsChangeEvent(p *Player, bytes []byte) {
	if len(bytes) != 4 || p.Match == nil {
		return
	}

	mods := *(*int32)(unsafe.Pointer(&bytes[0]))
	rx := (mods & 128) != 0
	if p.Match.Host == p.ID && !p.Match.FreeMod {
		mrx := (p.Match.Mods & 128) != 0
		if mrx != rx {
			for i := 0; i < 16; i++ {
				if p.Match.Slots[i].User != nil && p.Match.Slots[i].Status != 128 {
					p.Match.Slots[i].User.SetRelax(rx, true)
				}
			}
		}
		p.Match.Mods = mods
	} else if p.Match.FreeMod {
		if (mods & 64) != 0 && p.Match.Host == p.ID {
			if (mods & 512) != 0 {
				p.Match.Mods |= 512
			}
			p.Match.Mods |= 64
		} else if p.Match.Host == p.ID {
			p.Match.Mods &= ^(576)
		}
		mods &= ^(576)
		slot := p.Match.GetPlayerSlot(p)
		slot.Mods = mods
		p.SetRelax(rx, true)
	}
	p.Match.Update()
}

func MatchGotBeatmapEvent(p *Player) {
	if p.Match == nil {
		return
	}

	slot := p.Match.GetPlayerSlot(p)
	if slot == nil {
		MatchLeaveEvent(p)
		return
	}
	slot.Status = byte(int(slot.Status) & ^(16))
	p.Match.Update()
}

func MatchNoBeatmap(p *Player) {
	if p.Match == nil {
		return
	}

	slot := p.Match.GetPlayerSlot(p)
	if slot == nil {
		MatchLeaveEvent(p)
		return
	}
	slot.Status |= 16
	p.Match.Update()
}

func MatchSettingsChangeEvent(p *Player, bytes []byte) {
	if p.Match == nil || p.Match.Host != p.ID {
		return
	}
	p.Match.ReadMatch(bytes)
	p.Match.Update()
}

func MatchSlotLockEvent(p *Player, bytes []byte) {
	if len(bytes) != 4 || p.Match == nil || p.Match.Host != p.ID {
		return
	}
	s := bytes[0]
	if s > 15 {
		return
	}
	slot := &p.Match.Slots[s]
	if slot.User != nil {
		slot.User = nil
		slot.Status = 2
		slot.Team = 0
		slot.Mods = 0
	} else if slot.Status == 1 {
		slot.Status = 2
	} else {
		slot.Status = 1
	}
	p.Match.Update()
}

func MatchReadyEvent(p *Player) {
	if p.Match == nil {
		return
	}
	x := p.Match.GetPlayerSlot(p)
	x.Status = 8
	p.Match.Update()
}

func MatchUnReadyEvent(p *Player) {
	if p.Match == nil {
		return
	}
	x := p.Match.GetPlayerSlot(p)
	x.Status = 4
	p.Match.Update()
}

func MatchChangeSlotEvent(p *Player, bytes []byte) {
	if len(bytes) != 4 || p.Match == nil {
		return
	}

	s := bytes[0]
	if s > 15 {
		return
	}
	slot := p.Match.Slots[s]
	if slot.User != nil || slot.Status != 1 {
		return
	}
	psloti := p.Match.GetPlayerSlotIndex(p)
	p.Match.Slots[s] = p.Match.Slots[psloti]
	p.Match.Slots[psloti] = slot
	p.Match.Update()
}

func MatchLeaveEvent(p *Player) {
	if p.Match == nil {
		return
	}

	for i := 0; i < len(p.Match.Players); i++ {
		if p.Match.Players[i] == p {
			p.Match.Players[i] = p.Match.Players[len(p.Match.Players)-1]
			p.Match.Players[len(p.Match.Players)-1] = nil
			p.Match.Players = p.Match.Players[:len(p.Match.Players)-1]

			p.Queue.WritePacket(66, "#multiplayer")
			JoinLobby(p)
			if len(p.Match.Players) == 0 {
				for j := 0; j < len(Lobby); j++ {
					Lobby[j].Queue.WritePacket(26, p.Match.ID)
				}
				p.Match.Destroy()
				p.Match = nil
				return
			}
			if p.Match.Host == p.ID {
				for j := 0; j < 16; j++ {
					if p.Match.Slots[j].User != nil {
						p.Match.Host = p.Match.Slots[j].User.ID
					}
				}
			}
			p.Match.GetPlayerSlot(p).Clear()
			p.Match.Update()
			p.Match = nil
		}
	}
}

func MatchJoinEvent(p *Player, bytes []byte) {
	if p.Match != nil {
		MatchLeaveEvent(p)
	}

	id := *(*int16)(unsafe.Pointer(&bytes[0]))
	m := FindMatch(id)

	if i := 4; m == nil || !m.AddPlayer(p, ReadString(bytes, &i)) {
		p.Queue.WritePacket(37)
		return
	}
	p.Queue.WritePacket(36, m.MatchData())
	m.Update()
	p.Queue.WritePacket(64, "#multiplayer")
	p.Queue.WritePacket(65, "#multiplayer", "Multiplayer Game Channel", len(m.Players))
}

// Handlers
func handle(w http.ResponseWriter, req *http.Request) {
	if req.Header.Get("user-agent") != "osu!" {
		return
	}
	token := req.Header.Get("osu-token")
	if token == "" {
		handleLogin(w, req)
		return
	}

	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return
	}
	p := FindPlayer(token)
	if p == nil {
		w.WriteHeader(403)
		data := make([]byte, 11)
		data[0] = 5
		data[3] = 4
		*(*int32)(unsafe.Pointer(&data[7])) = -5
		_, _ = w.Write(data)
		return
	}
	s := Stream{Data: b, Pos: 0}
	for (s.Pos + 6) < len(b) {
		id := s.ReadInt16()
		s.Pos++
		length := s.ReadInt32()
		lpos := s.Pos + int(length)
		if lpos > len(b) {
			break
		}
		Data := s.Data[s.Pos:lpos]
		s.Pos = lpos
		switch id {
			case 4:
				p.Ping = time.Now().Unix()
			case 0:
				StatusUpdateEvent(p, Data)
			case 1:
				IrcMessageEvent(p, Data)
			case 2:
				RemovePlayer(p)
			case 3:
				StatsUpdateRequestEvent(p)
			case 16:
				StartSpectatingEvent(p, Data)
			case 17:
				StopSpectatingEvent(p)
			case 18:
				SpectateFramesEvent(p, Data)
			case 25:
				PrivateMessage(p, Data)
			case 29:
				LeaveLobby(p)
			case 30:
				JoinLobby(p)
			case 31:
				MatchCreateEvent(p, Data)
			case 32:
				MatchJoinEvent(p, Data)
			case 33:
				MatchLeaveEvent(p)
			case 38:
				MatchChangeSlotEvent(p, Data)
			case 39:
				MatchReadyEvent(p)
			case 40:
				MatchSlotLockEvent(p, Data)
			case 41:
				MatchSettingsChangeEvent(p, Data)
			case 44:
				MatchStartEvent(p, Data)
			case 49:
				MatchFinishedEvent(p)
			case 51:
				MatchModsChangeEvent(p, Data)
			case 52:
				MatchLoadedEvent(p)
			case 54:
				MatchNoBeatmap(p)
			case 55:
				MatchUnReadyEvent(p)
			case 56:
				MatchFailedEvent(p)
			case 59:
				MatchGotBeatmapEvent(p)
			case 60:
				MatchSkipEvent(p)
			case 63:
				ChannelJoinEvent(p, Data)
			case 78:
				ChannelLeaveEvent(p, Data)
			case 85:
				MultiStatsUpdateRequestEvent(p, Data)
			case 90:
				MatchChangePasswordEvent(p, Data)
			default:
				fmt.Printf("Unhandled Packet %d %d\n", id, length)
		}
	}
	_, _ = w.Write(p.Queue.Data.Data)
	p.Queue.Data = Stream{Data: make([]byte, 0)}
}

var Players []*Player
var Channels []*Channel
var Lobby []*Player
var Matches []*MultiplayerLobby
var Bot *Player
func handleLogin(w http.ResponseWriter, req *http.Request) {
	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return
	}
	body := strings.Split(string(b), "\n")
	if len(body) < 2 {
		return
	}
	username := body[0]
	password := body[1]
	if username == "" || password == "" {
		return
	}
	var id int32
	var dbpass string
	err = db.QueryRow("SELECT id, username, password_md5 FROM users WHERE username = ? OR username_safe = ?", username, username).
		Scan(&id, &username, &dbpass)
	ps := Packetstream{PType: 5, Items: make(map[int]interface{}), Data:Stream{Data: make([]byte, 0)}}
	if err != nil {
		if err == sql.ErrNoRows {
			ps.AddData(int32(-1))
			ps.WriteData()
			w.Header().Set("cho-token", "no")
			_, _ = w.Write(ps.Data.Data)
			return
		}
		fmt.Println(err.Error())
		return
	}
	if id == 0 || dbpass == "" || bcrypt.CompareHashAndPassword([]byte(dbpass), []byte(password)) != nil {
		ps.AddData(int32(-1))
		ps.WriteData()
		w.Header().Set("cho-token", "no")
		_, _ = w.Write(ps.Data.Data)
		return
	}
	fmt.Println(username + " logged in.")
	p := Player{ID: id, Username: username, Password: password, Stats: make([]UserStats, 7), Gamemode: 0}
	p.SafeUsername = strings.ToLower(username)
	p.Token = RandString(12)
	p.Country = 245
	p.Privileges = PermissionDeveloper | PermissionBAT | PermissionSupporter | PermissionPlayer
	p.Queue = Packetstream{Data:Stream{Data: make([]byte, 0)}, Items: make(map[int]interface{})}
	ps.AddData(id)
	ps.WriteData()

	ps.WritePacket(75, int32(29))
	ps.WritePacket(71, int32(p.Privileges))
	ps.WritePacket(24, "こんにちは、 " + username + "！")
	
	p.GetStats()
	PresencePacket(&ps, &p)
	StatsPacket(&ps, &p)
	PresencePacket(&ps, Bot)
	StatsPacket(&ps, Bot)

	ps.WritePacket(89)
	for i := 0; i < len(Channels); i++ {
		if Channels[i].AdminRead && !p.IsAdmin() {
			continue
		}
		ps.WritePacket(65, Channels[i].Name, Channels[i].Topic, Channels[i].Users())
	}

	w.Header().Set("cho-token", p.Token)
	_, _ = w.Write(ps.Data.Data)
	for i := 0; i < len(Players); i++ {
		if !Players[i].Bot {
			PresencePacket(&Players[i].Queue, &p)
			StatsPacket(&Players[i].Queue, &p)
		}
		PresencePacket(&p.Queue, Players[i])
		StatsPacket(&p.Queue, Players[i])
	}
	Players = append(Players, &p)
	f := p.GetFriends()
	if len(f) > 0 {
		p.Queue.WritePacket(72, f)
	}
}

// TODO: Web Handling
func handleWeb(w http.ResponseWriter, req *http.Request) {
	_, _ = w.Write([]byte("Hachimitsu's Web Handling is currently not available"))

}

func webError(err string) []byte {
	return []byte("error: " + err)
}

const ApiToken = "3ab77046686f352e4e1e319022b8893c85b29e88"
func ApiRequest(endpoint string, queryArgs ...string) ([]byte, error) {
	url := "https://osu.ppy.sh/api/" + endpoint + "?k=" + ApiToken
	for i := 0; i < len(queryArgs); i++ {
		url += "&" + queryArgs[i]
	}
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
		return []byte(""), err
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return []byte(""), err
	}
	
	return body, nil
}

const (
	MaxLeaderboard = 250
	relaxScoresQuery = "SELECT id,user,score,max_combo,300_count,100_count,50_count,katus_count,gekis_count,misses_count,play_mode,completed,full_combo,mods,time,username,pp FROM rx_scores INNER JOIN users ON users.id = rx_scores.userid WHERE completed = 3 AND beatmap_md5 = ? AND play_mode = ? AND users.privileges & 1 AND pp > 0 ORDER BY pp DESC"
	vanillaScoresQuery = "SELECT id,user,score,max_combo,300_count,100_count,50_count,katus_count,gekis_count,misses_count,play_mode,completed,full_combo,mods,time,username,pp FROM scores INNER JOIN users ON users.id = scores.userid WHERE completed = 3 AND beatmap_md5 = ? AND play_mode = ? AND users.privileges & 1 AND pp > 0 ORDER BY score DESC"
)
func handleLeaderboard(w http.ResponseWriter, req *http.Request) {
	/*_, _ = w.Write([]byte(`2|false|889980|409164|0
0
S3RL feat Mixie Moon - FriendZoned [Hard]
10.0

5003764|[bruh] Flame                        ✅|3074728|285|3|51|574|12|27|89|False|0|54849|153|1583675977|1`))
return*/
	username := req.URL.Query().Get("us")
	if username == "" {
		w.WriteHeader(408)
		return
	}
	password := req.URL.Query().Get("ha")
	if password == "" {
		w.WriteHeader(408)
		return
	}
	p := FindPlayerByUsername(username)
	if p == nil {
		w.WriteHeader(403)
		_, _ = w.Write(webError("nouser"))
		return
	}
	if password != p.Password {
		w.WriteHeader(403)
		_, _ = w.Write(webError("pass"))
		return
	}

	hash := req.URL.Query().Get("c")
	if hash == "" || len(hash) != 32 {
		_, _ = w.Write(webError("beatmap"))
		return
	}

	setID, err := strconv.Atoi(req.URL.Query().Get("i"))
	if err != nil || setID < 1 {
		_, _ = w.Write(webError("beatmap"))
		return
	}

	mode, err := strconv.Atoi(req.URL.Query().Get("m"))
	if err != nil || mode < 0 || mode > 3 {
		mode = 0
	}

	sType, err := strconv.Atoi(req.URL.Query().Get("v"))
	if err != nil {
		sType = ScoreboardType_None
	}

	mods, err := strconv.Atoi(req.URL.Query().Get("mods"))
	if err != nil {
		mods = 0
	}

	b := FindBeatmap(hash, setID)
	if b == nil {
		bytes, err := ApiRequest("get_beatmaps", "h=" + hash)
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(500)
			return
		}
		
		jsonf := []struct {
			SetID        		string      `json:"beatmapset_id"`
			ID          		string      `json:"beatmap_id"`
			Approved            string      `json:"approved"`
			TotalLength         string      `json:"total_length"`
			HitLength           string      `json:"hit_length"`
			Version             string      `json:"version"`
			FileMd5             string      `json:"file_md5"`
			DiffSize            string      `json:"diff_size"`
			DiffOverall         string      `json:"diff_overall"`
			DiffApproach        string      `json:"diff_approach"`
			DiffDrain           string      `json:"diff_drain"`
			Mode                string      `json:"mode"`
			CountNormal         string      `json:"count_normal"`
			CountSlider         string      `json:"count_slider"`
			CountSpinner        string      `json:"count_spinner"`
			SubmitDate          string      `json:"submit_date"`
			ApprovedDate        string      `json:"approved_date"`
			LastUpdate          string      `json:"last_update"`
			Artist              string      `json:"artist"`
			ArtistUnicode       interface{} `json:"artist_unicode"`
			Title               string      `json:"title"`
			TitleUnicode        interface{} `json:"title_unicode"`
			Creator             string      `json:"creator"`
			CreatorID           string      `json:"creator_id"`
			Bpm                 string      `json:"bpm"`
			Source              string      `json:"source"`
			Tags                string      `json:"tags"`
			GenreID             string      `json:"genre_id"`
			LanguageID          string      `json:"language_id"`
			FavouriteCount      string      `json:"favourite_count"`
			Rating              string      `json:"rating"`
			Storyboard          string      `json:"storyboard"`
			Video               string      `json:"video"`
			DownloadUnavailable string      `json:"download_unavailable"`
			AudioUnavailable    string      `json:"audio_unavailable"`
			Playcount           string      `json:"playcount"`
			Passcount           string      `json:"passcount"`
			Packs               string      `json:"packs"`
			MaxCombo            string      `json:"max_combo"`
			DiffAim             string      `json:"diff_aim"`
			DiffSpeed           string      `json:"diff_speed"`
			Difficultyrating    string      `json:"difficultyrating"`
		}{}
		err = json.Unmarshal(bytes, &jsonf)
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(500)
			return
		}
		
		if len(jsonf) != 1 {
			resp, err := http.Get("https://osu.ppy.sh/web/maps/" + req.URL.Query().Get("f"))
			if err != nil {
				fmt.Println(err)
				w.WriteHeader(500)
				return
			}
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				fmt.Println(err)
				w.WriteHeader(500)
				return
			}
			if string(body) == "" {
				w.Write([]byte(DefaultOnline))
			} else {
				w.Write([]byte("1|false"))
			}
			return
		} else {
			bmID, err := strconv.Atoi(jsonf[0].ID)
			if err != nil {
				fmt.Println(err)
				w.WriteHeader(500)
				return
			}
			setID, err := strconv.Atoi(jsonf[0].SetID)
			if err != nil {
				fmt.Println(err)
				w.WriteHeader(500)
				return
			}
			var status int
			switch jsonf[0].Approved[0] {
				case '-':
				case '0':
					status = BS_Pending
					break
				default:
				case '1':
					status = BS_Ranked
					break
				case '2':
					status = BS_Approved
					break
				case '3':
					status = BS_Qualified
					break
				case '4':
					status = BS_Loved
					break
			}
			b = &Beatmap{ID: bmID, SetID: setID, SongName: jsonf[0].Artist + " - " + jsonf[0].Title + "[" + jsonf[0].Version + "]", Status: status}
			_, err = db.Exec("INSERT INTO beatmaps (beatmap_id, beatmapset_id, song_name, ranked) VALUES (?, ?, ?, ?)", b.ID, b.SetID, b.SongName, b.Status)
		}
	}

	if b.Status < BS_Ranked {
		_, _ = w.Write([]byte("0|false"))
		return
	}

	relax := (mods & 128) != 0 && sType != ScoreboardType_Mods && mode != 3
	var personalBest *Score
	var scores []*Score

	var rows *sql.Rows
	if relax {
		rows, err = db.Query(relaxScoresQuery)
	} else {
		rows, err = db.Query(vanillaScoresQuery)
	}
	if err != nil {
		w.Write([]byte(b.Online()))
		return
	}
	
	defer rows.Close()
	for i := 0; rows.Next(); i++ {
		s := &Score{}
		s.ScoreFromRow(rows, "")
		if len(scores) < MaxLeaderboard {
			if sType == ScoreboardType_Friends && db.QueryRow("SELECT 1 FROM users_relationships WHERE user1 = ? AND user2 = ?", p.ID, s.UserID).Scan(new(int)) == sql.ErrNoRows {
				continue
			} else if sType == ScoreboardType_Mods && s.Mods != mods {
				continue
			}
			scores = append(scores, s)
			s.Rank = len(scores)
		} else {
			s.Rank = i
		}
		if s.UserID == p.ID {
			personalBest = s
		}
	}

	_, _ = w.Write([]byte(b.OnlineScores(personalBest, scores)))
}

func handleSubmit(w http.ResponseWriter, req *http.Request) {
	_, _ = w.Write([]byte("ok"))
	println("received play!")
	_, _ = db.Exec("UPDATE users_stats SET playcount_std = playcount_std + 1 WHERE id = 1009")
}

func handleScreenshotUpload(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		return
	}
	
	err := req.ParseMultipartForm(2147483647)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(500)
		return
	}
	
	username := req.PostFormValue("u")
	if username == "" {
		w.WriteHeader(408)
		return
	}
	password := req.PostFormValue("p")
	if password == "" {
		w.WriteHeader(408)
		return
	}
	p := FindPlayerByUsername(username)
	if p == nil {
		w.WriteHeader(403)
		_, _ = w.Write(webError("nouser"))
		return
	}
	if password != p.Password {
		w.WriteHeader(403)
		_, _ = w.Write(webError("pass"))
		return
	}

	f, _, _ := req.FormFile("ss")
	imageData, _ := ioutil.ReadAll(f)
	
	fileName := RandString(12)
	
	err = ioutil.WriteFile("./screenshots/" + fileName, imageData, 644)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(404)
		return
	}
	
	_, _ = w.Write([]byte("https://akatsuki.pw/ss/" + fileName))
}

func handleGetScreenshot(w http.ResponseWriter, req *http.Request) {
	
}

var db *sql.DB
func main() {
	sdb, err := sql.Open("mysql", "root:lol123@/ripplef")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	err = sdb.Ping()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	db = sdb

	SetupChannels()
	SetupCalls()
	Bot = &Player{ID: botId, Username: botName, Bot:true, Privileges:8, Country:245}
	SetupCommands()
	http.HandleFunc("/", handle)
	http.HandleFunc("/web/", handleWeb)
	http.HandleFunc("/web/osu-submit-modular-selector.php", handleSubmit)
	http.HandleFunc("/web/osu-osz2-getscores.php", handleLeaderboard)
	http.HandleFunc("/web/osu-screenshot.php", handleScreenshotUpload)
	http.HandleFunc("/ss/", handleGetScreenshot)
	fmt.Println("Listening")
	_ = http.ListenAndServe(":5001", nil)
}

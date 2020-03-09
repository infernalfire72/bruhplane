package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type ApiResponse struct {
	Code int `json:"code"`
	Status string `json:"status"`
}

func SetupCalls() {
	http.HandleFunc("/api/", handleApiFront)
	http.HandleFunc("/api/ping", handleApiPing)
	http.HandleFunc("/api/user/", handleApiUser)
	http.HandleFunc("/api/user/online", handleApiUserOnline)
	http.HandleFunc("/api/channels/", handleApiChannels)
}

type ApiSimpleUser struct {
	ID int32 `json:"id"`
	Username string `json:"username"`
	UsernameSafe string `json:"username_safe"`
}

type ApiExtendedUser struct {
	ApiSimpleUser
	Gamemode byte `json:"mode"`
	CountryCode byte `json:"osu_country_code"`
	Relax bool `json:"is_relax"`
	Channels []ApiChannel `json:"channels"`
	Spectating int32 `json:"spectating"`
	Spectators []int32 `json:"spectators"`
	Match int16 `json:"match"`
}

type ApiFullUser struct {
	ApiExtendedUser
	PingTime int64 `json:"ping_time"`
	Stats []UserStats `json:"stats"`
}

type ApiChannel struct {
	Name string `json:"name"`
	Topic string `json:"topic"`
	UserCount int16 `json:"user_count"`
}

type ApiChannelUsers struct {
	ApiChannel
	Users []ApiSimpleUser `json:"users"`
}

func handleApiChannels(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		return
	}
	name := getRequestQueryParam(request, "name")

	if name != "" {
		type ChannelResponseUser struct {
			ApiResponse
			ApiChannelUsers
		}
		c := FindChannel("#" + name)
		if c == nil {
			writer.WriteHeader(404)
			bytes, _ := json.Marshal(err404("channel not found"))
			_, _ = writer.Write(bytes)
			return
		}
		r := ChannelResponseUser{
			ApiResponse: success(),
			ApiChannelUsers: getApiChannelUsers(c),
		}
		bytes, _ := json.Marshal(r)
		_, _ = writer.Write(bytes)
	} else {
		ch := make([]ApiChannel, len(Channels))
		for i := 0; i < len(ch); i++ {
			ch[i] = getApiChannel(Channels[i])
		}
		type MultiChannelsResponse struct {
			ApiResponse
			Channels []ApiChannel `json:"channels"`
		}
		r := MultiChannelsResponse{
			ApiResponse: success(),
			Channels:    ch,
		}
		bytes, _ := json.Marshal(r)
		_, _ = writer.Write(bytes)
	}
}

func handleApiUserOnline(writer http.ResponseWriter, request *http.Request) {
	id := getRequestQueryParam(request, "id")
	var p *Player
	if id != "" {
		if int, err := strconv.Atoi(id); err == nil {
			p = FindPlayerById(int32(int))
		}
	}
	name := getRequestQueryParam(request, "name")
	if p == nil && name != "" {
		p = FindPlayerByUsername(name)
	}
	r := struct {
		ApiResponse
		IsOnline bool `json:"online"`
	}{IsOnline: p!=nil, ApiResponse: success()}
	bytes, err := json.Marshal(r)
	if err != nil {
		writer.WriteHeader(500)
		panic(err)
		return
	}
	_, _ = writer.Write(bytes)
}

func handleApiUser(writer http.ResponseWriter, request *http.Request) {
	id := getRequestQueryParam(request, "id")
	var p *Player
	if id != "" {
		if int, err := strconv.Atoi(id); err == nil {
			p = FindPlayerById(int32(int))
		}
	}
	name := getRequestQueryParam(request, "name")
	if p == nil && name != "" {
		p = FindPlayerByUsername(name)
	}
	var exuser ApiExtendedUser
	if p != nil {
		exuser = getApiExtendedUser(p)
	}  else {
		exuser = ApiExtendedUser{
			ApiSimpleUser: ApiSimpleUser{},
			Gamemode:      0,
			CountryCode:   0,
			Relax:         false,
			Channels:      nil,
			Spectating:    -1,
			Spectators:    nil,
			Match:         -1,
		}

		_ = db.QueryRow("SELECT id, username, username_safe FROM users WHERE id = ? OR username = ? OR username_safe = ?", id, name, name).Scan(&exuser.ApiSimpleUser.ID, &exuser.ApiSimpleUser.Username, &exuser.ApiSimpleUser.UsernameSafe)
	}

	if getRequestQueryParam(request, "full") == "1" {
		r := struct {
			ApiResponse
			ApiFullUser
		}{success(), ApiFullUser{ApiExtendedUser: exuser}}

		if p != nil {
			r.ApiFullUser.PingTime = p.Ping
			r.ApiFullUser.Stats = p.Stats
		} else {
			r.ApiFullUser.PingTime = -1
			dbp := Player{ID: exuser.ID, Stats: make([]UserStats, 7)}
			dbp.GetStats()
			r.ApiFullUser.Stats = dbp.Stats
		}

		bytes, err := json.Marshal(r)
		if err != nil {
			writer.WriteHeader(500)
			panic(err)
			return
		}
		_, _ = writer.Write(bytes)
	} else {
		r := struct {
			ApiResponse
			ApiExtendedUser
		}{success(), exuser}

		bytes, err := json.Marshal(r)
		if err != nil {
			writer.WriteHeader(500)
			panic(err)
			return
		}
		_, _ = writer.Write(bytes)
	}
}

func handleApiPing(writer http.ResponseWriter, request *http.Request) {
	bytes, err := json.Marshal(ApiResponse{200, "Hi there!"})
	if err != nil {
		writer.WriteHeader(500)
		panic(err)
		return
	}
	_, _ = writer.Write(bytes)
}

func handleApiFront(writer http.ResponseWriter, request *http.Request) {
	
}

func getRequestQueryParam(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}

func getApiChannel(c *Channel) ApiChannel {
	return ApiChannel{
		Name: c.Name,
		Topic:     c.Topic,
		UserCount: c.Users(),
	}
}

func getApiChannelUsers(c *Channel) ApiChannelUsers {
	ch := ApiChannelUsers{
		ApiChannel: getApiChannel(c),
	}
	ch.Users = make([]ApiSimpleUser, len(c.Players))
	for i := 0; i < len(c.Players); i++ {
		ch.Users[i] = getApiSimpleUser(c.Players[i])
	}
	return ch
}

func getApiSimpleUser(p *Player) ApiSimpleUser {
	return ApiSimpleUser{p.ID, p.Username, p.SafeUsername}
}

func getApiExtendedUser(p *Player) ApiExtendedUser {
	u := ApiExtendedUser{ApiSimpleUser: getApiSimpleUser(p), Gamemode: p.Gamemode, CountryCode: p.Country, Relax: p.Relax, Spectating: getPlayerId(p.Spectating)}
	u.Channels = make([]ApiChannel, len(p.Channels))
	for i := 0; i < len(p.Channels); i++ {
		u.Channels[i] = getApiChannel(p.Channels[i])
	}
	if p.Match != nil {
		u.Match = p.Match.ID
	}
	u.Spectators = make([]int32, len(p.Spectators))
	for i := 0; i < len(p.Spectators); i++ {
		u.Spectators[i] = p.Spectators[i].ID
	}
	return u
}

func getPlayerId(p *Player) int32 {
	if p == nil {
		return 0
	}
	return p.ID
}

func success() ApiResponse {
	return ApiResponse{
		Code:200,
		Status: "success",
	}
}

func err404(err string) ApiResponse {
	return ApiResponse{
		Code:   404,
		Status: err,
	}
}
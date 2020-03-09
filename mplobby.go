package main

import (
	"unsafe"
)

type MultiplayerSlot struct {
	Loaded, Skipped, Completed bool
	Status, Team byte
	User *Player
	Mods int32
}

func (m *MultiplayerSlot) IsFree() bool {
	return m.User == nil && m.Status == 1
}

func (m *MultiplayerSlot) Clear() {
	m.User = nil
	m.Status = 1
	m.Team = 0
	m.Mods = 0
}

func (m *MultiplayerSlot) Lock() {
	m.Clear()
	m.Status = 2
}

func (m *MultiplayerSlot) Open() {
	m.Status = 1
}

type MultiplayerLobby struct {
	ID int16
	Host, Creator, Beatmap, Mods, Seed int32
	Name, Password, BeatmapName, BeatmapHash string
	MatchRunning, FreeMod bool
	Gamemode, TeamType, MatchType, ScoreType byte

	Players []*Player
	Slots [16]MultiplayerSlot
}

func CreateMatch() *MultiplayerLobby {
	m := &MultiplayerLobby{Slots: [16]MultiplayerSlot{}}
	if len(Matches) == 0 {
		m.ID = 1
	} else {
		m.ID = Matches[len(Matches)-1].ID + 1
	}
	return m
}

func (m *MultiplayerLobby) ReadMatch(data []byte) {
	m.MatchRunning = data[2] == 1
	matchType := m.MatchType
	m.MatchType = data[3]

	if matchType != m.MatchType {
		if m.MatchType == 2 || m.MatchType == 3 {
			for i := 0; i < len(m.Slots); i++ {
				if m.Slots[i].User != nil {
					m.Slots[i].Team = byte((i % 2) + 1) // lol
				}
			}
		} else {
			for i := 0; i < len(m.Slots); i++ {
				if m.Slots[i].User != nil {
					m.Slots[i].Team = 0
				}
			}
		}
	}

	s := Stream{Data: data, Pos: 4}
	m.Mods = s.ReadInt32()
	m.Name = s.ReadString()
	m.Password = s.ReadString()
	m.BeatmapName = s.ReadString()
	m.Beatmap = s.ReadInt32()
	m.BeatmapHash = s.ReadString()
	for i := 0; i < len(m.Slots); i++ {
		m.Slots[i].Status = data[s.Pos]
		s.Pos++
	}

	for i := 0; i < len(m.Slots); i++ {
		m.Slots[i].Team = data[s.Pos]
		s.Pos++
	}

	for i := 0; i < 16; i++ {
		if (m.Slots[i].Status & 124) != 0 {

			s.Pos += 4
		}
	}
	s.Pos += 4
	m.Gamemode = s.ReadByte()
	m.ScoreType = s.ReadByte()
	m.TeamType = s.ReadByte()
	fm := m.FreeMod
	m.FreeMod = s.ReadByte() == 1

	if fm != m.FreeMod {
		if m.FreeMod {
			for i := 0; i < len(m.Slots); i++ {
				if m.Slots[i].User != nil {
					m.Slots[i].Mods = m.Mods
				}
			}
			m.Mods = 0
		} else {
			m.Mods = 0
			for i := 0; i < len(m.Slots); i++ {
				m.Slots[i].Mods = 0
			}
		}
	}

	if m.FreeMod { // is this even a thing?
		s.Pos += 4 * 16
	}
	m.Seed = s.ReadInt32()
}

func (m *MultiplayerLobby) MatchData() []byte {
	s := Stream{Data: make([]byte, 8), Pos: 8}
	*(*int16)(unsafe.Pointer(&s.Data[0])) = m.ID
	if m.MatchRunning {
		s.Data[2] = 1
	}
	s.Data[3] = m.MatchType
	*(*int32)(unsafe.Pointer(&s.Data[4])) = m.Mods
	s.WriteString(m.Name)
	s.WriteString(m.Password)
	s.WriteString(m.BeatmapName)
	s.WriteInt32(m.Beatmap)
	s.WriteString(m.BeatmapHash)

	s.Extend(32)
	for f := 0; f < 16; f++ {
		s.Data[s.Pos] = m.Slots[f].Status
		s.Pos++
	}

	for f := 0; f < 16; f++ {
		s.Data[s.Pos] = m.Slots[f].Team
		s.Pos++
	}
	for f := 0; f < 16; f++ {
		if m.Slots[f].User != nil {
			s.WriteInt32(m.Slots[f].User.ID)
		}
	}

	s.Extend(12)
	*(*int32)(unsafe.Pointer(&s.Data[s.Pos])) = m.Host
	s.Pos += 4
	s.Data[s.Pos] = m.Gamemode
	s.Pos++
	s.Data[s.Pos] = m.ScoreType
	s.Pos++
	s.Data[s.Pos] = m.TeamType
	s.Pos++
	if m.FreeMod {
		s.Data[s.Pos] = 1
	}
	s.Pos++
	if m.FreeMod {
		s.Extend(4 * 16)
		for i := 0; i < 16; i++ {
			*(*int32)(unsafe.Pointer(&s.Data[s.Pos])) = m.Slots[i].Mods
			s.Pos += 4
		}
	}
	*(*int32)(unsafe.Pointer(&s.Data[s.Pos])) = m.Seed
	return s.Data
}

func (m *MultiplayerLobby) Update() {
	mdata := m.MatchData()
	for i := 0; i < len(m.Players); i++ {
		m.Players[i].Queue.WritePacket(26, mdata)
	}

	for i := 0; i < len(Lobby); i++ {
		Lobby[i].Queue.WritePacket(26, mdata)
	}
}

func (m *MultiplayerLobby) AddPlayer(p *Player, pass string) bool {
	if pass != m.Password {
		return false
	}

	for i := 0; i < len(m.Slots); i++ {
		if (m.Slots[i].Status & 124) == 0 && m.Slots[i].User == nil {
			m.Players = append(m.Players, p)
			m.Slots[i].User = p
			m.Slots[i].Status = 4
			p.Match = m
			LeaveLobby(p)
			return true
		}
	}
	return false
}

func (m *MultiplayerLobby) Destroy() {
	for i := 0; i < len(Matches); i++ {
		if Matches[i] == m {
			Matches[i] = Matches[len(Matches)-1]
			Matches[len(Matches)-1] = nil
			Matches = Matches[:len(Matches)-1]
		}
	}

	for i := 0; i < len(m.Players); i++ {
		m.Players[i].Queue.WritePacket(28, m.ID)
		m.Players[i].Match = nil
	}

	for i := 0; i < len(Lobby); i++ {
		Lobby[i].Queue.WritePacket(28, m.ID)
	}
}

func (m *MultiplayerLobby) GetPlayerSlotIndex(p *Player) int8 {
	for i := int8(0); i < 16; i++ {
		if m.Slots[i].User == p {
			return i
		}
	}
	return -1
}

func (m *MultiplayerLobby) GetPlayerSlot(p *Player) *MultiplayerSlot {
	if s := m.GetPlayerSlotIndex(p); s != -1 {
		return &m.Slots[s]
	}
	return nil
}

func (m *MultiplayerLobby) CheckLoaded() bool {
	for i := 0; i < 16; i++ {
		if m.Slots[i].Status == 32 && !m.Slots[i].Loaded {
			return false
		}
	}
	return true
}

func (m *MultiplayerLobby) CheckSkip() bool {
	for i := 0; i < 16; i++ {
		if m.Slots[i].Status == 32 && !m.Slots[i].Skipped {
			return false
		}
	}
	return true
}

func (m *MultiplayerLobby) CheckFinished() bool {
	for i := 0; i < 16; i++ {
		if m.Slots[i].Status == 32 && !m.Slots[i].Completed {
			return false
		}
	}
	return true
}

func FindMatch(id int16) *MultiplayerLobby {
	for i := 0; i < len(Matches); i++ {
		if Matches[i].ID == id {
			return Matches[i]
		}
	}
	return nil
}
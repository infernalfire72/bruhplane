package main


func PresencePacket(ps *Packetstream, p *Player) {
	var rank int32
	if !p.Bot {
		rank = p.Stats[p.Gamemode].Rank
	}
	name := p.Username
	if p.FakeName != "" {
		name = p.FakeName
	}
	ps.WritePacket(83,
		p.ID,
		name,
		byte(24),
		p.Country,
		// byte((p.Privileges & 0x1f) | ((p.Gamemode & 0x7) << 5)),
		p.Privileges,
		float32(0),
		float32(0),
		rank)
}

func StatsPacketInterface(p *Player) []interface{} {
	gm := p.Gamemode
	if p.Relax {
		gm += 4
	}
	if gm > 7 {
		p.Gamemode = 0
		gm = 0
	}
	s := &p.Stats[gm]
	pp := s.Performance
	if pp > 32767 {
		pp = 0
	}
	i := [...]interface{}{p.ID,
		p.Action,
		p.ActionText,
		p.ActionHash,
		p.ActionMods,
		p.Gamemode,
		p.ActionBeatmap,
		s.RankedScore,
		s.Accuracy,
		s.Playcount,
		s.TotalScore,
		s.Rank,
		int16(pp),
	}

	return i[:]
}

func StatsPacket(ps *Packetstream, p *Player) {
	if p.Bot {
		ps.WritePacket(11,
			p.ID,
			p.Action,
			p.ActionText,
			p.ActionHash,
			p.ActionMods,
			p.Gamemode,
			p.ActionBeatmap,
			int64(0),
			float32(1),
			int32(0),
			int64(0),
			int32(0),
			int16(0))
		return
	}
	gm := p.Gamemode
	if p.Relax {
		gm += 4
	}
	if gm > 7 {
		p.Gamemode = 0
		gm = 0
	}
	s := &p.Stats[gm]
	pp := s.Performance
	if pp > 32767 {
		pp = 0
	}
	ps.WritePacket(11,
		p.ID,
		p.Action,
		p.ActionText,
		p.ActionHash,
		p.ActionMods,
		p.Gamemode,
		p.ActionBeatmap,
		s.RankedScore,
		s.Accuracy,
		s.Playcount,
		s.TotalScore,
		s.Rank,
		int16(pp))
}
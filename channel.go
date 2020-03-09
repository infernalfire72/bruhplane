package main

type Channel struct {
	Name, Topic string
	AdminRead, AdminWrite bool
	Players []*Player
}

func (c *Channel) Users() int16 {
	return int16(len(c.Players))
}

func FindChannel(name string) *Channel {
	for i := 0; i < len(Channels); i++ {
		if Channels[i].Name == name {
			return Channels[i]
		}
	}
	return nil
}

func (c *Channel) PlayerJoin(p *Player) bool {
	if c.AdminRead && !p.IsAdmin() {
		return false
	}
	c.Players = append(c.Players, p)
	p.Channels = append(p.Channels, c)
	return true
}

func (c *Channel) PlayerLeave(p *Player) {
	for i := 0; i < len(p.Channels); i++ {
		if p.Channels[i] == c {
			p.Channels[i] = p.Channels[len(p.Channels)-1]
			p.Channels[len(p.Channels)-1] = nil
			p.Channels = p.Channels[:len(c.Players)-1]
		}
	}

	for i := 0; i < len(c.Players); i++ {
		if c.Players[i] == p {
			c.Players[i] = c.Players[len(c.Players)-1]
			c.Players[len(c.Players)-1] = nil
			c.Players = c.Players[:len(c.Players)-1]
		}
	}
}

func (c *Channel) PlayerKick(p *Player) {
	c.PlayerLeave(p)
	p.Queue.WritePacket(66, c.Name)
}

func (c *Channel) Message(p *Player, Content string) {
	if c.AdminWrite && !p.IsAdmin() {
		return
	}

	name := p.Username
	if p.FakeName != "" {
		name = p.FakeName
	}
	for i := 0; i < len(c.Players); i++ {
		if c.Players[i] != p {
			c.Players[i].Queue.WritePacket(7, name, Content, c.Name, p.ID)
		}
	}
}

func SetupChannels() {
	Channels = make([]*Channel, 4)
	Channels[0] = &Channel{Name: "#osu", Topic: "Main osu!"}
	Channels[1] = &Channel{Name: "#announce", Topic: "Announcements", AdminWrite: true}
	Channels[2] = &Channel{Name: "#admin", Topic: "Admins are here", AdminRead: true, AdminWrite: true}
	Channels[3] = &Channel{Name: "#lobby", Topic: "Multiplayer Discussion and Advertising"}
}
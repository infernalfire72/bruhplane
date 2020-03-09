package main

import (
	"strings"
	"strconv"
)

const (
	PermissionPlayer = 1 << iota
	PermissionBAT
	PermissionSupporter
	PermissionAdmin
	PermissionDeveloper
	PermissionTournamentStaff
	PermissionModerator = PermissionBAT | PermissionSupporter
)

const cmdPrefix = "!"
const botName = "vmyui"
const botId = int32(999)

type Command struct {
	Name, Usage string
	Elevation int
	Exec func(p *Player, target string, args []string)
}

func handleSayCmd(p *Player, target string, args []string) {
	if target == p.Username {
		p.Queue.WritePacket(7, botName, strings.Join(args, " "), target, botId)
		return
	}

	c := FindChannel(target)
	if c == nil {
		return
	}
	for i := 0; i < len(c.Players); i++ {
		c.Players[i].Queue.WritePacket(7, botName, strings.Join(args, " "), target, botId)
	}
}

func handlePermCmd(p *Player, target string, args []string) {
	perm, err := strconv.Atoi(args[0])
	if err == nil {
		Bot.Privileges = byte(perm)
		PresencePacket(&p.Queue, Bot)
	}
	if target == p.Username {
		p.Queue.WritePacket(7, botName, "Done", target, botId)
		return
	}

	c := FindChannel(target)
	if c == nil {
		return
	}
	for i := 0; i < len(c.Players); i++ {
		c.Players[i].Queue.WritePacket(7, botName, "Done", target, botId)
	}

}

func handleFName(p *Player, target string, args []string) {
	name := strings.TrimSpace(strings.Join(args, " "))
	if name == "" {
		p.FakeName = ""
	} else {
		p.FakeName = name
	}
	for i := 0; i < len(Players); i++ {
		PresencePacket(&Players[i].Queue, p)
	}	

	if target == p.Username {
		p.Queue.WritePacket(7, botName, "Done.", target, botId)
		return
	}

	c := FindChannel(target)
	if c == nil {
		return
	}
	for i := 0; i < len(c.Players); i++ {
		c.Players[i].Queue.WritePacket(7, botName, "Done.", target, botId)
	}

}

func handleActionKick(p *Player, target string, args []string) {
	t := FindPlayerByUsernameSafe(args[0])
	if t == nil {
		println("not found")
		return
	}
	t.Queue.WritePacket(36)
	t.Queue.WritePacket(37)
}

func handleTestServer(p *Player, target string, args []string) {
	p.Queue.WritePacket(107, "c4.ppy.sh")
}

var Commands []*Command
func SetupCommands() {
	Commands = append(Commands, &Command{
		Name:  "say",
		Usage: "say <content>",
		Elevation: PermissionDeveloper,
		Exec:  handleSayCmd,
	})

	Commands = append(Commands, &Command{
		Name:  "perm",
		Usage: "say <content>",
		Elevation: PermissionDeveloper,
		Exec:  handlePermCmd,
	})

	Commands = append(Commands, &Command{
		Name:  "fname",
		Usage: "say <content>",
		Elevation: PermissionDeveloper,
		Exec:  handleFName,
	})

	Commands = append(Commands, &Command{
		Name:      "ak",
		Usage:     "ak <user>",
		Elevation: PermissionDeveloper,
		Exec:      handleActionKick,
	})

	Commands = append(Commands, &Command{
		Name:      "test",
		Usage:     "ak <user>",
		Elevation: PermissionDeveloper,
		Exec:      handleTestServer,
	})
}

func FindCommand(name string) *Command {
	for i := 0; i < len(Commands); i++ {
		if Commands[i].Name == name {
			return Commands[i]
		}
	}
	return nil
}
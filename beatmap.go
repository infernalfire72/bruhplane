package main

import (
	"fmt"
)

const (
	BS_Pending = iota
	BS_NeedUpdate
	BS_Ranked
	BS_Approved
	BS_Qualified
	BS_Loved
	BS_NOT_SUBMITTED = -1
	DefaultOnline = "-1|false"
)

type Beatmap struct {
	ID int `json:"id"`
	SetID int `json:"set_id"`
	SongName string `json:"song_name"`
	Status int `json:"status"`
	Hash string `json:"md5"`
}

func (b *Beatmap) Online() string {
	return fmt.Sprintf("%d|false|%d|%d", b.SongName, b.ID, b.SetID)
}

func (b *Beatmap) OnlineScores(personalBest *Score, scores []*Score) string {
	s := b.Online() + fmt.Sprintf("|%d\n0\n%s\n10\n", len(scores), b.SongName)
	if personalBest != nil {
		s += personalBest.Online()
	} else {
		s += "\n"
	}

	for i := 0; i < len(scores); i++ {
		s += scores[i].Online()
	}

	return s
}

func FindBeatmap(Hash string, setID int) *Beatmap {
	return nil
}
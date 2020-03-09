package main

import (
	"database/sql"
	"fmt"
)

type Score struct {
	ID int64
	UserID int32
	Username string
	TotalScore int
	Combo int16
	N50 int16
	N100 int16
	N300 int16
	NMiss int16
	NKatu int16
	NGeki int16
	Perfect bool
	Mods int
	Rank int
	DateTicks int64
	Mode byte
	Status int
	FileHash string

	Performance float32
}

func (s *Score) Online() string {
	var Perfect int8
	if s.Perfect {
		Perfect = 1
	}
	if (s.Mods & 128) != 0 {
		return fmt.Sprintf("%d|%s|%d|%d|%d|%d|%d|%d|%d|%d|%d|%d|%d|%d|%d|1\n", s.ID, s.Username, int(s.Performance), s.Combo, s.N50, s.N100, s.N300, s.NMiss, s.NKatu, s.NGeki, Perfect, s.Mods, s.UserID, s.Rank, s.DateTicks)
	}
	return fmt.Sprintf("%d|%s|%d|%d|%d|%d|%d|%d|%d|%d|%d|%d|%d|%d|%d|1\n", s.ID, s.Username, s.TotalScore, s.Combo, s.N50, s.N100, s.N300, s.NMiss, s.NKatu, s.NGeki, Perfect, s.Mods, s.UserID, s.Rank, s.DateTicks)
}

func (s *Score) ScoreFromRow(rows *sql.Rows, username string) {
	var (
		Perfect int8
		err error
	)
	if username == "" {
		err = rows.Scan(&s.ID, &s.UserID, &s.TotalScore, &s.Combo, &s.N300, &s.N100, &s.N50, &s.NKatu, &s.NGeki, &s.NMiss, &s.Mode, &s.Status, &Perfect, &s.Mods, &s.DateTicks, &s.Username, &s.Performance)
	} else {
		err = rows.Scan(&s.ID, &s.UserID, &s.TotalScore, &s.Combo, &s.N300, &s.N100, &s.N50, &s.NKatu, &s.NGeki, &s.NMiss, &s.Mode, &s.Status, &Perfect, &s.Mods, &s.DateTicks, &s.Performance)
		s.Username = username
	}
	if err != nil {
		fmt.Println(err)
		return
	}

	if Perfect == 1 {
		s.Perfect = true
	}
}

const (
	_ = iota
	ScoreboardType_None
	ScoreboardType_Mods
	ScoreboardType_Friends
	ScoreboardType_Country
)

func (s *Score) GetRank(Relax bool, boardType int) int {
	if s.Rank != 0 {
		return s.Rank
	}
	var err error
	if Relax {
		if boardType != ScoreboardType_Mods {
			err = db.QueryRow("SELECT COUNT(*) FROM rx_scores LEFT JOIN users ON users.id = rx_scores.id WHERE beatmap_md5 = ? AND (users.privileges & 3) AND mode = ? AND status = 3 AND mods & 128 AND pp >= ? ", s.FileHash, s.Mode, s.Performance).Scan(&s.Rank)
		} else {
			err = db.QueryRow("SELECT COUNT(*) FROM rx_scores LEFT JOIN users ON users.id = rx_scores.id WHERE beatmap_md5 = ? AND (users.privileges & 3) AND mode = ? AND status = 3 AND pp >= ? ", s.FileHash, s.Mode, s.Performance).Scan(&s.Rank)
		}
	} else {
		err = db.QueryRow("SELECT COUNT(*) FROM scores LEFT JOIN users ON users.id = scores.id WHERE beatmap_md5 = ? AND (users.privileges & 3) AND mode = ? AND status = 3 AND score >= ? ", s.FileHash, s.Mode, s.TotalScore).Scan(&s.Rank)
	}
	if err != nil {
		fmt.Println(err)
	}

	return s.Rank
}
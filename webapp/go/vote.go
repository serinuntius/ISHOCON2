package main

import (
	"context"
	"database/sql"
	"log"
	"strings"
)

// Vote Model
type Vote struct {
	ID          int
	UserID      int
	CandidateID int
	Keyword     string
	VotedCount  int
}

func createVote(ctx context.Context, userID int, candidateID int, keyword string, voteCount int) {
	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return
	}

	vote := Vote{}

	row := db.QueryRowContext(ctx, "SELECT keyword, voted_count FROM votes WHERE keyword = ?", keyword)
	err = row.Scan(&vote.Keyword, &vote.VotedCount)
	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	} else if err == sql.ErrNoRows {
		// no row => insert
		tx.ExecContext(ctx, "INSERT INTO votes (user_id, candidate_id, keyword, voted_count) VALUES", userID, candidateID, keyword, 1)
	} else {
		// update
		if _, err := tx.ExecContext(ctx,
			"UPDATE votes SET voted_count = ? WHERE id = ?",
			vote.VotedCount+1, vote.ID); err != nil {
			log.Fatal(err)
		}
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE users SET voted_count = voted_count + ? WHERE id = ?",
		voteCount, userID); err != nil {
		log.Fatal(err)
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE candidates SET voted_count = voted_count + ? WHERE id = ?",
		voteCount, candidateID); err != nil {
		log.Fatal(err)
	}

	if err := tx.Commit(); err != nil {
		rerr := tx.Rollback()
		log.Fatal(rerr)
	}
}

func getVoiceOfSupporter(ctx context.Context, candidateIDs []int) (voices []string) {
	args := []interface{}{}
	for _, candidateID := range candidateIDs {
		args = append(args, candidateID)
	}
	rows, err := db.QueryContext(ctx, `
    SELECT keyword
    FROM votes
    WHERE candidate_id IN (`+ strings.Join(strings.Split(strings.Repeat("?", len(candidateIDs)), ""), ",")+ `)
    ORDER BY voted_count DESC
    LIMIT 10`, args...)
	if err != nil {
		return nil
	}

	defer rows.Close()
	for rows.Next() {
		var keyword string
		err = rows.Scan(&keyword)
		if err != nil {
			panic(err.Error())
		}
		voices = append(voices, keyword)
	}
	return
}

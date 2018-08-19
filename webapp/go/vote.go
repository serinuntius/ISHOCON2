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

	row := db.QueryRowContext(ctx, "SELECT id, keyword, voted_count FROM votes WHERE keyword = ? AND user_id = ? AND candidate_id = ? FOR UPDATE", keyword, userID, candidateID)
	err = row.Scan(&vote.ID, &vote.Keyword, &vote.VotedCount)
	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	} else if err == sql.ErrNoRows {
		// no row => insert
		_, err := tx.ExecContext(ctx, "INSERT INTO votes (user_id, candidate_id, keyword, voted_count) VALUES (?, ?, ?, ?)", userID, candidateID, keyword, voteCount)
		if err != nil {
			log.Fatal(err)
		}
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
    SELECT
		keyword,
		SUM(voted_count) AS sum_vote
    FROM votes
    WHERE candidate_id IN (`+ strings.Join(strings.Split(strings.Repeat("?", len(candidateIDs)), ""), ",")+ `)
	GROUP BY votes.keyword
	ORDER BY voted_count DESC
    LIMIT 10`, args...)
	if err != nil {
		log.Fatal(err)
		return nil
	}

	defer rows.Close()
	for rows.Next() {
		var keyword string
		var votedCount int
		err = rows.Scan(&keyword, &votedCount)
		if err != nil {
			panic(err.Error())
		}
		voices = append(voices, keyword)
	}
	return
}

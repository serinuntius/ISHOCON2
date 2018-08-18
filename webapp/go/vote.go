package main

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// Vote Model
type Vote struct {
	ID          int
	UserID      int
	CandidateID int
	Keyword     string
}

func getVoteCountByCandidateID(ctx context.Context, candidateID int) (count int) {
	row := db.QueryRowContext(ctx, "SELECT COUNT(*) AS count FROM votes WHERE candidate_id = ?", candidateID)
	row.Scan(&count)
	return
}

func getUserVotedCount(ctx context.Context, userID int) (count int) {
	row := db.QueryRowContext(ctx, "SELECT COUNT(*) AS count FROM votes WHERE user_id = ?", userID)
	row.Scan(&count)
	return
}

func createVote(ctx context.Context, userID int, candidateID int, keyword string, voteCount int) {
	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return
	}

	valueStrings := make([]string, 0, voteCount)
	valueArgs := make([]interface{}, 0, voteCount*3) // 3 is column number
	for i := 0; i < voteCount; i++ {
		valueStrings = append(valueStrings, "(?, ?, ?)")
		valueArgs = append(valueArgs, userID)
		valueArgs = append(valueArgs, candidateID)
		valueArgs = append(valueArgs, keyword)
	}

	stmt := fmt.Sprintf("INSERT INTO votes (user_id, candidate_id, keyword) VALUES %s",
		strings.Join(valueStrings, ","))
	if _, err := tx.ExecContext(ctx, stmt, valueArgs...); err != nil {
		log.Fatal(err)
	}

	if tx.ExecContext(ctx,
		"UPDATE users SET voted_count = voted_count + ? WHERE id = ?",
		voteCount, userID); err != nil {
		log.Fatal(err)
	}

	if tx.ExecContext(ctx,
		"UPDATE candidates SET vote_count = vote_count + ? WHERE id = ?",
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
    GROUP BY keyword
    ORDER BY COUNT(*) DESC
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

package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"

	"github.com/go-redis/redis"
)

// Vote Model
type Vote struct {
	ID          int
	UserID      int
	CandidateID int
	Keyword     string
	VotedCount  int
}

func zkey(candidateId int) string {
	return fmt.Sprintf("candidates:%d", candidateId)
}

func createVote(ctx context.Context, userID int, candidateID int, keyword string, voteCount int) {
	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return
	}

	var wg sync.WaitGroup

	wg.Add(3)

	go func() {
		defer wg.Done()
		if _, err := rc.ZIncrBy(zkey(candidateID), float64(voteCount), keyword).Result();
			err != nil {
			log.Fatal(err)
		}
	}()


	go func() {
		defer wg.Done()
		if _, err := tx.ExecContext(ctx,
			"UPDATE users SET voted_count = voted_count + ? WHERE id = ?",
			voteCount, userID); err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		defer wg.Done()
		if _, err := tx.ExecContext(ctx,
			"UPDATE candidates SET voted_count = voted_count + ? WHERE id = ?",
			voteCount, candidateID); err != nil {
			log.Fatal(err)
		}
	}()

	wg.Wait()

	if err := tx.Commit(); err != nil {
		rerr := tx.Rollback()
		log.Fatal(rerr)
	}
}

func getVoiceOfSupporter(candidateIDs []int) (voices []string) {
	var results []redis.Z
	for _, cID := range candidateIDs {
		keywords, err := rc.ZRevRangeWithScores(zkey(cID), 0, 9).Result()
		if err != nil {
			log.Fatal(err)
		}
		results = append(results, keywords...)
	}

	if len(candidateIDs) > 1 {
		// 複数のときはいい感じにしてあげる必要がある
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})

		for _, r := range results {
			keyword := r.Member
			if k, ok := keyword.(string); ok {
				voices = append(voices, k)
			} else {
				// string以外が入っていることはありえない
				log.Fatal("string以外が入っていることはありえない(たぶん)")
			}
		}
	} else {
		// idが1つのときは10返ってきたやつをそのまま返すで OK
		for _, r := range results {
			keyword := r.Member
			if k, ok := keyword.(string); ok {
				voices = append(voices, k)
			} else {
				// string以外が入っていることはありえない
				log.Fatal("string以外が入っていることはありえない(たぶん)")
			}
		}
	}

	return
}

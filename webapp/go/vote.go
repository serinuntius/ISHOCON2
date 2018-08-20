package main

import (
	"context"
	"fmt"
	"log"
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
	politicalParty := candidateIdMap[candidateID].PoliticalParty

	_, err := rc.ZIncrBy(politicalParty, float64(voteCount), keyword).Result()
	if err != nil {
		log.Fatal(err)
	}

	_, err = rc.ZIncrBy(candidateKey(candidateID), float64(voteCount), keyword).Result()
	if err != nil {
		log.Fatal(err)
	}

	_, err = rc.ZIncrBy(kojinKey(), float64(voteCount), candidateVotedCountKey(candidateID)).Result()
	if err != nil {
		log.Fatal(err)
	}

	_, err = rc.IncrBy(userKey(userID), int64(voteCount)).Result()
	if err != nil {
		log.Fatal(err)
	}
}

func getVoiceOfSupporter(candidateID int) (voices []string) {
	politicalParty := candidateIdMap[candidateID].PoliticalParty

	voices, err := rc.ZRevRange(politicalParty, 0, 10).Result()
	if err != nil {
		log.Fatal(err)
	}

	return
}

func getVoiceOfSupporterByParties(politicalParty string) (voices []string) {
	voices, err := rc.ZRevRange(politicalParty, 0, 10).Result()
	if err != nil {
		log.Fatal(err)
	}

	return
}

func candidateVotedCountKey(candidateID int) string {
	return fmt.Sprintf("candidateVotedCount:%d", candidateID)
}

func candidateKey(candidateID int) string {
	return "candidate:" + string(candidateID)
}

func userKey(userID int) string {
	return "user:" + string(userID)
}

func kojinKey() string {
	return "kojinkey"
}

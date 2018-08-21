package main

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
)

// Vote Model
type Vote struct {
	ID          int
	UserID      int
	CandidateID int
	Keyword     string
	VotedCount  int
}

func createVote(ctx context.Context, userID int, candidateID int, keyword string, voteCount int) error {
	politicalParty := candidateIdMap[candidateID].PoliticalParty

	_, err := rc.ZIncrBy(politicalParty, float64(voteCount), keyword).Result()
	if err != nil {
		return errors.Wrap(err, "")
	}

	_, err = rc.ZIncrBy(candidateKey(candidateID), float64(voteCount), keyword).Result()
	if err != nil {
		return errors.Wrap(err, "")
	}

	_, err = rc.ZIncrBy(kojinKey(), float64(voteCount), candidateVotedCountKey(candidateID)).Result()
	if err != nil {
		return errors.Wrap(err, "")
	}

	_, err = rc.IncrBy(userKey(userID), int64(voteCount)).Result()
	if err != nil {
		return errors.Wrap(err, "")
	}

	return nil
}

func getVoiceOfSupporter(candidateID int) ([]string, error) {
	politicalParty := candidateIdMap[candidateID].PoliticalParty

	voices, err := rc.ZRevRange(politicalParty, 0, 10).Result()
	if err != nil {
		return nil, errors.Wrap(err, "")
	}

	return voices, nil
}

func getVoiceOfSupporterByParties(politicalParty string) ([]string, error) {
	voices, err := rc.ZRevRange(politicalParty, 0, 10).Result()
	if err != nil {
		return nil, errors.Wrap(err, "")
	}

	return voices, nil
}

func candidateVotedCountKey(candidateID int) string {
	return fmt.Sprintf("candidateVotedCount:%d", candidateID)
}

func candidateKey(candidateID int) string {
	return fmt.Sprintf("candidate:%d", candidateID)
}

func userKey(userID int) string {
	return fmt.Sprintf("user:%d", userID)
}

func kojinKey() string {
	return "kojinkey"
}

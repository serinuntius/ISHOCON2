package main

import (
	"os"
	"sync"
	"time"
)

func voteScenario(m *sync.Mutex, finishTime time.Time) bool {
	voteSet := setupVotes(50, false)
	resps := map[bool]int{}
	resp := true

	for _, vote := range voteSet {
		resp = postVote(vote)
		resps[resp]++
		if resp == false {
			postMessage("投票に失敗しました at POST /vote")
			os.Exit(1)
		}
	}

	return updateScore("POST", resps, m, finishTime)
}

func invalidVoteScenario(m *sync.Mutex, finishTime time.Time) bool {
	voteSet := setupVotes(50, false)
	resps := map[bool]int{}
	resp := true

	for _, vote := range voteSet {
		r := getRand(1, 3)
		if r == 1 {
			vote.Name = "hoge"
		} else if r == 2 {
			vote.Address = "hoge"
		} else {
			vote.Mynumber = "hoge"
		}
		resp = postVote(vote)
		resps[resp]++
		if resp == false {
			postMessage("投票に失敗しました at POST /vote")
			os.Exit(1)
		}
	}

	return updateScore("POST", resps, m, finishTime)
}

func indexScenario(m *sync.Mutex, finishTime time.Time) bool {
	resps := map[bool]int{}
	resp := true

	for i := 0; i < 4; i++ {
		resp = getIndex()
		resps[resp]++
		resp = getCSS()
		resps[resp]++
	}
	return updateScore("GET", resps, m, finishTime)
}

func candidateScenario(m *sync.Mutex, finishTime time.Time) bool {
	resps := map[bool]int{}
	resp := true

	for i := 0; i < 4; i++ {
		resp = getCandidate()
		resps[resp]++
		resp = getCSS()
		resps[resp]++
	}
	return updateScore("GET", resps, m, finishTime)
}

func politicalPartyScenario(m *sync.Mutex, finishTime time.Time) bool {
	resps := map[bool]int{}
	resp := true

	for i := 0; i < 4; i++ {
		resp = getPoliticalParty()
		resps[resp]++
		resp = getCSS()
		resps[resp]++
	}
	return updateScore("GET", resps, m, finishTime)
}

// 以下、スコア計算用
func updateScore(method string, resps map[bool]int, m *sync.Mutex, finishTime time.Time) (isContinue bool) {
	m.Lock()
	defer m.Unlock()
	if method == "GET" {
		totalScore = totalScore + resps[true]*2
		totalScore = totalScore - resps[false]*100
	} else {
		totalScore = totalScore + resps[true]*1
	}
	totalResp[true] = totalResp[true] + resps[true]
	totalResp[false] = totalResp[false] + resps[false]
	if time.Now().After(finishTime) {
		isContinue = false
	} else {
		isContinue = true
	}
	return isContinue
}

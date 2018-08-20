package main

import (
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"database/sql"
	"html/template"
	"log"

	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
	"github.com/serinuntius/graqt"
)

var (
	db           *sql.DB
	rc           *redis.Client
	traceEnabled = os.Getenv("GRAQT_TRACE")
	driverName   = "mysql"
	candidates   []Candidate

	candidateMap   map[string]int
	candidateIdMap map[int]Candidate
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func NewRedisClient() error {
	rc = redis.NewClient(&redis.Options{
		Network:  "unix",
		Addr:     "/var/run/redis/redis-server.sock",
		Password: "",
		DB:       0,
	})

	_, err := rc.Ping().Result()
	if err != nil {
		return err
	}
	return nil
}

func main() {
	if traceEnabled == "1" {
		// driverNameは絶対にこれでお願いします。
		driverName = "mysql-tracer"
		graqt.SetRequestLogger("log/request.log")
		graqt.SetQueryLogger("log/query.log")
	}

	// database setting
	user := getEnv("ISHOCON2_DB_USER", "ishocon")
	pass := getEnv("ISHOCON2_DB_PASSWORD", "ishocon")
	dbname := getEnv("ISHOCON2_DB_NAME", "ishocon2")
	var err error
	db, err = sql.Open(driverName, user+":"+pass+"@unix(/var/run/mysqld/mysqld.sock)/"+dbname)
	if err != nil {
		log.Fatal(err)
	}

	if err := NewRedisClient(); err != nil {
		log.Fatal(err)
	}

	db.SetMaxIdleConns(20)
	db.SetMaxOpenConns(40)
	db.SetConnMaxLifetime(300 * time.Second)

	gin.SetMode(gin.DebugMode)
	//gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	pprof.Register(r)

	//r.Use(static.Serve("/css", static.LocalFile("public/css", true)))
	if traceEnabled == "1" {
		r.Use(graqt.RequestIdForGin())
	}

	r.FuncMap = template.FuncMap{"indexPlus1": func(i int) int { return i + 1 }}

	r.LoadHTMLGlob("templates/*.tmpl")

	// session store
	//store := sessions.NewCookieStore([]byte("mysession"))
	//store.Options(sessions.Options{HttpOnly: true})
	//r.Use(sessions.Sessions("showwin_happy", store))

	// template cache store
	store := persistence.NewInMemoryStore(time.Minute)

	// GET /
	r.GET("/", cache.CachePage(store, time.Minute, func(c *gin.Context) {
		// 1 ~ 10 の取得
		results, err := rc.ZRevRangeWithScores(kojinKey(), 0, 10).Result()
		if err != nil {
			log.Fatal(err)
		}

		// 最下位
		resultsWorst, err := rc.ZRangeWithScores(kojinKey(), 0, 0).Result()
		results = append(results, resultsWorst...)

		// 1 ~ 10と最下位の分枠を用意しておく
		candidateIDs := make([]int, len(results))
		votedCounts := make([]int, len(results))

		for i, r := range results {
			candidateVotedCountKey, votedCount := r.Member, r.Score
			if cKey, ok := candidateVotedCountKey.(string); ok {
				idx := strings.Index(cKey, ":")
				candidateID := cKey[idx+1:]
				candidateIDs[i], err = strconv.Atoi(candidateID)
				if err != nil {
					log.Fatal(err)
				}
				votedCounts[i] = int(votedCount)
			} else {
				log.Fatal(candidateVotedCountKey)
			}
		}

		sexRatio := map[string]int{
			"men":   0,
			"women": 0,
		}

		//partyElectionResults := [4]PartyElectionResult{}
		partyResultMap := make(map[string]int, 4)

		var cs []CandidateElectionResult

		for idx, cID := range candidateIDs {
			sex := candidateIdMap[cID].Sex
			partyName := candidateIdMap[cID].PoliticalParty

			if sex == "男" {
				sexRatio["men"] += votedCounts[idx]
			} else {
				sexRatio["women"] += votedCounts[idx]
			}

			cs = append(cs, CandidateElectionResult{
				ID:             cID,
				Name:           candidateIdMap[cID].Name,
				PoliticalParty: partyName,
				Sex:            sex,
				VotedCount:     votedCounts[idx],
			})
			partyResultMap[partyName] += votedCounts[idx]
		}

		partyResults := make([]PartyElectionResult, len(partyResultMap))
		idx := 0
		for name, count := range partyResultMap {
			r := PartyElectionResult{
				PoliticalParty: name,
				VoteCount:      count,
			}
			partyResults[idx] = r
			idx++
		}

		// 投票数でソート
		sort.Slice(partyResults, func(i, j int) bool { return partyResults[i].VoteCount > partyResults[j].VoteCount })

		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"candidates": cs,
			"parties":    partyResults,
			"sexRatio":   sexRatio,
		})
	}))

	// GET /candidates/:candidateID(int)
	r.GET("/candidates/:candidateID", cache.CachePage(store, time.Minute, func(c *gin.Context) {
		candidateID, _ := strconv.Atoi(c.Param("candidateID"))
		candidate, err := getCandidate(c, candidateID)
		if err != nil {
			c.Redirect(http.StatusFound, "/")
		}

		keywords := getVoiceOfSupporter(candidateID)
		votedCount, err := rc.Get(candidateVotedCountKey(candidateID)).Int64()
		if err != nil {
			log.Fatal(err)
		}

		c.HTML(http.StatusOK, "candidate.tmpl", gin.H{
			"candidate": candidate,
			"votes":     votedCount,
			"keywords":  keywords,
		})
	}))

	// GET /political_parties/:name(string)

	r.GET("/political_parties/:name", cache.CachePage(store, time.Minute, func(c *gin.Context) {
		partyName := c.Param("name")

		candidates := getCandidatesByPoliticalParty(c, partyName)
		var votes int

		for _, c := range candidates {
			votedCount, err := rc.Get(candidateVotedCountKey(c.ID)).Int64()
			if err != nil {
				log.Fatal(err)
			}

			votes += int(votedCount)
		}
		keywords := getVoiceOfSupporterByParties(partyName)

		c.HTML(http.StatusOK, "political_party.tmpl", gin.H{
			"politicalParty": partyName,
			"votes":          votes,
			"candidates":     candidates,
			"keywords":       keywords,
		})
	}))

	// GET /vote
	r.GET("/vote", func(c *gin.Context) {
		c.HTML(http.StatusOK, "vote.tmpl", gin.H{
			"candidates": candidates,
			"message":    "",
		})
	})

	// POST /vote
	r.POST("/vote", func(c *gin.Context) {
		if c.PostForm("candidate") == "" {
			voteError(c, "候補者を記入してください")
			return
		} else if c.PostForm("keyword") == "" {
			voteError(c, "投票理由を記入してください")
			return
		}

		candidateID, ok := candidateMap[c.PostForm("candidate")]
		if !ok {
			voteError(c, "候補者を正しく記入してください")
			return
		}

		user, userErr := getUser(c, c.PostForm("name"), c.PostForm("address"), c.PostForm("mynumber"))
		if userErr != nil {
			voteError(c, "個人情報に誤りがあります")
			return
		}

		voteCount, _ := strconv.Atoi(c.PostForm("vote_count"))

		if user.Votes < voteCount+user.VotedCount {
			voteError(c, "投票数が上限を超えています")
			return
		}

		createVote(c, user.ID, candidateID, c.PostForm("keyword"), voteCount)

		store.Flush()

		c.HTML(http.StatusOK, "vote.tmpl", gin.H{
			"candidates": candidates,
			"message":    "投票に成功しました",
		})
	})

	r.GET("/initialize", func(c *gin.Context) {
		db.Exec("DELETE FROM votes")

		rc.FlushAll()
		store.Flush()

		candidates = getAllCandidate(c)

		candidateMap = make(map[string]int, len(candidates))
		candidateIdMap = make(map[int]Candidate, len(candidates))
		for _, c := range candidates {
			candidateMap[c.Name] = c.ID
			candidateIdMap[c.ID] = c
		}

		c.String(http.StatusOK, "Finish")
	})

	r.RunUnix("/var/run/go/go.sock")
	//r.Run(":8080")
}

func voteError(c *gin.Context, msg string) {
	c.HTML(http.StatusOK, "vote.tmpl", gin.H{
		"message": msg,
	})
}

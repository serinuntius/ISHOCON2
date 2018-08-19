package main

import (
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"database/sql"
	"html/template"
	"log"

	"github.com/gin-gonic/contrib/sessions"
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
	partyNames   []string
	candidateMap map[string]int
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

	// redis connect
	if err := NewRedisClient(); err != nil {
		log.Fatal(err)
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

	db.SetMaxIdleConns(20)
	db.SetMaxOpenConns(40)
	db.SetConnMaxLifetime(300 * time.Second)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()

		_, err = db.Exec("UPDATE candidates SET voted_count = 0")
		if err != nil {
			log.Println(err)
		}
	}()

	go func() {
		defer wg.Done()
		// ALTER TABLE users DROP voted_count;
		// ALTER TABLE users ADD voted_count INTEGER default 0;

		_, err = db.Exec("UPDATE users SET voted_count = 0")
		if err != nil {
			log.Println(err)
		}
	}()

	wg.Wait()

	gin.SetMode(gin.DebugMode)
	//gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	//r.Use(static.Serve("/css", static.LocalFile("public/css", true)))
	if traceEnabled == "1" {
		r.Use(graqt.RequestIdForGin())
	}

	r.FuncMap = template.FuncMap{"indexPlus1": func(i int) int { return i + 1 }}

	r.LoadHTMLGlob("templates/*.tmpl")

	// session store
	store := sessions.NewCookieStore([]byte("mysession"))
	store.Options(sessions.Options{HttpOnly: true})
	r.Use(sessions.Sessions("showwin_happy", store))

	// GET /
	r.GET("/", func(c *gin.Context) {
		electionResults := getElectionResult(c)

		// 上位10人と最下位のみ表示
		tmp := make([]CandidateElectionResult, len(electionResults))
		copy(tmp, electionResults)
		cs := tmp[:10]
		cs = append(cs, tmp[len(tmp)-1])

		partyResultMap := make(map[string]int, len(cs))

		sexRatio := map[string]int{
			"men":   0,
			"women": 0,
		}

		for _, r := range electionResults {
			if r.Sex == "男" {
				sexRatio["men"] += r.VotedCount
			} else if r.Sex == "女" {
				sexRatio["women"] += r.VotedCount
			}
			partyResultMap[r.PoliticalParty] += r.VotedCount
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
	})

	// GET /candidates/:candidateID(int)
	r.GET("/candidates/:candidateID", func(c *gin.Context) {
		candidateID, _ := strconv.Atoi(c.Param("candidateID"))
		candidate, err := getCandidate(c, candidateID)
		if err != nil {
			c.Redirect(http.StatusFound, "/")
		}

		candidateIDs := []int{candidateID}
		keywords := getVoiceOfSupporter(candidateIDs)

		c.HTML(http.StatusOK, "candidate.tmpl", gin.H{
			"candidate": candidate,
			"votes":     candidate.VotedCount,
			"keywords":  keywords,
		})
	})

	// GET /political_parties/:name(string)
	r.GET("/political_parties/:name", func(c *gin.Context) {
		partyName := c.Param("name")

		candidates := getCandidatesByPoliticalParty(c, partyName)
		candidateIDs := []int{}
		var votes int

		for _, c := range candidates {
			candidateIDs = append(candidateIDs, c.ID)
			votes += c.VotedCount
		}
		keywords := getVoiceOfSupporter(candidateIDs)

		c.HTML(http.StatusOK, "political_party.tmpl", gin.H{
			"politicalParty": partyName,
			"votes":          votes,
			"candidates":     candidates,
			"keywords":       keywords,
		})
	})

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

		c.HTML(http.StatusOK, "vote.tmpl", gin.H{
			"candidates": candidates,
			"message":    "投票に成功しました",
		})
	})

	r.GET("/initialize", func(c *gin.Context) {
		db.Exec("DELETE FROM votes")
		rc.FlushAll()

		candidates = getAllCandidate(c)

		candidateMap = make(map[string]int, len(candidates))
		for _, c := range candidates {
			candidateMap[c.Name] = c.ID
		}

		partyNames = getAllPartyName(c)

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

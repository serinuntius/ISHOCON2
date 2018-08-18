package main

import (
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"

	"database/sql"
	"html/template"
	"log"

	"github.com/gin-gonic/contrib/sessions"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/serinuntius/graqt"
)

var (
	db           *sql.DB
	traceEnabled = os.Getenv("GRAQT_TRACE")
	driverName   = "mysql"
	candidates   []Candidate
	partyNames   []string
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
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
	db, err = sql.Open(driverName, user+":"+pass+"@/"+dbname)
	if err != nil {
		log.Fatal(err)
	}

	db.SetMaxIdleConns(5)

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
		keywords := getVoiceOfSupporter(c, candidateIDs)

		c.HTML(http.StatusOK, "candidate.tmpl", gin.H{
			"candidate": candidate,
			"votes":     candidate.VotedCount,
			"keywords":  keywords,
		})
	})

	// GET /political_parties/:name(string)
	r.GET("/political_parties/:name", func(c *gin.Context) {
		partyName := c.Param("name")
		var votes int
		electionResults := getElectionResult(c)
		for _, r := range electionResults {
			if r.PoliticalParty == partyName {
				votes += r.VotedCount
			}
		}

		candidates := getCandidatesByPoliticalParty(c, partyName)
		candidateIDs := []int{}
		for _, c := range candidates {
			candidateIDs = append(candidateIDs, c.ID)
		}
		keywords := getVoiceOfSupporter(c, candidateIDs)

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

		candidate, cndErr := getCandidateByName(c, c.PostForm("candidate"))
		if cndErr != nil {
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

		//for i := 1; i <= voteCount; i++ {
		createVote(c, user.ID, candidate.ID, c.PostForm("keyword"), voteCount)
		//}

		c.HTML(http.StatusOK, "vote.tmpl", gin.H{
			"candidates": candidates,
			"message":    "投票に成功しました",
		})
	})

	r.GET("/initialize", func(c *gin.Context) {
		db.Exec("DELETE FROM votes")

		if false {
			_, err := db.Exec("ALTER TABLE candidates ADD voted_count INTEGER default 0")
			if err != nil {
				log.Println(err)
			}

			_, err = db.Exec("ALTER TABLE users ADD voted_count INTEGER default 0")
			if err != nil {
				log.Println(err)
			}

			_, err = db.Exec("ALTER TABLE votes ADD voted_count INTEGER default 0")
			if err != nil {
				log.Println(err)
			}

			//ALTER TABLE votes change keyword  keyword varchar(191);
			_, err = db.Exec("ALTER TABLE votes change keyword keyword varchar(191)")
			if err != nil {
				log.Println(err)
			}

			// ALTER TABLE votes drop INDEX user_id

			_, err = db.Exec("ALTER TABLE votes ADD INDEX candidate_id_voted_count_idx(candidate_id,voted_count DESC)")
			if err != nil {
				log.Println(err)
			}

			_, err = db.Exec("ALTER TABLE votes ADD INDEX keyword_idx(keyword)")
			if err != nil {
				log.Println(err)
			}
		}

		candidates = getAllCandidate(c)
		partyNames = getAllPartyName(c)

		c.String(http.StatusOK, "Finish")
	})

	r.Run(":8080")
}

func voteError(c *gin.Context, msg string) {
	c.HTML(http.StatusOK, "vote.tmpl", gin.H{
		"message": msg,
	})
}

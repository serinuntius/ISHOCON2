package main

// 初期化(N秒以内)
import (
	"crypto/tls"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/http2"
)

func getInitialize() {
	postMessage("Start GET /initialize")
	finishTime := time.Now().Add(10 * time.Second)
	httpsRequest("GET", "/initialize", nil)
	if time.Now().Sub(finishTime) > 0 {
		postMessage("Timeover at GET /initialize")
		os.Exit(1)
	}
}

func postVote(v Vote) bool {
	prm := url.Values{}
	prm.Add("name", v.Name)
	prm.Add("address", v.Address)
	prm.Add("mynumber", v.Mynumber)
	prm.Add("candidate", v.Candidate)
	prm.Add("keyword", v.Keyword)
	prm.Add("vote_count", v.VoteCount)
	if httpsRequest("POST", "/vote", prm) == 200 {
		return true
	}
	return false
}

func getIndex() bool {
	if httpsRequest("GET", "/", nil) == 200 {
		return true
	}
	return false
}

func getCandidate() bool {
	id := strconv.Itoa(getRand(1, 30))
	if httpsRequest("GET", "/candidates/"+id, nil) == 200 {
		return true
	}
	return false
}

func getPoliticalParty() bool {
	set := []string{"国民元気党", "国民10人大活躍党", "夢実現党", "国民平和党"}
	party := set[getRand(0, 3)]
	if httpsRequest("GET", "/political_parties/"+party, nil) == 200 {
		return true
	}
	return false
}

func getCSS() bool {
	if httpsRequest("GET", "/css/bootstrap.min.css", nil) == 200 {
		return true
	}
	return false
}

var clients []http.Client

func createClients(size int) {
	clients = make([]http.Client, size)
	for i := 0; i < size; i++ {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		if err := http2.ConfigureTransport(tr); err != nil {
			log.Fatalf("Failed to configure h2 transport: %s", err)
		}
		clients[i] = http.Client{Transport: tr}
	}
	rand.Seed(time.Now().Unix())
}

func httpsRequest(method string, path string, params url.Values) int {
	req, _ := http.NewRequest(method, host+path, strings.NewReader(params.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := clients[rand.Intn(len(clients))]

	resp, err := client.Do(req)
	if err != nil {
		postMessage(method + " " + path + " へのリクエストが成功しませんでした。")
		return 500
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

func httpsRequestDoc(method string, path string, params url.Values) *goquery.Document {
	req, _ := http.NewRequest(method, host+path, strings.NewReader(params.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := clients[rand.Intn(len(clients))]

	resp, err := client.Do(req)
	if err != nil {
		postMessage(method + " " + path + " へのリクエストが成功しませんでした。")
		os.Exit(1)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		postMessage(method + " " + path + " へのリクエストが成功しませんでした。")
		os.Exit(1)
	}

	return doc
}

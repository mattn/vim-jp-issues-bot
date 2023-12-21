package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/garyburd/go-oauth/oauth"
	_ "github.com/lib/pq"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

const name = "vim-jp-issues-bot"

const version = "0.0.5"

var revision = "HEAD"

const (
	updateURL = "https://api.twitter.com/1.1/statuses/update.json"
	issuesURL = "https://api.github.com/repos/vim-jp/issues/issues?state=all"
)

var (
	oauthClient = oauth.Client{
		TemporaryCredentialRequestURI: "https://api.twitter.com/oauth/request_token",
		ResourceOwnerAuthorizationURI: "https://api.twitter.com/oauth/authenticate",
		TokenRequestURI:               "https://api.twitter.com/oauth/access_token",
	}
)

type GitHubIssue struct {
	bun.BaseModel `bun:"table:vim_jp_issue,alias:f"`

	ID        int       `bun:"id,pk,notnull" json:"id"`
	Number    int       `bun:"number,notnull" json:"number"`
	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
}

type Issue struct {
	URL           string `json:"url"`
	RepositoryURL string `json:"repository_url"`
	LabelsURL     string `json:"labels_url"`
	CommentsURL   string `json:"comments_url"`
	EventsURL     string `json:"events_url"`
	HtmlURL       string `json:"html_url"`
	ID            int    `json:"id"`
	Number        int    `json:"number"`
	Title         string `json:"title"`
	User          struct {
		Login             string `json:"login"`
		ID                int    `json:"id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HtmlURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"user"`
}

func postTweet(token *oauth.Credentials, status string) error {
	param := make(url.Values)
	param.Set("status", status)
	oauthClient.SignParam(token, "POST", updateURL, param)
	resp, err := http.PostForm(updateURL, url.Values(param))
	if err != nil {
		log.Println("failed to post tweet:", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Println("failed to post tweet")
		return err
	}
	return nil
}

func main() {
	var skip bool
	var dsn string
	var clientToken, clientSecret, accessToken, accessSecret string
	var ver bool

	flag.BoolVar(&skip, "skip", false, "Skip tweet")
	flag.StringVar(&dsn, "dsn", os.Getenv("VIM_JP_ISSUES_BOT_DSN"), "Database source")
	flag.StringVar(&clientToken, "client-token", os.Getenv("VIM_JP_ISSUES_BOT_CLIENT_TOKEN"), "Twitter ClientToken")
	flag.StringVar(&clientSecret, "client-secret", os.Getenv("VIM_JP_ISSUES_BOT_CLIENT_SECRET"), "Twitter ClientSecret")
	flag.StringVar(&accessToken, "access-token", os.Getenv("VIM_JP_ISSUES_BOT_ACCESS_TOKEN"), "Twitter AccessToken")
	flag.StringVar(&accessSecret, "access-secret", os.Getenv("VIM_JP_ISSUES_BOT_ACCESS_SECRET"), "Twitter AccessSecret")
	flag.BoolVar(&ver, "v", false, "show version")
	flag.Parse()

	if ver {
		fmt.Println(version)
		os.Exit(0)
	}

	oauthClient.Credentials.Token = clientToken
	oauthClient.Credentials.Secret = clientSecret
	token := &oauth.Credentials{
		Token:  accessToken,
		Secret: accessSecret,
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}

	bundb := bun.NewDB(db, pgdialect.New())
	defer bundb.Close()

	_, err = bundb.NewCreateTable().Model((*GitHubIssue)(nil)).IfNotExists().Exec(context.Background())
	if err != nil {
		log.Println(err)
		return
	}

	flag.Parse()
	var issues []Issue

	resp, err := http.Get(issuesURL)
	if err != nil {
		log.Fatal(err)
	}
	err = json.NewDecoder(resp.Body).Decode(&issues)
	if err != nil {
		log.Fatal(err)
	}

	for i, j := 0, len(issues)-1; i < j; i, j = i+1, j-1 {
		issues[i], issues[j] = issues[j], issues[i]
	}

	for _, issue := range issues {
		gi := GitHubIssue{
			ID:     issue.ID,
			Number: issue.Number,
		}
		_, err := bundb.NewInsert().Model(&gi).Exec(context.Background())
		if err != nil {
			if !strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
				log.Println(err)
			}
			continue
		}

		content := fmt.Sprintf("Issue %d: %s %s #vimeditor", issue.Number, issue.Title, issue.HtmlURL)
		runes := []rune(content)
		if len(runes) > 140 {
			content = fmt.Sprintf("Issue %d: %s %s #vimeditor", issue.Number, string(issue.Title[:len(issue.Title)-len(runes)+140]), issue.HtmlURL)
		}
		if skip {
			log.Printf("%q", content)
			continue
		}
		err = postTweet(token, content)
		if err != nil {
			log.Println(err)
			continue
		}
	}
}

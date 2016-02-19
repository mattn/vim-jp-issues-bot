package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/garyburd/go-oauth/oauth"
)

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
	dry        = flag.Bool("dry", false, "dry-run")
	silent        = flag.Bool("s", false, "no post")
	configFile = flag.String("c", "config.json", "path to config.json")
	issuesFile = flag.String("f", "issues.json", "path to issues.json")
)

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
	res, err := http.PostForm(updateURL, url.Values(param))
	if err != nil {
		log.Println("failed to post tweet:", err)
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Println("failed to post tweet:", err)
		return err
	}
	return nil
}

func main() {
	flag.Parse()
	var token oauth.Credentials
	var oldIssues, newIssues []Issue

	f, err := os.Open(*configFile)
	if err != nil {
		log.Fatal(err)
	}
	err = json.NewDecoder(f).Decode(&token)
	if err != nil {
		log.Fatal(err)
	}
	f.Close()

	f, err = os.Open(*issuesFile)
	if err != nil {
		log.Fatal(err)
	}
	err = json.NewDecoder(f).Decode(&oldIssues)
	if err != nil {
		log.Fatal(err)
	}
	f.Close()

	resp, err := http.Get(issuesURL)
	if err != nil {
		log.Fatal(err)
	}
	err = json.NewDecoder(resp.Body).Decode(&newIssues)
	if err != nil {
		log.Fatal(err)
	}

	for i, j := 0, len(newIssues)-1; i < j; i, j = i+1, j-1 {
		newIssues[i], newIssues[j] = newIssues[j], newIssues[i]
	}

	updated := 0
	for _, newIssue := range newIssues {
		exists := false
		for _, oldIssue := range oldIssues {
			if newIssue.ID == oldIssue.ID {
				exists = true
				break
			}
		}
		if !exists {
			oldIssues = append(oldIssues, newIssue)
			updated++

			status := fmt.Sprintf("Issue %d: %s %s #vimeditor", newIssue.Number, newIssue.Title, newIssue.HtmlURL)
			log.Println(status)
			if !*dry && !*silent {
				err = postTweet(&token, status)
				if err != nil {
					log.Println(err)
				}
			}
		}
	}

	if updated == 0 {
		return
	}

	b, err := json.MarshalIndent(oldIssues, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	if !*dry {
		ioutil.WriteFile(*issuesFile, b, 0644)
	}
}

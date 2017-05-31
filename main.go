package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Config contains the site configuration.
type Config struct {
	Categories []string `json:"Categories"`
}

// Ah: https://godoc.org/github.com/google/go-github/github
// Github API structs.
type Url struct {
	Url string `json:"url"`
}

type SHA struct {
	SHA string `json:"sha"`
}

type Tree struct {
	BaseTree string      `json:"base_tree,omitempty"`
	SHA      string      `json:"sha,omitempty"`
	Entries  []TreeEntry `json:"tree,omitempty"`
}

type TreeEntry struct {
	SHA     string `json:"sha,omitempty"`
	Path    string `json:"path,omitempty"`
	Mode    string `json:"mode,omitempty"`
	Type    string `json:"type,omitempty"`
	Size    string `json:"size,omitempty"`
	Content string `json:"content,omitempty"`
}

type GitObject struct {
	Url string `json:"url"`
	SHA string `json:"sha"`
}

type GithubRefResult struct {
	Object GitObject `json:"object"`
}

// Youtube
type YoutubeResult struct {
	NextPageToken string `json:"nextPageToken"`
	Items         []Item `json:"items"`
}

type Item struct {
	Snippet Snippet `json:"snippet"`
}

type Snippet struct {
	Title      string     `json:"title"`
	Position   int        `json:"position"`
	ResourceId ResourceId `json:"resourceId"`
}

type ResourceId struct {
	VideoId string `json:"videoId"`
}

type Video struct {
	Title    string `json:"title"`
	Position int    `json:"position"`
	Id       string `json:"id"`
}

// RW
type Playlist struct {
	Title      string `json:"title"`
	Id         string `json:"id"`
	DefaultImg string `json:"defaultImg"`
}

type OrgPlaylist struct {
	Title      string `json:"name"`
	Id         string `json:"playlist_id"`
	DefaultImg string `json:"thumbnail_url"`
}

var config Config
var videos []Video
var videoList []Item
var commitSHA string
var treeSHA string
var client github.Client
var trees Tree

// TODO: the handling here should be elsewhere
// make this do one thing - and bundle the trees to streamline.
func addToTree(path, content string) {
	tree := TreeEntry{}
	tree.Type = "blob"
	tree.Mode = "100644"
	tree.Content = content
	tree.Path = path
	trees.Entries = append(trees.Entries, tree)
}

func commitTrees(trees Tree) {
	treeSHA = createTree(trees)
	// New commit grab the sha
	commitSHA = createCommit(treeSHA)
	// Update refs
	updateRefs(commitSHA)
}

func createTree(trees Tree) string {
	treeJson, err := json.Marshal(trees)
	if err != nil {
		fmt.Printf("failed to marshal tree %s\n", err)
	}
	body := githubRequest("POST", "https://api.github.com/repos/rwapps/video_backups/git/trees", "201 Created", treeJson)
	treeResult := SHA{}
	if err := json.Unmarshal(body, &treeResult); err != nil {
		fmt.Printf("failed to decode resp.Body %s\n", err)
	}
	return treeResult.SHA
}

func createCommit(treeSHA string) string {
	payload := fmt.Sprintf("{ \"message\": \"updating playlists\", \"tree\": %q, \"parents\": [ %q ] }", treeSHA, commitSHA)
	body := githubRequest("POST", "https://api.github.com/repos/rwapps/video_backups/git/commits", "201 Created", []byte(payload))
	commitSHAs := SHA{}
	if err := json.Unmarshal(body, &commitSHAs); err != nil {
		fmt.Printf("failed to decode resp.Body %s\n", err)
	}
	return commitSHAs.SHA
}

func updateRefs(commitSHA string) {
	payload := fmt.Sprintf("{ \"sha\": %q }", commitSHA)
	body := githubRequest("PATCH", "https://api.github.com/repos/rwapps/video_backups/git/refs/heads/master", "200 OK", []byte(payload))
	updateResult := GithubRefResult{}
	if err := json.Unmarshal(body, &updateResult); err != nil {
		fmt.Printf("failed to decode resp.Body %s\n", err)
	}
}

func backupPlaylists(category string, playlists []Playlist) {
	for _, p := range playlists {
		videoList = videoList[:0]
		videos = videos[:0]
		videos = getVideos(p.Id, "")
		content, err := json.Marshal(videos)
		if err != nil {
			fmt.Printf("failed to marshal videos %s\n", err)
		}
		output := fmt.Sprintf("{ \"defaultImg\": %q, \"videos\": %s }", p.DefaultImg, content)
		// Sanitize filenames - stumbled on "Refugees/Migrants Emergency - Europe"
		if strings.Contains(p.Title, "/") {
			p.Title = strings.Replace(p.Title, "/", "-", -1)
		}
		path := fmt.Sprintf("%s/%s.json", category, p.Title)
		addToTree(path, output)
	}
}

func getVideos(playlistId, nextPageToken string) []Video {
	u, err := url.Parse("https://www.googleapis.com/youtube/v3/playlistItems")
	if err != nil {
		fmt.Println("couldn't parse api url")
	}
	q := u.Query()
	q.Set("part", "snippet")
	q.Set("maxResults", "50")
	q.Set("fields", "nextPageToken,items/snippet(position,title,resourceId/videoId)")
	q.Set("playlistId", playlistId)
	q.Set("key", os.Getenv("YOUTUBEAPIKEY"))
	q.Set("pageToken", nextPageToken)
	u.RawQuery = q.Encode()
	resp, err := http.Get(u.String())
	if err != nil {
		fmt.Printf("failed to get from gapis %s\n", err)
	}
	result := YoutubeResult{}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Printf("failed to decode resp.Body %s\n", err)
	}
	videoList = append(videoList, result.Items...)
	if result.NextPageToken != "" {
		nextPageToken = result.NextPageToken
		result.NextPageToken = ""
		getVideos(playlistId, nextPageToken)
	} else {
		for _, vid := range videoList {
			v := Video{}
			v.Title = vid.Snippet.Title
			v.Position = vid.Snippet.Position
			v.Id = vid.Snippet.ResourceId.VideoId
			videos = append(videos, v)
		}
	}
	return videos
}

func githubRequest(verb, u, status string, input []byte) []byte {
	req, err := http.NewRequest(verb, u, bytes.NewBuffer(input))
	if err != nil {
		log.Fatal("Cannot make request for trees.")
	}
	req.Header.Set("Authorization", "token "+os.Getenv("GITHUBTOKEN"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Cannot get trees from github.")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("failed to readall body")
	}

	if resp.Status != status {
		panic(fmt.Sprintf("Failed status test, error body:\n %s\n", body))
	}
	return body
}

func getRwPlaylists(category string) []byte {
	u := fmt.Sprintf("http://reliefweb.int/sites/reliefweb.int/files/playlists/%s.json", category)
	resp, err := http.Get(u)
	if err != nil {
		fmt.Println("failed to get url")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("failed to readall body")
	}
	return body
}

func preparePlaylists(category string, rwPlaylists []byte) []Playlist {
	var playlists []Playlist
	if category == "organization" {
		var orgPlaylists map[string]OrgPlaylist
		err := json.Unmarshal(rwPlaylists, &orgPlaylists)
		if err != nil {
			fmt.Printf("failed to unmarshal playlists %v\n", rwPlaylists)
		}
		for _, p := range orgPlaylists {
			playlist := Playlist{}
			playlist.Title = p.Title
			playlist.Id = p.Id
			playlist.DefaultImg = p.DefaultImg
			playlists = append(playlists, playlist)
		}
	} else {
		err := json.Unmarshal(rwPlaylists, &playlists)
		if err != nil {
			fmt.Printf("failed to unmarshal playlists %v\n", rwPlaylists)
		}
	}
	return playlists
}

// init read the configuration file and initialize github SHAs
func init() {
	ctx := context.TODO()
	data, err := ioutil.ReadFile("/go/src/github.com/rwapps/video_gists/config/config.json")
	if err != nil {
		log.Fatal("Cannot read configuration file.")
	}
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Fatal("Invalid configuration file.")
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUBTOKEN")},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)
	ref, _, err := client.Git.GetRef(ctx, "rwapps", "video_backups", "heads/master")
	if err != nil {
		log.Fatal("git getref error")
	}
	commitSHA = *ref.Object.SHA
	repoCommit, _, err := client.Repositories.GetCommit(ctx, "rwapps", "video_backups", commitSHA)
	if err != nil {
		log.Fatal("git getcommit error")
	}
	treeSHA = *repoCommit.Commit.Tree.SHA
	trees.BaseTree = treeSHA
}

func main() {
	for _, category := range config.Categories {
		fmt.Printf("category %v\n", category)
		rwPlaylists := getRwPlaylists(category)
		path := fmt.Sprintf("%s/playlist.json", category)
		addToTree(path, string(rwPlaylists))

		playlists := preparePlaylists(category, rwPlaylists)
		backupPlaylists(category, playlists)
	}
	commitTrees(trees)
}

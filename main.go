// TODO: create new gists for rwapps (log in). Add the gist ids, get it working. The app can use those.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
)

// Theme: https://gist.github.com/rwapps/41c8a9f38f177fcee358c4758f3f87ea

// Config contains the site configuration.
type Config struct {
	//GistToken string `json:"GistToken"`
	YoutubeApiKey string `json:"YoutubeApiKey"`
	Lists         []List `json:"Lists"`
	GithubToken   string `json:"GithubToken"`
  Categories    []string `json:"Categories"`
}

type List struct {
	Name   string `json:"name"`
	GistId string `json:"gistId"`
}

var config Config

type Playlist struct {
	Title      string `json:"title"`
	Id         string `json:"id"`
	DefaultImg string `json:"defaultImg"`
}

// {
//   "sha": "afb11d8b3cfaa0806bb2237e17fd3604de327e88",
//   "url": "https://api.github.com/repos/rwapps/video_backups/git/trees/afb11d8b3cfaa0806bb2237e17fd3604de327e88",
//   "tree": [
//     {
//       "path": "country",
//       "mode": "040000",
//       "type": "tree",
//       "sha": "e937c29ac5c10aea13bbea6e20ea06483ad1e25f",
//       "url": "https://api.github.com/repos/rwapps/video_backups/git/trees/e937c29ac5c10aea13bbea6e20ea06483ad1e25f"
//     },
//     {
//       "path": "organization",
//       "mode": "040000",
//       "type": "tree",
//       "sha": "a6bdbb4b44b125533e131ed1b367a7e8d2842ad2",
//       "url": "https://api.github.com/repos/rwapps/video_backups/git/trees/a6bdbb4b44b125533e131ed1b367a7e8d2842ad2"
//     },
//     {
//       "path": "topic",
//       "mode": "040000",
//       "type": "tree",
//       "sha": "f06d533e6dceb916bb6f342f734552b3d5d65aa9",
//       "url": "https://api.github.com/repos/rwapps/video_backups/git/trees/f06d533e6dceb916bb6f342f734552b3d5d65aa9"
//     }
//   ],
//   "truncated": false
// }
type CommitTree struct {
	BaseTree string `json:"base_tree"`
	Trees    []Tree `json:"tree"`
}

type Tree struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	Sha  string `json:"sha"`
}

type Url struct {
	Url string `json:"url"`
}

type ResultTree struct {
	Path string `json:"path"`
	Sha  string `json:"sha"`
}

type Sha struct {
	Sha string `json:"sha"`
}

type GithubTreesResult struct {
	Sha   string       `json:"sha"`
	Trees []ResultTree `json:"tree"`
}

type GithubCommitResult struct {
	Tree    Url   `json:"tree"`
	Parents []Sha `json:"parents"`
}

type GithubRefResult struct {
	Object Url `json:"object"`
}

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

var videos []Video

var videoList []Item

type OrgPlaylist struct {
	Title      string `json:"name"`
	Id         string `json:"playlist_id"`
	DefaultImg string `json:"thumbnail_url"`
}

var currentSha string

var parentCommit string


func backupOrgPlaylists(category string, body []byte) {

	var playlists map[string]OrgPlaylist
	err := json.Unmarshal(body, &playlists)
	if err != nil {
		fmt.Printf("failed to unmarshal body %v\n", body)
	}
	for _, p := range playlists {
		videoList = videoList[:0]
		videos = videos[:0]
		videos = getVideos(p.Id, "")
		content, err := json.Marshal(videos)
		if err != nil {
			fmt.Printf("failed to marshal videos %s\n", err)
		}
		output := fmt.Sprintf("{ \"defaultImg\": %s, \"videos\": %s }", p.DefaultImg, content)
		// Sanitize filenames - stumbled on "Refugees/Migrants Emergency - Europe"
		if strings.Contains(p.Title, "/") {
			p.Title = strings.Replace(p.Title, "/", "-", -1)
		}
		err = ioutil.WriteFile(fmt.Sprintf("data/%s/%s.json", category, p.Title), []byte(output), 0644)
		if err != nil {
			fmt.Printf("failed to write file %s\n", err)
		}
	}
}

func createBlob(content string) string {
		// create a blob, grab the sha
		payload := fmt.Sprintf("{ \"content\": %q, \"encoding\": \"utf-8\" }", content)
		req, err := http.NewRequest("POST", "https://api.github.com/repos/rwapps/video_backups/git/blobs", bytes.NewBuffer([]byte(payload)))

		if err != nil {
			log.Fatal("Cannot make post for blob.")
		}
		req.Header.Set("Authorization", "token "+config.GithubToken)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Fatal("Cannot post blob.")
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("failed to readall body")
		}

		if resp.Status == "201 Created" {
			fmt.Println("Success")
		} else {
			// TODO: fail here.
			fmt.Printf("Failed posting blob, error body\n %s\n", body)
		}

		blobResult := Sha{}
		if err := json.Unmarshal(body, &blobResult); err != nil {
			fmt.Printf("failed to decode resp.Body %s\n", err)
		}

		return blobResult.Sha

}

func backupPlaylists(category string, body []byte) {
  // TODO - do this for organizations too

  tree := CommitTree{}
  tree.BaseTree = currentSha

  blobTree := Tree{}
  blobTree.Sha = createBlob(string(body))
  blobTree.Path = fmt.Sprintf("%s/playlist.json", category)
  blobTree.Type = "blob"
  blobTree.Mode = "100644"
  tree.Trees = append(tree.Trees, blobTree)

	var playlists []Playlist
	err := json.Unmarshal(body, &playlists)
	if err != nil {
		fmt.Printf("failed to unmarshal body %v\n", body)
	}
	for _, p := range playlists {
		blobTree = Tree{}
		videoList = videoList[:0]
		videos = videos[:0]
		videos = getVideos(p.Id, "")
		content, err := json.Marshal(videos)
		if err != nil {
			fmt.Printf("failed to marshal videos %s\n", err)
		}
		output := fmt.Sprintf("{ \"defaultImg\": %s, \"videos\": %s }", p.DefaultImg, content)
		// Sanitize filenames - stumbled on "Refugees/Migrants Emergency - Europe"
		if strings.Contains(p.Title, "/") {
			p.Title = strings.Replace(p.Title, "/", "-", -1)
		}

		blobTree.Sha = createBlob(output)
		blobTree.Path = fmt.Sprintf("%s/%s.json", category, p.Title)
		blobTree.Type = "blob"
		blobTree.Mode = "100644"

		tree.Trees = append(tree.Trees, blobTree)

		//err = ioutil.WriteFile(fmt.Sprintf("data/%s/%s.json", category, p.Title), []byte(output), 0644)
		//if err != nil {
		//	fmt.Printf("failed to write file %s\n", err)
		//}
	}
	// create a tree, grab the sha
	treeJson, err := json.Marshal(tree)
	if err != nil {
		fmt.Printf("failed to marshal tree %s\n", err)
	}
	req, err := http.NewRequest("POST", "https://api.github.com/repos/rwapps/video_backups/git/trees", bytes.NewBuffer(treeJson))
	if err != nil {
		log.Fatal("Cannot make post for tree.")
	}
	req.Header.Set("Authorization", "token "+config.GithubToken)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Cannot create tree.")
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("failed to readall body")
	}
	if resp.Status == "201 Created" {
		fmt.Println("Success")
	} else {
		// TODO: fail here.
		fmt.Printf("Failed creating tree, error body\n %s\n", body)
	}

	treeResult := Sha{}
	if err := json.Unmarshal(body, &treeResult); err != nil {
		fmt.Printf("failed to decode resp.Body %s\n", err)
	}

	// New commit
	fmt.Println(treeResult.Sha)
	payload := fmt.Sprintf("{ \"message\": \"updating %s\", \"tree\": %q, \"parents\": [ %q ] }", category, treeResult.Sha, parentCommit)
	req, err = http.NewRequest("POST", "https://api.github.com/repos/rwapps/video_backups/git/commits", bytes.NewBuffer([]byte(payload)))

	if err != nil {
		log.Fatal("Cannot make post for commit.")
	}
	req.Header.Set("Authorization", "token "+config.GithubToken)

	client = &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		log.Fatal("Cannot post commit.")
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("failed to readall body")
	}

	if resp.Status == "201 Created" {
		fmt.Println("Success")
	} else {
		// TODO: fail here.
		fmt.Printf("Failed posting commit, error body\n %s\n", body)
	}

	commitResult := Sha{}
	if err := json.Unmarshal(body, &commitResult); err != nil {
		fmt.Printf("failed to decode resp.Body %s\n", err)
	}

	fmt.Println(commitResult.Sha)

	// Update head
	payload = fmt.Sprintf("{ \"sha\": %q }", commitResult.Sha)
  body = githubRequest("PATCH",  "https://api.github.com/repos/rwapps/video_backups/git/refs/heads/master", "200 OK", []byte(payload))
	// req, err = http.NewRequest("PATCH", "https://api.github.com/repos/rwapps/video_backups/git/refs/heads/master", bytes.NewBuffer([]byte(payload)))

	// if err != nil {
	// 	log.Fatal("Cannot make patch for update.")
	// }
	// req.Header.Set("Authorization", "token "+config.GithubToken)

	// client = &http.Client{}
	// resp, err = client.Do(req)
	// if err != nil {
	// 	log.Fatal("Cannot post commit.")
	// }
	// defer resp.Body.Close()
	// body, err = ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	fmt.Println("failed to readall body")
	// }

	// if resp.Status == "200 OK" {
	// 	fmt.Println("Success")
	// } else {
	// 	fmt.Printf("Failed patching update head, error body\n %s\n", body)
	// }

	updateResult := GithubRefResult{}
	if err := json.Unmarshal(body, &updateResult); err != nil {
		fmt.Printf("failed to decode resp.Body %s\n", err)
	}

	fmt.Printf("updateResult %v\n", updateResult)
}

func getVideos(playlistId, nextPageToken string) []Video {

	fmt.Printf("playlist id %s\n", playlistId)
	u, err := url.Parse("https://www.googleapis.com/youtube/v3/playlistItems")
	if err != nil {
		fmt.Println("couldn't parse api url")
	}
	q := u.Query()
	q.Set("part", "snippet")
	q.Set("maxResults", "50")
	q.Set("fields", "nextPageToken,items/snippet(position,title,resourceId/videoId)")
	q.Set("playlistId", playlistId)
	q.Set("key", config.YoutubeApiKey)
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

func getCommitUrl() string {
  // Get the commit url
  url := "https://api.github.com/repos/rwapps/video_backups/git/refs/heads/master"
  body := githubRequest("GET", url, "200 OK", nil)
  //req, err := http.NewRequest("GET", url, nil)
  //if err != nil {
  //  log.Fatal("Cannot make request for current references.")
  //}
  //req.Header.Set("Authorization", "token "+config.GithubToken)

  //client := &http.Client{}
  //resp, err := client.Do(req)
  //if err != nil {
  //  log.Fatal("Cannot get current references from github.")
  //}
  //defer resp.Body.Close()
  //body, err := ioutil.ReadAll(resp.Body)
  //if err != nil {
  //  fmt.Println("failed to readall body")
  //}

  //if resp.Status != "200 OK" {
  //  panic(fmt.Sprintf("Failed getting commit references, error body\n %s\n", body))
  //}

  refResult := GithubRefResult{}
  if err := json.Unmarshal(body, &refResult); err != nil {
    fmt.Printf("failed to decode resp.Body %s\n", err)
  }

  return refResult.Object.Url
}

func getTreeUrl(commitUrl string) string {
  body := githubRequest("GET", commitUrl, "200 OK", nil)
  //req, err := http.NewRequest("GET", commitUrl, nil)
  //if err != nil {
  //  log.Fatal("Cannot make request for tree.")
  //}
  //req.Header.Set("Authorization", "token "+config.GithubToken)

  //client := &http.Client{}
  //resp, err := client.Do(req)
  //if err != nil {
  //  log.Fatal("Cannot get current references from github.")
  //}
  //defer resp.Body.Close()
  //body, err := ioutil.ReadAll(resp.Body)
  //if err != nil {
  //  fmt.Println("failed to readall body")
  //}

  //if resp.Status != "200 OK" {
  //  panic(fmt.Sprintf("Failed getting tree url, error body\n %s\n", body))
  //}

  commitResult := GithubCommitResult{}
  if err := json.Unmarshal(body, &commitResult); err != nil {
    fmt.Printf("failed to decode resp.Body %s\n", err)
  }
  parentCommit = commitResult.Parents[0].Sha

  return commitResult.Tree.Url
}

func githubRequest(verb, url, status string, input []byte) []byte {
  req, err := http.NewRequest(verb, url, bytes.NewBuffer(input))
  if err != nil {
    log.Fatal("Cannot make request for trees.")
  }
  req.Header.Set("Authorization", "token "+config.GithubToken)

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
    panic(fmt.Sprintf("Failed getting trees, error body\n %s\n", body))
  }

  return body
}

func setCurrentTree(treeUrl, category string) bool {
  body := githubRequest("GET", treeUrl, "200 OK", nil)
  treesResult := GithubTreesResult{}
  if err := json.Unmarshal(body, &treesResult); err != nil {
    fmt.Printf("failed to decode resp.Body %s\n", err)
  }
  for _, tree := range treesResult.Trees {
    if tree.Path == category {
      currentSha = tree.Sha

      return true
    }
  }

  return false
}

func getRwPlaylists(category string) []byte {
  urlpath := fmt.Sprintf("http://reliefweb.int/sites/reliefweb.int/files/playlists/%s.json", category)
  resp, err := http.Get(urlpath)
  if err != nil {
    fmt.Println("failed to get url")
  }
  defer resp.Body.Close()
  body, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    fmt.Println("failed to readall body")
  }

  return body
  //err = ioutil.WriteFile(fmt.Sprintf("data/%s/playlists.json", category), body, 0644)
  //if err != nil {
  //  fmt.Printf("failed to write file %s\n", err)
  //}
}

// init read the configuration file and creates the trello client.
func init() {
	// Read configuration.
	data, err := ioutil.ReadFile("./config/config.json")
	if err != nil {
		log.Fatal("Cannot read configuration file.")
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Fatal("Invalid configuration file.")
	}
}

func main() {
	for _, category := range config.Categories {
		fmt.Printf("category %v\n", category)

    commitUrl := getCommitUrl()

    treeUrl := getTreeUrl(commitUrl)

    success := setCurrentTree(treeUrl, category)
    if !success {
      fmt.Println("failed getting current tree")
    }

    rwPlaylists := getRwPlaylists(category)

		if category == "organization" {
			continue
			//backupOrgPlaylists(category, body)
		} else {
			backupPlaylists(category, rwPlaylists)
		}
		//go backupPlaylists(category, body)

		//updateGist("theme", "e0214aefcad4c93249dd026204723e78", body)

	}
	i := true
	if i == true {
		return
	}
	commitChanges()
}

//func updateGist(name string, gistId string, content []byte) {
//  //token := config.GistToken
//	//token := "1a15aa7a23f899c4b23f4375a0c9ce908cb788f0"
//	url := "https://api.github.com/gists/" + gistId
//
//	payload := fmt.Sprintf("{ \"files\": { \"%s.json\": { \"content\": %q } } }", name, content)
//
//	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer([]byte(payload)))
//	if err != nil {
//		panic(err)
//	}
//	req.Header.Set("Authorization", "token "+token)
//	req.Header.Set("Content-Type", "application/json")
//	req.Header.Set("User-Agent", "lazysoundsystem")
//
//	client := &http.Client{}
//	resp, err := client.Do(req)
//	if err != nil {
//		panic(err)
//	}
//	defer resp.Body.Close()
//	body, err := ioutil.ReadAll(resp.Body)
//	if err != nil {
//		fmt.Println("failed to readall body")
//	}
//
//	if resp.Status == "200 OK" {
//		fmt.Println("Success")
//	} else {
//		fmt.Printf("Failed updating gist, error body\n %s\n", body)
//	}
//}

func commitChanges() {

	// Do we need to save it first? Can we push as we create it?
	// At least, keep a track with write-file, logging file paths to a map.
	// Then go through that map, pushing.

	// 2. Get commit HEAD points to:
	// As above  "sha": "4b01080926406be85b827deaed1a75a4daf3049a",
	// For tree:
	//     "sha": "afb11d8b3cfaa0806bb2237e17fd3604de327e88",
	//     "url": "https://api.github.com/repos/rwapps/video_backups/git/trees/afb11d8b3cfaa0806bb2237e17fd3604de327e88"

	// FROM:
	// curl -H "Authorization: token f3c47780ec3f1f6cb78aeabee2f37945d40fdc7c" https://api.github.com/repos/rwapps/video_backups/git/commits/4b01080926406be85b827deaed1a75a4daf3049a
	// {
	//   "sha": "4b01080926406be85b827deaed1a75a4daf3049a",
	//   "url": "https://api.github.com/repos/rwapps/video_backups/git/commits/4b01080926406be85b827deaed1a75a4daf3049a",
	//   "html_url": "https://github.com/rwapps/video_backups/commit/4b01080926406be85b827deaed1a75a4daf3049a",
	//   "author": {
	//     "name": "Andy Footner",
	//     "email": "andyfootner@netscape.net",
	//     "date": "2016-11-04T14:14:16Z"
	//   },
	//   "committer": {
	//     "name": "Andy Footner",
	//     "email": "andyfootner@netscape.net",
	//     "date": "2016-11-04T14:14:16Z"
	//   },
	//   "tree": {
	//     "sha": "afb11d8b3cfaa0806bb2237e17fd3604de327e88",
	//     "url": "https://api.github.com/repos/rwapps/video_backups/git/trees/afb11d8b3cfaa0806bb2237e17fd3604de327e88"
	//   },
	//   "message": "remove unnecessary gitignore",
	//   "parents": [
	//     {
	//       "sha": "65fc43acfec725e5e27488622f1b00a33ecd36c8",
	//       "url": "https://api.github.com/repos/rwapps/video_backups/git/commits/65fc43acfec725e5e27488622f1b00a33ecd36c8",
	//       "html_url": "https://github.com/rwapps/video_backups/commit/65fc43acfec725e5e27488622f1b00a33ecd36c8"
	//     }
	//   ]
	// }

	// curl -H "Authorization: token f3c47780ec3f1f6cb78aeabee2f37945d40fdc7c" https://api.github.com/repos/rwapps/video_backups/git/trees/afb11d8b3cfaa0806bb2237e17fd3604de327e88
	// {
	//   "sha": "afb11d8b3cfaa0806bb2237e17fd3604de327e88",
	//   "url": "https://api.github.com/repos/rwapps/video_backups/git/trees/afb11d8b3cfaa0806bb2237e17fd3604de327e88",
	//   "tree": [
	//     {
	//       "path": "country",
	//       "mode": "040000",
	//       "type": "tree",
	//       "sha": "e937c29ac5c10aea13bbea6e20ea06483ad1e25f",
	//       "url": "https://api.github.com/repos/rwapps/video_backups/git/trees/e937c29ac5c10aea13bbea6e20ea06483ad1e25f"
	//     },
	//     {
	//       "path": "organization",
	//       "mode": "040000",
	//       "type": "tree",
	//       "sha": "a6bdbb4b44b125533e131ed1b367a7e8d2842ad2",
	//       "url": "https://api.github.com/repos/rwapps/video_backups/git/trees/a6bdbb4b44b125533e131ed1b367a7e8d2842ad2"
	//     },
	//     {
	//       "path": "topic",
	//       "mode": "040000",
	//       "type": "tree",
	//       "sha": "f06d533e6dceb916bb6f342f734552b3d5d65aa9",
	//       "url": "https://api.github.com/repos/rwapps/video_backups/git/trees/f06d533e6dceb916bb6f342f734552b3d5d65aa9"
	//     }
	//   ],
	//   "truncated": false
	// }

	//curl -H "Authorization: token f3c47780ec3f1f6cb78aeabee2f37945d40fdc7c" https://api.github.com/repos/rwapps/video_backups/git/trees/e937c29ac5c10aea13bbea6e20ea06483ad1e25f

	// Accept: application/vnd.github.v3+json
	// https://api.github.com
	//	url := "https://api.github.com/gists/" + gistId
	//
	//	payload := fmt.Sprintf("{ \"files\": { \"%s.json\": { \"content\": %q } } }", name, content)
	//
	//	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer([]byte(payload)))
	//	if err != nil {
	//		panic(err)
	//	}
	//	req.Header.Set("Authorization", "token "+token)
	//	req.Header.Set("Content-Type", "application/json")
	//	req.Header.Set("User-Agent", "lazysoundsystem")
	//
	//	client := &http.Client{}
	//	resp, err := client.Do(req)
	//	if err != nil {
	//		panic(err)
	//	}
	//	defer resp.Body.Close()
	//	body, err := ioutil.ReadAll(resp.Body)
	//	if err != nil {
	//		fmt.Println("failed to readall body")
	//	}
	//
	//	if resp.Status == "200 OK" {
	//		fmt.Println("Success")
	//	} else {
	//		fmt.Printf("Failed updating gist, error body\n %s\n", body)
	//	}

	i := true
	if i == true {
		return
	}
	// Add changes to repo, commit and push.
	cmd := exec.Command("/bin/sh", "-c", "cd data && git add . && git commit -m 'automatic update' && git push origin master")
	err := cmd.Run()
	if err != nil {
		fmt.Printf("failed to run commands %s\n", err)
	}
}

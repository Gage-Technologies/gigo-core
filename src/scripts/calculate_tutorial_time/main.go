package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gage-technologies/gigo-lib/config"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"gopkg.in/yaml.v2"
)

const (
	timeCostPerLargeWord  = 450 * time.Millisecond
	timeCostPerMediumWord = 300 * time.Millisecond
	timeCostPerSmallWord  = 100 * time.Millisecond
)

var (
	tutorialDetectionRegex = regexp.MustCompile("\\.gigo\\/\\.tutorials\\/tutorial-\\d+.md")
)

type Config struct {
	TitaniumConfig config.TitaniumConfig `yaml:"tidb"`
	GiteaConfig    config.GiteaConfig    `yaml:"gitea"`
}

func isBinary(data []byte) bool {
	for _, b := range data {
		if b > 0x7f {
			return true
		}
	}
	return false
}

func updatePost(db *ti.Database, vcsClient *git.VCSClient, author int64, id int64) {
	// retrieve the list of tutorials in the repo's tutorial directory
	files, _, err := vcsClient.GiteaClient.ListContents(
		fmt.Sprintf("%d", author),
		fmt.Sprintf("%d", id),
		"main",
		".gigo/.tutorials",
	)
	if err != nil {
		fmt.Printf("failed to get contents of tutorial dir: %v\n", err)
		return
	}

	// create a duration to hold the time estimates
	duration := time.Duration(0)

	// loop through each file, determine if it is a valid tutorial file, and count the time estimates
	for _, f := range files {
		// check against the tutorial detection regex
		if !tutorialDetectionRegex.MatchString(f.Path) {
			continue
		}

		// retrieve the contents of the file
		file, _, err := vcsClient.GiteaClient.GetContents(
			fmt.Sprintf("%d", author),
			fmt.Sprintf("%d", id),
			"main",
			f.Path,
		)
		if err != nil {
			fmt.Printf("failed to get contents of tutorial file: %v\n", err)
			continue
		}

		// ensure the contents are valid UTF-8
		valid := utf8.ValidString(*file.Content)
		if !valid {
			continue
		}

		// ensure that the file size does not exceed 1MB
		if file.Size > 1024*1024 {
			continue
		}

		// decode the raw text from base64
		rawDecodedText, err := base64.StdEncoding.DecodeString(*file.Content)
		if err != nil {
			fmt.Printf("failed to decode contents: %v\n", err)
			continue
		}

		// check if the decoded text is binary
		if isBinary(rawDecodedText) {
			return
		}

		// count the words in the file
		words := strings.FieldsFunc(string(rawDecodedText), func(r rune) bool {
			switch r {
			// whitespace
			case ' ', '\t', '\n':
				return true
			// code characters
			case '<', '>', '{', '}', '(', ')', '[', ']':
				return true
			// punctuation
			case '.', ',', ';', '!', '?', '"', '-', ':':
				return true
			// special characters
			case '/', '\\', '_', '#', '@', '%', '^', '&', '=', '+', '$', '~', '`', '|', '*':
				return true
			default:
				return false
			}
		})

		// multiply the word count with the average time cost per word
		for _, word := range words {
			if len(word) < 3 {
				duration += timeCostPerSmallWord
				continue
			}
			if len(word) < 12 {
				duration += timeCostPerMediumWord
				continue
			}
			duration += timeCostPerLargeWord
		}
	}

	// update the estimated time on the database
	_, err = db.DB.Exec(
		"update post set estimated_tutorial_time = ? where _id = ?",
		duration, id,
	)
	if err != nil {
		fmt.Printf("failed to update estimated time for project: %v\n", err)
		return
	}
}

func main() {
	configPath := flag.String("c", "config.yml", "Path to the configuration file")
	flag.Parse()

	cfgBuf, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatalf("failed to read config file: %v", err)
	}

	var cfg Config
	err = yaml.Unmarshal(cfgBuf, &cfg)
	if err != nil {
		log.Fatalf("failed to parse config file: %v", err)
	}

	db, err := ti.CreateDatabase(cfg.TitaniumConfig.TitaniumHost, cfg.TitaniumConfig.TitaniumPort, "mysql", cfg.TitaniumConfig.TitaniumUser,
		cfg.TitaniumConfig.TitaniumPassword, cfg.TitaniumConfig.TitaniumName)
	if err != nil {
		log.Fatal("failed to create titanium database: ", err)
	}

	vcsClient, err := git.CreateVCSClient(cfg.GiteaConfig.HostUrl, cfg.GiteaConfig.Username, cfg.GiteaConfig.Password, false)
	if err != nil {
		log.Fatal("failed to create vsc client: ", err)
	}

	// retrieve all of the post authors and ids
	res, err := db.DB.Query("select _id, author_id from post")
	if err != nil {
		log.Fatal("failed to query post table: ", err)
	}

	posts := make([]models.Post, 0)
	for res.Next() {
		var id int64
		var author int64
		err = res.Scan(&id, &author)
		if err != nil {
			log.Fatal("failed to scan row: ", err)
			continue
		}

		posts = append(posts, models.Post{AuthorID: author, ID: id})
	}

	// close explicitly
	res.Close()

	// calculate the tutorial time for each post
	for i := 0; i < len(posts); i++ {
		fmt.Printf("%d/%d: %d/%d\n", i+1, len(posts), posts[i].AuthorID, posts[i].ID)
		updatePost(db, vcsClient, posts[i].AuthorID, posts[i].ID)
	}

	fmt.Println("done.")
}

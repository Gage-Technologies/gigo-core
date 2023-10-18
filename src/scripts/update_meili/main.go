package main

import (
	"flag"
	"fmt"
	"github.com/gage-technologies/gigo-lib/config"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/search"
	"gopkg.in/yaml.v3"
	"log"
	"os"
)

type Config struct {
	TitaniumConfig config.TitaniumConfig `yaml:"tidb"`
	MeiliConfig    config.MeiliConfig    `yaml:"meili"`
}

func indexPosts(db *ti.Database, meili *search.MeiliSearchEngine) {
	res, err := db.DB.Query("select * from post")
	if err != nil {
		log.Fatal(err)
	}

	buf := make([]interface{}, 0)
	for res.Next() {
		post, err := models.PostFromSQLNative(db, res)
		if err != nil {
			log.Fatal(err)
		}

		buf = append(buf, post)

		if len(buf) >= 1000 {
			err = meili.AddDocuments("posts", buf...)
			if err != nil {
				log.Fatal(err)
			}

			buf = make([]interface{}, 0)
		}
	}

	if len(buf) > 0 {
		err = meili.AddDocuments("posts", buf...)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func indexComments(db *ti.Database, meili *search.MeiliSearchEngine) {
	res, err := db.DB.Query("select * from comment")
	if err != nil {
		log.Fatal(err)
	}

	buf := make([]interface{}, 0)
	for res.Next() {
		comment, err := models.CommentFromSQLNative(db, res)
		if err != nil {
			log.Fatal(err)
		}

		buf = append(buf, comment)

		if len(buf) >= 1000 {
			err = meili.AddDocuments("comment", buf...)
			if err != nil {
				log.Fatal(err)
			}

			buf = make([]interface{}, 0)
		}
	}

	if len(buf) > 0 {
		err = meili.AddDocuments("comment", buf...)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func indexUsers(db *ti.Database, meili *search.MeiliSearchEngine) {
	res, err := db.DB.Query("select * from users")
	if err != nil {
		log.Fatal(err)
	}

	buf := make([]interface{}, 0)
	for res.Next() {
		user, err := models.UserFromSQLNative(db, res)
		if err != nil {
			log.Fatal(err)
		}

		buf = append(buf, user.ToSearch())

		if len(buf) >= 1000 {
			err = meili.AddDocuments("users", buf...)
			if err != nil {
				log.Fatal(err)
			}

			buf = make([]interface{}, 0)
		}
	}

	if len(buf) > 0 {
		err = meili.AddDocuments("users", buf...)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func indexTags(db *ti.Database, meili *search.MeiliSearchEngine) {
	res, err := db.DB.Query("select * from tag")
	if err != nil {
		log.Fatal(err)
	}

	buf := make([]interface{}, 0)
	for res.Next() {
		tag, err := models.TagFromSQLNative(res)
		if err != nil {
			log.Fatal(err)
		}

		buf = append(buf, tag.ToSearch())

		if len(buf) >= 1000 {
			err = meili.AddDocuments("tags", buf...)
			if err != nil {
				log.Fatal(err)
			}

			buf = make([]interface{}, 0)
		}
	}

	if len(buf) > 0 {
		err = meili.AddDocuments("tags", buf...)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func indexThreadComments(db *ti.Database, meili *search.MeiliSearchEngine) {
	res, err := db.DB.Query("select * from thread_comment")
	if err != nil {
		log.Fatal(err)
	}

	buf := make([]interface{}, 0)
	for res.Next() {
		threadComment, err := models.ThreadCommentFromSQLNative(res)
		if err != nil {
			log.Fatal(err)
		}

		buf = append(buf, threadComment)

		if len(buf) >= 1000 {
			err = meili.AddDocuments("thread_comment", buf...)
			if err != nil {
				log.Fatal(err)
			}

			buf = make([]interface{}, 0)
		}
	}

	if len(buf) > 0 {
		err = meili.AddDocuments("thread_comment", buf...)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func indexDiscussions(db *ti.Database, meili *search.MeiliSearchEngine) {
	res, err := db.DB.Query("select * from discussion")
	if err != nil {
		log.Fatal(err)
	}

	buf := make([]interface{}, 0)
	for res.Next() {
		discussion, err := models.DiscussionFromSQLNative(db, res)
		if err != nil {
			log.Fatal(err)
		}

		buf = append(buf, discussion)

		if len(buf) >= 1000 {
			err = meili.AddDocuments("discussion", buf...)
			if err != nil {
				log.Fatal(err)
			}

			buf = make([]interface{}, 0)
		}
	}

	if len(buf) > 0 {
		err = meili.AddDocuments("discussion", buf...)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func indexWorkspaceConfigs(db *ti.Database, meili *search.MeiliSearchEngine) {
	res, err := db.DB.Query("select * from workspace_config")
	if err != nil {
		log.Fatal(err)
	}

	buf := make([]interface{}, 0)
	for res.Next() {
		workspaceConfig, err := models.WorkspaceConfigFromSQLNative(db, res)
		if err != nil {
			log.Fatal(err)
		}

		buf = append(buf, workspaceConfig)

		if len(buf) >= 1000 {
			err = meili.AddDocuments("workspace_configs", buf...)
			if err != nil {
				log.Fatal(err)
			}

			buf = make([]interface{}, 0)
		}
	}

	if len(buf) > 0 {
		err = meili.AddDocuments("workspace_configs", buf...)
		if err != nil {
			log.Fatal(err)
		}
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

	meili, err := search.CreateMeiliSearchEngine(cfg.MeiliConfig)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to create meili search engine: %v", err))
	}

	fmt.Println("Indexing users...")
	indexUsers(db, meili)

	fmt.Println("Indexing tags...")
	indexTags(db, meili)

	fmt.Println("Indexing thread comments...")
	indexThreadComments(db, meili)

	fmt.Println("Indexing discussions...")
	indexDiscussions(db, meili)

	fmt.Println("Indexing workspace configs...")
	indexWorkspaceConfigs(db, meili)

	fmt.Println("Indexing posts...")
	indexPosts(db, meili)

	fmt.Println("Indexing comments...")
	indexComments(db, meili)

	fmt.Println("Done!")
}

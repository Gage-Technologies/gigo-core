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
	TitaniumConfig config.TitaniumConfig `yaml:"ti_config"`
	MeiliConfig    config.MeiliConfig    `yaml:"meili_config"`
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

	// delete all of the docs in the workspace_config index
	for {
		res, err := meili.Search("workspace_configs", &search.Request{
			Query: "",
			Limit: 1000,
		})
		if err != nil {
			log.Fatal(err)
		}

		ids := make([]interface{}, 0)
		for {
			ok, err := res.Next()
			if err != nil {
				log.Fatal(err)
			}

			if !ok {
				break
			}

			var c models.WorkspaceConfig
			err = res.Scan(&c)
			if err != nil {
				log.Fatal(err)
			}

			ids = append(ids, c.ID)
		}

		if len(ids) == 0 {
			break
		}

		err = meili.DeleteDocuments("workspace_configs", ids...)
		if err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println("Indexing workspace configs...")
	indexWorkspaceConfigs(db, meili)

	fmt.Println("Done!")
}

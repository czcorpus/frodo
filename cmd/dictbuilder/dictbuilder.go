// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/rs/zerolog/log"

	"frodo/cnf"
	"frodo/corpus"
	"frodo/db/mysql"
	dictActions "frodo/dictionary/actions"
	"frodo/liveattrs/laconf"
)

var (
	version   string
	buildDate string
	gitCommit string
)

func main() {
	runCmd := flag.NewFlagSet("run the dictionary build command", flag.ExitOnError)
	confgenCmd := flag.NewFlagSet("generate a config template using a server conf.", flag.ExitOnError)
	versionCmd := flag.NewFlagSet("show version", flag.ExitOnError)

	runCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "mkdict\n\nUsage:\n\t%s [options] run [config.json]\n\n", filepath.Base(os.Args[0]))
		runCmd.PrintDefaults()
	}
	confgenCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "\n\t%s [options] confgen [server config.json]\n\n", filepath.Base(os.Args[0]))
		confgenCmd.PrintDefaults()
	}
	versionCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "\n\t%s version\n", filepath.Base(os.Args[0]))
		versionCmd.PrintDefaults()
	}

	generalUsage := func() {
		fmt.Fprintf(os.Stderr, "mkdict - create a dictionary out of monitoring corpus verticals\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\t%s [options] run\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "\t%s [options] confgen [server config.json] [corpname]\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "\t%s help [command]\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "\t%s version\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	var action string
	if len(os.Args) > 1 {
		action = os.Args[1]
	}

	switch action {
	case "run":
		runCmd.Parse(os.Args[2:])
		run(runCmd.Arg(0))
	case "confgen":
		confgenCmd.Parse(os.Args[2:])
		generateConf(confgenCmd.Arg(0), confgenCmd.Arg(1))
	case "version":
		fmt.Printf("mkdict %s\nbuild date: %s\nlast commit: %s\n", version, buildDate, gitCommit)
	case "help":
		if len(os.Args) > 2 {
			helpCmd := os.Args[2]
			switch helpCmd {
			case "run":
				runCmd.Usage()
			case "confgen":
				confgenCmd.Usage()
			case "version":
				versionCmd.Usage()
			default:
				fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", helpCmd)
				generalUsage()
			}
		} else {
			generalUsage()
		}
	default:
		generalUsage()
	}
}

func run(configFilePath string) {

	// Load configuration from JSON file
	configRaw, err := os.Open(configFilePath)
	if err != nil {
		fmt.Println("Error opening configuration file:", err)
		return
	}
	defer configRaw.Close()

	var config DictbuilderConfig
	if err := json.NewDecoder(configRaw).Decode(&config); err != nil {
		fmt.Println("Error decoding configuration file:", err)
		return
	}
	logging.SetupLogging(config.Logging)

	// Run liveattrs job
	verts := []string{}
	now := time.Now()
	for i := 1; i <= config.NumOfLookbackDays; i++ {
		vertDate := now.AddDate(0, 0, -i)
		verts = append(verts, path.Join(config.VerticalDir, fmt.Sprintf("%s.vrt", vertDate.Format("2006-01-02"))))
	}

	liveattrsPath := fmt.Sprintf("liveAttributes/%s/data", config.TempCorpname)
	liveattrsParams := "noCorpusDbUpdate=1" // required so the liveattrs job don't search for the corpus in the database
	liveAttrsArgs := laconf.PatchArgs{
		VerticalFiles: verts,
	}

	log.Info().Msg("Running live attributes job")
	if err := doJob(config.API.BaseURL, liveattrsPath, liveattrsParams, liveAttrsArgs); err != nil {
		log.Error().Err(err).Msg("Error running live attributes job")
		return
	}

	// Run ngrams job
	ngramsPath := fmt.Sprintf("dictionary/%s/ngrams", config.TempCorpname)
	ngramsParams := fmt.Sprintf("append=0&ngramSize=%d", config.NGramSize)
	ngramsArgs := dictActions.NGramsReqArgs{
		PosTagset:             corpus.TagsetCSCNC2020,
		UsePartitionedTable:   false,
		MinFreq:               1,
		SkipGroupedNameSearch: true, // required so the ngrams job don't search for the corpus in the database
	}
	log.Info().Msg("Running ngrams job")
	if err := doJob(config.API.BaseURL, ngramsPath, ngramsParams, ngramsArgs); err != nil {
		log.Error().Err(err).Msg("Error running live attributes job")
		return
	}

	// Rename tables in database
	db, err := mysql.OpenDB(*config.Database)
	if err != nil {
		log.Error().Err(err).Msg("Error opening database connection")
		return
	}
	defer db.Close()
	for _, tableSuffix := range []string{"colcounts", "liveattrs_entry", "term_search", "word"} {
		log.Info().Msgf("Replacing table %s_%s -> %s_%s", config.TempCorpname, tableSuffix, config.Corpname, tableSuffix)
		if err := replaceTable(db, config.Corpname, config.TempCorpname, tableSuffix); err != nil {
			log.Error().Err(err).Msgf("Error replacing table %s_%s", config.Corpname, tableSuffix)
			return
		}
	}
	log.Info().Msg("Job done!")
}

func generateConf(serverConfPath string, corpname string) {
	conf := cnf.LoadConfig(serverConfPath)
	var mkdirConf DictbuilderConfig
	mkdirConf.Database = conf.LiveAttrs.DB
	mkdirConf.API = struct {
		BaseURL string "json:\"baseUrl\""
	}{
		BaseURL: fmt.Sprintf("http://%s:%d", conf.ListenAddress, conf.ListenPort),
	}
	mkdirConf.NumOfLookbackDays = 365
	mkdirConf.NGramSize = 1
	mkdirConf.VerticalDir = "/cnk/common/korpus/vertikaly/monitora"
	if corpname == "" {
		corpname = "my_corpus"
	}
	mkdirConf.Corpname = corpname
	mkdirConf.TempCorpname = "tmp_" + corpname
	jsonData, err := json.Marshal(mkdirConf)
	if err != nil {
		log.Error().Err(err).Msg("failed to store template json")
		os.Exit(1)
	}
	fmt.Println(string(jsonData))
}

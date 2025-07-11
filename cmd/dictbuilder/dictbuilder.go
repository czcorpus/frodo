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

	"frodo/common"
	dictActions "frodo/dictionary/actions"
	"frodo/liveattrs/laconf"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Dictbuilder\n\nUsage:\n\t%s [options] start [config.json]\n\t%s [options] version\n", filepath.Base(os.Args[0]), filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()
	configFilePath := flag.Arg(0)

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
	for i := 1; i <= config.NumberOfDays; i++ {
		vertDate := now.AddDate(0, 0, -i)
		verts = append(verts, path.Join(config.VerticalDir, fmt.Sprintf("%s.vrt", vertDate.Format("2006-01-02"))))
	}

	liveattrsPath := fmt.Sprintf("liveAttributes/%s/data", config.TempCorpname)
	liveattrsParams := "noCorpusUpdate=1" // required so the liveattrs job don't search for the corpus in the database
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
		PosTagset:             common.TagsetCSCNC2020,
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
	db, err := connectDB(config)
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

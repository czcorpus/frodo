// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
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

package cnf

import (
	"encoding/json"
	"frodo/corpus"
	"frodo/jobs"
	"frodo/kontext"
	"frodo/liveattrs"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/rs/zerolog/log"
)

const (
	dfltServerWriteTimeoutSecs = 10
	dfltLanguage               = "en"
	dfltMaxNumConcurrentJobs   = 4
	dfltVertMaxNumErrors       = 100
)

// Conf is a global configuration of the app
type Conf struct {
	ListenAddress          string                `json:"listenAddress"`
	ListenPort             int                   `json:"listenPort"`
	ServerReadTimeoutSecs  int                   `json:"serverReadTimeoutSecs"`
	ServerWriteTimeoutSecs int                   `json:"serverWriteTimeoutSecs"`
	CorporaSetup           *corpus.CorporaSetup  `json:"corporaSetup"`
	Logging                logging.LoggingConf   `json:"logging"`
	CNCDB                  *corpus.DatabaseSetup `json:"cncDb"`
	LiveAttrs              *liveattrs.Conf       `json:"liveAttrs"`
	Jobs                   *jobs.Conf            `json:"jobs"`
	Kontext                *kontext.Conf         `json:"kontext"`
	Language               string                `json:"language"`
	srcPath                string
}

func (conf *Conf) GetLocation() *time.Location { // TODO
	loc, err := time.LoadLocation("Europe/Prague")
	if err != nil {
		log.Fatal().Msg("failed to initialize location")
	}
	return loc
}

// GetSourcePath returns an absolute path of a file
// the config was loaded from.
func (conf *Conf) GetSourcePath() string {
	if filepath.IsAbs(conf.srcPath) {
		return conf.srcPath
	}
	var cwd string
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "[failed to get working dir]"
	}
	return filepath.Join(cwd, conf.srcPath)
}

func LoadConfig(path string) *Conf {
	if path == "" {
		log.Fatal().Msg("Cannot load config - path not specified")
	}
	rawData, err := os.ReadFile(path)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot load config")
	}
	var conf Conf
	conf.srcPath = path
	err = json.Unmarshal(rawData, &conf)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot load config")
	}
	return &conf
}

func ApplyDefaults(conf *Conf) {
	if conf.ServerWriteTimeoutSecs == 0 {
		conf.ServerWriteTimeoutSecs = dfltServerWriteTimeoutSecs
		log.Warn().Msgf(
			"serverWriteTimeoutSecs not specified, using default: %d",
			dfltServerWriteTimeoutSecs,
		)
	}
	if conf.LiveAttrs.VertMaxNumErrors == 0 {
		conf.LiveAttrs.VertMaxNumErrors = dfltVertMaxNumErrors
		log.Warn().Msgf(
			"liveAttrs.vertMaxNumErrors not specified, using default: %d",
			dfltVertMaxNumErrors,
		)
	}
	if conf.Language == "" {
		conf.Language = dfltLanguage
		log.Warn().Msgf("language not specified, using default: %s", conf.Language)
	}
	if conf.Jobs.MaxNumConcurrentJobs == 0 {
		v := dfltMaxNumConcurrentJobs
		if v >= runtime.NumCPU() {
			v = runtime.NumCPU()
		}
		conf.Jobs.MaxNumConcurrentJobs = v
		log.Warn().Msgf("jobs.maxNumConcurrentJobs not specified, using default %d", v)
	}
}

// ------- live attributes and stuff

type LAConf struct {
	LA   *liveattrs.Conf
	Corp *corpus.CorporaSetup
}

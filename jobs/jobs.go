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

package jobs

import (
	"encoding/gob"
	"frodo/mail"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type Conf struct {
	StatusDataPath       string                 `json:"statusDataPath"`
	MaxNumConcurrentJobs int                    `json:"maxNumConcurrentJobs"`
	MaxNumRestarts       int                    `json:"maxNumRestarts"`
	EmailNotification    mail.EmailNotification `json:"emailNotification"`
}

// GeneralJobInfo defines a general job information
type GeneralJobInfo interface {

	// GetID should provide unique identifier of the job
	// (across all the possible implementations)
	GetID() string

	// GetType returns a speicific job type (e.g. "liveattrs-create")
	GetType() string

	// GetStartDT provides a datetime information when the job started
	GetStartDT() JSONTime

	// GetCorpus provides a corpus name the job is related to
	GetCorpus() string

	// IsFinished returns true if the job has finished (either successfully or not)
	IsFinished() bool

	// AsFinished sets the internal status to a finished state and returns an update
	// instance. It is expected that the lastest stored information (e.g. an error)
	// is preserved and proper update time information is stored. It is OK not to create
	// a clone for the returned value.
	AsFinished() GeneralJobInfo

	// GetNumRestarts returns how many times was the job restarted. For the normally run
	// job, this should be always 0. The number > 0 is expect to happen e.g. in case the
	// service is shut down while some jobs are running.
	GetNumRestarts() int

	// GetError returns status error (if any) or nil
	GetError() error

	// WithError creates a clone of the status with error set to the provided value.
	// The 'Updated' property is also set to the current time.
	WithError(err error) GeneralJobInfo

	// CompactVersion produces simplified, unified job info for quick job reviews
	CompactVersion() JobInfoCompact

	// FullInfo produces JSON-friendly object containing all the information about the job
	FullInfo() any
}

// JobInfoList is just a list of any jobs
type JobInfoList []GeneralJobInfo

// Serialize gob-encodes the list and stores
// it to a specified path
func (jil JobInfoList) Serialize(path string) error {
	fw, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer fw.Close()
	enc := gob.NewEncoder(fw)
	return enc.Encode(jil)
}

// LoadJobList loads gob-encoded job list
// from a specified path
func LoadJobList(path string) (JobInfoList, error) {
	fw, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	dec := gob.NewDecoder(fw)
	ans := make(JobInfoList, 0, 50)
	err = dec.Decode(&ans)
	return ans, err
}

func (jil JobInfoList) Len() int {
	return len(jil)
}

func (jil JobInfoList) Less(i, j int) bool {
	return jil[i].GetStartDT().Before(jil[j].GetStartDT())
}

func (jil JobInfoList) Swap(i, j int) {
	jil[i], jil[j] = jil[j], jil[i]
}

func clearOldJobs(data map[string]GeneralJobInfo) {
	curr := CurrentDatetime()
	numRemoved := 0
	for k, v := range data {
		if curr.Sub(v.GetStartDT()) > time.Duration(168)*time.Hour {
			delete(data, k)
			numRemoved++
		}
	}
	if numRemoved > 0 {
		log.Info().Msgf("removed %d old job(s)", numRemoved)
	}
}

// FindJob searches a job by providing either full id or its prefix.
// In case a prefix is used and there is more than one job matching the
// prefix, nil is returned
func FindJob(syncJobs map[string]GeneralJobInfo, jobID string) GeneralJobInfo {
	var ans GeneralJobInfo
	for ident, job := range syncJobs {
		if strings.HasPrefix(ident, jobID) {
			if ans != nil {
				return nil
			}
			ans = job
		}
	}
	return ans
}

// ClearJob removes finished job by providing full id.
func ClearFinishedJob(syncJobs map[string]GeneralJobInfo, jobID string) (GeneralJobInfo, bool) {
	job, ok := syncJobs[jobID]
	if ok {
		if job.IsFinished() {
			delete(syncJobs, jobID)
			return job, true
		}
		return job, false
	}
	return nil, false
}

// JobInfoCompact is a simplified and unified version of
// any specific job information
type JobInfoCompact struct {
	ID       string   `json:"id"`
	CorpusID string   `json:"corpusId"`
	Type     string   `json:"type"`
	Start    JSONTime `json:"start"`
	Update   JSONTime `json:"update"`
	Finished bool     `json:"finished"`
	OK       bool     `json:"ok"`
}

// JobInfoListCompact represents a list of jobs for quick reviews
// (i.e. any type-specific information is discarded)
type JobInfoListCompact []*JobInfoCompact

func (cjil JobInfoListCompact) Len() int {
	return len(cjil)
}

func (cjil JobInfoListCompact) Less(i, j int) bool {
	return cjil[i].Start.Before(cjil[j].Start)
}

func (cjil JobInfoListCompact) Swap(i, j int) {
	cjil[i], cjil[j] = cjil[j], cjil[i]
}

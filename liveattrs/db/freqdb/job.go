// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Institute of the Czech National Corpus,
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

package freqdb

import (
	"frodo/jobs"
	"time"
)

type NgramJobInfoArgs struct {
}

// NgramJobInfo
type NgramJobInfo struct {
	ID          string           `json:"id"`
	Type        string           `json:"type"`
	CorpusID    string           `json:"corpusId"`
	Start       jobs.JSONTime    `json:"start"`
	Update      jobs.JSONTime    `json:"update"`
	Finished    bool             `json:"finished"`
	Error       error            `json:"error,omitempty"`
	NumRestarts int              `json:"numRestarts"`
	Args        NgramJobInfoArgs `json:"args"`
	Result      genNgramsStatus  `json:"result"`
}

func (j NgramJobInfo) GetID() string {
	return j.ID
}

func (j NgramJobInfo) GetType() string {
	return j.Type
}

func (j NgramJobInfo) GetStartDT() jobs.JSONTime {
	return j.Start
}

func (j NgramJobInfo) GetNumRestarts() int {
	return j.NumRestarts
}

func (j NgramJobInfo) GetCorpus() string {
	return j.CorpusID
}

func (j NgramJobInfo) AsFinished() jobs.GeneralJobInfo {
	j.Update = jobs.CurrentDatetime()
	j.Finished = true
	return j
}

func (j NgramJobInfo) IsFinished() bool {
	return j.Finished
}

func (j NgramJobInfo) FullInfo() any {
	return struct {
		ID          string           `json:"id"`
		Type        string           `json:"type"`
		CorpusID    string           `json:"corpusId"`
		Start       jobs.JSONTime    `json:"start"`
		Update      jobs.JSONTime    `json:"update"`
		Finished    bool             `json:"finished"`
		Error       string           `json:"error,omitempty"`
		OK          bool             `json:"ok"`
		NumRestarts int              `json:"numRestarts"`
		Args        NgramJobInfoArgs `json:"args"`
		Result      genNgramsStatus  `json:"result"`
	}{
		ID:          j.ID,
		Type:        j.Type,
		CorpusID:    j.CorpusID,
		Start:       j.Start,
		Update:      j.Update,
		Finished:    j.Finished,
		Error:       jobs.ErrorToString(j.Error),
		OK:          j.Error == nil,
		NumRestarts: j.NumRestarts,
		Args:        j.Args,
		Result:      j.Result,
	}
}

func (j NgramJobInfo) CompactVersion() jobs.JobInfoCompact {
	item := jobs.JobInfoCompact{
		ID:       j.ID,
		Type:     j.Type,
		CorpusID: j.CorpusID,
		Start:    j.Start,
		Update:   j.Update,
		Finished: j.Finished,
		OK:       true,
	}
	item.OK = j.Error == nil
	return item
}

func (j NgramJobInfo) GetError() error {
	return j.Error
}

func (j NgramJobInfo) WithError(err error) jobs.GeneralJobInfo {
	return &NgramJobInfo{
		ID:          j.ID,
		Type:        j.Type,
		CorpusID:    j.CorpusID,
		Start:       j.Start,
		Update:      jobs.JSONTime(time.Now()),
		Finished:    true,
		Error:       err,
		Result:      j.Result,
		NumRestarts: j.NumRestarts,
	}
}

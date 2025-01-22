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

package corpus

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/google/uuid"

	"frodo/jobs"
)

const (
	jobTypeSyncCNK = "sync-cnk"
)

type CorpusInfoProvider interface {
	LoadInfo(corpusID string) (*DBInfo, error)
}

// Actions contains all the server HTTP REST actions
type Actions struct {
	conf         *CorporaSetup
	osSignal     chan os.Signal
	jobsConf     *jobs.Conf
	jobActions   *jobs.Actions
	infoProvider CorpusInfoProvider
}

func (a *Actions) RestartJob(jinfo *JobInfo) error {
	err := a.jobActions.TestAllowsJobRestart(jinfo)
	if err != nil {
		return err
	}
	jinfo.Start = jobs.CurrentDatetime()
	jinfo.NumRestarts++
	jinfo.Update = jobs.CurrentDatetime()

	fn := func(updateJobChan chan<- jobs.GeneralJobInfo) {
		defer close(updateJobChan)
		resp, err := synchronizeCorpusData(&a.conf.CorpusDataPath, jinfo.CorpusID)
		if err != nil {
			updateJobChan <- jinfo.WithError(err)

		} else {
			newJinfo := *jinfo
			newJinfo.Result = &resp

			updateJobChan <- newJinfo.AsFinished()
		}
	}
	a.jobActions.EnqueueJob(&fn, jinfo)
	log.Info().Msgf("Restarted corpus job %s", jinfo.ID)
	return nil
}

// SynchronizeCorpusData synchronizes data between CNC corpora data and KonText data
// for a specified corpus (the corpus must be explicitly allowed in the configuration).
func (a *Actions) SynchronizeCorpusData(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	subdir := ctx.Param("subdir")
	if subdir != "" {
		corpusID = filepath.Join(subdir, corpusID)
	}
	if !a.conf.AllowsSyncForCorpus(corpusID) {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("Corpus synchronization forbidden for '%s'", corpusID), http.StatusUnauthorized)
		return
	}

	jobID, err := uuid.NewUUID()
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("Failed to start synchronization job for '%s'", corpusID), http.StatusUnauthorized)
		return
	}

	if prevRunning, ok := a.jobActions.LastUnfinishedJobOfType(corpusID, jobTypeSyncCNK); ok {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("Cannot run synchronization - the previous job '%s' have not finished yet", prevRunning), http.StatusConflict)
		return
	}

	jobKey := jobID.String()
	jobRec := &JobInfo{
		ID:       jobKey,
		Type:     jobTypeSyncCNK,
		CorpusID: corpusID,
		Start:    jobs.CurrentDatetime(),
	}

	// now let's define and enqueue the actual synchronization
	fn := func(updateJobChan chan<- jobs.GeneralJobInfo) {
		defer close(updateJobChan)
		resp, err := synchronizeCorpusData(&a.conf.CorpusDataPath, corpusID)
		if err != nil {
			jobRec.Error = err
		}
		jobRec.Result = &resp
		updateJobChan <- jobRec.AsFinished()
	}
	a.jobActions.EnqueueJob(&fn, jobRec)

	uniresp.WriteJSONResponse(ctx.Writer, jobRec.FullInfo())
}

// NewActions is the default factory
func NewActions(
	conf *CorporaSetup,
	jobsConf *jobs.Conf,
	jobActions *jobs.Actions,
	infoProvider CorpusInfoProvider,
) *Actions {
	return &Actions{
		conf:         conf,
		jobsConf:     jobsConf,
		jobActions:   jobActions,
		infoProvider: infoProvider,
	}
}

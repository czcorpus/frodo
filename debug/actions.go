// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
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

package debug

import (
	"fmt"
	"net/http"

	"frodo/jobs"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type storedDummyJob struct {
	jobInfo   jobs.DummyJobInfo
	jobUpdate chan jobs.GeneralJobInfo
}

// Actions contains all the server HTTP REST actions
type Actions struct {
	finishSignals map[string]chan<- bool
	jobActions    *jobs.Actions
}

// GetCorpusInfo provides some basic information about stored data
func (a *Actions) CreateDummyJob(ctx *gin.Context) {
	jobID, err := uuid.NewUUID()
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("failed to create dummy job"), http.StatusUnauthorized)
		return
	}

	jobInfo := &jobs.DummyJobInfo{
		ID:       jobID.String(),
		Type:     "dummy-job",
		Start:    jobs.CurrentDatetime(),
		CorpusID: "dummy",
	}
	if ctx.Request.URL.Query().Get("error") == "1" {
		jobInfo.Error = fmt.Errorf("dummy error")
	}
	finishSignal := make(chan bool)
	fn := func(upds chan<- jobs.GeneralJobInfo) {
		defer close(upds)
		<-finishSignal
		jobInfo.Result = &jobs.DummyJobResult{Payload: "Job Done!"}
		upds <- jobInfo.AsFinished()
	}
	a.jobActions.EnqueueJob(&fn, jobInfo)
	a.finishSignals[jobID.String()] = finishSignal
	uniresp.WriteJSONResponse(ctx.Writer, jobInfo)
}

func (a *Actions) FinishDummyJob(ctx *gin.Context) {
	finish, ok := a.finishSignals[ctx.Param("jobId")]
	if ok {
		delete(a.finishSignals, ctx.Param("jobId"))
		defer close(finish)
		finish <- true
		if storedJob, ok := a.jobActions.GetJob(ctx.Param("jobId")); ok {
			// TODO please note that here we typically won't see the
			// final storedJob value (updated elsewhere in a different
			// goroutine). So it may be a bit confusing.
			uniresp.WriteJSONResponse(ctx.Writer, storedJob.FullInfo())

		} else {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("job not found"), http.StatusNotFound)
		}

	} else {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("job not found"), http.StatusNotFound)
	}
}

// NewActions is the default factory
func NewActions(jobActions *jobs.Actions) *Actions {
	return &Actions{
		finishSignals: make(map[string]chan<- bool),
		jobActions:    jobActions,
	}
}

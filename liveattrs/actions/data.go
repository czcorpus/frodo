// Copyright 2020 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2020 Institute of the Czech National Corpus,
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

package actions

import (
	"fmt"
	"frodo/jobs"
	"frodo/kontext"
	"frodo/liveattrs"
	"frodo/liveattrs/db"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	vteCnf "github.com/czcorpus/vert-tagextract/v3/cnf"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Create starts a process of creating fresh liveattrs data for a a specified corpus.
//
// URL args:
//   - reconfigure - ignore the stored liveattrs config (if any) and generate a new one
//     based on corpus properties and provided PatchArgs. The resulting new config will
//     be stored replacing the previous one. In case the option is not set, then the
//     provided PatchArgs will be applied only to a temporary copy of a respective config
//     keeping the stored value intact.
//
// request body:
//
//	The method expects laconf.PatchArgs JSON
func (a *Actions) Create(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to generate liveattrs for %s: %w"
	reconfigure := ctx.Request.URL.Query().Get("reconfigure") == "1"

	var err error
	var conf *vteCnf.VTEConf
	if !reconfigure {
		conf, err = a.laConfCache.Get(corpusID)
	}

	jsonArgs, err := a.getPatchArgs(ctx.Request)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			err,
			http.StatusBadRequest,
		)
		return
	}

	if err := jsonArgs.ValidateDataWindow(); err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			err,
			http.StatusBadRequest,
		)
		return
	}

	if conf == nil {
		var newConf *vteCnf.VTEConf
		var err error
		newConf, err = a.createConf(corpusID, jsonArgs)
		if err != nil && err != ErrorMissingVertical {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
			return
		}

		err = a.laConfCache.Save(newConf)
		if err != nil {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
			return
		}

		conf, err = a.laConfCache.Get(corpusID)
		if err != nil {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
			return
		}
	}

	runtimeConf := *conf
	a.applyPatchArgs(&runtimeConf, jsonArgs)
	if !runtimeConf.HasConfiguredVertical() {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusConflict)
		return
	}

	// TODO search collisions only in liveattrs type jobs
	jobID, err := uuid.NewUUID()
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusUnauthorized)
		return
	}

	if prevRunning, ok := a.jobActions.LastUnfinishedJobOfType(corpusID, liveattrs.JobType); ok {
		err := fmt.Errorf("the previous job %s not finished yet", prevRunning.GetID())
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(baseErrTpl, corpusID, err),
			http.StatusConflict,
		)
		return
	}

	append := ctx.Request.URL.Query().Get("append")
	noCorpusUpdate := ctx.Request.URL.Query().Get("noCorpusUpdate")
	status := &liveattrs.LiveAttrsJobInfo{
		ID:       jobID.String(),
		CorpusID: corpusID,
		Start:    jobs.CurrentDatetime(),
		Args: liveattrs.JobInfoArgs{
			VteConf:        runtimeConf,
			Append:         append == "1",
			NoCorpusUpdate: noCorpusUpdate == "1",
		},
	}
	a.generateData(status)
	uniresp.WriteJSONResponseWithStatus(ctx.Writer, http.StatusCreated, status.FullInfo())
}

// Delete removes all the live attributes data for a corpus
func (a *Actions) Delete(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to delete configuration for %s"
	corpusDBInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	tx0, err := a.laDB.DB().Begin()
	err = db.DeleteTable(
		tx0,
		corpusDBInfo.GroupedName(),
		corpusID,
	)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		tx0.Rollback()
		return
	}
	tx1, err := a.cncDB.StartTx()
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	err = a.cncDB.UnsetLiveAttrs(tx1, corpusID)
	if err != nil {
		tx1.Rollback()
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	// Now we commit tx0 and tx1 deliberately before soft reset below as a failed operation of
	// cache reset does no permanent damage.
	// Also please note that the two independent transactions tx0, tx1 here cannot provide
	// behavior of a single transaction which means the operation may end up in a
	// non-consistent state (if tx0 commits and tx1 fails).
	err = tx0.Commit()
	if err != nil {
		tx1.Rollback()
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	err = tx1.Commit() // in case this fails we're screwed as tx0 is already commited
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(
			baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	err = kontext.SendSoftReset(a.conf.KonText)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, map[string]any{"ok": true})
}

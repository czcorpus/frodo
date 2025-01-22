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

package cncdb

import (
	"database/sql"
	"frodo/corpus"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/czcorpus/cnc-gokit/uniresp"
)

// DataHandler describes functions expected from
// CNC information system database as needed by KonText
// (and possibly other apps).
type DataHandler interface {
	UpdateSize(transact *sql.Tx, corpus string, size int64) error
	UpdateDescription(transact *sql.Tx, corpus, descCs, descEn string) error
	GetSimpleQueryDefaultAttrs(corpus string) ([]string, error)
	GetCorpusTagsetAttrs(corpus string) ([]string, error)
	UpdateDefaultViewOpts(transact *sql.Tx, corpus string, defaultViewOpts DefaultViewOpts) error
	StartTx() (*sql.Tx, error)
	CommitTx(transact *sql.Tx) error
	RollbackTx(transact *sql.Tx) error
}

type updateSizeResp struct {
	OK bool `json:"ok"`
}

// Actions contains all the server HTTP REST actions
type Actions struct {
	conf  *corpus.DatabaseSetup
	cConf *corpus.CorporaSetup
	db    DataHandler
}

// NewActions is the default factory
func NewActions(
	conf *corpus.DatabaseSetup,
	cConf *corpus.CorporaSetup,
	db DataHandler,
) *Actions {
	return &Actions{
		conf:  conf,
		cConf: cConf,
		db:    db,
	}
}

func (a *Actions) InferKontextDefaults(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")

	defaultViewAttrs, err := a.db.GetSimpleQueryDefaultAttrs(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("Failed to get simple query default attrs: %w", err), http.StatusInternalServerError)
		return
	}
	defaultViewOpts := DefaultViewOpts{
		Attrs: defaultViewAttrs,
	}

	if len(defaultViewOpts.Attrs) == 0 {
		regDoc, err := corpus.GetRegistry(a.cConf.GetFirstValidRegistry(corpusID, ""))
		if err != nil {
			uniresp.RespondWithErrorJSON(
				ctx,
				err,
				http.StatusInternalServerError,
			)
			return
		}
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer, uniresp.NewActionError("Failed to get corpus attrs: %w", err), http.StatusInternalServerError)
			return
		}

		defaultViewOpts.Attrs = append(defaultViewOpts.Attrs, "word")
		for _, attr := range regDoc.PosAttrs {
			if attr.Name == "lemma" {
				defaultViewOpts.Attrs = append(defaultViewOpts.Attrs, "lemma")
				break
			}
		}
	}

	tagsetAttrs, err := a.db.GetCorpusTagsetAttrs(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("Failed to get corpus tagset attrs: %w", err), http.StatusInternalServerError)
		return
	}
	defaultViewOpts.Attrs = append(defaultViewOpts.Attrs, tagsetAttrs...)

	tx, err := a.db.StartTx()
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("Failed to start database transaction: %w", err), http.StatusInternalServerError)
		return
	}
	err = a.db.UpdateDefaultViewOpts(tx, corpusID, defaultViewOpts)
	if err != nil {
		tx.Rollback()
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("Failed to update `default_view_opts`: %w", err), http.StatusInternalServerError)
		return
	}
	tx.Commit()

	uniresp.WriteJSONResponse(ctx.Writer, defaultViewOpts)
}

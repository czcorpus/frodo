// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Charles University, Faculty of Arts,
//                Department of Linguistics
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

package ltsearch

import (
	"frodo/db/mysql"
	"frodo/liveattrs/laconf"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/czcorpus/vert-tagextract/v3/livetokens"
	"github.com/gin-gonic/gin"
)

type inputArgs struct {
	Attrs map[string][]string `json:"attrs"`
	Feats map[string][]string `json:"udFeats"`
}

type Actions struct {
	db          *mysql.Adapter
	laConfCache *laconf.LiveAttrsBuildConfProvider
}

func (a *Actions) Query(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	var inputFilter inputArgs
	if err := ctx.BindJSON(&inputFilter); err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			err,
			http.StatusBadRequest,
		)
		return
	}

	conf, err := a.laConfCache.Get(corpusID)
	if err == laconf.ErrorNoSuchConfig {
		uniresp.RespondWithErrorJSON(
			ctx,
			err,
			http.StatusNotFound,
		)
		return

	} else if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			err,
			http.StatusInternalServerError,
		)
		return
	}

	searcher := &livetokens.Searcher{
		Attrs: conf.LiveTokens,
		DB:    a.db.DB(),
	}
	attrs := make([]livetokens.AttrAndVal, 0, len(inputFilter.Attrs))
	for k, v := range inputFilter.Attrs {
		attrs = append(
			attrs,
			livetokens.AttrAndVal{
				Name:   k,
				Values: v,
			},
		)
	}

	feats := make([]livetokens.AttrAndVal, 0, len(inputFilter.Feats))
	for k, v := range inputFilter.Feats {
		feats = append(
			feats,
			livetokens.AttrAndVal{
				Name:   k,
				Values: v,
			},
		)
	}

	result, err := searcher.GetAvailableValues(ctx, corpusID, attrs, feats)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			err,
			http.StatusInternalServerError,
		)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, result)
}

func NewActions(db *mysql.Adapter, laConfCache *laconf.LiveAttrsBuildConfProvider) *Actions {
	return &Actions{
		db:          db,
		laConfCache: laConfCache,
	}
}

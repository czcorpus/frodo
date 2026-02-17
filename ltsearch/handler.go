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
	"errors"
	"fmt"
	"frodo/db/mysql"
	"frodo/liveattrs/laconf"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	registryDir string
}

// Query provides the core "live tokens" functionality for filtering
// token attribute values based on their subset selection.
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

type confResp struct {
	Conf  livetokens.AttrList `json:"conf"`
	Error error               `json:"error,omitempty"`
}

// Conf returns a configuration for corpus' live tokens.
// It can be used to test whether the functionality is available
// for a specific corpus.
func (a *Actions) Conf(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	conf, err := a.laConfCache.Get(corpusID)
	if err == laconf.ErrorNoSuchConfig {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusNotFound)
		return
	}
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	if len(conf.LiveTokens) > 0 {
		uniresp.WriteJSONResponse(ctx.Writer, confResp{Conf: conf.LiveTokens})
		return
	}
	uniresp.RespondWithErrorJSON(ctx, errors.New("live tokens not available"), http.StatusNotFound)
}

func (a *Actions) AutoConf(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	conf, err := a.laConfCache.Get(corpusID)
	if err == laconf.ErrorNoSuchConfig {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusNotFound)
		return
	}
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	if ctx.Query("overwrite") != "1" && len(conf.LiveTokens) > 0 {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("cannot overwrite existing config without explicit overwrite argument"), http.StatusConflict)
		return
	}

	var profileConf livetokens.AttrList

	if ctx.Query("profile") != "" {
		pConf, ok := profiles[ctx.Query("profile")]
		if !ok {
			uniresp.RespondWithErrorJSON(ctx, errors.New("profile not found"), http.StatusNotFound)
			return
		}
		profileConf = pConf

	} else {
		if ctx.Query("attrs") == "" {
			uniresp.RespondWithErrorJSON(
				ctx,
				fmt.Errorf("cannot determine which attributes to use"),
				http.StatusBadRequest,
			)
			return
		}
		attrs := strings.Split(ctx.Query("attrs"), ",")
		udFeatsAttr := ctx.Query("udFeatsAttr")
		profileConf = make(livetokens.AttrList, len(attrs))
		for i, attr := range attrs {
			profileConf[i] = livetokens.Attr{
				Name:      attr,
				IsUDFeats: attr == udFeatsAttr,
			}
		}
	}
	for i := range profileConf {
		profileConf[i].VertIdx = -1
	}

	if err := AutoConf(filepath.Join(a.registryDir, corpusID), profileConf); err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	conf.LiveTokens = profileConf
	if err := a.laConfCache.Save(conf); err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, profileConf)
}

// acwp is an ad-hoc type for retrunign information
// about a single corpus config creation
type acwp struct {
	Corpus string              `json:"corpus"`
	Conf   livetokens.AttrList `json:"conf,omitempty"`
	Error  string              `json:"error,omitempty"`
}

// acwpResponse is the response for AutoConfWithProfile
type acwpResponse struct {
	Items []acwp `json:"items"`
	Error error  `json:"error,omitempty"`
}

// AutoConfWithProfile updates all matching corpora (typically all the language versions
// of some InterCorp version).
func (a *Actions) AutoConfWithProfile(ctx *gin.Context) {
	profileID := ctx.Param("profileId")
	pConf, ok := profiles[profileID]
	if !ok {
		uniresp.RespondWithErrorJSON(ctx, errors.New("profile not found"), http.StatusNotFound)
		return
	}
	regFiles, err := os.ReadDir(a.registryDir)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, errors.New("failed to read registry directory"), http.StatusInternalServerError)
		return
	}

	var resp acwpResponse
	resp.Items = make([]acwp, 0, 60)
	for _, f := range regFiles {
		if !strings.HasPrefix(f.Name(), profileID+"_") {
			continue
		}
		pConfNew := make(livetokens.AttrList, len(pConf))
		copy(pConfNew, pConf)

		status := acwp{Corpus: f.Name()}
		vteConf, err := a.laConfCache.Get(f.Name())
		if err != nil {
			status.Error = err.Error()

		} else {
			regPath := filepath.Join(a.registryDir, f.Name())
			if err := AutoConf(regPath, pConfNew); err != nil {
				status.Error = err.Error()

			} else {
				vteConf.LiveTokens = pConfNew
				if err := a.laConfCache.Save(vteConf); err != nil {
					status.Error = err.Error()

				} else {
					status.Conf = pConfNew
				}
			}
		}
		resp.Items = append(resp.Items, status)
	}
	uniresp.WriteJSONResponse(ctx.Writer, resp)
}

// NewActions creates an http action handler for the "live tokens" endpoint.
func NewActions(db *mysql.Adapter, laConfCache *laconf.LiveAttrsBuildConfProvider, registryDir string) *Actions {
	return &Actions{
		db:          db,
		laConfCache: laConfCache,
		registryDir: registryDir,
	}
}

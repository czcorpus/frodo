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
	"encoding/json"
	"fmt"
	"frodo/corpus"
	"frodo/liveattrs/laconf"
	"io"
	"net/http"
	"path/filepath"

	"github.com/czcorpus/cnc-gokit/uniresp"
	vteCnf "github.com/czcorpus/vert-tagextract/v3/cnf"
	"github.com/czcorpus/vert-tagextract/v3/db"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func (a *Actions) getPatchArgs(req *http.Request) (*laconf.PatchArgs, error) {
	var jsonArgs laconf.PatchArgs
	err := json.NewDecoder(req.Body).Decode(&jsonArgs)
	if err == io.EOF {
		err = nil
	}
	if jsonArgs.GetTagsetAttr() == "" {
		ta := "tag"
		log.Warn().Str("value", ta).Msg("filling missing value of tagsetAttr in patchArgs")
		jsonArgs.TagsetAttr = &ta
	}
	if jsonArgs.GetTagsetName() == "" {
		tn := corpus.TagsetCSCNC2020
		log.Warn().Str("value", tn.String()).Msg("filling missing value of tagsetName in patchArgs")
		jsonArgs.TagsetName = &tn
	}
	return &jsonArgs, err
}

// createConf creates a data extraction configuration
// (for vert-tagextract library) based on provided corpus
// (= effectively a vertical file) and request data
// (where it expects JSON version of liveattrsJsonArgs).
func (a *Actions) createConf(
	corpusID string,
	jsonArgs *laconf.PatchArgs,
) (*vteCnf.VTEConf, error) {
	corpusInfo, err := corpus.GetCorpusInfo(corpusID, a.conf.Corp, false)
	if err != nil {
		return nil, err
	}
	corpusDBInfo, err := a.cncDB.LoadInfo(corpusID)
	if err != nil {
		return nil, err
	}

	conf, err := laconf.Create(
		a.conf.LA,
		corpusInfo,
		corpusDBInfo,
		jsonArgs,
	)
	if err != nil {
		return conf, err
	}

	err = a.applyPatchArgs(conf, jsonArgs)
	if err != nil {
		return conf, fmt.Errorf("failed to create conf: %w", err)
	}

	err = a.ensureVerticalFile(conf, corpusInfo)
	if err != nil {
		return conf, fmt.Errorf("failed to create conf: %w", err)
	}
	return conf, err
}

// ViewConf		 godoc
// @Summary      ViewConf shows actual liveattrs processing configuration
// @Description  ViewConf shows actual liveattrs processing configuration. Note: passwords are replaced with multiple asterisk characters.
// @Produce      json
// @Param        corpusId path string true "Used corpus"
// @Param 		 noCache query int false "Get uncached data" default(0)
// @Success      200 {object} vteCnf.VTEConf
// @Router       /liveAttributes/{corpusId}/conf [get]
func (a *Actions) ViewConf(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to get liveattrs conf for %s: %w"
	var conf *vteCnf.VTEConf
	var err error
	if ctx.Request.URL.Query().Get("noCache") == "1" {
		conf, err = a.laConfCache.GetUncachedWithoutPasswords(corpusID)

	} else {
		conf, err = a.laConfCache.GetWithoutPasswords(corpusID)
	}
	if err == laconf.ErrorNoSuchConfig {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusNotFound)
		return

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, conf)
}

// CreateConf godoc
// @Summary      CreateConf creates a new liveattrs processing configuration for a specified corpus
// @Description  In case user does not fill in the information regarding n-gram processing, no defaults are used. To attach n-gram information automatically, PatchConfig is used (with URL arg. auto-kontext-setup=1).
// @Accept  	 json
// @Produce      json
// @Param        corpusId path string true "Used corpus"
// @Param 		 patchArgs body laconf.PatchArgs true "Config data"
// @Success      200 {object} vteCnf.VTEConf
// @Router       /liveAttributes/{corpusId}/conf [put]
func (a *Actions) CreateConf(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to create liveattrs config for %s: %w"
	jsonArgs, err := a.getPatchArgs(ctx.Request)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			err,
			http.StatusBadRequest,
		)
	}
	newConf, err := a.createConf(corpusID, jsonArgs)
	if err == ErrorMissingVertical {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusConflict)
		return

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return
	}
	err = a.laConfCache.Clear(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return
	}
	err = a.laConfCache.Save(newConf)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return
	}
	expConf := newConf.WithoutPasswords()
	uniresp.WriteJSONResponse(ctx.Writer, &expConf)
}

// FlushCache godoc
// @Summary      FlushCache removes an actual cached liveattrs configuration for a specified corpus
// @Description  FlushCache removes an actual cached liveattrs configuration for a specified corpus. This is mostly useful in cases where a manual editation of liveattrs config was done and we need Frodo to use the actual file version.
// @Produce      json
// @Param        corpusId path string true "Used corpus"
// @Success      200 {object} any
// @Router       /liveAttributes/{corpusId}/confCache [delete]
func (a *Actions) FlushCache(ctx *gin.Context) {
	ok := a.laConfCache.Uncache(ctx.Param("corpusId"))
	if !ok {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("config not in cache"), http.StatusNotFound)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, map[string]bool{"ok": true})
}

// PatchConfig godoc
// @Summary      PatchConfig allows for updating liveattrs processing configuration
// @Description  It also allows a semi-automatic mode (using url query argument auto-kontext-setup=1) where the columns to be fetched from a corresponding vertical and other parameters with respect to a typical CNC setup used for its corpora.
// @Accept  	 json
// @Produce      json
// @Param        corpusId path string true "Used corpus"
// @Param 		 patchArgs body laconf.PatchArgs true "Config data"
// @Param 		 auto-kontext-setup query int false "Use semi-automatic mode" default(0)
// @Success      200 {object} vteCnf.VTEConf
// @Router       /liveAttributes/{corpusId}/conf [patch]
func (a *Actions) PatchConfig(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	conf, err := a.laConfCache.Get(corpusID)
	if err == laconf.ErrorNoSuchConfig {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("no such config"), http.StatusNotFound)
		return
	}

	inferNgramColsStr, ok := ctx.GetQuery("auto-kontext-setup")
	if !ok {
		inferNgramColsStr = "0"
	}
	inferNgramCols := inferNgramColsStr == "1"

	jsonArgs, err := a.getPatchArgs(ctx.Request)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadRequest)
		return
	}
	if jsonArgs == nil && !inferNgramCols {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("no update data provided"), http.StatusBadRequest)
		return
	}

	if inferNgramCols {
		regPath := filepath.Join(a.conf.Corp.RegistryDirPaths[0], corpusID)
		corpTagsets, err := a.cncDB.GetCorpusTagsets(corpusID)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		tagset := corpus.GetFirstSupportedTagset(corpTagsets)
		if tagset == "" {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		attrMapping, err := corpus.InferQSAttrMapping(regPath, tagset)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		columns := attrMapping.GetColIndexes()
		if jsonArgs.Ngrams == nil {
			jsonArgs.Ngrams = &vteCnf.NgramConf{
				VertColumns: make(db.VertColumns, 0, len(columns)),
			}

		} else if len(jsonArgs.Ngrams.VertColumns) > 0 {
			jsonArgs.Ngrams.VertColumns = make(db.VertColumns, 0, len(columns))
		}
		jsonArgs.Ngrams.NgramSize = 1
		for _, v := range columns {
			jsonArgs.Ngrams.VertColumns = append(
				jsonArgs.Ngrams.VertColumns, db.VertColumn{Idx: v},
			)
		}
	}

	err = a.applyPatchArgs(conf, jsonArgs)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadRequest)
		return
	}

	corpusInfo, err := corpus.GetCorpusInfo(corpusID, a.conf.Corp, false)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	err = a.ensureVerticalFile(conf, corpusInfo)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	a.laConfCache.Save(conf)
	out := conf.WithoutPasswords()
	uniresp.WriteJSONResponse(ctx.Writer, &out)
}

// QSDefaults godoc
// @Summary      QSDefaults shows the default configuration for extracting n-grams
// @Description  QSDefaults shows the default configuration for extracting n-grams for KonText query suggestion engine and KonText tag builder widget. This is mostly for overview purposes.
// @Produce      json
// @Param        corpusId path string true "Used corpus"
// @Success      200 {object} vteCnf.NgramConf
// @Router       /liveAttributes/{corpusId}/qsDefaults [get]
func (a *Actions) QSDefaults(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	regPath := filepath.Join(a.conf.Corp.RegistryDirPaths[0], corpusID)
	corpTagsets, err := a.cncDB.GetCorpusTagsets(corpusID)
	tagset := corpus.GetFirstSupportedTagset(corpTagsets)
	if tagset == "" {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("no supported tagset"), http.StatusUnprocessableEntity)
		return
	}
	attrMapping, err := corpus.InferQSAttrMapping(regPath, tagset)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	ans := vteCnf.NgramConf{
		NgramSize: 1,
		VertColumns: db.VertColumns{
			db.VertColumn{Idx: attrMapping.Word, Role: "word"},
			db.VertColumn{Idx: attrMapping.Lemma, Role: "lemma"},
			db.VertColumn{Idx: attrMapping.Sublemma, Role: "sublemma"},
			db.VertColumn{Idx: attrMapping.Tag, Role: "tag"},
		},
	}

	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

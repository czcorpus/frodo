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

package actions

import (
	"fmt"
	"frodo/dictionary"
	"net/http"

	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

const (
	defaultSimFreqRangeCoeff  = 0.2
	defaultSimFreqMaxNumItems = 20
)

// CreateQuerySuggestions godoc
// @Summary      Create query suggestions for a specified corpus
// @Produce      json
// @Param        corpusId path string true "Used corpus"
// @Success      200 {string} string
// @Router       /dictionary/{corpusId}/querySuggestions [post]
func (a *Actions) CreateQuerySuggestions(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	// TODO
	uniresp.WriteJSONResponse(ctx.Writer, corpusID)
}

// GetQuerySuggestions godoc
// @Summary      Get query suggestions for a specified corpus
// @Produce      json
// @Param        corpusId path string true "Used corpus"
// @Param        term path string true "Search term"
// @Param        no-multivalues query int false "Forbid multivalues" default(0)
// @Param        pos query string false "Search part of speach"
// @Success      200 {object} map[string]any
// @Router       /dictionary/{corpusId}/querySuggestions/{term} [get]
// @Router       /dictionary/{corpusId}/search/{term} [get]
func (a *Actions) GetQuerySuggestions(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	term := ctx.Param("term")
	noMultivalues := ctx.Query("no-multivalues") == "1"
	caseSensitive := ctx.Query("case-sensitive") == "1"

	mvOpts := dictionary.SearchWithMultivalues()
	if noMultivalues {
		mvOpts = dictionary.SearchWithNoOp()
	}

	pos := ctx.Query("pos")
	posOpts := dictionary.SearchWithNoOp()
	if pos != "" {
		posOpts = dictionary.SearchWithPoS(pos)
	}

	items, err := dictionary.Search(
		ctx,
		a.laDB,
		corpusID,
		dictionary.SearchWithAnyValue(term),
		dictionary.SearchWithAnyValueCS(caseSensitive),
		mvOpts,
		posOpts,
	)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	ans := map[string]any{
		"matches": items,
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

// SimilarARFWords godoc
// @Summary      Get similar arf words
// @Produce      json
// @Param        corpusId path string true "Used corpus"
// @Param        term path string true "Search term"
// @Param        pos query string false "Search part of speach"
// @Param        rangeCoeff query float64 false "Search range coefficient" default(0.2) minimum(0) maximum(1)
// @Param        maxkItems query int false "Maximum number of items" default(20)
// @Success      200 {object} map[string]any
// @Router       /dictionary/{corpusId}/similarARFWords/{term} [get]
func (a *Actions) SimilarARFWords(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	word := ctx.Param("term")
	pos := ctx.Query("pos")
	lemma := ctx.Query("lemma")
	rangeCoeff, ok := unireq.GetURLFloatArgOrFail(ctx, "rangeCoeff", defaultSimFreqRangeCoeff)
	if !ok {
		return
	}
	if rangeCoeff <= 0 || rangeCoeff >= 1 {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("rangeCoeff must be from interval (0, 1)"), http.StatusBadRequest)
		return
	}
	maxNumItems, ok := unireq.GetURLIntArgOrFail(ctx, "maxkItems", defaultSimFreqMaxNumItems)
	if !ok {
		return
	}

	corpusInfo, err := a.corpusMeta.LoadInfo(corpusID)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("failed to get info about corpus %s: %w", corpusID, err),
			http.StatusBadRequest,
		)
		return
	}

	if corpusInfo.Size <= 0 {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf(
				"cannot calculate list of words, reported corpus size for %s is zero (invalid record in db?)",
				corpusID,
			),
			http.StatusBadRequest,
		)
		return
	}

	termSrch, err := dictionary.Search(
		ctx,
		a.laDB,
		corpusID,
		dictionary.SearchWithWord(word),
		dictionary.SearchWithLemma(lemma),
		dictionary.SearchWithPoS(pos),
		dictionary.SearchWithLimit(1),
	)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	if len(termSrch) > 0 {
		items, err := dictionary.SimilarARFWords(
			ctx,
			a.laDB,
			corpusID,
			termSrch[0],
			rangeCoeff,
			maxNumItems,
		)
		for i := range items {
			items[i].IPM = float64(items[i].Count) / float64(corpusInfo.Size)
		}
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		ans := map[string]any{
			"matches": items,
		}
		uniresp.WriteJSONResponse(ctx.Writer, ans)

	} else {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("no values found"), http.StatusNotFound)
		return
	}

}

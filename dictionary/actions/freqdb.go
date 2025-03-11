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

func (a *Actions) CreateQuerySuggestions(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	// TODO
	uniresp.WriteJSONResponse(ctx.Writer, corpusID)
}

func (a *Actions) GetQuerySuggestions(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	term := ctx.Param("term")

	items, err := dictionary.Search(
		ctx,
		a.laDB,
		corpusID,
		dictionary.SearchWithAnyValue(term),
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

func (a *Actions) SimilarARFWords(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	word := ctx.Param("term")
	pos := ctx.Query("pos")
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

	corpusInfo, err := a.cncDB.LoadInfo(corpusID)
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
		dictionary.SearchWithLemma(word),
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

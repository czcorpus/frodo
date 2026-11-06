// Copyright 2026 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2026 Institute of the Czech National Corpus,
// Faculty of Arts, Charles University
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

package lex

import (
	"errors"
	"frodo/db/mysql"
	"frodo/dictionary"
	dictActions "frodo/dictionary/actions"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type LexDictResponse struct {
	Matches     []dictionary.Lemma `json:"matches"`
	Suggestions []string           `json:"suggestions"`
}

type LexExtraData struct {
	CorpusId string  `json:"corpusId"`
	Variant  LexItem `json:"variant"`
}

type Actions struct {
	db             *mysql.Adapter
	dictActions    *dictActions.Actions
	sourcePriority []Source
}

// SearchWord godoc
// @Summary      Search a term in the lexicon and corresponding corpus
// @Description  Searches the term (including its "se"/"si" variants) in the lexicon sources,
// @Description  resolves all variants of the found entry and joins them with corpus data
// @Description  (frequencies, forms, sublemmas). Corpus-only sublemma variants of the found
// @Description  lemmas are appended as well. Additional lexicon candidates and typo corrections
// @Description  are returned as suggestions.
// @Produce      json
// @Param        corpusId path string true "Used corpus"
// @Param        term path string true "Searched term"
// @Success      200 {object} LexDictResponse{matches=[]dictionary.Lemma{extraData=LexExtraData}}
// @Failure      400 {object} uniresp.ActionError
// @Failure      500 {object} uniresp.ActionError
// @Router       /dictionary/lex/{corpusId}/search/{term} [get]
func (actions *Actions) SearchWord(ctx *gin.Context) {
	resp, err := actions.search(ctx, ctx.Param("corpusId"), ctx.Param("term"))
	if errors.Is(err, ErrInvalidCorpusName) {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadRequest)
		return
	}
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, resp)
}

func NewActions(db *mysql.Adapter, dictActions *dictActions.Actions, sourcePriority []Source) *Actions {
	return &Actions{
		db:             db,
		dictActions:    dictActions,
		sourcePriority: sourcePriority,
	}
}

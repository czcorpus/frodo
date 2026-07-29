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
	"cmp"
	"context"
	"fmt"
	"frodo/db/mysql"
	"frodo/dictionary"
	dictActions "frodo/dictionary/actions"
	"net/http"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/czcorpus/cnc-gokit/util"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type LexExtraData struct {
	CorpusId   string  `json:"corpusId"`
	MainSource Source  `json:"mainSource"`
	Variant    LexItem `json:"variant"`
}

type Handler struct {
	db             *mysql.Adapter
	dictActions    *dictActions.Actions
	sourcePriority []Source
}

func (actions *Handler) searchCorpusEntry(ctx context.Context, corpusId, lemma, pos string) (*dictionary.Lemma, error) {
	if lemma == "" {
		return nil, nil
	}

	posArg := dictionary.SearchWithNoOp()
	if pos != "" {
		posArg = dictionary.SearchWithPoS(pos)
	}

	datasetSize, err := actions.dictActions.GetDatasetSize(corpusId)
	if err != nil {
		return nil, err
	}

	ans, err := dictionary.Search(
		ctx,
		actions.db,
		corpusId,
		dictionary.SearchWithLemma(lemma),
		dictionary.SearchWithDatasetSizeForIPM(int(datasetSize)),
		posArg,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find lemma: %w", err)
	}
	if len(ans) > 0 {
		if len(ans) > 1 {
			log.Warn().Str("lemma", lemma).Str("pos", pos).Int("numMatches", len(ans)).Msg("Multiple matches found for lemma in corpus")
		}
		return &ans[0], nil
	}
	return nil, nil
}

func (actions *Handler) SearchWord(ctx *gin.Context) {
	corpusId := ctx.Param("corpusId")
	term := ctx.Param("term")

	typoSuggestions, err := SearchTypoSuggestions(ctx, actions.db.DB(), term)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	searchCandidates, err := actions.getSearchCandidates(ctx, corpusId, term)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	if len(searchCandidates) == 0 {
		ans := map[string]any{
			"matches":     []dictionary.Lemma{},
			"suggestions": typoSuggestions,
		}
		uniresp.WriteJSONResponse(ctx.Writer, ans)
		return
	}

	usedCandidate := searchCandidates[0]
	// unused search candidates will be suggestions
	suggestions := append(collections.SliceMap(searchCandidates[1:], func(item SearchCandidate, i int) string {
		return item.Value
	}), typoSuggestions...)
	// search variants of the best candidate
	lexItems, err := SearchVariants(ctx, actions.db.DB(), usedCandidate.Value, usedCandidate.Source)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	// just in case..., should not happen, since searched item is certainly in dictionary, `mainSource` exists
	// TODO? corpus source
	if lexItems == nil {
		ans := map[string]any{
			"matches":     []dictionary.Lemma{},
			"suggestions": suggestions,
		}
		uniresp.WriteJSONResponse(ctx.Writer, ans)
		return
	}

	lexItems = sortVariants(lexItems, usedCandidate.Source)
	// for each variant, search for its entry in the corpus, if not found, create a new entry with minimal data
	variants := make([]dictionary.Lemma, 0, len(lexItems))
	for i, item := range lexItems {
		corpusEntry, err := actions.searchCorpusEntry(ctx, corpusId, item.Lemma, item.Pos)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		// corpus entry needs to replace "B" gender with "MI"
		lexSpecifier := cmp.Or(util.Ternary(item.Gender == GenderMascAnimInan, "MI", item.Gender), item.Aspect)
		if corpusEntry == nil {
			corpusEntry = &dictionary.Lemma{
				ID:        fmt.Sprintf("lex-%d", i),
				Lemma:     item.Lemma,
				PoS:       item.Pos,
				Specifier: lexSpecifier,
				Forms:     []dictionary.Form{{Value: item.Lemma, Sublemma: item.Lemma}},
				Sublemmas: []dictionary.Sublemma{{Value: item.Lemma}},
			}
		} else {
			corpusEntry.ID = fmt.Sprintf("corp-%d", i)
			corpusEntry.Specifier = cmp.Or(corpusEntry.Specifier, lexSpecifier)
			corpusEntry.Sublemmas = collections.SliceFilter(corpusEntry.Sublemmas, func(sublemma dictionary.Sublemma, i int) bool {
				return sublemma.Value == item.Lemma
			})
			corpusEntry.Forms = collections.SliceFilter(corpusEntry.Forms, func(form dictionary.Form, i int) bool {
				return form.Sublemma == item.Lemma
			})
		}
		corpusEntry.ExtraData = LexExtraData{
			CorpusId:   corpusId,
			MainSource: usedCandidate.Source,
			Variant:    item,
		}
		variants = append(variants, *corpusEntry)
		// remove variant from suggestions if present
		suggestions = collections.SliceFilter(suggestions, func(v string, i int) bool { return v != corpusEntry.Lemma })
	}

	ans := map[string]any{
		"matches":     variants,
		"suggestions": suggestions,
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func NewHandler(db *mysql.Adapter, dictActions *dictActions.Actions) *Handler {
	return &Handler{
		db:             db,
		dictActions:    dictActions,
		sourcePriority: []Source{SourceASSC, SourceIJP},
	}
}

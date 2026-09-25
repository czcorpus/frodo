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
	CorpusId      string  `json:"corpusId"`
	VariantSource Source  `json:"variantSource"`
	Variant       LexItem `json:"variant"`
}

type Handler struct {
	db             *mysql.Adapter
	dictActions    *dictActions.Actions
	sourcePriority []Source
}

func (actions *Handler) searchCorpusEntry(ctx context.Context, corpusId, sublemma, pos string) (*dictionary.Lemma, error) {
	if sublemma == "" {
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
		dictionary.SearchWithSublemma(sublemma),
		dictionary.SearchWithDatasetSizeForIPM(int(datasetSize)),
		posArg,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find sublemma: %w", err)
	}
	if len(ans) > 0 {
		if len(ans) > 1 {
			log.Warn().Str("sublemma", sublemma).Str("pos", pos).Int("numMatches", len(ans)).Msg("Multiple matches found for sublemma in corpus")
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

	// first candidate will be used for search, the rest will be used as suggestions
	usedCandidate := searchCandidates[0]
	suggestions := append(collections.SliceMap(searchCandidates[1:], func(item SearchCandidate, i int) string {
		return item.Value
	}), typoSuggestions...)

	// get variants from one source
	lexItems, err := SearchVariants(ctx, actions.db.DB(), usedCandidate.Value, usedCandidate.Source)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	// just in case..., should not happen, since searched item is certainly in dictionary, `variantSource` exists
	// TODO? corpus source
	if lexItems == nil {
		ans := map[string]any{
			"matches":     []dictionary.Lemma{},
			"suggestions": suggestions,
		}
		uniresp.WriteJSONResponse(ctx.Writer, ans)
		return
	}

	// apply special transformations before getting source data
	lexItems, err = ApplyTransformations(ctx, actions.db.DB(), lexItems,
		DTIJCR_MergeItems,
	)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	// for each variant, join source data
	for i, item := range lexItems {
		sources, err := SearchSources(ctx, actions.db.DB(), item.Key)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		lexItems[i].Sources = sources
	}

	// apply special transformations after getting source data
	lexItems, err = ApplyTransformations(ctx, actions.db.DB(), lexItems,
		DTIJCR_ResolvePos(actions.sourcePriority),
		JoinFromIJP_NAP_To_C,
		JoinToIJP_C_To_NAP,
		IJP_ResolvePos(actions.sourcePriority),
		JoinFromSSC_M_To_IB,
		SortTransformation(usedCandidate.Source),
	)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	// search corpus entry for each variant
	// if not found, create a new entry with minimal data
	variants := make([]dictionary.Lemma, 0, len(lexItems))
	for i, item := range lexItems {
		corpusEntry, err := actions.searchCorpusEntry(ctx, corpusId, item.Key.Lemma, item.Key.Pos)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		// corpus entry needs to replace "B" gender with "MI"
		lexSpecifier := cmp.Or(util.Ternary(item.Key.Gender == GenderMascAnimInan, "MI", item.Key.Gender), item.Key.Aspect)
		if corpusEntry == nil {
			corpusEntry = &dictionary.Lemma{
				ID:        fmt.Sprintf("lex-%d", i),
				Lemma:     item.Key.Lemma,
				PoS:       item.Key.Pos,
				Specifier: lexSpecifier,
				Forms:     []dictionary.Form{{Value: item.Key.Lemma, Sublemma: item.Key.Lemma}},
				Sublemmas: []dictionary.Sublemma{{Value: item.Key.Lemma}},
			}
		} else {
			corpusEntry.ID = fmt.Sprintf("corp-%d", i)
			corpusEntry.Specifier = cmp.Or(corpusEntry.Specifier, lexSpecifier)
			// use corpus PoS if IJP is used
			if item.PosSource == SourceIJP {
				item.PosSource = SourceCorpus
				item.Key.Pos = corpusEntry.PoS
			}
		}
		corpusEntry.ExtraData = LexExtraData{
			CorpusId:      corpusId,
			VariantSource: usedCandidate.Source,
			Variant:       item,
		}
		variants = append(variants, *corpusEntry)
		// remove variant from suggestions if present
		suggestions = collections.SliceFilter(suggestions, func(v string, i int) bool { return v != item.Key.Lemma })
	}

	ans := map[string]any{
		"matches":     variants,
		"suggestions": suggestions,
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func NewHandler(db *mysql.Adapter, dictActions *dictActions.Actions, sourcePriority []Source) *Handler {
	return &Handler{
		db:             db,
		dictActions:    dictActions,
		sourcePriority: sourcePriority,
	}
}

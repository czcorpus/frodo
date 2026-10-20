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
	"strings"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/czcorpus/cnc-gokit/util"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type LexDictResponse struct {
	Matches     []dictionary.Lemma `json:"matches"`
	Suggestions []string           `json:"suggestions"`
}

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
			variantInfo := make([]string, 0, len(ans))
			for _, item := range ans {
				variantInfo = append(variantInfo, fmt.Sprintf("%s/%s", item.Lemma, item.PoS))
			}
			log.Warn().
				Str("sublemma", sublemma).
				Str("pos", pos).
				Strs("variants", variantInfo).
				Int("numMatches", len(ans)).
				Msg("Multiple matches found for sublemma in corpus")
			entry := ans[0]
			for _, item := range ans[1:] {
				entry.PoS += item.PoS
				entry.Specifier += item.Specifier
			}
			return &entry, nil
		}
		return &ans[0], nil
	}
	return nil, nil
}

func (actions *Handler) SearchWord(ctx *gin.Context) {
	corpusId := ctx.Param("corpusId")
	searchTerm := ctx.Param("term")

	typoSuggestions, err := SearchTypoSuggestions(ctx, actions.db.DB(), searchTerm)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	searchCandidates, err := actions.getSearchCandidates(ctx, corpusId, searchTerm)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	if len(searchCandidates) == 0 {
		uniresp.WriteJSONResponse(ctx.Writer, LexDictResponse{
			Matches:     []dictionary.Lemma{},
			Suggestions: typoSuggestions,
		})
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
		uniresp.WriteJSONResponse(ctx.Writer, LexDictResponse{
			Matches:     []dictionary.Lemma{},
			Suggestions: suggestions,
		})
		return
	}

	// apply special transformations before getting source data
	lexItems, err = ApplyTransformations(ctx, actions.db.DB(), actions.sourcePriority, usedCandidate.Source, lexItems,
		DTIJCR__MergeItems,
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
	lexItems, err = ApplyTransformations(ctx, actions.db.DB(), actions.sourcePriority, usedCandidate.Source, lexItems,
		DTIJCR__ResolvePos,
		IJP__C_Is_NAP,
		IJP__RemoveUninflected,
		IJP__ResolveVariantKey,
		SortTransformation(usedCandidate.Source),
		ResolveGroups,
	)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	// search corpus entry for each variant
	// if not found, create a new entry with minimal data
	variants := make([]dictionary.Lemma, 0, len(lexItems))
	for i, item := range lexItems {
		lemmatizedTerms, err := actions.getLematizedSearchCandidates(ctx, corpusId, item.Key.Lemma)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		// select the most similar candidate
		usedLemmatizedTerm := lemmatizedTerms[0]
		specifier := util.Ternary(usedLemmatizedTerm.isMultiWord(), "", cmp.Or(
			// corpus entry needs to replace "B" and "U" gender with "MI"
			util.Ternary(item.Key.Gender == GenderMascAnimInan || item.Key.Gender == GenderMascUnknown, "MI", item.Key.Gender),
			item.Key.Aspect,
		))
		pos := util.Ternary(usedLemmatizedTerm.isMultiWord(), "", item.Key.Pos)

		var corpusEntries []*dictionary.Lemma
		var corpusEntry *dictionary.Lemma
		for _, term := range strings.Fields(usedLemmatizedTerm.Value) {
			corpusEntry, err = actions.searchCorpusEntry(ctx, corpusId, term, pos)
			if err != nil {
				uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
				return
			}

			if corpusEntry == nil {
				corpusEntry = &dictionary.Lemma{
					ID:        "lex",
					Lemma:     term,
					PoS:       pos,
					Specifier: specifier,
					Forms:     []dictionary.Form{{Value: term, Sublemma: term}},
					Sublemmas: []dictionary.Sublemma{{Value: term}},
				}

			} else {
				corpusEntry.ID = "corp"
				corpusEntry.Specifier = cmp.Or(corpusEntry.Specifier, specifier)
			}
			corpusEntries = append(corpusEntries, corpusEntry)
		}

		if len(corpusEntries) > 1 {
			mergedSublemmas := make([]string, 0, len(corpusEntries))
			mergedLemmas := make([]string, 0, len(corpusEntries))
			mergedPos := make([]string, 0, len(corpusEntries))
			mergedSpecifiers := make([]string, 0, len(corpusEntries))
			allCorp := true
			for _, subentry := range corpusEntries {
				if allCorp && !strings.HasPrefix(subentry.ID, "corp") {
					allCorp = false
				}
				mergedLemmas = append(mergedLemmas, subentry.Lemma)
				mergedPos = append(mergedPos, subentry.PoS)
				mergedSpecifiers = append(mergedSpecifiers, subentry.Specifier)
				if len(subentry.Sublemmas) > 0 && subentry.Sublemmas[0].Value != "" {
					mergedSublemmas = append(mergedSublemmas, subentry.Sublemmas[0].Value)
				} else if subentry.Lemma != "" {
					mergedSublemmas = append(mergedSublemmas, subentry.Lemma)
				}
			}
			mergedLemmaText := strings.Join(mergedLemmas, " ")
			mergedSublemmaText := strings.Join(mergedSublemmas, " ")
			corpusEntry = &dictionary.Lemma{
				ID:        fmt.Sprintf("corp-%d", i),
				Lemma:     mergedLemmaText,
				PoS:       strings.Join(mergedPos, " "),
				Specifier: strings.Join(mergedSpecifiers, " "),
				Forms:     []dictionary.Form{{Value: item.Key.Lemma, Sublemma: mergedSublemmaText}},
				Sublemmas: []dictionary.Sublemma{{Value: mergedSublemmaText}},
			}
			if !allCorp {
				corpusEntry.ID = fmt.Sprintf("lex-%d", i)
			}

		} else {
			if corpusEntry.ID == "corp" {
				item.Sources[SourceCorpus] = []LexID{
					{Key: LexKey{
						Lemma:  corpusEntry.Lemma,
						Pos:    corpusEntry.PoS,
						Gender: util.Ternary(corpusEntry.PoS == PosNoun, corpusEntry.Specifier, ""),
						Aspect: util.Ternary(corpusEntry.PoS == PosVerb, corpusEntry.Specifier, ""),
					}},
				}
				// use corpus PoS if pos source is unknown
				if item.PosSource == SourceEmpty {
					item.PosSource = SourceCorpus
				}
			}
			corpusEntry.ID = fmt.Sprintf("%s-%d", corpusEntry.ID, i)
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

	uniresp.WriteJSONResponse(ctx.Writer, LexDictResponse{
		Matches:     variants,
		Suggestions: suggestions,
	})
}

func NewHandler(db *mysql.Adapter, dictActions *dictActions.Actions, sourcePriority []Source) *Handler {
	return &Handler{
		db:             db,
		dictActions:    dictActions,
		sourcePriority: sourcePriority,
	}
}

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
)

type LexDictResponse struct {
	Matches     []dictionary.Lemma `json:"matches"`
	Suggestions []string           `json:"suggestions"`
}

type LexExtraData struct {
	CorpusId string  `json:"corpusId"`
	Variant  LexItem `json:"variant"`
}

type Handler struct {
	db             *mysql.Adapter
	dictActions    *dictActions.Actions
	sourcePriority []Source
}

func (actions *Handler) addOtherSublemmaVariants(ctx context.Context, corpusId string, variants []dictionary.Lemma) ([]dictionary.Lemma, error) {
	result := make([]dictionary.Lemma, 0, len(variants))
	result = append(result, variants...)

	for _, variant := range variants {
		if len(variant.Sublemmas) == 0 {
			continue
		}
		currentSublemma := variant.Sublemmas[0].Value
		otherVariants, err := actions.searchOtherSublemmata(ctx, corpusId, variant.Lemma, currentSublemma, variant.PoS)
		if err != nil {
			return nil, err
		}
		for _, otherVariant := range otherVariants {
			if len(otherVariant.Sublemmas) == 0 {
				continue
			}
			otherSublemma := otherVariant.Sublemmas[0].Value
			// lemma is not compared - entries not found in corpus use lexicon lemma,
			// which may differ from corpus lemma (e.g. lex "rekursivní" vs. corp "rekurzivní")
			if collections.SliceFindIndex(result, func(item dictionary.Lemma) bool {
				return item.PoS == otherVariant.PoS &&
					len(item.Sublemmas) > 0 && item.Sublemmas[0].Value == otherSublemma
			}) != -1 {
				continue
			}
			candidate := otherVariant
			candidate.ID = fmt.Sprintf("corp-%d", len(result)-len(variants))
			candidate.ExtraData = LexExtraData{
				CorpusId: corpusId,
				Variant: LexItem{
					Key: LexKey{
						Lemma:  otherSublemma,
						Pos:    candidate.PoS,
						Gender: util.Ternary(candidate.PoS == PosNoun, candidate.Specifier, ""),
						Aspect: util.Ternary(candidate.PoS == PosVerb, candidate.Specifier, ""),
					},
					PosSource:     SourceCorpus,
					VariantSource: SourceCorpus,
					Sources: map[Source][]LexID{
						SourceCorpus: {{
							Key: LexKey{
								Lemma:  otherSublemma,
								Pos:    candidate.PoS,
								Gender: util.Ternary(candidate.PoS == PosNoun, candidate.Specifier, ""),
								Aspect: util.Ternary(candidate.PoS == PosVerb, candidate.Specifier, ""),
							},
						}},
					},
					// corpus variants are special group
					// set to something high, but different than other variants
					Group: 999,
				},
			}
			result = append(result, candidate)
		}
	}
	return result, nil
}

func (actions *Handler) generatePossibleSearchTerms(searchTerm string) []string {
	baseTerm := searchTerm
	parts := strings.Fields(searchTerm)
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		if collections.SliceContains([]string{"si", "sis", "se", "ses"}, lastPart) {
			baseTerm = strings.Join(parts[:len(parts)-1], " ")
		}
	}
	return []string{
		baseTerm,
		baseTerm + " se",
		baseTerm + " si",
	}
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
func (actions *Handler) SearchWord(ctx *gin.Context) {
	corpusId := ctx.Param("corpusId")
	searchTerm := ctx.Param("term")
	if err := ValidateCorpusName(corpusId); err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadRequest)
		return
	}

	var err error
	var searchCandidates []LemmatizedTerm
	typoSuggestions := make([]string, 0, 10)
	for _, searchTermVariant := range actions.generatePossibleSearchTerms(searchTerm) {
		if values, err := SearchTypoSuggestions(ctx, actions.db.DB(), searchTermVariant); err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		} else {
			for _, v := range values {
				if !collections.SliceContains(typoSuggestions, v) {
					typoSuggestions = append(typoSuggestions, v)
				}
			}
		}

		if values, err := actions.getSearchCandidates(ctx, corpusId, searchTermVariant); err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		} else if len(values) > 0 {
			searchCandidates = values
			break
		}
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
	suggestions := append(collections.SliceMap(searchCandidates[1:], func(item LemmatizedTerm, i int) string {
		return item.Term
	}), typoSuggestions...)

	var lexItems []LexItem
	// get term variants for selected candidate
	for _, searchTermVariant := range actions.generatePossibleSearchTerms(usedCandidate.Term) {
		searchCandidat := usedCandidate
		searchCandidat.Term = searchTermVariant
		data, err := SearchVariants(ctx, actions.db.DB(), searchCandidat)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		lexItems = append(lexItems, data...)
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
		usedLemmatizedTerm, err := LemmatizeTermWithLex(ctx, actions.db.DB(), item.Key.Lemma)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		if usedLemmatizedTerm == nil {
			lemmatizedTerms, err := LemmatizeTermWithCorpus(ctx, actions.db.DB(), corpusId, item.Key.Lemma)
			if err != nil {
				uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
				return
			} else if len(lemmatizedTerms) == 0 {
				usedLemmatizedTerm = &LemmatizedTerm{Term: item.Key.Lemma, Lemma: item.Key.Lemma, Sublemma: item.Key.Lemma}
			} else {
				usedLemmatizedTerm = &lemmatizedTerms[0]
			}
		}

		specifier := util.Ternary(usedLemmatizedTerm.isMultiWord(), "", cmp.Or(
			// corpus entry needs to replace "B" and "U" gender with "MI"
			util.Ternary(item.Key.Gender == GenderMascAnimInan || item.Key.Gender == GenderMascUnknown, "MI", item.Key.Gender),
			item.Key.Aspect,
		))

		var corpusEntries []*dictionary.Lemma
		var corpusEntry *dictionary.Lemma
		// use lemmatized term only for multi-word terms, otherwise use the lemma from lexicon
		searchLemma := util.Ternary(usedLemmatizedTerm.isMultiWord(), usedLemmatizedTerm.Lemma, item.Key.Lemma)
		searchSublemma := util.Ternary(usedLemmatizedTerm.isMultiWord(), usedLemmatizedTerm.Sublemma, item.Key.Lemma)
		searchPos := util.Ternary(usedLemmatizedTerm.isMultiWord(), "", item.Key.Pos)
		lemmaValues := strings.Fields(searchLemma)
		sublemmaValues := strings.Fields(searchSublemma)
		for i, sublemma := range sublemmaValues {
			lemma := sublemma
			if i < len(lemmaValues) {
				lemma = lemmaValues[i]
			}

			corpusEntry, err = actions.searchCorpusEntry(ctx, corpusId, lemma, sublemma, searchPos)
			if err != nil {
				uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
				return
			}

			if corpusEntry == nil {
				corpusEntry = &dictionary.Lemma{
					ID:        "lex",
					Lemma:     sublemma,
					PoS:       searchPos,
					Specifier: specifier,
					Forms:     []dictionary.Form{{Value: sublemma, Sublemma: sublemma}},
					Sublemmas: []dictionary.Sublemma{{Value: sublemma}},
				}

			} else {
				corpusEntry.ID = "corp"
				corpusEntry.Specifier = cmp.Or(corpusEntry.Specifier, specifier)
			}
			corpusEntries = append(corpusEntries, corpusEntry)
		}
		if corpusEntry == nil {
			corpusEntry = &dictionary.Lemma{
				ID:        "lex",
				Lemma:     item.Key.Lemma,
				PoS:       searchPos,
				Specifier: specifier,
				Forms:     []dictionary.Form{{Value: item.Key.Lemma, Sublemma: item.Key.Lemma}},
				Sublemmas: []dictionary.Sublemma{{Value: item.Key.Lemma}},
			}
		}

		// create multi word corpus entry from multiple single word entries
		if len(corpusEntries) > 1 {
			corpusEntry = actions.mergeMultiWordCorpusEntries(item, corpusEntries)
		}

		if corpusEntry.ID == "corp" {
			item.Sources[SourceCorpus] = []LexID{
				{Key: LexKey{
					Lemma: corpusEntry.Sublemmas[0].Value,
					Pos:   corpusEntry.PoS,
					// TODO, get specifier from corpus
					Gender: util.Ternary(corpusEntry.PoS == PosNoun, corpusEntry.Specifier, ""),
					Aspect: util.Ternary(corpusEntry.PoS == PosVerb, corpusEntry.Specifier, ""),
				}},
			}
			// use corpus PoS if pos source is unknown
			if item.PosSource == SourceEmpty && len(corpusEntries) > 0 {
				item.PosSource = SourceCorpus
			}
		}
		corpusEntry.ID = fmt.Sprintf("%s-%d", corpusEntry.ID, i)

		if collections.SliceFindIndex(corpusEntry.Sublemmas, func(sublemma dictionary.Sublemma) bool {
			return sublemma.Value == usedCandidate.Sublemma
		}) != -1 && collections.SliceFindIndex(corpusEntry.Forms, func(form dictionary.Form) bool {
			return form.Value == searchTerm
		}) == -1 {
			corpusEntry.Forms = append(corpusEntry.Forms, dictionary.Form{Value: searchTerm, Sublemma: usedLemmatizedTerm.Sublemma})
		}

		corpusEntry.ExtraData = LexExtraData{
			CorpusId: corpusId,
			Variant:  item,
		}
		variants = append(variants, *corpusEntry)
		// remove variant from suggestions if present
		suggestions = collections.SliceFilter(suggestions, func(v string, i int) bool { return v != item.Key.Lemma })
	}

	variants, err = actions.addOtherSublemmaVariants(ctx, corpusId, variants)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
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

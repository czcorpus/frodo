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
	"context"
	"database/sql"
	"fmt"

	"github.com/czcorpus/cnc-gokit/collections"
)

type LexTransform func(context.Context, *sql.DB, []LexItem) ([]LexItem, error)

func samePos(data []LexID) bool {
	for _, item := range data {
		if item.Pos != data[0].Pos {
			return false
		}
	}
	return true
}

func ApplyTransformations(ctx context.Context, db *sql.DB, data []LexItem, transforms ...LexTransform) ([]LexItem, error) {
	var err error
	for _, transform := range transforms {
		if transform == nil {
			continue
		}
		data, err = transform(ctx, db, data)
		if err != nil {
			return nil, fmt.Errorf("failed to transform data: %w", err)
		}

	}
	return data, nil
}

func SortTransformation(sortBySource Source) func(ctx context.Context, db *sql.DB, data []LexItem) ([]LexItem, error) {
	return func(ctx context.Context, db *sql.DB, data []LexItem) ([]LexItem, error) {
		return sortVariants2(data, sortBySource), nil
	}
}

func DTIJCR_MergeItems(ctx context.Context, db *sql.DB, data []LexItem) ([]LexItem, error) {
	// making DTIJCR group from any DTIJ, D, T, I, J, R item
	var result []LexItem
	for _, item := range data {
		if item.Key.Pos != PosNum {
			if item.Key.Pos == PosDTIJ ||
				item.Key.Pos == PosAdv ||
				item.Key.Pos == PosPart ||
				item.Key.Pos == PosInter ||
				item.Key.Pos == PosConj ||
				item.Key.Pos == PosPrep {
				item.Key.Pos = PosDTIJCR
			}
			if collections.SliceFindIndex(result, func(v LexItem) bool { return item.Key == v.Key }) == -1 {
				result = append(result, item)
			}
		}
	}
	// join C item to DTIJCR group, if some exists
	for _, item := range data {
		if item.Key.Pos == PosNum {
			item.Key.Pos = PosDTIJCR
			if collections.SliceFindIndex(result, func(v LexItem) bool { return item.Key == v.Key }) == -1 {
				item.Key.Pos = PosNum
				result = append(result, item)
			}
		}
	}

	return result, nil
}

func DTIJCR_ResolvePos(sourcePriority []Source) func(ctx context.Context, db *sql.DB, data []LexItem) ([]LexItem, error) {
	// reduce DTIJCR to only one value, if possible
	return func(ctx context.Context, db *sql.DB, data []LexItem) ([]LexItem, error) {
		for i, item := range data {
			if item.Key.Pos == PosDTIJCR {
				for _, source := range sourcePriority {
					if v, ok := item.Sources[source]; ok && samePos(v) {
						data[i].PosSource = source
						data[i].Key.Pos = v[0].Pos
						break
					}
				}
			}
		}
		return data, nil
	}
}

func IJP_ResolvePos(sourcePriority []Source) func(ctx context.Context, db *sql.DB, data []LexItem) ([]LexItem, error) {
	// IJP should never be source of PoS
	// select highest available source from sourcePriority list
	return func(ctx context.Context, db *sql.DB, data []LexItem) ([]LexItem, error) {
		for i, item := range data {
			if item.PosSource == SourceIJP {
				for _, source := range sourcePriority {
					if v, ok := item.Sources[source]; source != SourceIJP && ok && samePos(v) {
						data[i].PosSource = source
						data[i].Key.Pos = v[0].Pos
						break
					}
				}
			}
		}
		return data, nil
	}
}

func JoinFromIJP_NAP_To_C(ctx context.Context, db *sql.DB, data []LexItem) ([]LexItem, error) {
	// if data pos == C and not uninflected and no IJP source
	// add to data IJP source with pos N|A|P
	for i, item := range data {
		if !item.HasSource(SourceIJP) && item.Key.Pos == PosNum && !item.Key.Uninflected {
			for _, pos := range []string{PosNoun, PosAdj, PosPron} {
				search := LexKey{
					Lemma:       item.Key.Lemma,
					Pos:         pos,
					Gender:      GenderUnknown,
					Aspect:      AspectUnknown,
					Uninflected: false,
					Plurality:   PluralityUnknown,
				}
				ids, err := SearchLexItemID(ctx, db, search, SourceIJP)
				if err != nil {
					return nil, fmt.Errorf("failed to join N|A|P to C in IJP: %w", err)
				}
				if len(ids) != 0 {
					data[i].Sources[SourceIJP] = ids
				}
			}
		}
	}
	return data, nil
}

func JoinToIJP_C_To_NAP(ctx context.Context, db *sql.DB, data []LexItem) ([]LexItem, error) {
	for i, item := range data {
		if item.PosSource == SourceIJP && (item.Key.Pos == PosNoun || item.Key.Pos == PosAdj || item.Key.Pos == PosPron) && len(item.Sources) == 1 {
			search := LexKey{
				Lemma:       item.Key.Lemma,
				Pos:         PosNum,
				Gender:      GenderUnknown,
				Aspect:      AspectUnknown,
				Uninflected: false,
				Plurality:   PluralityUnknown,
			}
			sources, err := SearchSources(ctx, db, search)
			if err != nil {
				return nil, fmt.Errorf("failed to search C for N|A|P in IJP: %w", err)
			}
			if len(sources) > 0 {
				sources[SourceIJP] = item.Sources[SourceIJP]
				data[i].Sources = sources
			}
		}
	}
	return data, nil
}

func JoinFromSSC_M_To_IB(ctx context.Context, db *sql.DB, data []LexItem) ([]LexItem, error) {
	// if data gender == I || B and no SSC source
	// add to data SSC source with gender M
	// (SSC source does not distinct masculine genders)
	for i, item := range data {
		if !item.HasSource(SourceSSC) && (item.Key.Gender == GenderMascInan || item.Key.Gender == GenderMascAnimInan) {
			search := LexKey{
				Lemma:       item.Key.Lemma,
				Pos:         item.Key.Pos,
				Gender:      GenderMascAnim,
				Aspect:      item.Key.Aspect,
				Uninflected: item.Key.Uninflected,
				Plurality:   0,
			}
			ids, err := SearchLexItemID(ctx, db, search, SourceSSC)
			if err != nil {
				return nil, fmt.Errorf("failed to join masculine gender SSC data: %w", err)
			}
			if len(ids) != 0 {
				data[i].Sources[SourceSSC] = ids
			}
		}
	}
	return data, nil
}

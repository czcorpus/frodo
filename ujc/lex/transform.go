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
	"github.com/rs/zerolog/log"
)

type LexTransform func(context.Context, *sql.DB, Source, []LexItem) ([]LexItem, error)

func samePos(sources []LexID) bool {
	for _, item := range sources {
		if item.Key.Pos != sources[0].Key.Pos {
			return false
		}
	}
	return true
}

func samePlurality(sources []LexID) bool {
	for _, item := range sources {
		if item.Key.Plurality != sources[0].Key.Plurality {
			return false
		}
	}
	return true
}

func ApplyTransformations(ctx context.Context, db *sql.DB, variantSource Source, variants []LexItem, transforms ...LexTransform) ([]LexItem, error) {
	var err error
	for _, transform := range transforms {
		if transform == nil {
			continue
		}
		variants, err = transform(ctx, db, variantSource, variants)
		if err != nil {
			return nil, fmt.Errorf("failed to transform data: %w", err)
		}

	}
	return variants, nil
}

func Identity(ctx context.Context, db *sql.DB, variantSource Source, variants []LexItem) ([]LexItem, error) {
	return variants, nil
}

func SortTransformation(sortBySource Source) func(ctx context.Context, db *sql.DB, variantSource Source, variants []LexItem) ([]LexItem, error) {
	return func(ctx context.Context, db *sql.DB, variantSource Source, variants []LexItem) ([]LexItem, error) {
		return sortVariants2(variants, sortBySource), nil
	}
}

func DTIJCR_MergeItems(ctx context.Context, db *sql.DB, variantSource Source, variants []LexItem) ([]LexItem, error) {
	// making DTIJCR group from any DTIJ, D, T, I, J, R item
	var result []LexItem
	for _, item := range variants {
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
	for _, item := range variants {
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

func DTIJCR_ResolvePos(sourcePriority []Source) func(ctx context.Context, db *sql.DB, variantSource Source, variants []LexItem) ([]LexItem, error) {
	// reduce DTIJCR to only one value, if possible
	return func(ctx context.Context, db *sql.DB, variantSource Source, variants []LexItem) ([]LexItem, error) {
		for i, item := range variants {
			if item.Key.Pos == PosDTIJCR {
				for _, source := range sourcePriority {
					if v, ok := item.Sources[source]; ok {
						variants[i].PosSource = source
						if samePos(v) {
							variants[i].Key.Pos = v[0].Key.Pos
						}
						break
					}
				}
			}
		}
		return variants, nil
	}
}

func IJP_ResolveVariantKey(sourcePriority []Source) func(ctx context.Context, db *sql.DB, variantSource Source, variants []LexItem) ([]LexItem, error) {
	// IJP should never be source of PoS/plurality
	// select highest available source from sourcePriority list
	return func(ctx context.Context, db *sql.DB, variantSource Source, variants []LexItem) ([]LexItem, error) {
		for i, item := range variants {
			if item.PosSource == SourceIJP {
				for _, source := range sourcePriority {
					if v, ok := item.Sources[source]; source != SourceIJP && ok && samePos(v) {
						item.PosSource = source
						item.Key.Pos = v[0].Key.Pos
						if samePlurality(v) {
							item.Key.Plurality = v[0].Key.Plurality
						} else {
							log.Debug().Any("key", item.Key).Msgf("found multiple plurality items, can not resolve")
						}
						break
					}
				}
				if item.PosSource == SourceIJP {
					item.PosSource = SourceEmpty
				}
				variants[i] = item
			}
		}
		return variants, nil
	}
}

func JoinFromIJP_NAP_To_C(ctx context.Context, db *sql.DB, variantSource Source, variants []LexItem) ([]LexItem, error) {
	// if data pos == C and not uninflected and no IJP source
	// add to data IJP source with pos N|A|P
	for i, item := range variants {
		if !item.HasSource(SourceIJP) && item.Key.Pos == PosNum && item.Key.Uninflected == UninflectedFalse {
			for _, pos := range []string{PosNoun, PosAdj, PosPron} {
				search := LexKey{
					Lemma:       item.Key.Lemma,
					Pos:         pos,
					Gender:      GenderUnknown,
					Aspect:      AspectUnknown,
					Uninflected: UninflectedFalse,
					Plurality:   PluralityUnknown,
				}
				ids, err := SearchLexItemID(ctx, db, search, SourceIJP)
				if err != nil {
					return nil, fmt.Errorf("failed to join N|A|P to C in IJP: %w", err)
				}
				if len(ids) != 0 {
					variants[i].Sources[SourceIJP] = ids
				}
			}
		}
	}
	return variants, nil
}

func JoinToIJP_C_To_NAP(ctx context.Context, db *sql.DB, variantSource Source, variants []LexItem) ([]LexItem, error) {
	for i, item := range variants {
		if item.PosSource == SourceIJP && (item.Key.Pos == PosNoun || item.Key.Pos == PosAdj || item.Key.Pos == PosPron) && len(item.Sources) == 1 {
			search := LexKey{
				Lemma:       item.Key.Lemma,
				Pos:         PosNum,
				Gender:      GenderUnknown,
				Aspect:      AspectUnknown,
				Uninflected: UninflectedFalse,
				Plurality:   PluralityUnknown,
			}
			sources, err := SearchSources(ctx, db, search)
			if err != nil {
				return nil, fmt.Errorf("failed to search C for N|A|P in IJP: %w", err)
			}
			if len(sources) > 0 {
				sources[SourceIJP] = item.Sources[SourceIJP]
				variants[i].Sources = sources
			}
		}
	}
	return variants, nil
}

func IJP_RemoveUninflected(ctx context.Context, db *sql.DB, variantSource Source, variants []LexItem) ([]LexItem, error) {
	// remove ijp source from uninflected data if inflected variant exists
	if variantSource != SourceIJP {
		for i, item := range variants {
			if item.Key.Uninflected == UninflectedTrue && item.HasSource(SourceIJP) {
				search := item.Key
				search.Uninflected = UninflectedFalse
				if hasVariant(search, variants) {
					delete(variants[i].Sources, SourceIJP)
				}
			}
		}
	}
	return variants, nil
}

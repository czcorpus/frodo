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
)

type LexTransform func(context.Context, *sql.DB, []LexItem) ([]LexItem, error)

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

func JoinToPluarlityFromIJP(ctx context.Context, db *sql.DB, data []LexItem) ([]LexItem, error) {
	for i, item := range data {
		_, ok := item.Sources[SourceIJP]
		if item.Plurality != 0 && !ok {
			search := LexItem{
				Lemma:       item.Lemma,
				Pos:         item.Pos,
				Gender:      item.Gender,
				Aspect:      item.Aspect,
				Uninflected: false, // TODO
				Plurality:   0,
			}
			ids, err := SearchLexItemID(ctx, db, search, SourceIJP)
			if err != nil {
				return nil, fmt.Errorf("failed to join inflected IJP data: %w", err)
			}
			data[i].Sources[SourceIJP] = ids
		}
	}
	return data, nil
}

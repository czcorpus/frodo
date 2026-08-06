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

package ssc

import (
	"context"
	"database/sql"
	"fmt"
	"frodo/ujc/lex"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

func InsertDictChunk(ctx context.Context, tx *sql.Tx, data []SrcFileRow) error {
	var insTpl strings.Builder
	dataArgs := make([]any, 0, len(data)*7)
	for i, v := range data {
		if i > 0 {
			insTpl.WriteString(", ")
		}
		insTpl.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		gender := sql.NullString{String: v.Gender, Valid: v.Gender != ""}
		aspect := sql.NullString{String: v.Aspect, Valid: v.Aspect != ""}
		plurality := sql.NullString{String: v.Plurality, Valid: v.Plurality != ""}
		parentID := sql.NullString{String: v.ParentID, Valid: v.ParentID != ""}
		sortOrder, err := strconv.Atoi(v.SortOrder)
		if err != nil {
			log.Fatal().Msgf("Invalid sort order %s", v.SortOrder)
		}
		dataArgs = append(dataArgs, v.EntryID, v.Homonymy, sortOrder-1, v.Variant, v.Pos, gender, aspect, v.Uninflected, plurality, lex.SourceASSC, v.EntryID, parentID)
	}
	_, err := tx.ExecContext(
		ctx,
		fmt.Sprintf(
			"INSERT INTO lex_dictionary (group_id, homonym, group_order, lemma, pos, gender, aspect, uninflected, plurality, source, external_id, external_parent_id) VALUES %s",
			insTpl.String(),
		),
		dataArgs...,
	)
	if err != nil {
		log.Warn().Err(err).Msg("failed to insert row chunk, trying one by one")
		// try one by one and ignore errors:
		for _, item := range data {
			gender := sql.NullString{String: item.Gender, Valid: item.Gender != ""}
			aspect := sql.NullString{String: item.Aspect, Valid: item.Aspect != ""}
			plurality := sql.NullString{String: item.Plurality, Valid: item.Plurality != ""}
			parentID := sql.NullString{String: item.ParentID, Valid: item.ParentID != ""}
			sortOrder, err := strconv.Atoi(item.SortOrder)
			if err != nil {
				log.Fatal().Msgf("Invalid sort order %s", item.SortOrder)
			}
			_, err = tx.ExecContext(
				ctx,
				"INSERT INTO lex_dictionary (group_id, homonym, group_order, lemma, pos, gender, aspect, uninflected, plurality, source, external_id, external_parent_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ",
				item.EntryID, item.Homonymy, sortOrder-1, item.Variant, item.Pos, gender, aspect, item.Uninflected, plurality, lex.SourceSSC, item.EntryID, parentID,
			)
			if err != nil {
				log.Error().Err(err).Any("values", item).Msg("failed to insert single row, ignoring")
			}

		}

	}
	return nil
}

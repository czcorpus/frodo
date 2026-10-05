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

package ijp

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
		insTpl.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		groupId := sql.NullString{String: v.GroupID, Valid: v.GroupID != ""}

		externalIDParts := strings.Split(strings.TrimPrefix(v.ExternalID, "__"), "_")
		homonym := 0
		if len(externalIDParts) > 1 {
			if h, err := strconv.Atoi(externalIDParts[len(externalIDParts)-1]); err == nil {
				homonym = h
			}
		}

		groupOrder := 0
		if strings.HasPrefix(v.ExternalID, "__") {
			if gro, err := strconv.Atoi(externalIDParts[1]); err == nil {
				groupOrder = gro
			}
		}

		gender := sql.NullString{String: v.Gender, Valid: v.Gender != ""}
		aspect := sql.NullString{String: v.Aspect, Valid: v.Aspect != ""}
		plurality := lex.PluralityUnknown
		uninflected := lex.UninflectedFalse
		dataArgs = append(dataArgs, groupId, homonym, groupOrder, v.Variant, v.Pos, gender, aspect, plurality, uninflected, lex.SourceIJP, v.ExternalID)
	}
	_, err := tx.ExecContext(
		ctx,
		fmt.Sprintf(
			"INSERT INTO lex_dictionary (group_id, homonym, group_order, lemma, pos, gender, aspect, plurality, uninflected, source, external_id) VALUES %s",
			insTpl.String(),
		),
		dataArgs...,
	)
	if err != nil {
		log.Warn().Err(err).Msg("failed to insert row chunk, trying one by one")
		// try one by one and ignore errors:
		for _, item := range data {
			groupId := sql.NullString{String: item.GroupID, Valid: item.GroupID != ""}

			externalIDParts := strings.Split(strings.TrimPrefix("__", item.ExternalID), "_")
			homonym := 0
			if len(externalIDParts) > 1 {
				if h, err := strconv.Atoi(externalIDParts[len(externalIDParts)-1]); err == nil {
					homonym = h
				}
			}

			groupOrder := 0
			if strings.HasPrefix(item.ExternalID, "__") {
				if gro, err := strconv.Atoi(externalIDParts[1]); err == nil {
					groupOrder = gro
				}
			}

			gender := sql.NullString{String: item.Gender, Valid: item.Gender != ""}
			aspect := sql.NullString{String: item.Aspect, Valid: item.Aspect != ""}
			plurality := lex.PluralityUnknown
			uninflected := lex.UninflectedFalse
			_, err := tx.ExecContext(
				ctx,
				"INSERT INTO lex_dictionary (group_id, homonym, group_order, lemma, pos, gender, aspect, plurality, uninflected, source, external_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ",
				groupId, homonym, groupOrder, item.Variant, item.Pos, gender, aspect, plurality, uninflected, lex.SourceIJP, item.ExternalID,
			)
			if err != nil {
				log.Error().Err(err).Any("values", item).Msg("failed to insert single row, ignoring")
			}

		}

	}
	return nil
}

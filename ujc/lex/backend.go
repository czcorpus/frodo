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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/czcorpus/cnc-gokit/collections"
)

type Source string

const (
	SourceASSC Source = "assc"
	SourceIJP  Source = "ijp"
	SourceSSJC Source = "ssjc"
	SourceSJC  Source = "sjc"

	POSAdj   = "A"
	POSAbb   = "B"
	PosNum   = "C"
	POSAdv   = "D"
	POSFore  = "F"
	POSInter = "I"
	POSConj  = "J"
	POSNoun  = "N"
	POSPron  = "P"
	POSPrep  = "R"
	POSSegm  = "S"
	POSPart  = "T"
	POSVerb  = "V"
	POSUnkn  = "X"
	POSPunc  = "Z"
	POSDTIJ  = "DTIJ"

	GenderMascAnim     = "M"
	GenderMascInan     = "I"
	GenderMascAnimInan = "B"
	GenderFem          = "F"
	GenderNeut         = "N"

	AspectPerf = "P"
	AspectImp  = "I"
	AspectBoth = "B"

	UninflectedFalse = 0
	UninflectedTrue  = 1

	PluralityNone    = 0
	PluralityPlural  = 1
	PluralityAlways  = 2
	PluralityUsually = 3
	PluralityOnly    = 4

	TableName = "lex_dictionary"
)

var dictionaryTable = `
CREATE TABLE %s (
	group_id VARCHAR(100),
	homonym TINYINT DEFAULT 0 NOT NULL,
	group_order TINYINT DEFAULT 0 NOT NULL,

	lemma VARCHAR(100) NOT NULL,
	pos VARCHAR(4) NOT NULL,
	gender VARCHAR(1),
	aspect VARCHAR(1),
	uninflected TINYINT DEFAULT 0 NOT NULL,
	plurality TINYINT DEFAULT 0 NOT NULL,
	
	source VARCHAR(8) NOT NULL,
	external_id VARCHAR(100) NOT NULL,
	external_parent_id VARCHAR(100),

	-- This column automatically calculates the normalized search key
	search_key VARCHAR(100) COLLATE utf8mb4_unicode_ci GENERATED ALWAYS AS (
		REPLACE(REPLACE(LOWER(lemma), 'y', 'i'), 'z', 's')
	) STORED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;`

func CreateTables(ctx context.Context, db *sql.DB) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", TableName)); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(dictionaryTable, TableName)); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	return tx, nil
}

func SearchTypoSuggestions(ctx context.Context, db *sql.DB, term string) ([]string, error) {
	row, err := db.QueryContext(
		ctx,
		"SELECT DISTINCT lemma "+
			"FROM lex_dictionary "+
			"WHERE search_key = REPLACE(REPLACE(LOWER(?), 'y', 'i'), 'z', 's');",
		term,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search match: %w", err)
	}
	defer row.Close()

	suggestions := make([]string, 0, 5)
	for row.Next() {
		var lemma string
		if err := row.Scan(&lemma); err != nil {
			if err == sql.ErrNoRows {
				return suggestions, nil
			}
			return nil, fmt.Errorf("failed to scan suggestions: %w", err)
		}
		suggestions = append(suggestions, lemma)
	}
	suggestions = collections.SliceFilter(suggestions, func(v string, i int) bool {
		return !strings.EqualFold(v, term)
	})
	return suggestions, nil
}

func SearchAvailableSources(ctx context.Context, db *sql.DB, lemma string) ([]Source, error) {
	row, err := db.QueryContext(
		ctx,
		"SELECT DISTINCT source "+
			"FROM lex_dictionary "+
			"WHERE lemma = ?;",
		lemma,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search available sources: %w", err)
	}
	defer row.Close()

	sources := make([]Source, 0)
	for row.Next() {
		var source Source
		if err := row.Scan(&source); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to scan available source: %w", err)
		}
		sources = append(sources, source)
	}

	return sources, nil
}

func SearchVariants(ctx context.Context, db *sql.DB, lemma string, mainSource Source) ([]LexItem, error) {
	row, err := db.QueryContext(
		ctx,
		`
		-- aggregate external ids to JSON array for each lemma and its variants, grouped by source
		SELECT lemma, pos, gender, aspect, uninflected, plurality, JSON_OBJECTAGG(source, idents) AS sources
		FROM (
			-- get external source identifiers for the lemma and its variants
			SELECT sub.lemma as lemma, sub.pos as pos, sub.gender as gender, sub.aspect as aspect, sub.uninflected as uninflected, sub.plurality as plurality, source, JSON_ARRAYAGG(JSON_OBJECT('id', external_id, 'parentId', external_parent_id, 'groupOrder', group_order, 'homonym', homonym) ORDER BY homonym) AS idents
			FROM (
				-- find available variants, get exact lemmata and their variants based on group_id and source
				SELECT DISTINCT lemma, pos, gender, aspect, uninflected, plurality
				FROM lex_dictionary AS l
				JOIN (
					SELECT DISTINCT group_id, source FROM lex_dictionary WHERE lemma = ? AND source = ? AND group_id IS NOT NULL
				) AS g
				ON g.group_id = l.group_id AND g.source = l.source
				UNION
				SELECT DISTINCT lemma, pos, gender, aspect, uninflected, plurality
				FROM lex_dictionary AS l
				WHERE lemma = ? AND source = ? AND group_id IS NULL
			) AS sub
			JOIN lex_dictionary AS l2
			ON l2.lemma = sub.lemma AND l2.pos = sub.pos AND (l2.gender = sub.gender OR (l2.gender IS NULL AND sub.gender IS NULL)) AND (l2.aspect = sub.aspect OR (l2.aspect IS NULL AND sub.aspect IS NULL)) AND l2.uninflected = sub.uninflected AND l2.plurality = sub.plurality
			GROUP BY lemma, pos, gender, aspect, uninflected, plurality, source
		) AS sub2
		GROUP BY lemma, pos, gender, aspect, uninflected, plurality`,
		lemma, mainSource, lemma, mainSource,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search the term: %w", err)
	}
	defer row.Close()

	data := make([]LexItem, 0, 5)
	for row.Next() {
		var genderArg, aspectArg sql.NullString
		var uninflectedArg int64
		var jsonSources string
		item := LexItem{}
		if err := row.Scan(&item.Lemma, &item.Pos, &genderArg, &aspectArg, &uninflectedArg, &item.Plurality, &jsonSources); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to scan the term: %w", err)
		}
		item.Uninflected = uninflectedArg != 0
		if genderArg.Valid {
			item.Gender = genderArg.String
		}
		if aspectArg.Valid {
			item.Aspect = aspectArg.String
		}
		// parse jsonIdents into srchItem.Idents
		if err := json.Unmarshal([]byte(jsonSources), &item.Sources); err != nil {
			return nil, fmt.Errorf("failed to search the term: %w", err)
		}
		data = append(data, item)
	}

	return data, nil
}

func PruneData(ctx context.Context, tx *sql.Tx, source Source) error {
	_, err := tx.ExecContext(
		ctx,
		"DELETE FROM lex_dictionary WHERE source = ?",
		source,
	)
	if err != nil {
		return err
	}
	return nil
}

func SearchLexItemID(ctx context.Context, db *sql.DB, lexItem LexItem, source Source) ([]LexID, error) {
	// Build WHERE clause dynamically so empty Gender/Aspect are searched as NULL
	where := make([]string, 0, 8)
	args := make([]interface{}, 0, 8)
	where = append(where, "lemma = ?")
	args = append(args, lexItem.Lemma)
	where = append(where, "pos = ?")
	args = append(args, lexItem.Pos)

	if lexItem.Gender == "" {
		where = append(where, "gender IS NULL")
	} else {
		where = append(where, "gender = ?")
		args = append(args, lexItem.Gender)
	}

	if lexItem.Aspect == "" {
		where = append(where, "aspect IS NULL")
	} else {
		where = append(where, "aspect = ?")
		args = append(args, lexItem.Aspect)
	}

	// uninflected stored as tinyint; convert bool to int
	uninflectedInt := 0
	if lexItem.Uninflected {
		uninflectedInt = 1
	}
	where = append(where, "uninflected = ?")
	args = append(args, uninflectedInt)

	where = append(where, "plurality = ?")
	args = append(args, lexItem.Plurality)

	where = append(where, "source = ?")
	args = append(args, source)

	query := "SELECT external_id, external_parent_id, group_order, homonym FROM lex_dictionary WHERE " + strings.Join(where, " AND ")

	row, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search the term: %w", err)
	}
	defer row.Close()

	lexIds := make([]LexID, 0, 1)
	for row.Next() {
		var lexId LexID
		var externalParentID sql.NullString
		if err := row.Scan(&lexId.ID, &externalParentID, &lexId.GroupOrder, &lexId.Homonym); err != nil {
			if err == sql.ErrNoRows {
				return lexIds, nil
			}
			return nil, fmt.Errorf("failed to scan the lex id: %w", err)
		}
		lexId.ParentID = externalParentID.String
		lexIds = append(lexIds, lexId)
	}

	return lexIds, nil
}

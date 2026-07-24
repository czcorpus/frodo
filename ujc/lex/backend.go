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
	"frodo/dictionary"
	"sort"
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

	POSOrder    = "NAPCVDRJTI"
	GenderOrder = "MIBFN"
	AspectOrder = "PIB"

	TableName = "lex_dictionary"
)

func morphologySort(item1 LexItem, item2 LexItem) bool {
	var orderMap, orderData1, orderData2 string
	if item1.Pos == "N" && item2.Pos == "N" {
		// order by gender if both items are nouns
		orderMap, orderData1, orderData2 = GenderOrder, item1.Gender, item2.Gender
	} else if item1.Pos == "V" && item2.Pos == "V" {
		// order by aspect if both items are verbs
		orderMap, orderData1, orderData2 = AspectOrder, item1.Aspect, item2.Aspect
	} else {
		// order by PoS for other items
		orderMap, orderData1, orderData2 = POSOrder, item1.Pos, item2.Pos
	}
	orderIndex1 := strings.Index(orderMap, orderData1)
	orderIndex2 := strings.Index(orderMap, orderData2)
	if orderIndex1 == -1 {
		orderIndex1 = len(orderMap)
	}
	if orderIndex2 == -1 {
		orderIndex2 = len(orderMap)
	}
	return orderIndex1 < orderIndex2
}

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

func SearchMatches(ctx context.Context, db *sql.DB, lemma string, source Source) ([]dictionary.Lemma, error) {
	// TODO this does not do much now, but we could implement fuzzy search to provide suggestions
	row, err := db.QueryContext(
		ctx,
		"SELECT DISTINCT lemma, pos "+
			"FROM lex_dictionary "+
			"WHERE lemma = ? AND source = ? ",
		lemma, source,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search match: %w", err)
	}
	defer row.Close()

	matches := make([]dictionary.Lemma, 0)
	i := 0
	for row.Next() {
		// just bare minimum for WaG to process the match
		match := dictionary.Lemma{
			ID:        fmt.Sprintf("lex-%s-%d", source, i),
			Forms:     make([]dictionary.Form, 0, 1),
			Sublemmas: make([]dictionary.Sublemma, 0, 1),
		}
		if err := row.Scan(&match.Lemma, &match.PoS); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to scan match: %w", err)
		}
		match.Forms = append(match.Forms, dictionary.Form{Value: match.Lemma, Sublemma: match.Lemma})
		match.Sublemmas = append(match.Sublemmas, dictionary.Sublemma{Value: match.Lemma})
		matches = append(matches, match)
	}

	return matches, nil
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

	// Get first items of groups
	firstGroupItems := collections.SliceReduce(data, func(acc []LexItem, curr LexItem, i int) []LexItem {
		groupIdx := collections.SliceFindIndex(acc, func(v LexItem) bool {
			return v.Sources[mainSource][0].ID == curr.Sources[mainSource][0].ID
		})
		if groupIdx == -1 {
			return append(acc, curr)
		}
		if acc[groupIdx].Sources[mainSource][0].GroupOrder > curr.Sources[mainSource][0].GroupOrder {
			acc[groupIdx] = curr
		}
		return acc
	}, make([]LexItem, 0, 5))

	// Sort first items of groups
	sort.Slice(firstGroupItems, func(i, j int) bool {
		// first order by Lemma
		if firstGroupItems[i].Lemma != firstGroupItems[j].Lemma {
			return firstGroupItems[i].Lemma < firstGroupItems[j].Lemma
		}
		// then by homonymy
		if firstGroupItems[i].Sources[mainSource][0].Homonym != firstGroupItems[j].Sources[mainSource][0].Homonym {
			return firstGroupItems[i].Sources[mainSource][0].Homonym < firstGroupItems[j].Sources[mainSource][0].Homonym
		}
		return morphologySort(firstGroupItems[i], firstGroupItems[j])
	})

	// groupID order map
	groupOrder := make(map[string]int)
	for i, v := range firstGroupItems {
		groupOrder[v.Sources[mainSource][0].ID] = i
	}

	// sort groups all data
	sort.Slice(data, func(i, j int) bool {
		if data[i].Sources[mainSource][0].ID != data[j].Sources[mainSource][0].ID {
			return groupOrder[data[i].Sources[mainSource][0].ID] < groupOrder[data[j].Sources[mainSource][0].ID]
		}
		if data[i].Sources[mainSource][0].GroupOrder != data[j].Sources[mainSource][0].GroupOrder {
			return data[i].Sources[mainSource][0].GroupOrder < data[j].Sources[mainSource][0].GroupOrder
		}
		return morphologySort(data[i], data[j])
	})

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

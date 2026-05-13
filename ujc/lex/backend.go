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

	"github.com/agnivade/levenshtein"
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

	GenderMascAnim = "M"
	GenderMascInan = "I"
	GenderFem      = "F"
	GenderNeut     = "N"

	AspectPerf = "P"
	AspectImp  = "I"
	AspectBoth = "B"

	TableName = "lex_dictionary"
)

var dictionaryTable = `
CREATE TABLE %s (
	group_id INT NOT NULL,
	homonym INT,

	lemma VARCHAR(100) NOT NULL,
	pos VARCHAR(1) NOT NULL,
	gender VARCHAR(1),
	aspect VARCHAR(1),
	
	source VARCHAR(8) NOT NULL,
	external_id VARCHAR(100) NOT NULL,
	external_parent_id VARCHAR(100)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_czech_ci`

func CreateTables(ctx context.Context, db *sql.DB) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", TableName)); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(dictionaryTable, TableName, TableName)); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	return tx, nil
}

func SearchMatches(ctx context.Context, db *sql.DB, lemma string, source Source) ([]dictionary.Lemma, error) {
	row, err := db.QueryContext(
		ctx,
		"SELECT DISTINCT lemma, pos "+
			"FROM lex_dictionary "+
			"WHERE lemma = ? AND source = ? "+
			"GROUP BY lemma, pos",
		lemma, source,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search match: %w", err)
	}
	defer row.Close()

	matches := make([]dictionary.Lemma, 0)
	i := 0
	for row.Next() {
		match := dictionary.Lemma{
			ID:        fmt.Sprintf("match-%s-%d", source, i),
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

func SearchTerm(ctx context.Context, db *sql.DB, lemma string, pos string) ([]LexItem, error) {
	row, err := db.QueryContext(
		ctx,
		"SELECT lemma, pos, gender, aspect, JSON_OBJECTAGG(source, idents) AS sources "+
			"FROM ( "+
			"SELECT lemma, pos, gender, aspect, source, JSON_ARRAYAGG(JSON_OBJECT('id', external_id, 'parentId', external_parent_id) ORDER BY homonym) AS idents "+
			"FROM lex_dictionary AS l "+
			"JOIN ( "+
			"SELECT DISTINCT group_id FROM lex_dictionary WHERE lemma = ? AND pos = ? "+
			") AS g ON g.group_id = l.group_id "+
			"GROUP BY lemma, pos, gender, aspect, source "+
			") AS sub "+
			"GROUP BY lemma, pos, gender, aspect",
		lemma, pos,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search the term: %w", err)
	}
	defer row.Close()

	data := make([]LexItem, 0)
	for row.Next() {
		var genderArg, aspectArg sql.NullString
		var jsonSources string
		item := LexItem{}
		if err := row.Scan(&item.Lemma, &item.Pos, &genderArg, &aspectArg, &jsonSources); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to scan the term: %w", err)
		}
		if genderArg.Valid {
			item.Gender = genderArg.String
		}
		if aspectArg.Valid {
			item.Gender = genderArg.String
		}
		// parse jsonIdents into srchItem.Idents
		if err := json.Unmarshal([]byte(jsonSources), &item.Sources); err != nil {
			return nil, fmt.Errorf("failed to search the term: %w", err)
		}
		item.relevanceScore = levenshtein.ComputeDistance(lemma, item.Lemma)
		data = append(data, item)
	}
	sort.Slice(data, func(i, j int) bool {
		return data[i].relevanceScore < data[j].relevanceScore
	})

	return data, nil
}

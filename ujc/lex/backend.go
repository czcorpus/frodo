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
)

type Source string

type LexData struct {
	MainSource Source    `json:"mainSource"`
	LexItems   []LexItem `json:"lexItems"`
}

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

var sourcePriority = []Source{SourceASSC, SourceIJP}

var dictionaryTable = `
CREATE TABLE %s (
	group_id INT NOT NULL,

	lemma VARCHAR(100) NOT NULL,
	pos VARCHAR(1) NOT NULL,
	gender VARCHAR(1),
	
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

func SearchTerm(ctx context.Context, db *sql.DB, lemma string) ([]LexItem, error) {
	row, err := db.QueryContext(
		ctx,
		"SELECT lemma, pos, gender, JSON_OBJECTAGG(source, idents) AS sources "+
			"FROM ( "+
			"SELECT lemma, pos, gender, source, JSON_ARRAYAGG(JSON_OBJECT('id', external_id, 'parentId', external_parent_id)) AS idents "+
			"FROM lex_dictionary AS l "+
			"JOIN ( "+
			"SELECT DISTINCT group_id FROM lex_dictionary WHERE lemma = ? "+
			") AS g ON g.group_id = l.group_id "+
			"GROUP BY lemma, pos, gender, source "+
			") AS sub "+
			"GROUP BY lemma, pos, gender",
		lemma,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search the term: %w", err)
	}
	defer row.Close()

	data := make([]LexItem, 0)
	for row.Next() {
		var genderArg, posArg sql.NullString
		var jsonSources string
		item := LexItem{}
		if err := row.Scan(&item.Lemma, &posArg, &genderArg, &jsonSources); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to search the term: %w", err)
		}
		if posArg.Valid {
			item.Pos = posArg.String
		}
		if genderArg.Valid {
			item.Gender = genderArg.String
		}
		// parse jsonIdents into srchItem.Idents
		if err := json.Unmarshal([]byte(jsonSources), &item.Sources); err != nil {
			return nil, fmt.Errorf("failed to search the term: %w", err)
		}
		data = append(data, item)
	}

	for i, item := range data {
		for _, source := range sourcePriority {
			if _, ok := item.Sources[source]; ok {
				item.MainSource = source
				data[i] = item
				break
			}
		}
	}
	return data, nil
}

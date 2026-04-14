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

func SearchTerm(ctx context.Context, db *sql.DB, lemma string) (*LexData, error) {
	row, err := db.QueryContext(
		ctx,
		"select lemma, pos, gender, source, JSON_ARRAYAGG(JSON_OBJECT('id', external_id, 'parentId', external_parent_id)) as idents "+
			"FROM ( "+
			fmt.Sprintf("(select distinct group_id from %s where lemma = ?) as g ", TableName)+
			fmt.Sprintf("join %s as l on g.group_id = l.group_id ", TableName)+
			") "+
			"GROUP BY lemma, pos, gender, source",
		lemma,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search the term: %w", err)
	}
	defer row.Close()

	var sources map[Source][]LexItem
	for row.Next() {
		var genderArg, posArg sql.NullString
		var jsonIdents string
		srchItem := LexItem{}
		if err := row.Scan(&srchItem.Lemma, &posArg, &genderArg, &srchItem.Source, &jsonIdents); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to search the term: %w", err)
		}
		if posArg.Valid {
			srchItem.Pos = posArg.String
		}
		if genderArg.Valid {
			srchItem.Gender = genderArg.String
		}
		// parse jsonIdents into srchItem.Idents
		if err := json.Unmarshal([]byte(jsonIdents), &srchItem.Idents); err != nil {
			return nil, fmt.Errorf("failed to search the term: %w", err)
		}
		if sources == nil {
			sources = make(map[Source][]LexItem)
		}
		if _, ok := sources[srchItem.Source]; !ok {
			sources[srchItem.Source] = []LexItem{}
		}
		sources[srchItem.Source] = append(sources[srchItem.Source], srchItem)
	}

	for _, source := range sourcePriority {
		if items, ok := sources[source]; ok {
			lexItems, err := mergeSources(items, sources)
			if err != nil {
				return nil, fmt.Errorf("failed to merge sources: %w", err)
			}
			return &LexData{
				MainSource: source,
				LexItems:   lexItems,
			}, nil
		}
	}
	return nil, nil
}

func mergeSources(mainItems []LexItem, extraData map[Source][]LexItem) ([]LexItem, error) {
	var extraItems []LexItem
	for source, items := range extraData {
		if source != mainItems[0].Source {
			extraItems = append(extraItems, items...)
		}
	}

	for i, mItem := range mainItems {
		if mItem.ExtraSources == nil {
			mItem.ExtraSources = make(map[Source][]string)
		}
		for _, eItem := range extraItems {
			if mItem.Lemma == eItem.Lemma && mItem.Pos == eItem.Pos && mItem.Gender == eItem.Gender {
				idents, ok := mItem.ExtraSources[eItem.Source]
				if !ok {
					idents = make([]string, len(eItem.Idents))
					for j, ident := range eItem.Idents {
						idents[j] = ident.ID
					}
				} else {
					for _, ident := range eItem.Idents {
						idents = append(idents, ident.ID)
					}
				}
				mItem.ExtraSources[eItem.Source] = idents
				mainItems[i] = mItem
			}
		}
	}
	return mainItems, nil
}

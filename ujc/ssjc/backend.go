// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
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

package ssjc

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

type TablePrefix string

const (
	TablePrefixSSJC TablePrefix = "ssjc"
	TablePrefixSJC  TablePrefix = "sjc"
)

func (tf TablePrefix) Validate() error {
	if tf != TablePrefixSSJC && tf != TablePrefixSJC {
		return fmt.Errorf("unsupported value of TablePrefix variable")
	}
	return nil
}

var dictionaryTable = `
CREATE TABLE %s_headword (
    id VARCHAR(100) COLLATE utf8mb4_bin PRIMARY KEY,
    parent_id VARCHAR(100) COLLATE utf8mb4_bin,
    headword VARCHAR(100) NOT NULL,
    headword_type VARCHAR(100) NOT NULL,
    pos VARCHAR(20),
    gender VARCHAR(20),
    aspect VARCHAR(20),
    FOREIGN KEY (parent_id) REFERENCES %s_headword(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_czech_ci`

func CreateTables(ctx context.Context, db *sql.DB, tablePrefix TablePrefix) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create s(s)jc tables: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s_headword", tablePrefix)); err != nil {
		return nil, fmt.Errorf("failed to create s(s)jc tables: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(dictionaryTable, tablePrefix, tablePrefix)); err != nil {
		return nil, fmt.Errorf("failed to create s(s)jc tables: %w", err)
	}
	return tx, nil
}

func InsertDictChunk(ctx context.Context, tx *sql.Tx, tablePrefix TablePrefix, data []SrcFileRow) error {
	var insTpl strings.Builder
	dataArgs := make([]any, 0, len(data)*7)
	for i, v := range data {
		if i > 0 {
			insTpl.WriteString(", ")
		}
		insTpl.WriteString("(?, ?, ?, ?, ?, ?, ?)")
		parent := sql.NullString{String: v.ParentID, Valid: v.ParentID != ""}
		dataArgs = append(dataArgs, v.ID, parent, v.Headword, v.HeadwordType, v.Pos, v.Gender, v.Aspect)
	}
	_, err := tx.ExecContext(
		ctx,
		fmt.Sprintf(
			"INSERT INTO %s_headword (id, parent_id, headword, headword_type, pos, gender, aspect) VALUES %s",
			tablePrefix,
			insTpl.String(),
		),
		dataArgs...,
	)
	if err != nil {
		log.Warn().Err(err).Msg("failed to insert row chunk, trying one by one")
		// try one by one and ignore errors:
		for _, item := range data {
			parent := sql.NullString{String: item.ParentID, Valid: item.ParentID != ""}
			_, err := tx.ExecContext(
				ctx,
				fmt.Sprintf(
					"INSERT INTO %s_headword (id, parent_id, headword, headword_type, pos, gender, aspect) VALUES (?, ?, ?, ?, ?, ?, ?) ",
					tablePrefix,
				),
				item.ID, parent, item.Headword, item.HeadwordType, item.Pos, item.Gender, item.Aspect,
			)
			if err != nil {
				log.Error().Err(err).Any("values", item).Msg("failed to insert single row, ignoring")
			}

		}

	}
	return nil
}

func SearchTerm(ctx context.Context, db *sql.DB, tablePrefix TablePrefix, term string) (HeadWordEntry, error) {
	// find the head
	row := db.QueryRowContext(
		ctx,
		"SELECT id, parent_id, headword, headword_type, pos, gender, aspect "+
			fmt.Sprintf("FROM %s_headword ", tablePrefix)+
			"WHERE headword = ?",
		term,
	)
	var srchItem SrcFileRow
	var parentArg sql.NullString
	if err := row.Scan(&srchItem.ID, &parentArg, &srchItem.Headword, &srchItem.HeadwordType, &srchItem.Pos, &srchItem.Gender, &srchItem.Aspect); err != nil {
		if err == sql.ErrNoRows {
			return HeadWordEntry{}, nil
		}
		return HeadWordEntry{}, fmt.Errorf("failed to search the term: %w", err)
	}

	var parent string
	if parentArg.Valid {
		parent = parentArg.String

	} else {
		parent = srchItem.ID
	}
	rows, err := db.QueryContext(
		ctx,
		"SELECT headword, pos, gender, aspect "+
			fmt.Sprintf("FROM %s_headword ", tablePrefix)+
			"WHERE parent_id = ?",
		parent,
	)
	if err != nil {
		return HeadWordEntry{}, fmt.Errorf("failed to search the term: %w", err)
	}
	parentSrch := make([]SubHeadWord, 0, 10)
	for rows.Next() {
		var item SubHeadWord
		if err := rows.Scan(&item.Headword, &item.Pos, &item.Gender, &item.Aspect); err != nil {
			return HeadWordEntry{}, fmt.Errorf("failed to search the term: %w", err)
		}
		parentSrch = append(parentSrch, item)
	}
	return HeadWordEntry{
		ID:       srchItem.ID,
		Headword: srchItem.Headword,
		Pos:      srchItem.Pos,
		Gender:   srchItem.Gender,
		Aspect:   srchItem.Aspect,
		Children: parentSrch,
	}, nil

}

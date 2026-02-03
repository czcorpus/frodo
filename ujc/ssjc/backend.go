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

var SSJC_table = `
CREATE TABLE ssjc_headword (
    id VARCHAR(100) COLLATE utf8mb4_bin PRIMARY KEY,
    parent_id VARCHAR(100) COLLATE utf8mb4_bin,
    headword VARCHAR(100) NOT NULL,
    headword_type VARCHAR(100) NOT NULL,
    pos VARCHAR(20),
    gender VARCHAR(20),
    aspect VARCHAR(20),
    FOREIGN KEY (parent_id) REFERENCES ssjc_headword(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_czech_ci`

func CreateTables(ctx context.Context, db *sql.DB) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create ssjc tables: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS ssjc_headword"); err != nil {
		return nil, fmt.Errorf("failed to create ssjc tables: %w", err)
	}
	if _, err := tx.ExecContext(ctx, SSJC_table); err != nil {
		return nil, fmt.Errorf("failed to create ssjc tables: %w", err)
	}
	return tx, nil
}

func InsertDictChunk(ctx context.Context, tx *sql.Tx, data []SSJCFileRow) error {
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
			"INSERT INTO ssjc_headword (id, parent_id, headword, headword_type, pos, gender, aspect) VALUES %s",
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
					"INSERT INTO ssjc_headword (id, parent_id, headword, headword_type, pos, gender, aspect) VALUES (?, ?, ?, ?, ?, ?, ?) ",
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

type SubHeadWord struct {
	Headword string `json:"headword"`
	Pos      string `json:"pos"`
	Gender   string `json:"gender"`
	Aspect   string `json:"aspect"`
}

type HeadWordEntry struct {
	ID       string        `json:"id"`
	Headword string        `json:"headword"`
	Pos      string        `json:"pos"`
	Gender   string        `json:"gender"`
	Aspect   string        `json:"aspect"`
	Children []SubHeadWord `json:"variants"`
}

func (e HeadWordEntry) IsZero() bool {
	return e.ID == ""
}

func SearchTerm(ctx context.Context, db *sql.DB, term string) (HeadWordEntry, error) {
	// find the head
	row := db.QueryRowContext(
		ctx,
		"SELECT id, parent_id, headword, headword_type, pos, gender, aspect "+
			"FROM ssjc_headword "+
			"WHERE headword = ?",
		term,
	)
	var srchItem SSJCFileRow
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
			"FROM ssjc_headword "+
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

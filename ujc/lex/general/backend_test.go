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

package general

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestGenerateLexSearchTable(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE lex_dictionary (
			lemma TEXT NOT NULL
		);
		CREATE TABLE corp_word (
			id TEXT,
			value TEXT,
			lemma TEXT,
			sublemma TEXT,
			count INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE corp_term_search (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			word_id TEXT NOT NULL,
			value TEXT,
			value_lc TEXT
		);
		CREATE TABLE lex_search (
			term TEXT NOT NULL,
			lemma TEXT,
			sublemma TEXT,
			lemma_lc TEXT,
			sublemma_lc TEXT,
			term_lc TEXT
		);
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO lex_dictionary (lemma) VALUES
			('karel'),
			('Jana Nováková'),
			('dům');
		INSERT INTO corp_word (id, value, lemma, sublemma, count) VALUES
			('w0', 'karel', 'karel', 'karel', 12),
			('w1', 'jana', 'Jana', 'Jana', 10),
			('w2', 'nováková', 'Novák', 'Novák', 7),
			('w3', 'dům', 'dům', 'dům', 9);
		INSERT INTO corp_term_search (word_id, value, value_lc) VALUES
			('w0', 'Karel', 'karel'),
			('w1', 'Jana', 'jana'),
			('w2', 'Nováková', 'nováková'),
			('w3', 'dům', 'dům');
	`)
	require.NoError(t, err)

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)

	err = GenerateLexSearchTable(context.Background(), tx, "corp")
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	rows, err := db.Query(`SELECT term, lemma, sublemma, term_lc, lemma_lc, sublemma_lc FROM lex_search ORDER BY term`)
	require.NoError(t, err)
	defer rows.Close()

	got := map[string]struct {
		lemma       string
		sublemma    string
		sublemma_lc string
		term_lc     string
	}{}
	for rows.Next() {
		var term, lemma, sublemma, term_lc, lemma_lc, sublemma_lc string
		require.NoError(t, rows.Scan(&term, &lemma, &sublemma, &term_lc, &lemma_lc, &sublemma_lc))
		got[term] = struct {
			lemma       string
			sublemma    string
			sublemma_lc string
			term_lc     string
		}{lemma: lemma, sublemma: sublemma, sublemma_lc: sublemma_lc, term_lc: term_lc}
	}
	require.NoError(t, rows.Err())
	require.Len(t, got, 3)
	require.Equal(t, "dům", got["dům"].lemma)
	require.Equal(t, "dům", got["dům"].sublemma)
	require.Equal(t, "dům", got["dům"].sublemma_lc)
	require.Equal(t, "dům", got["dům"].term_lc)
	require.Equal(t, "Jana Novák", got["Jana Nováková"].lemma)
	require.Equal(t, "Jana Novák", got["Jana Nováková"].sublemma)
	require.Equal(t, "jana novák", got["Jana Nováková"].sublemma_lc)
	require.Equal(t, "jana nováková", got["Jana Nováková"].term_lc)
	require.Equal(t, "karel", got["karel"].lemma)
	require.Equal(t, "karel", got["karel"].sublemma)
	require.Equal(t, "karel", got["karel"].sublemma_lc)
	require.Equal(t, "karel", got["karel"].term_lc)
}

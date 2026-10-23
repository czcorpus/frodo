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
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

type lemmatizedTerm struct {
	Term     string
	Lemma    string
	Sublemma string
}

func lemmatizeLexTerm(ctx context.Context, tx *sql.Tx, corpname, term string) (lemmatizedTerm, error) {
	result := lemmatizedTerm{Term: term, Lemma: "", Sublemma: ""}
	parts := strings.Fields(term)
	if len(parts) == 0 {
		return result, nil
	}

	lemmaParts := make([]string, 0, len(parts))
	sublemmaParts := make([]string, 0, len(parts))
	for _, part := range parts {
		rows, err := tx.QueryContext(
			ctx,
			fmt.Sprintf(
				"SELECT DISTINCT lemma, sublemma "+
					"FROM %s_term_search AS ts "+
					"JOIN %s_word AS w ON ts.word_id = w.id "+
					"WHERE ts.value_lc = LOWER(?) "+
					"ORDER BY count DESC",
				corpname, corpname,
			),
			part,
		)
		if err != nil {
			return result, fmt.Errorf("failed to lemmatize part %q: %w", part, err)
		}

		var lemma, sublemma string
		var usedLemma, usedSublemma string
		for rows.Next() {
			if err := rows.Scan(&lemma, &sublemma); err != nil {
				rows.Close()
				return result, fmt.Errorf("failed to lemmatize part %q: %w", part, err)
			}
			if part == sublemma {
				usedLemma, usedSublemma = lemma, sublemma
				break
			} else if usedLemma == "" && usedSublemma == "" {
				usedLemma, usedSublemma = lemma, sublemma
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return result, fmt.Errorf("failed to lemmatize part %q: %w", part, err)
		}
		if err := rows.Close(); err != nil {
			return result, fmt.Errorf("failed to close rows for part %q: %w", part, err)
		}

		if usedLemma == "" || usedSublemma == "" {
			return result, nil
		}
		lemmaParts = append(lemmaParts, usedLemma)
		sublemmaParts = append(sublemmaParts, usedSublemma)
	}
	result.Lemma = strings.Join(lemmaParts, " ")
	result.Sublemma = strings.Join(sublemmaParts, " ")
	return result, nil
}

func GenerateLexSearchTable(ctx context.Context, tx *sql.Tx, corpname string) error {
	log.Info().Msg("Cleaning old data...")
	if _, err := tx.ExecContext(ctx, "DELETE FROM lex_search"); err != nil {
		return fmt.Errorf("failed to reset lex_search: %w", err)
	}

	log.Info().Msg("Getting dictionary terms...")
	rows, err := tx.QueryContext(
		ctx,
		"SELECT DISTINCT lemma FROM lex_dictionary ORDER BY lemma",
	)
	if err != nil {
		return fmt.Errorf("failed to select distinct lex dictionary terms: %w", err)
	}

	terms := make([]string, 0)
	for rows.Next() {
		var term string
		if err := rows.Scan(&term); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan term: %w", err)
		}
		terms = append(terms, term)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("failed to iterate lex_dictionary rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("failed to close lex_dictionary rows: %w", err)
	}

	log.Info().Msg("Lemmatization in progress...")
	totalTerms := len(terms)
	for i, term := range terms {
		if i%100 == 0 {
			percent := 0.0
			if totalTerms > 0 {
				percent = float64(i+1) * 100.0 / float64(totalTerms)
			}
			barLen := 30
			filled := int(percent * float64(barLen) / 100.0)
			if filled > barLen {
				filled = barLen
			}
			fmt.Printf("\r[%s%s] %d/%d %.0f%%",
				strings.Repeat("=", filled),
				strings.Repeat(" ", barLen-filled),
				i+1,
				totalTerms,
				percent,
			)
		}

		lTerm, err := lemmatizeLexTerm(ctx, tx, corpname, term)
		if err != nil {
			return err
		}

		var lemmaNullString, sublemmaNullString sql.NullString
		lemmaNullString.String = lTerm.Lemma
		lemmaNullString.Valid = lTerm.Lemma != ""
		sublemmaNullString.String = lTerm.Sublemma
		sublemmaNullString.Valid = lTerm.Sublemma != ""
		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO lex_search (term, lemma, sublemma, term_lc, lemma_lc, sublemma_lc) VALUES (?, ?, ?, LOWER(?), LOWER(?), LOWER(?))",
			lTerm.Term,
			lemmaNullString,
			sublemmaNullString,
			lTerm.Term,
			lemmaNullString,
			sublemmaNullString,
		); err != nil {
			return fmt.Errorf("failed to insert lex search row for %q: %w", lTerm, err)
		}
	}
	fmt.Println()
	return nil
}

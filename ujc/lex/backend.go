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
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/czcorpus/cnc-gokit/collections"
)

type Source string

const (
	SourceASSC   Source = "assc"
	SourceIJP    Source = "ijp"
	SourceSSJC   Source = "ssjc"
	SourcePSJC   Source = "psjc"
	SourceSJC    Source = "sjc"
	SourceSSC    Source = "ssc"
	SourceCorpus Source = "cnc"
	SourceEmpty  Source = "---"

	PosAdj   = "A"
	PosAbb   = "B"
	PosNum   = "C"
	PosAdv   = "D"
	PosFore  = "F"
	PosInter = "I"
	PosConj  = "J"
	PosNoun  = "N"
	PosPron  = "P"
	PosPrep  = "R"
	PosSegm  = "S"
	PosPart  = "T"
	PosVerb  = "V"
	PosUnkn  = "X"
	PosPunc  = "Z"
	PosDTIJ  = "DTIJ"

	PosDTIJCR = "DTIJCR"

	GenderMascAnim     = "M"
	GenderMascInan     = "I"
	GenderMascAnimInan = "B"
	GenderMascUnknown  = "U"
	GenderFem          = "F"
	GenderNeut         = "N"
	GenderUnknown      = "X" // mainly for search purposes

	AspectPerf    = "P"
	AspectImp     = "I"
	AspectBoth    = "B"
	AspectUnknown = "X" // mainly for search purposes

	UninflectedFalse   = 0
	UninflectedTrue    = 1
	UninflectedUnknown = 2 // mainly for search purposes

	PluralityNone    = 0
	PluralityPlural  = 1
	PluralityAlways  = 2
	PluralityUsually = 3
	PluralityOnly    = 4
	PluralityUnknown = 5 // mainly for search purposes
)

var dictionaryTable = `
CREATE TABLE lex_dictionary (
	group_id VARCHAR(100),
	homonym TINYINT DEFAULT 0 NOT NULL,
	group_order TINYINT DEFAULT 0 NOT NULL,

	lemma VARCHAR(100) NOT NULL,
	pos VARCHAR(4) NOT NULL,
	gender VARCHAR(1),
	aspect VARCHAR(1),
	uninflected TINYINT DEFAULT 2 NOT NULL,
	plurality TINYINT DEFAULT 5 NOT NULL,
	
	source VARCHAR(8) NOT NULL,
	external_id TEXT NOT NULL,
	external_parent_id TEXT,

	INDEX idx_lex_dictionary_lemma_source_group_id (lemma, source, group_id),
	INDEX idx_lex_dictionary_lemma_pos_source (lemma, pos, source),
	INDEX idx_lex_dictionary_lemma_source (lemma, source)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;`

var suggestionsTable = `
CREATE TABLE lex_suggestions (
	lemma TEXT NOT NULL,
	-- This column automatically calculates the normalized search key
	search_key TEXT COLLATE utf8mb4_unicode_ci GENERATED ALWAYS AS (
		REPLACE(REPLACE(REPLACE(LOWER(lemma), 'y', 'i'), 'z', 's'), 'ž', 'š')
	) STORED,

	CONSTRAINT unique_lex_suggestions_lemma UNIQUE (lemma),
	INDEX idx_lex_suggestions_search_key (search_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;`

var searchTable = `
CREATE TABLE lex_search (
	term TEXT NOT NULL,
	lemma TEXT,
	sublemma TEXT,
	term_lc TEXT NOT NULL,
	lemma_lc TEXT,
	sublemma_lc TEXT,

	CONSTRAINT unique_lex_search_term UNIQUE (term),
	INDEX idx_lex_search_lemma_lc (lemma_lc),
	INDEX idx_lex_search_sublemma_lc (sublemma_lc),
	INDEX idx_lex_search_term_lc (term_lc)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;`

func CreateTables(ctx context.Context, db *sql.DB) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS lex_dictionary"); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, dictionaryTable); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS lex_suggestions"); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, suggestionsTable); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS lex_search"); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, searchTable); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	return tx, nil
}

func SearchTypoSuggestions(ctx context.Context, db *sql.DB, term string) ([]string, error) {
	row, err := db.QueryContext(
		ctx,
		"SELECT lemma "+
			"FROM lex_suggestions "+
			"WHERE search_key = REPLACE(REPLACE(REPLACE(LOWER(?), 'y', 'i'), 'z', 's'), 'ž', 'š');",
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
			return nil, fmt.Errorf("failed to scan suggestions: %w", err)
		}
		suggestions = append(suggestions, lemma)
	}
	if err := row.Err(); err != nil {
		return nil, fmt.Errorf("failed to read suggestions: %w", err)
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
			return nil, fmt.Errorf("failed to scan available source: %w", err)
		}
		sources = append(sources, source)
	}
	if err := row.Err(); err != nil {
		return nil, fmt.Errorf("failed to read available sources: %w", err)
	}
	return sources, nil
}

func SearchVariants(ctx context.Context, db *sql.DB, lemmatizedTerm LemmatizedTerm) ([]LexItem, error) {
	row, err := db.QueryContext(
		ctx,
		`
			-- find available variants, get exact lemmata and their variants based on group_id and source
			SELECT DISTINCT lemma, pos, gender, aspect, uninflected, plurality
			FROM lex_dictionary AS l
			JOIN (
				SELECT DISTINCT group_id, source
				FROM lex_dictionary
				WHERE lemma = ? AND source = ? AND group_id IS NOT NULL
			) AS g
			ON g.group_id = l.group_id AND g.source = l.source
			UNION
			SELECT DISTINCT lemma, pos, gender, aspect, uninflected, plurality
			FROM lex_dictionary AS l
			WHERE lemma = ? AND source = ? AND group_id IS NULL
		`,
		lemmatizedTerm.Term, lemmatizedTerm.Source, lemmatizedTerm.Term, lemmatizedTerm.Source,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search variants: %w", err)
	}
	defer row.Close()

	data := make([]LexItem, 0, 5)
	for row.Next() {
		var genderArg, aspectArg sql.NullString
		key := LexKey{}
		if err := row.Scan(&key.Lemma, &key.Pos, &genderArg, &aspectArg, &key.Uninflected, &key.Plurality); err != nil {
			return nil, fmt.Errorf("failed to scan variants: %w", err)
		}
		if genderArg.Valid {
			key.Gender = genderArg.String
		}
		if aspectArg.Valid {
			key.Aspect = aspectArg.String
		}
		data = append(data, LexItem{Key: key, PosSource: lemmatizedTerm.Source, VariantSource: lemmatizedTerm.Source})
	}
	if err := row.Err(); err != nil {
		return nil, fmt.Errorf("failed to read variants: %w", err)
	}
	return data, nil
}

// lexKeyConditions creates WHERE conditions matching gender, aspect, uninflected and plurality
// of the lex key. Empty gender/aspect is searched as NULL, unknown values match anything
// and masculine genders also match the unknown masculine gender.
func lexKeyConditions(lexKey LexKey) ([]string, []any) {
	where := make([]string, 0, 4)
	args := make([]any, 0, 8)
	if lexKey.Gender == "" {
		where = append(where, "gender is NULL")
	} else if lexKey.Gender == GenderMascUnknown {
		where = append(where, "gender IN (?, ?, ?, ?)")
		args = append(args, GenderMascUnknown, GenderMascAnim, GenderMascInan, GenderMascAnimInan)
	} else if lexKey.Gender == GenderMascAnim || lexKey.Gender == GenderMascInan || lexKey.Gender == GenderMascAnimInan {
		where = append(where, "gender IN (?, ?)")
		args = append(args, GenderMascUnknown, lexKey.Gender)
	} else if lexKey.Gender != GenderUnknown {
		where = append(where, "gender IN (?, ?)")
		args = append(args, lexKey.Gender, GenderUnknown)
	}
	if lexKey.Aspect == "" {
		where = append(where, "aspect IS NULL")
	} else if lexKey.Aspect != AspectUnknown {
		where = append(where, "aspect = ?")
		args = append(args, lexKey.Aspect)
	}
	if lexKey.Uninflected != UninflectedUnknown {
		where = append(where, "(uninflected = ? OR uninflected = ?)")
		args = append(args, lexKey.Uninflected, UninflectedUnknown)
	}
	if lexKey.Plurality != PluralityUnknown {
		where = append(where, "(plurality = ? OR plurality = ?)")
		args = append(args, lexKey.Plurality, PluralityUnknown)
	}
	return where, args
}

func SearchSources(ctx context.Context, db *sql.DB, lexKey LexKey) (map[Source][]LexID, error) {
	// if lexItem.Pos is 'X', do not filter by pos (accept any pos)
	where := []string{"lemma = ?"}
	args := []any{lexKey.Lemma}
	if lexKey.Pos == PosDTIJCR {
		where = append(where, "pos IN (?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args, PosDTIJ, PosAdv, PosPart, PosInter, PosConj, PosNum, PosPrep, PosUnkn)
	} else if lexKey.Pos != PosUnkn {
		where = append(where, "pos IN (?, ?)")
		args = append(args, lexKey.Pos, PosUnkn)
	}

	keyWhere, keyArgs := lexKeyConditions(lexKey)
	where = append(where, keyWhere...)
	args = append(args, keyArgs...)

	query := `
		SELECT source, JSON_ARRAYAGG(JSON_OBJECT('id', external_id, 'parentId', external_parent_id, 'groupId', COALESCE(group_id, ''), 'groupOrder', group_order, 'homonym', homonym, 'key', JSON_OBJECT('pos', pos, 'gender', COALESCE(gender, ''), 'aspect', COALESCE(aspect, ''), 'uninflected', uninflected, 'plurality', plurality)) ORDER BY homonym) AS idents
		FROM lex_dictionary
		WHERE ` + strings.Join(where, " AND ") + `
		GROUP BY source
		`

	row, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search sources: %w", err)
	}
	defer row.Close()

	sources := make(map[Source][]LexID)
	for row.Next() {
		var source Source
		var jsonIdents string
		if err := row.Scan(&source, &jsonIdents); err != nil {
			return nil, fmt.Errorf("failed to scan sources: %w", err)
		}
		var idents []LexID
		if err := json.Unmarshal([]byte(jsonIdents), &idents); err != nil {
			return nil, fmt.Errorf("failed to unmarshal sources: %w", err)
		}
		sources[source] = idents
	}
	if err := row.Err(); err != nil {
		return nil, fmt.Errorf("failed to read sources: %w", err)
	}
	return sources, nil
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

func SearchLexItemID(ctx context.Context, db *sql.DB, lexKey LexKey, source Source) ([]LexID, error) {
	where := []string{"lemma = ?", "pos = ?"}
	args := []any{lexKey.Lemma, lexKey.Pos}

	keyWhere, keyArgs := lexKeyConditions(lexKey)
	where = append(where, keyWhere...)
	args = append(args, keyArgs...)

	where = append(where, "source = ?")
	args = append(args, source)

	query := "SELECT external_id, external_parent_id, group_id, group_order, homonym, pos, gender, aspect, uninflected, plurality FROM lex_dictionary WHERE " + strings.Join(where, " AND ")

	row, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search the term: %w", err)
	}
	defer row.Close()

	lexIds := make([]LexID, 0, 1)
	for row.Next() {
		var lexId LexID
		var externalParentID sql.NullString
		var groupID sql.NullString
		var genderArg, aspectArg sql.NullString
		if err := row.Scan(&lexId.ID, &externalParentID, &groupID, &lexId.GroupOrder, &lexId.Homonym, &lexId.Key.Pos, &genderArg, &aspectArg, &lexId.Key.Uninflected, &lexId.Key.Plurality); err != nil {
			return nil, fmt.Errorf("failed to scan the lex id: %w", err)
		}
		lexId.ParentID = externalParentID.String
		lexId.GroupID = groupID.String
		if genderArg.Valid {
			lexId.Key.Gender = genderArg.String
		}
		if aspectArg.Valid {
			lexId.Key.Aspect = aspectArg.String
		}
		lexIds = append(lexIds, lexId)
	}
	if err := row.Err(); err != nil {
		return nil, fmt.Errorf("failed to read lex ids: %w", err)
	}
	return lexIds, nil
}

var (
	// corpus name is used to build table names, so it must be a plain SQL identifier
	corpusNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

	ErrInvalidCorpusName = errors.New("invalid corpus name")
)

func ValidateCorpusName(corpname string) error {
	if !corpusNameRegexp.MatchString(corpname) {
		return fmt.Errorf("%w: %q", ErrInvalidCorpusName, corpname)
	}
	return nil
}

type LemmatizedTerm struct {
	Term     string
	Lemma    string
	Sublemma string

	Source   Source
	Distance int
}

func (lt *LemmatizedTerm) isMultiWord() bool {
	return strings.Contains(lt.Term, " ")
}

func LemmatizeTermWithCorpus(ctx context.Context, db *sql.DB, corpname, term string) ([]LemmatizedTerm, error) {
	if err := ValidateCorpusName(corpname); err != nil {
		return nil, err
	}
	parts := strings.Fields(term)
	if len(parts) == 0 {
		return []LemmatizedTerm{}, nil
	}

	result := make([]LemmatizedTerm, 0, 10)
	for _, part := range parts {
		rows, err := db.QueryContext(
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

		newResult, err := func() ([]LemmatizedTerm, error) {
			defer rows.Close()
			var lemma, sublemma string
			newResult := make([]LemmatizedTerm, 0, 10)
			for rows.Next() {
				if err := rows.Scan(&lemma, &sublemma); err != nil {
					return nil, err
				}
				if len(result) == 0 {
					newResult = append(newResult, LemmatizedTerm{Term: term, Lemma: lemma, Sublemma: sublemma})
				} else {
					for _, r := range result {
						r.Lemma += " " + lemma
						r.Sublemma += " " + sublemma
						newResult = append(newResult, r)
					}
				}
			}
			return newResult, rows.Err()
		}()
		if err != nil {
			return result, fmt.Errorf("failed to lemmatize part %q: %w", part, err)
		}
		if len(newResult) == 0 {
			return []LemmatizedTerm{}, nil
		}
		result = newResult
	}
	return result, nil
}

func LemmatizeTermWithLex(ctx context.Context, db *sql.DB, term string) (*LemmatizedTerm, error) {
	var lemmaNull, sublemmaNull sql.NullString
	row := db.QueryRowContext(
		ctx,
		"SELECT lemma, sublemma "+
			"FROM lex_search "+
			"WHERE term = ?",
		term,
	)
	err := row.Scan(&lemmaNull, &sublemmaNull)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to lemmatize lex term %q: %w", term, err)
	}
	return &LemmatizedTerm{Term: term, Lemma: lemmaNull.String, Sublemma: sublemmaNull.String}, nil
}

func SearchLexTerms(ctx context.Context, db *sql.DB, searchTerm LemmatizedTerm) ([]LemmatizedTerm, error) {
	result := make([]LemmatizedTerm, 0, 4)

	var term string
	var lemmaNull, sublemmaNull sql.NullString
	rows, err := db.QueryContext(
		ctx,
		"SELECT DISTINCT term, lemma, sublemma "+
			"FROM lex_search "+
			"WHERE term_lc = LOWER(?) OR lemma_lc = LOWER(?) OR sublemma_lc = LOWER(?)",
		searchTerm.Term,
		searchTerm.Lemma,
		searchTerm.Sublemma,
	)
	if err != nil {
		return result, fmt.Errorf("failed to search lex term %q: %w", searchTerm.Term, err)
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&term, &lemmaNull, &sublemmaNull)
		if err != nil {
			return result, fmt.Errorf("failed to search lex term %q: %w", searchTerm.Term, err)
		}
		result = append(result, LemmatizedTerm{
			Term:     term,
			Lemma:    lemmaNull.String,
			Sublemma: sublemmaNull.String,
			Distance: levenshtein.ComputeDistance(term, searchTerm.Term),
		})
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("failed to search lex term %q: %w", searchTerm.Term, err)
	}
	// exact term match first, then sublemma and lemma matches, then the closest terms
	slices.SortFunc(result, func(a, b LemmatizedTerm) int {
		return cmp.Or(
			preferEqual(a.Term, b.Term, searchTerm.Term),
			preferEqual(a.Sublemma, b.Sublemma, searchTerm.Sublemma),
			preferEqual(a.Lemma, b.Lemma, searchTerm.Lemma),
			cmp.Compare(a.Distance, b.Distance),
			strings.Compare(a.Term, b.Term),
		)
	})
	return result, nil
}

// preferEqual is a sort comparator ordering values equal to the target first
func preferEqual(a, b, target string) int {
	switch {
	case a == target && b != target:
		return -1
	case a != target && b == target:
		return 1
	}
	return 0
}

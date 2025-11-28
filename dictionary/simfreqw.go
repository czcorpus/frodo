// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
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

package dictionary

import (
	"context"
	"fmt"
	"frodo/db/mysql"
)

type rowsAndErr struct {
	Rows []Lemma
	Err  error
}

// SimilarARFWords calculates nearest items with similar ARF frequency to the provided `lemma`.
// As this function generates quite a demanding SQL query, it is required to provide also a search
// range coefficient (searchRangeCoeff). The searched range is then like this:
// right interval: [lemma.simFreqsScore ... lemma.simFreqsScore * (1 + searchRangeCoeff)]
// left interval:  [lemma.simFreqsScore ... lemma.simFreqsScore * (1 - searchRangeCoeff)]
// This may sometimes lead to a situation where there will be no near items found but it should
// be quite rare.
func SimilarARFWords(
	ctx context.Context,
	db *mysql.Adapter,
	groupedName string,
	lemma Lemma,
	searchRangeCoeff float64,
	maxValues int,
) ([]Lemma, error) {
	if !lemma.CanDoSimFreqScores() {
		return []Lemma{}, nil
	}
	if searchRangeCoeff <= 0 || searchRangeCoeff >= 1 {
		panic("SimilarARFWords - searchRangeCoeff must be from interval (0, 1)")
	}
	upperScoreLim := lemma.SimFreqScore * (1.0 + searchRangeCoeff)
	lowerScoreLim := lemma.SimFreqScore * (1.0 - searchRangeCoeff)
	halfl := maxValues / 2
	// SQL note: even if it is not optimal in regards to getting the closest N values,
	// we need to provide forced ranges (lower_bound...lemma_freq and lemma_freq...upper_bound)
	// where to search as otherwise the query runs for too long
	rows, err := db.DB().QueryContext(
		ctx,
		fmt.Sprintf(
			"(SELECT '-', w.lemma, '-', SUM(w.count), "+
				"w.pos, 0, 1, AVG(w.sim_freqs_score) "+
				"FROM %s_word AS w "+
				"WHERE w.sim_freqs_score BETWEEN ? AND ? AND w.ngram = 1 "+
				"GROUP BY w.lemma, w.pos "+
				"ORDER BY w.sim_freqs_score ASC, w.lemma, w.pos, w.sublemma, w.value "+
				"LIMIT ?) "+
				"UNION "+
				"(SELECT '-', w.lemma, '-', SUM(w.count), "+
				"w.pos, 0, 1, AVG(w.sim_freqs_score) "+
				"FROM %s_word AS w "+
				"WHERE w.sim_freqs_score BETWEEN ? AND ? AND w.ngram = 1 "+
				"GROUP BY w.lemma, w.pos "+
				"ORDER BY w.sim_freqs_score DESC, w.lemma, w.pos, w.sublemma, w.value "+
				"LIMIT ? )",
			groupedName,
			groupedName,
		),
		lemma.SimFreqScore, upperScoreLim, halfl,
		lowerScoreLim, lemma.SimFreqScore, halfl,
	)

	if err != nil {
		return []Lemma{}, fmt.Errorf("failed to get similar freq. words: %w", err)
	}
	defer rows.Close()
	ans, err := processRowsSync(rows, false)
	if err != nil {
		return []Lemma{}, fmt.Errorf("failed to get similar freq. words: %w", err)
	}
	return ans, nil
}

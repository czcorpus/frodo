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
	"strings"
)

func SimilarARFWords(
	ctx context.Context,
	db *mysql.Adapter,
	groupedName string,
	lemma Lemma,
) ([]Lemma, error) {
	if lemma.NgramSize > 1 {
		return []Lemma{}, nil
	}
	whereSQL := make([]string, 0, 5)
	whereArgs := make([]any, 0, 5)
	rngLeft := lemma.SimFreqScore - lemma.SimFreqScore*0.05
	rngRight := lemma.SimFreqScore + lemma.SimFreqScore*0.05

	whereSQL = append(whereSQL, "w.sim_freqs_score <= ?")
	whereArgs = append(whereArgs, rngRight)
	whereSQL = append(whereSQL, "w.sim_freqs_score >= ?")
	whereArgs = append(whereArgs, rngLeft)
	whereSQL = append(whereSQL, "w.ngram = 1")

	rows, err := db.DB().QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT w.value, w.lemma, w.sublemma, w.count, "+
				"w.pos, w.arf, w. ngram, w.sim_freqs_score "+
				"FROM %s_word AS w "+
				"WHERE %s "+
				"ORDER BY w.arf DESC, w.lemma, w.pos, w.sublemma, w.value "+
				"LIMIT 20",
			groupedName,
			strings.Join(whereSQL, " AND "),
		),
		whereArgs...,
	)
	if err != nil {
		return []Lemma{}, fmt.Errorf("failed to get similar freq. words: %w", err)
	}
	ans, err := processRowsSync(rows, false)
	if err != nil {
		return []Lemma{}, fmt.Errorf("failed to get similar freq. words: %w", err)
	}
	return ans, nil
}

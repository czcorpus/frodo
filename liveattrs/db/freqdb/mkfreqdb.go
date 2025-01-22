// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Institute of the Czech National Corpus,
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

/*
Implementation note:
The current implementation is not general enough as it expects specific
tagset and positional attribute types and order.
*/

package freqdb

import (
	"context"
	"database/sql"
	"fmt"
	"frodo/jobs"
	"frodo/liveattrs/db"
	"math"
	"strings"
	"time"

	"github.com/czcorpus/vert-tagextract/v3/ptcount/modders"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	reportEachNthItem   = 50000
	duplicateRowErrNo   = 1062
	NonWordCSCNC2020Tag = "X@-------------"
)

type NgramFreqGenerator struct {
	db          *sql.DB
	groupedName string
	corpusName  string
	posFn       *modders.StringTransformerChain
	jobActions  *jobs.Actions
	qsaAttrs    QSAttributes
}

func (nfg *NgramFreqGenerator) createTables(tx *sql.Tx) error {
	errMsgTpl := "failed to create tables: %w"

	if _, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s_term_search", nfg.groupedName)); err != nil {
		return fmt.Errorf(errMsgTpl, err)
	}
	if _, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s_word", nfg.groupedName)); err != nil {
		return fmt.Errorf(errMsgTpl, err)
	}
	if _, err := tx.Exec(fmt.Sprintf(
		`CREATE TABLE %s_word (
		id varchar(40),
		value VARCHAR(80),
		lemma VARCHAR(80),
		sublemma VARCHAR(80),
		pos VARCHAR(80),
		count INTEGER,
		arf FLOAT,
		initial_cap TINYINT NOT NULL DEFAULT 0,
		PRIMARY KEY (id)
		) COLLATE utf8mb4_bin`,
		nfg.groupedName)); err != nil {
		return fmt.Errorf(errMsgTpl, err)
	}
	if _, err := tx.Exec(fmt.Sprintf(
		`CREATE TABLE %s_term_search (
			id int auto_increment,
			word_id varchar(40) NOT NULL,
			value VARCHAR(80),
			PRIMARY KEY (id),
			FOREIGN KEY (word_id) REFERENCES %s_word(id)
		) COLLATE utf8mb4_bin`,
		nfg.groupedName, nfg.groupedName)); err != nil {
		return fmt.Errorf(errMsgTpl, err)
	}

	if _, err := tx.Exec(fmt.Sprintf(
		`CREATE index %s_term_search_value_idx ON %s_term_search(value)`,
		nfg.groupedName, nfg.groupedName,
	)); err != nil {
		return fmt.Errorf(errMsgTpl, err)
	}
	if _, err := tx.Exec(fmt.Sprintf(
		`CREATE index %s_word_pos_idx ON %s_word(pos)`,
		nfg.groupedName, nfg.groupedName,
	)); err != nil {
		return fmt.Errorf(errMsgTpl, err)
	}
	return nil
}

// procLine processes current ngRecord item (= vertical file line containing a token data)
// with respect to currently processed currLemma and collected sublemmas.
// Please note that should the method to work as expected, it is critical to process
// the token data ordered by word, sublemma, lemma. Otherwise, the procLine method
// won't be able to detect end of the current lemma forms (and sublemmas).
func (nfg *NgramFreqGenerator) procLine(
	tx *sql.Tx,
	word *ngRecord,
) error {

	if _, err := tx.Exec(fmt.Sprintf(
		`INSERT INTO %s_word (id, value, lemma, sublemma, pos, count, arf, initial_cap)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, nfg.groupedName),
		word.hashId,
		word.word,
		word.lemma,
		word.sublemma,
		nfg.posFn.Transform(word.tag),
		word.abs,
		word.arf,
		word.initialCap,
	); err != nil {
		return fmt.Errorf("failed to process word line: %w", err)
	}
	for trm, _ := range map[string]bool{word.word: true, word.lemma: true, word.sublemma: true} {
		if _, err := tx.Exec(
			fmt.Sprintf(
				`INSERT INTO %s_term_search (value, word_id) VALUES (?, ?)`,
				nfg.groupedName,
			),
			trm,
			word.hashId,
		); err != nil {
			return fmt.Errorf("failed to process word line: %w", err)
		}
	}
	return nil
}

func (nfg *NgramFreqGenerator) findTotalNumLines() (int, error) {
	// TODO the following query is not general enough
	row := nfg.db.QueryRow(
		fmt.Sprintf(
			"SELECT COUNT(*) "+
				"FROM %s_colcounts "+
				"WHERE %s <> ? ", nfg.groupedName, nfg.qsaAttrs.ExportCols("tag")[0]),
		NonWordCSCNC2020Tag,
	)
	if row.Err() != nil {
		return -1, row.Err()
	}
	var ans int
	err := row.Scan(&ans)
	if err != nil {
		return -1, err
	}
	return ans, nil
}

// run generates n-grams (structured into 'word', 'lemma', 'sublemma') into intermediate database
// An existing database transaction must be provided along with current calculation status (which is
// progressively updated) and a status channel where the status is sent each time some significant
// update is encountered (typically - a chunk of items is finished or an error occurs)
func (nfg *NgramFreqGenerator) run(
	ctx context.Context,
	tx *sql.Tx,
	currStatus *genNgramsStatus,
	statusChan chan<- genNgramsStatus,
) error {

	total, err := nfg.findTotalNumLines()
	log.Debug().Int("numProcess", total).Msg("starting to process colcounts table for ngrams")
	if err != nil {
		return fmt.Errorf("failed to run n-gram generator: %w", err)
	}
	estim, err := db.EstimateProcTimeSecs(nfg.db, "ngrams", total)
	if err == db.ErrorEstimationNotAvail {
		log.Warn().Msgf("processing estimation not (yet) available for %s", nfg.corpusName)
		estim = -1
	} else if err != nil {
		return fmt.Errorf("failed to run n-gram generator: %w", err)
	}
	if estim > 0 {
		currStatus.TimeEstimationSecs = estim
		statusChan <- *currStatus
	}
	log.Info().Msgf(
		"About to process %d lines of raw n-grams for corpus %s. Time estimation (seconds): %d",
		total, nfg.corpusName, estim)
	var numStop int
	t0 := time.Now()

	rows, err := nfg.db.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT hash_id, %s, `count` AS abs, arf, initial_cap "+
				"FROM %s_colcounts "+
				"WHERE col%d <> ? ",
			strings.Join(nfg.qsaAttrs.ExportCols("word", "sublemma", "lemma", "tag"), ", "),
			nfg.groupedName,
			nfg.qsaAttrs.Tag,
		),
		NonWordCSCNC2020Tag,
	)
	log.Debug().Msg("table selection done, moving to import")
	if err != nil {
		return fmt.Errorf("failed to run n-gram generator: %w", err)
	}
	var numProcessed int
	for rows.Next() {
		rec := new(ngRecord)
		err := rows.Scan(
			&rec.hashId,
			&rec.word,
			&rec.lemma,
			&rec.sublemma,
			&rec.tag,
			&rec.abs,
			&rec.arf,
			&rec.initialCap,
		)
		if err != nil {
			return fmt.Errorf("failed to run n-gram generator: %w", err)
		}
		if isStopNgram(rec.lemma) {
			numStop++
			continue
		}
		if err := nfg.procLine(tx, rec); err != nil {
			return fmt.Errorf("failed to run n-gram generator: %w", err)
		}
		numProcessed++
		if numProcessed%reportEachNthItem == 0 {
			procTime := time.Since(t0).Seconds()
			currStatus.NumProcLines = numProcessed
			currStatus.AvgSpeedItemsPerSec = int(math.RoundToEven(float64(numProcessed) / procTime))
			statusChan <- *currStatus
		}
		select {
		case <-ctx.Done():
			if currStatus.Error != nil {
				currStatus.Error = fmt.Errorf("ngram generator cancelled")
				statusChan <- *currStatus
				return nil
			}
		default:
		}
	}
	currStatus.NumProcLines = numProcessed
	procTime := time.Since(t0).Seconds()
	currStatus.AvgSpeedItemsPerSec = int(math.RoundToEven(float64(numProcessed) / procTime))
	if err := db.AddProcTimeEntry(
		nfg.db,
		"ngrams",
		total,
		numProcessed,
		procTime,
	); err != nil {
		log.Err(err).Msg("failed to write proc_time statistics")
		currStatus.Error = err
	}
	statusChan <- *currStatus
	log.Info().Msgf("num stop words: %d", numStop)
	return nil
}

// generateSync (synchronously) generates n-grams from raw liveattrs data
// provided statusChan is closed by the method once
// the operation finishes
func (nfg *NgramFreqGenerator) generateSync(ctx context.Context, statusChan chan<- genNgramsStatus) {
	var status genNgramsStatus
	tx, err := nfg.db.Begin()
	if err != nil {
		tx.Rollback()
		status.Error = err
		statusChan <- status
		return
	}
	err = nfg.createTables(tx)
	status.TablesReady = true
	statusChan <- status
	if err != nil {
		tx.Rollback()
		status.Error = err
		statusChan <- status
		return
	}
	err = nfg.run(ctx, tx, &status, statusChan)
	if err != nil {
		tx.Rollback()
		status.Error = err
		statusChan <- status
		return
	}
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		status.Error = err
		statusChan <- status
	}
}

func (nfg *NgramFreqGenerator) Generate(corpusID string) (NgramJobInfo, error) {
	return nfg.GenerateAfter(corpusID, "")
}

func (nfg *NgramFreqGenerator) GenerateAfter(corpusID, parentJobID string) (NgramJobInfo, error) {
	jobID, err := uuid.NewUUID()
	if err != nil {
		return NgramJobInfo{}, err
	}
	status := NgramJobInfo{
		ID:       jobID.String(),
		Type:     "ngram-generating",
		CorpusID: corpusID,
		Start:    jobs.CurrentDatetime(),
		Update:   jobs.CurrentDatetime(),
		Finished: false,
		Args:     NgramJobInfoArgs{},
	}
	fn := func(updateJobChan chan<- jobs.GeneralJobInfo) {
		statusChan := make(chan genNgramsStatus)
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		go func(runStatus NgramJobInfo) {
			defer close(updateJobChan)
			for statUpd := range statusChan {
				runStatus.Result = statUpd
				runStatus.Error = statUpd.Error
				runStatus.Update = jobs.CurrentDatetime()
				updateJobChan <- runStatus
				if runStatus.Error != nil {
					runStatus.Finished = true
					cancel()
				}
			}
			runStatus.Update = jobs.CurrentDatetime()
			runStatus.Finished = true
			updateJobChan <- runStatus
		}(status)
		nfg.generateSync(ctx, statusChan)
		close(statusChan)
	}
	if parentJobID != "" {
		nfg.jobActions.EqueueJobAfter(&fn, &status, parentJobID)

	} else {
		nfg.jobActions.EnqueueJob(&fn, &status)
	}
	return status, nil
}

func NewNgramFreqGenerator(
	db *sql.DB,
	jobActions *jobs.Actions,
	groupedName string,
	corpusName string,
	posFn *modders.StringTransformerChain,
	qsaAttrs QSAttributes,
) *NgramFreqGenerator {
	return &NgramFreqGenerator{
		db:          db,
		jobActions:  jobActions,
		groupedName: groupedName,
		corpusName:  corpusName,
		posFn:       posFn,
		qsaAttrs:    qsaAttrs,
	}
}

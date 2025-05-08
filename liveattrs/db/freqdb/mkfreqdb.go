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
	"frodo/db/mysql"
	"frodo/jobs"
	"frodo/liveattrs/db"
	"math"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/util"
	"github.com/czcorpus/vert-tagextract/v3/ptcount/modders"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
)

const (
	reportEachNthItem   = 10000
	procChunkSize       = 50000
	duplicateRowErrNo   = 1062
	NonWordCSCNC2020Tag = "X@-------------"
)

type NgramFreqGenerator struct {
	db              *mysql.Adapter
	customDBDataDir string
	groupedName     string
	corpusName      string
	appendExisting  bool
	ngramSize       int
	posFn           *modders.StringTransformerChain
	jobActions      *jobs.Actions
	qsaAttrs        QSAttributes
}

func (nfg *NgramFreqGenerator) createTables(tx *sql.Tx) error {
	errMsgTpl := "failed to create tables: %w"

	if _, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s_term_search", nfg.groupedName)); err != nil {
		return fmt.Errorf(errMsgTpl, err)
	}
	if _, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s_word", nfg.groupedName)); err != nil {
		return fmt.Errorf(errMsgTpl, err)
	}
	dataDirSQL := util.Ternary(
		nfg.customDBDataDir != "",
		fmt.Sprintf(" DATA DIRECTORY '%s'", nfg.customDBDataDir),
		"",
	)
	if _, err := tx.Exec(
		fmt.Sprintf(
			`CREATE TABLE %s_word (
			id varchar(40),
			value TEXT,
			lemma TEXT,
			sublemma TEXT,
			pos VARCHAR(20),
			count INTEGER,
			ngram TINYINT NOT NULL,
			arf FLOAT,
			sim_freqs_score FLOAT NOT NULL DEFAULT 0,
			initial_cap TINYINT NOT NULL DEFAULT 0,
			PRIMARY KEY (id)
			) COLLATE utf8mb4_bin %s`,
			nfg.groupedName,
			dataDirSQL,
		),
	); err != nil {
		return fmt.Errorf(errMsgTpl, err)
	}
	if _, err := tx.Exec(fmt.Sprintf(
		`CREATE TABLE %s_term_search (
			id int auto_increment,
			word_id varchar(40) NOT NULL,
			value TEXT,
			PRIMARY KEY (id),
			FOREIGN KEY (word_id) REFERENCES %s_word(id)
		) COLLATE utf8mb4_bin %s`,
		nfg.groupedName, nfg.groupedName, dataDirSQL)); err != nil {
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
	if _, err := tx.Exec(fmt.Sprintf(
		`create index %s_word_sim_freqs_score_idx on %s_word(sim_freqs_score, ngram)`,
		nfg.groupedName, nfg.groupedName,
	)); err != nil {
		return fmt.Errorf(errMsgTpl, err)
	}
	if _, err := tx.Exec(fmt.Sprintf(
		`create index %s_word_lemma_idx on %s_word(lemma)`,
		nfg.groupedName, nfg.groupedName,
	)); err != nil {
		return fmt.Errorf(errMsgTpl, err)
	}
	return nil
}

// determineSimFreqsScore calculates simFreqScore for all the provided words
// The words must be in proper order (ordered by lemma, pos) so the method
// is able to sum everything properly and fill in the final value to all the
// words within the group.
func (nfg *NgramFreqGenerator) determineSimFreqsScore(words []*ngRecord) {
	var currLemma, currPos string
	var prevLemmaStart int
	var score float64
	for i, word := range words {
		if word.lemma != currLemma || word.tag[:1] != currPos {
			for j := prevLemmaStart; j < i; j++ {
				words[j].simFreqsScore = score
			}
			prevLemmaStart = i
			score = 0
		}
		currLemma = word.lemma
		currPos = word.tag[:1]
		score += word.arf
	}
	for j := prevLemmaStart; j < len(words); j++ {
		words[j].simFreqsScore = score
	}
}

// procLineGroup processes provided list of ngRecord items (= vertical file line containing
// a token data) with respect to currently processed currLemma and collected sublemmas.
//
// Please note that should the method to work as expected, it is critical to process
// the token data ordered by word, sublemma, lemma. Otherwise, the procLine method
// won't be able to detect end of the current lemma forms (and sublemmas).
func (nfg *NgramFreqGenerator) procLineGroup(
	tx *sql.Tx,
	words []*ngRecord,
) error {
	valPlaceholders := make([]string, len(words))
	queryArgs := make([]any, 0, len(words)*9)

	for i := 0; i < len(words); i++ {
		valPlaceholders[i] = "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
		queryArgs = append(
			queryArgs,
			words[i].hashId,
			words[i].word,
			words[i].lemma,
			words[i].sublemma,
			strings.Join(
				collections.SliceMap(
					strings.Split(words[i].tag, " "),
					func(v string, i int) string {
						return nfg.posFn.Transform(v)
					},
				),
				" ",
			),
			words[i].abs,
			words[i].arf,
			words[i].initialCap,
			words[i].ngramSize,
			words[i].simFreqsScore,
		)
	}

	if _, err := tx.Exec(
		fmt.Sprintf(
			`INSERT INTO %s_word (id, value, lemma, sublemma, pos, count, arf, initial_cap, ngram, sim_freqs_score)
			VALUES %s`,
			nfg.groupedName,
			strings.Join(valPlaceholders, ", "),
		),
		queryArgs...,
	); err != nil {
		return fmt.Errorf("failed to process word line: %w", err)
	}
	// srch term insert
	// word/lemma/sublemma can be the same so we cannot determine
	// exact size of stPlaceholders and stArgs below
	stPlaceholders := make([]string, 0, 3*len(words))
	stArgs := make([]any, 0, 3*len(words))
	for _, word := range words {
		for trm := range map[string]bool{word.word: true, word.lemma: true, word.sublemma: true} {
			stPlaceholders = append(stPlaceholders, "(?, ?)")
			stArgs = append(stArgs, trm, word.hashId)
		}
	}
	if _, err := tx.Exec(
		fmt.Sprintf(
			`INSERT INTO %s_term_search (value, word_id) VALUES %s`,
			nfg.groupedName,
			strings.Join(stPlaceholders, ", "),
		),
		stArgs...,
	); err != nil {
		return fmt.Errorf("failed to process word line: %w", err)
	}
	return nil
}

func (nfg *NgramFreqGenerator) findTotalNumLines() (int, error) {
	// TODO the following query is not general enough
	row := nfg.db.DB().QueryRow(
		fmt.Sprintf(
			"SELECT COUNT(*) "+
				"FROM %s_colcounts "+
				"WHERE %s <> ? AND ngram_size = ? ",
			nfg.groupedName,
			nfg.qsaAttrs.ExportCols("tag")[0],
		),
		NonWordCSCNC2020Tag,
		nfg.ngramSize,
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

func (nfg *NgramFreqGenerator) preloadCols(
	ctx context.Context,
	totalItems int64,
	baseStatus genNgramsStatus,
	statusCh chan<- genNgramsStatus,
) []*ngRecord {
	baseStatus.CurrAction = "preloading cols"
	rows, err := nfg.db.DB().QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT hash_id, %s, `count` AS abs, arf, initial_cap "+
				"FROM %s_colcounts "+
				"WHERE col%d <> ? AND ngram_size = ? ",
			strings.Join(nfg.qsaAttrs.ExportCols("word", "sublemma", "lemma", "tag"), ", "),
			nfg.groupedName,
			nfg.qsaAttrs.Tag,
		),
		NonWordCSCNC2020Tag,
		nfg.ngramSize,
	)
	if err != nil {
		baseStatus.Error = fmt.Errorf("failed to select data for the chunk: %w", err)
		statusCh <- baseStatus
		return []*ngRecord{}
	}
	ngrams := make([]*ngRecord, 0, totalItems)
	for rowNum := 1; rows.Next(); rowNum++ {
		baseStatus.NumProcLines = rowNum
		rec := &ngRecord{ngramSize: nfg.ngramSize}
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
			baseStatus.Error = fmt.Errorf("failed to process db row %d for the chunk: %w", rowNum, err)
			statusCh <- baseStatus
			return []*ngRecord{}
		}
		if isStopNgram(rec.lemma) {
			continue
		}
		ngrams = append(ngrams, rec)
	}

	slices.SortFunc(
		ngrams,
		func(a, b *ngRecord) int { return strings.Compare(a.lemma+a.tag, b.lemma+b.tag) },
	)
	baseStatus.CurrAction = fmt.Sprintf("sorting ngram list of size %d", len(ngrams))
	statusCh <- baseStatus
	nfg.determineSimFreqsScore(ngrams)
	return ngrams
}

func (nfg *NgramFreqGenerator) procChunk(
	ctx context.Context,
	ngrams []*ngRecord,
	baseStatus genNgramsStatus,
	t0 time.Time,
	statusCh chan<- genNgramsStatus,
) bool {
	if len(ngrams) == 0 {
		return true
	}
	baseStatus.CurrAction = fmt.Sprintf("starting to process chunkID %d of size %d", baseStatus.ChunkID, len(ngrams))
	statusCh <- baseStatus
	tx, err := nfg.db.DB().Begin()
	if err != nil {
		tx.Rollback()
		baseStatus.Error = fmt.Errorf("failed to process chunk: %w", err)
		statusCh <- baseStatus
		return false
	}

	baseStatus.CurrAction = fmt.Sprintf("processing selected rows for the chunk")
	statusCh <- baseStatus

	rowBatch := make([]*ngRecord, 0, 100)

	procRowBatch := func(rowNum int, batch []*ngRecord) bool {
		if err := nfg.procLineGroup(tx, batch); err != nil {
			tx.Rollback()
			baseStatus.Error = fmt.Errorf(
				"failed to process db row %d for chunkID %d: %w", rowNum, baseStatus.ChunkID, err)
			statusCh <- baseStatus
			return false
		}
		procTime := time.Since(t0).Seconds()
		if (baseStatus.ChunkID*procChunkSize+rowNum)%reportEachNthItem == 0 {
			baseStatus.AvgSpeedItemsPerSec = int(math.RoundToEven(float64(baseStatus.ChunkID*procChunkSize+rowNum) / procTime))
			statusCh <- baseStatus

			if err := db.AddProcTimeEntry(
				nfg.db.DB(),
				"ngrams",
				baseStatus.TotalLines,
				baseStatus.ChunkID*procChunkSize+rowNum,
				procTime,
			); err != nil {
				log.Err(err).Msg("failed to write proc_time statistics (ignoring the error)")
			}
		}
		return true
	}

	var rowNum int
	for i, rec := range ngrams {
		rowNum = i + 1
		rowBatch = append(rowBatch, rec)

		if len(rowBatch) == 100 {
			if ok := procRowBatch(rowNum, rowBatch); !ok {
				return false
			}
			rowBatch = make([]*ngRecord, 0, 100)
		}
		select {
		case <-ctx.Done():
			baseStatus.Error = fmt.Errorf("action cancelled")
			statusCh <- baseStatus
			return false
		default:
		}
	}
	if len(rowBatch) > 0 {
		if ok := procRowBatch(rowNum, rowBatch); !ok {
			return false
		}
	}

	if err := tx.Commit(); err != nil {
		baseStatus.Error = fmt.Errorf("failed to commit transaction: %w", err)
		statusCh <- baseStatus
		return false
	}

	return true
}

// run generates n-grams (structured into 'word', 'lemma', 'sublemma') into intermediate database
// An existing database transaction must be provided along with current calculation status (which is
// progressively updated) and a status channel where the status is sent each time some significant
// update is encountered (typically - a chunk of items is finished or an error occurs)
func (nfg *NgramFreqGenerator) run(
	ctx context.Context,
	statusChan chan<- genNgramsStatus,
) bool {
	baseStatus := genNgramsStatus{
		CorpusID:    nfg.corpusName,
		TablesReady: true,
		CurrAction:  "starting to process colcounts table for ngrams",
	}
	total, err := nfg.findTotalNumLines()
	if err != nil {
		baseStatus.Error = fmt.Errorf("failed to run n-gram generator: %w", err)
		statusChan <- baseStatus
		return false
	}
	baseStatus.TotalLines = total
	estim, err := db.EstimateProcTimeSecs(nfg.db.DB(), "ngrams", total)
	if err == db.ErrorEstimationNotAvail {
		baseStatus.ClientWarn = fmt.Sprintf("processing estimation not (yet) available for %s", nfg.corpusName)
		statusChan <- baseStatus
		estim = -1

	} else if err != nil {
		baseStatus.Error = fmt.Errorf("failed to run n-gram generator: %w", err)
		statusChan <- baseStatus
		return false
	}
	if estim > 0 {
		baseStatus.TimeEstimationSecs = estim
		baseStatus.CurrAction = "prepared for processing, calculated time estimation"
		statusChan <- baseStatus
	}
	log.Info().Msgf(
		"About to process %d lines of raw n-grams for corpus %s. Time estimation (seconds): %d",
		total, nfg.corpusName, estim)
	t0 := time.Now()

	ngrams := nfg.preloadCols(ctx, int64(total), baseStatus, statusChan)
	if len(ngrams) == 0 {
		return false
	}

	numChunks := int(math.Ceil(float64(len(ngrams)) / float64(procChunkSize)))
	for i := 0; i < numChunks; i++ {
		baseStatus := genNgramsStatus{
			CorpusID:           nfg.corpusName,
			ChunkID:            i,
			TotalLines:         total,
			TablesReady:        true,
			TimeEstimationSecs: estim,
			NumProcLines:       i * procChunkSize,
		}
		if ok := nfg.procChunk(
			ctx,
			ngrams[i*procChunkSize:min((i+1)*procChunkSize, len(ngrams))],
			baseStatus,
			t0,
			statusChan,
		); !ok {
			return false
		}
	}
	return true
}

func (nfg *NgramFreqGenerator) tablesExist() (bool, error) {
	row := nfg.db.DB().QueryRow(
		`SELECT COUNT(*) > 0 FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`,
		nfg.db.DBName(), nfg.groupedName+"_word",
	)
	var ans bool
	err := row.Scan(&ans)
	if err != nil {
		return false, err
	}
	return ans, nil
}

// generateSync (synchronously) generates n-grams from raw liveattrs data
// provided statusChan is closed by the method once
// the operation finishes
func (nfg *NgramFreqGenerator) generateSync(
	ctx context.Context,
	statusChan chan<- genNgramsStatus,
) {
	var status genNgramsStatus
	tx, err := nfg.db.DB().Begin()
	if err != nil {
		tx.Rollback()
		status.Error = err
		statusChan <- status
		return
	}

	tblEx, err := nfg.tablesExist()
	if err != nil {
		status.Error = fmt.Errorf("failed to generate ngrams: %w", err)
		statusChan <- status
		return
	}
	if nfg.appendExisting && !tblEx {
		status.Error = fmt.Errorf("failed to generate ngrams: using append mode but tables are missing")
		statusChan <- status
		return
	}
	if !nfg.appendExisting {
		err = nfg.createTables(tx)
	}

	status.TablesReady = true
	statusChan <- status
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
		return
	}
	nfg.run(ctx, statusChan)
}

// GenerateAfter creates a new job to generate ngrams. In case
// parentJobID is not empty, the new job will start after the parent
// finishes.
func (nfg *NgramFreqGenerator) GenerateAfter(parentJobID string) (NgramJobInfo, error) {
	jobID, err := uuid.NewUUID()
	if err != nil {
		return NgramJobInfo{}, err
	}
	jobStatus := NgramJobInfo{
		ID:       jobID.String(),
		Type:     "ngram-generating",
		CorpusID: nfg.corpusName,
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
				if statUpd.ClientWarn != "" {
					log.Warn().
						Str("corpusId", statUpd.CorpusID).
						Int("totalLines", statUpd.TotalLines).
						Int("numProcLines", statUpd.NumProcLines).
						Int("chunkId", statUpd.ChunkID).
						Int("avgSpeedItemsPerSec", statUpd.AvgSpeedItemsPerSec).
						Int("timeEstimationSecs", statUpd.TimeEstimationSecs).
						Str("currAction", statUpd.CurrAction).
						Msg(statUpd.ClientWarn)

				} else if statUpd.Error != nil {
					log.Error().
						Str("corpusId", statUpd.CorpusID).
						Int("totalLines", statUpd.TotalLines).
						Int("numProcLines", statUpd.NumProcLines).
						Int("chunkId", statUpd.ChunkID).
						Int("avgSpeedItemsPerSec", statUpd.AvgSpeedItemsPerSec).
						Int("timeEstimationSecs", statUpd.TimeEstimationSecs).
						Str("currAction", statUpd.CurrAction).
						Err(statUpd.Error).
						Msg("failed to process ngram job")

				} else {
					log.Info().
						Str("corpusId", statUpd.CorpusID).
						Int("totalLines", statUpd.TotalLines).
						Int("numProcLines", statUpd.NumProcLines).
						Int("chunkId", statUpd.ChunkID).
						Int("avgSpeedItemsPerSec", statUpd.AvgSpeedItemsPerSec).
						Int("timeEstimationSecs", statUpd.TimeEstimationSecs).
						Str("currAction", statUpd.CurrAction).
						Err(statUpd.Error).
						Msg("reporting job status")
				}

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
		}(jobStatus)
		nfg.generateSync(ctx, statusChan)
		close(statusChan)
		if err := nfg.db.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close import-tuned connection")
		}
	}
	if parentJobID != "" {
		nfg.jobActions.EqueueJobAfter(&fn, &jobStatus, parentJobID)

	} else {
		nfg.jobActions.EnqueueJob(&fn, &jobStatus)
	}
	return jobStatus, nil
}

func NewNgramFreqGenerator(
	db *mysql.Adapter,
	jobActions *jobs.Actions,
	groupedName string,
	corpusName string,
	customDBDataDir string,
	appendExisting bool,
	ngramSize int,
	posFn *modders.StringTransformerChain,
	qsaAttrs QSAttributes,
) *NgramFreqGenerator {
	return &NgramFreqGenerator{
		db:              db,
		jobActions:      jobActions,
		groupedName:     groupedName,
		corpusName:      corpusName,
		customDBDataDir: customDBDataDir,
		ngramSize:       ngramSize,
		posFn:           posFn,
		qsaAttrs:        qsaAttrs,
		appendExisting:  appendExisting,
	}
}

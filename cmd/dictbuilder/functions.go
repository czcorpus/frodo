// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"frodo/db/mysql"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/rs/zerolog/log"
)

func replaceTable(db *mysql.Adapter, corpusName string, tmpCorpusName string, suffix string) error {
	// Delete existing table if it exists
	_, err := db.DB().Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s_%s", corpusName, suffix))
	if err != nil {
		return err
	}

	// Rename table in the database
	_, err = db.DB().Exec(fmt.Sprintf("ALTER TABLE %s_%s RENAME TO %s_%s", tmpCorpusName, suffix, corpusName, suffix))
	if err != nil {
		return err
	}

	return nil
}

// refreshJobStatus asks for job status via http and updates provided job object
// (which is a simplified subset of all possible job statuses we can encounter here).
// The function also returns raw job info response for later processing.
func refreshJobStatus(jobURL string, job *JobStatus) ([]byte, error) {
	resp, err := http.Get(jobURL)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}

	if err := json.Unmarshal(body, job); err != nil {
		return []byte{}, err
	}

	return body, nil
}

func doJob(
	ctx context.Context,
	api string,
	jobPath string,
	jobParams url.Values,
	jobArgs any,
	maxProcTime time.Duration,
	onFinalStatus func(response []byte) error,
) error {
	jobURL, err := url.JoinPath(api, jobPath)
	if err != nil {
		return err
	}
	if len(jobParams) > 0 {
		jobURL += "?" + jobParams.Encode()
	}

	args, err := json.Marshal(jobArgs)
	if err != nil {
		return err
	}
	resp, err := http.Post(jobURL, "application/json", bytes.NewBuffer(args))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var job JobStatus
	if err := json.Unmarshal(body, &job); err != nil {
		return err
	}
	if job.Error != "" {
		return errors.New(job.Error)
	}

	// periodically check job status
	refreshURL, err := url.JoinPath(api, "jobs", job.ID)
	if err != nil {
		return err
	}
	log.Info().Msgf("Job started with ID: %s", job.ID)
	var rawJobStatus []byte
	t0 := time.Now()
	for !job.Finished && time.Since(t0) < maxProcTime {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
		respData, err := refreshJobStatus(refreshURL, &job)
		if err != nil {
			return err
		}
		rawJobStatus = respData
	}

	if !job.Finished {
		return fmt.Errorf("job timeout - took %.2f seconds", time.Since(t0).Seconds())
	}
	if !job.OK {
		return errors.New(job.Error)
	}

	if onFinalStatus != nil {
		if err := onFinalStatus(rawJobStatus); err != nil {
			return fmt.Errorf("job's onFinalStatus failed: %w", err)
		}
	}

	log.Info().Msg("Job finished successfully")
	return nil
}

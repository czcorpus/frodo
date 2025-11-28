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

func refreshJobStatus(jobURL string, job *JobStatus) error {
	resp, err := http.Get(jobURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, job); err != nil {
		return err
	}

	return nil
}

func doJob(api string, jobPath string, jobParams url.Values, jobArgs any) error {
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
	for !job.Finished {
		time.Sleep(5 * time.Second)
		if err := refreshJobStatus(refreshURL, &job); err != nil {
			return err
		}
	}

	if !job.OK {
		return errors.New(job.Error)
	}
	log.Info().Msg("Job finished successfully")
	return nil
}

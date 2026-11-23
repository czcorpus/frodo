// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Institute of the Czech National Corpus,
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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"frodo/jobs"
)

type utilization struct {
	MaxNumConcurrentJobs int     `json:"maxNumConcurrentJobs"`
	CurrentRunningJobs   int     `json:"currentRunningJobs"`
	Utilization          float64 `json:"utilization"`
	JobQueueLength       int     `json:"jobQueueLength"`
}

type apiClient struct {
	baseURL string
	http    *http.Client
}

func newAPIClient(baseURL string) *apiClient {
	return &apiClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *apiClient) get(path string) ([]byte, error) {
	resp, err := c.http.Get(c.baseURL + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		return nil, fmt.Errorf("GET %s: %s: %s", path, resp.Status, msg)
	}
	return body, nil
}

func (c *apiClient) jobList(unfinishedOnly bool) (jobs.JobInfoListCompact, error) {
	path := "/jobs?compact=1"
	if unfinishedOnly {
		path += "&unfinishedOnly=1"
	}
	body, err := c.get(path)
	if err != nil {
		return nil, err
	}
	var ans jobs.JobInfoListCompact
	if err := json.Unmarshal(body, &ans); err != nil {
		return nil, fmt.Errorf("failed to decode job list: %w", err)
	}
	return ans, nil
}

func (c *apiClient) utilization() (utilization, error) {
	var ans utilization
	body, err := c.get("/jobs/utilization")
	if err != nil {
		return ans, err
	}
	if err := json.Unmarshal(body, &ans); err != nil {
		return ans, fmt.Errorf("failed to decode utilization: %w", err)
	}
	return ans, nil
}

// jobDetail returns the full job information as indented JSON
// (the structure differs among job types).
func (c *apiClient) jobDetail(jobID string) (string, error) {
	body, err := c.get("/jobs/" + url.PathEscape(jobID))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, body, "", "  "); err != nil {
		return "", fmt.Errorf("failed to decode job detail: %w", err)
	}
	return buf.String(), nil
}

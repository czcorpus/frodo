// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
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

package ssjc

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type SSJCFileRow struct {
	ParentID     string
	ID           string
	Headword     string
	HeadwordType string
	Pos          string
	Gender       string
	Aspect       string
}

func ReadTSV(path string) ([]SSJCFileRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open TSV file: %w", err)
	}
	defer f.Close()

	var rows []SSJCFileRow
	scanner := bufio.NewScanner(f)
	lineNum := 0
	scanner.Scan() // first line header // TODO configurable
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 7 {
			return nil, fmt.Errorf("line %d: expected 7 fields, got %d", lineNum, len(fields))
		}
		rows = append(rows, SSJCFileRow{
			ParentID:     fields[0],
			ID:           fields[1],
			Headword:     fields[2],
			HeadwordType: fields[3],
			Pos:          fields[4],
			Gender:       fields[5],
			Aspect:       fields[6],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read TSV file: %w", err)
	}
	return rows, nil
}

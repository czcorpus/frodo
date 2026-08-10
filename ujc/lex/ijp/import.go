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

package ijp

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

const (
	procChunkSize = 5
)

type SrcFileRow struct {
	GroupID    string
	ExternalID string
	Variant    string
	Also       string
	Pos        string
	Gender     string
	Aspect     string
}

type importDataChunk struct {
	Items    []SrcFileRow
	Error    error
	NotFatal bool
}

func ReadTSV(ctx context.Context, path string) (<-chan importDataChunk, error) {
	ans := make(chan importDataChunk, 50)
	go func() {
		defer close(ans)
		f, err := os.Open(path)
		if err != nil {
			ans <- importDataChunk{Error: fmt.Errorf("failed to open TSV file: %w", err)}
			return
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		lineNum := 0
		scanner.Scan() // first line header // TODO configurable

		chunk := make([]SrcFileRow, procChunkSize)
		i := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if line == "" {
				continue
			}
			fields := strings.Split(line, "\t")
			if len(fields) != 7 {
				ans <- importDataChunk{Error: fmt.Errorf("line %d: expected 7 fields, got %d", lineNum, len(fields))}
				return
			}
			chunk[i] = SrcFileRow{
				GroupID:    fields[0],
				ExternalID: fields[1],
				Variant:    fields[2],
				Also:       fields[3],
				Pos:        fields[4],
				Gender:     fields[5],
				Aspect:     fields[6],
			}
			if chunk[i].Pos == "" {
				ans <- importDataChunk{Error: fmt.Errorf("line %d: empty pos", lineNum), NotFatal: true}
				continue
			}
			if i == procChunkSize-1 {
				select {
				case <-ctx.Done():
					return
				default:
				}

				ans <- importDataChunk{Items: chunk}
				i = 0
				chunk = make([]SrcFileRow, procChunkSize)

			} else {
				i++
			}
		}
		if err := scanner.Err(); err != nil {
			ans <- importDataChunk{Error: fmt.Errorf("failed to read TSV file: %w", err)}
		}
		if i > 0 {
			ans <- importDataChunk{Items: chunk[:i]}
		}
	}()

	return ans, nil
}

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

package main

import (
	"bufio"
	"context"
	"fmt"
	"frodo/ujc/ssjc"
	"os"
	"strings"
)

type recType int

const (
	recTypeParent recType = 0
	recTypeChild  recType = 1
	procChunkSize         = 5
)

type importDataChunk struct {
	Items []ssjc.SrcFileRow
	Error error
}

func ReadTSV(ctx context.Context, path string, recType recType) (<-chan importDataChunk, error) {
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

		chunk := make([]ssjc.SrcFileRow, procChunkSize)
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
			if recType == recTypeChild && fields[0] != "" || recType == recTypeParent && fields[0] == "" {
				chunk[i] = ssjc.SrcFileRow{
					ParentID:     fields[0],
					ID:           fields[1],
					Headword:     fields[2],
					HeadwordType: fields[3],
					Pos:          fields[4],
					Gender:       fields[5],
					Aspect:       fields[6],
				}
				if i == procChunkSize-1 {
					select {
					case <-ctx.Done():
						return
					default:
					}

					ans <- importDataChunk{Items: chunk}
					i = 0
					chunk = make([]ssjc.SrcFileRow, procChunkSize)

				} else {
					i++
				}
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

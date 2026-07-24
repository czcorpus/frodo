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

package assc

import (
	"bufio"
	"context"
	"fmt"
	"frodo/ujc/lex"
	"os"
	"strconv"
	"strings"

	"github.com/czcorpus/cnc-gokit/util"
)

const (
	procChunkSize = 5
)

type SrcFileRow struct {
	ParentID    string
	EntryID     string
	Type        string
	SortOrder   string
	Variant     string
	LemmaType   string
	Homonymy    string
	Pos         string
	Gender      string
	Aspect      string
	Uninflected string
	Plurality   string
	Changed     string
}

type importDataChunk struct {
	Items []SrcFileRow
	Error error
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
			if len(fields) != 13 {
				ans <- importDataChunk{Error: fmt.Errorf("line %d: expected 13 fields, got %d", lineNum, len(fields))}
				return
			}
			pluralityIndexMap := map[string]int{"": lex.PluralityNone, "množné": lex.PluralityPlural, "jen množné": lex.PluralityAlways, "zprav. množné": lex.PluralityUsually, "pomnožné": lex.PluralityOnly}
			plurality, ok := pluralityIndexMap[fields[11]]
			if !ok {
				ans <- importDataChunk{Error: fmt.Errorf("unknown plurality value: %s", fields[11])}
			}
			chunk[i] = SrcFileRow{
				ParentID:    fields[0],
				EntryID:     fields[1],
				Type:        fields[2],
				SortOrder:   fields[3],
				Variant:     fields[4],
				LemmaType:   fields[5],
				Homonymy:    fields[6],
				Pos:         fields[7],
				Gender:      util.Ternary(fields[8] == "-", "", fields[8]),
				Aspect:      util.Ternary(fields[9] == "-", "", fields[9]),
				Uninflected: util.Ternary(fields[10] == "", "0", "1"),
				Plurality:   strconv.Itoa(plurality),
				Changed:     fields[12],
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

// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Institute of the Czech National Corpus,
//
//	Faculty of Arts, Charles University
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

package metadb

import (
	"context"
	"database/sql"
	"frodo/corpus"

	"github.com/czcorpus/mquery-common/corp"
)

type NoOpResult struct {
}

func (res *NoOpResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (res *NoOpResult) RowsAffected() (int64, error) {
	return 0, nil
}

// ------------------

type NoOpTx struct {
}

func (tx *NoOpTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return &NoOpResult{}, nil
}

func (tx *NoOpTx) Exec(query string, args ...any) (sql.Result, error) {
	return &NoOpResult{}, nil
}

func (tx *NoOpTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return &sql.Rows{}, nil
}

func (tx *NoOpTx) Query(query string, args ...any) (*sql.Rows, error) {
	return &sql.Rows{}, nil
}

func (tx *NoOpTx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return &sql.Row{}
}

func (tx *NoOpTx) QueryRow(query string, args ...any) *sql.Row {
	return &sql.Row{}
}

func (tx *NoOpTx) Commit() error {
	return nil
}

func (tx *NoOpTx) Rollback() error {
	return nil
}

// ----------

type NoOpWriter struct {
}

func (w *NoOpWriter) StartTx() (SQLTx, error) {
	return &NoOpTx{}, nil
}

func (w *NoOpWriter) UnsetLiveAttrs(transact SQLTx, corpus string) error {
	return nil
}

func (w *NoOpWriter) SetLiveAttrs(
	transact SQLTx,
	corpus, bibIDStruct, bibIDAttr, tagAttr string,
	tagsetName corp.SupportedTagset,
) error {
	return nil
}

func (w *NoOpWriter) IfMissingAddCorpusBibMetadata(
	transact SQLTx,
	corpus, bibIDStruct, bibIDAttr, tagAttr string,
	tagsetName corp.SupportedTagset,
) error {
	return nil
}

// ------------------------------------

type StaticProvider struct {
	Corpora []corp.CorpusSetup
}

func (prov *StaticProvider) findEntry(corpusID string) corp.CorpusSetup {
	for _, v := range prov.Corpora {
		if v.ID == corpusID {
			return v
		}
	}
	return corp.CorpusSetup{}
}

func (prov *StaticProvider) LoadInfo(corpusID string) (*corpus.DBInfo, error) {
	info := prov.findEntry(corpusID)
	if info.ID == "" {
		// TODO: Not a great type for error here but must be compatible with sql backend
		return nil, sql.ErrNoRows
	}
	return &corpus.DBInfo{
		Name:               info.ID,
		Size:               info.Size,
		Active:             1,
		Locale:             "",
		HasLimitedVariant:  false,
		ParallelCorpus:     "",
		BibLabelAttr:       info.BibLabelAttr,
		BibIDAttr:          info.BibIDAttr,
		BibGroupDuplicates: 0,
	}, nil
}

func (prov *StaticProvider) LoadAliasedInfo(corpusID, aliasOf string) (*corpus.DBInfo, error) {
	var ans *corpus.DBInfo
	var err error
	if aliasOf != "" {
		ans, err = prov.LoadInfo(aliasOf)
		if err != nil {
			return nil, err
		}
		ans.Name = corpusID
		return ans, nil

	} else {
		return prov.LoadInfo(corpusID)
	}
}

func (prov *StaticProvider) GetCorpusTagsets(corpusID string) ([]corp.SupportedTagset, error) {
	info := prov.findEntry(corpusID)
	if info.ID == "" {
		return []corp.SupportedTagset{}, nil
	}
	return info.Tagsets, nil
}

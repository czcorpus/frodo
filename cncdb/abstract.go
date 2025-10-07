package cncdb

import (
	"context"
	"database/sql"
	"frodo/corpus"

	"github.com/czcorpus/mquery-common/corp"
)

type Provider interface {
	LoadInfo(corpusID string) (*corpus.DBInfo, error)

	GetCorpusTagsets(corpusID string) ([]corp.SupportedTagset, error)

	// LoadAliasedInfo loads info of corpus aliasOf as if it were corpus corpusID - i.e. the
	// data will be from aliasOf except for the name.
	// It is ok to provide an empty aliasOf in which case, the behavior will be just like
	// when calling LoadInfo
	LoadAliasedInfo(corpusID, aliasOf string) (*corpus.DBInfo, error)
}

type SQLTx interface {
	Exec(query string, args ...any) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryRow(query string, args ...any) *sql.Row
	Commit() error
	Rollback() error
}

type SQLUpdater interface {
	StartTx() (SQLTx, error)

	UnsetLiveAttrs(transact SQLTx, corpus string) error

	SetLiveAttrs(
		transact SQLTx,
		corpus, bibIDStruct, bibIDAttr, tagAttr string,
		tagsetName corp.SupportedTagset,
	) error

	IfMissingAddCorpusBibMetadata(
		transact SQLTx,
		corpus, bibIDStruct, bibIDAttr, tagAttr string,
		tagsetName corp.SupportedTagset,
	) error
}

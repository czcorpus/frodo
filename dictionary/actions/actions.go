// Copyright 2020 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2020 Institute of the Czech National Corpus,
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

package actions

import (
	"context"
	"frodo/corpus"
	"frodo/db/mysql"
	"frodo/general"
	"frodo/jobs"
	"frodo/liveattrs/laconf"
	"frodo/metadb"
)

type Actions struct {
	corpConf *corpus.CorporaSetup

	laConfCache *laconf.LiveAttrsBuildConfProvider

	// ctx controls cancellation
	ctx context.Context

	// jobStopChannel receives job ID based on user interaction with job HTTP API in
	// case users asks for stopping the vte process
	jobStopChannel <-chan string

	jobActions *jobs.Actions

	// laDB is a live-attributes-specific database where Frodo needs full privileges
	laDB *mysql.Adapter

	laCustomNgramDataDirPath string

	corpusMeta metadb.Provider

	corpusMetaW metadb.SQLUpdater
}

// NewActions is the default factory for Actions
func NewActions(
	ctx context.Context,
	corpConf *corpus.CorporaSetup,
	jobStopChannel <-chan string,
	jobActions *jobs.Actions,
	corpusMeta metadb.Provider,
	corpusMetaW metadb.SQLUpdater,
	laDB *mysql.Adapter,
	laCustomNgramDataDirPath string,
	laConfRegistry *laconf.LiveAttrsBuildConfProvider,
	version general.VersionInfo,
) *Actions {
	actions := &Actions{
		ctx:                      ctx,
		corpConf:                 corpConf,
		jobActions:               jobActions,
		jobStopChannel:           jobStopChannel,
		laConfCache:              laConfRegistry,
		corpusMeta:               corpusMeta,
		corpusMetaW:              corpusMetaW,
		laDB:                     laDB,
		laCustomNgramDataDirPath: laCustomNgramDataDirPath,
	}
	return actions
}

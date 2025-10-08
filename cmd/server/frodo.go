// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
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
	"context"
	"encoding/gob"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"frodo/cnf"
	"frodo/db/mysql"
	"frodo/debug"
	dictActions "frodo/dictionary/actions"
	"frodo/docs"
	"frodo/general"
	"frodo/jobs"
	"frodo/liveattrs"
	laActions "frodo/liveattrs/actions"
	"frodo/liveattrs/laconf"
	"frodo/metadb"
	"frodo/root"

	_ "frodo/translations"
)

var (
	version   string
	buildDate string
	gitCommit string
)

func init() {
	gob.Register(&liveattrs.LiveAttrsJobInfo{})
	gob.Register(&liveattrs.IdxUpdateJobInfo{})
}

// @title           FRODO - Frequency Registry Of Dictionary Objects
// @description     Frequency database of corpora metadata and word forms. It is mostly used along with CNC's other applications for fast overview data retrieval. In KonText, it's mainly the "liveattrs" function, in WaG, it works as a core word/ngram dictionary.

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost
// @BasePath  /

// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/
func main() {
	version := general.VersionInfo{
		Version:   version,
		BuildDate: buildDate,
		GitCommit: gitCommit,
	}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "FRODO - Frequency Registry of Dictionary Objects\n\nUsage:\n\t%s [options] start [config.json]\n\t%s [options] version\n",
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()
	action := flag.Arg(0)
	if action == "version" {
		fmt.Printf("frodo %s\nbuild date: %s\nlast commit: %s\n", version.Version, version.BuildDate, version.GitCommit)
		return

	} else if action != "start" {
		log.Fatal().Msgf("Unknown action %s", action)
	}
	conf := cnf.LoadConfig(flag.Arg(1))
	logging.SetupLogging(conf.Logging)
	if conf.CNCDB == nil {
		if err := conf.CorporaSetup.Load(); err != nil {
			log.Fatal().
				Err(err).
				Str("targetDirectory", conf.CorporaSetup.CorporaConfDir).
				Msg("failed to load corpora configs")
		}

	} else {
		log.Info().Msg("CNCDB is configured, corpora info will be loaded from there")
	}
	log.Info().Msg("Starting FRODO")
	cnf.ApplyDefaults(conf)

	docs.SwaggerInfo.Version = version.Version
	docs.SwaggerInfo.Host = fmt.Sprintf("%s:%d", conf.ListenAddress, conf.ListenPort)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop()
	}()

	var corpusMeta metadb.Provider
	var corpusMetaW metadb.SQLUpdater
	var corpusMetaErr error

	if conf.CNCDB != nil {
		cTableName := "corpora"
		if conf.CNCDB.OverrideCorporaTableName != "" {
			log.Warn().Msgf(
				"Overriding default corpora table name to '%s'", conf.CNCDB.OverrideCorporaTableName)
			cTableName = conf.CNCDB.OverrideCorporaTableName
		}
		pcTableName := "parallel_corpus"
		if conf.CNCDB.OverridePCTableName != "" {
			log.Warn().Msgf(
				"Overriding default parallel corpora table name to '%s'", conf.CNCDB.OverridePCTableName)
			pcTableName = conf.CNCDB.OverridePCTableName
		}

		tmp, err := metadb.NewCNCMySQLHandler(
			conf.CNCDB.Host,
			conf.CNCDB.User,
			conf.CNCDB.Passwd,
			conf.CNCDB.Name,
			cTableName,
			pcTableName,
		)
		if err != nil {
			corpusMetaErr = err

		} else {
			corpusMeta = tmp
			corpusMetaW = tmp
		}
		log.Info().Msgf("using CNC corpus info SQL database: %s@%s", conf.CNCDB.Name, conf.CNCDB.Host)

	} else {
		corpusMeta = &metadb.StaticProvider{Corpora: conf.CorporaSetup.GetAllCorpora()}
		corpusMetaW = &metadb.NoOpWriter{}
		log.Info().Msgf("using static corpora info from directory: %s", conf.CorporaSetup.CorporaConfDir)
	}

	if corpusMetaErr != nil {
		log.Fatal().Err(corpusMetaErr)
	}

	laDB, err := mysql.OpenDB(*conf.LiveAttrs.DB)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	var dbInfo string
	if conf.LiveAttrs.DB.Type == "mysql" {
		dbInfo = fmt.Sprintf("%s@%s", conf.LiveAttrs.DB.Name, conf.LiveAttrs.DB.Host)

	} else {
		log.Fatal().Msg("only mysql liveattrs backend is supported")
	}
	log.Info().Msgf("LiveAttrs SQL database(s): %s", dbInfo)

	if !conf.Logging.Level.IsDebugMode() {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(logging.GinMiddleware())
	engine.Use(uniresp.AlwaysJSONContentType())
	engine.NoMethod(uniresp.NoMethodHandler)
	engine.NoRoute(uniresp.NotFoundHandler)

	rootActions := root.Actions{Version: version, Conf: conf}

	jobStopChannel := make(chan string)
	jobActions := jobs.NewActions(conf.Jobs, conf.Language, ctx, jobStopChannel)

	laConfRegistry := laconf.NewLiveAttrsBuildConfProvider(
		conf.LiveAttrs.ConfDirPath,
		conf.LiveAttrs.DB,
	)

	liveattrsActions := laActions.NewActions(
		laActions.LAConf{
			LA:      conf.LiveAttrs,
			KonText: conf.Kontext,
			Corp:    conf.CorporaSetup,
		},
		ctx,
		jobStopChannel,
		jobActions,
		corpusMeta,
		corpusMetaW,
		laDB,
		laConfRegistry,
		version,
	)

	for _, dj := range jobActions.GetDetachedJobs() {
		if dj.IsFinished() {
			continue
		}
		switch tdj := dj.(type) {
		case *liveattrs.LiveAttrsJobInfo:
			err := liveattrsActions.RestartLiveAttrsJob(ctx, tdj)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to restart job %s. The job will be removed.", tdj.ID)
			}
			jobActions.ClearDetachedJob(tdj.ID)
		case *liveattrs.IdxUpdateJobInfo:
			err := liveattrsActions.RestartIdxUpdateJob(tdj)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to restart job %s. The job will be removed.", tdj.ID)
			}
			jobActions.ClearDetachedJob(tdj.ID)
		default:
			log.Error().Msg("unknown detached job type")
		}
	}

	engine.GET(
		"/", rootActions.RootAction)
	engine.GET(
		"/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	engine.POST(
		"/liveAttributes/:corpusId/data", liveattrsActions.Create)
	engine.DELETE(
		"/liveAttributes/:corpusId/data", liveattrsActions.Delete)
	engine.GET(
		"/liveAttributes/:corpusId/conf", liveattrsActions.ViewConf)
	engine.PUT(
		"/liveAttributes/:corpusId/conf", liveattrsActions.CreateConf)
	engine.PATCH(
		"/liveAttributes/:corpusId/conf", liveattrsActions.PatchConfig)
	engine.GET(
		"/liveAttributes/:corpusId/qsDefaults", liveattrsActions.QSDefaults)
	engine.DELETE(
		"/liveAttributes/:corpusId/confCache", liveattrsActions.FlushCache)
	engine.POST(
		"/liveAttributes/:corpusId/query", liveattrsActions.Query)
	engine.POST(
		"/liveAttributes/:corpusId/fillAttrs", liveattrsActions.FillAttrs)
	engine.POST(
		"/liveAttributes/:corpusId/selectionSubcSize",
		liveattrsActions.GetAdhocSubcSize)
	engine.POST(
		"/liveAttributes/:corpusId/attrValAutocomplete",
		liveattrsActions.AttrValAutocomplete)
	engine.POST(
		"/liveAttributes/:corpusId/getBibliography",
		liveattrsActions.GetBibliography)
	engine.POST(
		"/liveAttributes/:corpusId/findBibTitles",
		liveattrsActions.FindBibTitles)
	engine.GET(
		"/liveAttributes/:corpusId/stats", liveattrsActions.Stats)
	engine.POST(
		"/liveAttributes/:corpusId/updateIndexes",
		liveattrsActions.UpdateIndexes)
	engine.POST(
		"/liveAttributes/:corpusId/mixSubcorpus",
		liveattrsActions.MixSubcorpus)
	engine.GET(
		"/liveAttributes/:corpusId/inferredAtomStructure",
		liveattrsActions.InferredAtomStructure)
	engine.POST(
		"/liveAttributes/:corpusId/documentList",
		liveattrsActions.DocumentList)
	engine.POST(
		"/liveAttributes/:corpusId/numMatchingDocuments",
		liveattrsActions.NumMatchingDocuments)

	dictActionsHandler := dictActions.NewActions(
		ctx,
		conf.CorporaSetup,
		jobStopChannel,
		jobActions,
		corpusMeta,
		corpusMetaW,
		laDB,
		conf.LiveAttrs.CustomNgramTablesDataDir,
		laConfRegistry,
		version,
	)

	engine.POST(
		"/dictionary/:corpusId/ngrams",
		dictActionsHandler.GenerateNgrams)
	engine.POST(
		"/dictionary/:corpusId/querySuggestions",
		dictActionsHandler.CreateQuerySuggestions)
	engine.GET(
		"/dictionary/:corpusId/querySuggestions/:term",
		dictActionsHandler.GetQuerySuggestions)
	engine.GET(
		"/dictionary/:corpusId/search/:term",
		dictActionsHandler.GetQuerySuggestions)
	engine.GET(
		"/dictionary/:corpusId/similarARFWords/:term",
		dictActionsHandler.SimilarARFWords)

	engine.GET(
		"/jobs", jobActions.JobList)
	engine.GET(
		"/jobs/utilization", jobActions.Utilization)
	engine.GET(
		"/jobs/:jobId", jobActions.JobInfo)
	engine.DELETE(
		"/jobs/:jobId", jobActions.Delete)
	engine.GET(
		"/jobs/:jobId/clearIfFinished", jobActions.ClearIfFinished)
	engine.GET(
		"/jobs/:jobId/emailNotification", jobActions.GetNotifications)
	engine.GET(
		"/jobs/:jobId/emailNotification/:address",
		jobActions.CheckNotification)
	engine.PUT(
		"/jobs/:jobId/emailNotification/:address",
		jobActions.AddNotification)
	engine.DELETE(
		"/jobs/:jobId/emailNotification/:address",
		jobActions.RemoveNotification)

	if conf.Logging.Level.IsDebugMode() {
		debugActions := debug.NewActions(jobActions)
		engine.POST("/debug/createJob", debugActions.CreateDummyJob)
		engine.POST("/debug/finishJob/:jobId", debugActions.FinishDummyJob)
	}

	log.Info().Msgf("starting to listen at %s:%d", conf.ListenAddress, conf.ListenPort)
	srv := &http.Server{
		Handler:      engine,
		Addr:         fmt.Sprintf("%s:%d", conf.ListenAddress, conf.ListenPort),
		WriteTimeout: time.Duration(conf.ServerWriteTimeoutSecs) * time.Second,
		ReadTimeout:  time.Duration(conf.ServerReadTimeoutSecs) * time.Second,
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Error().Err(err).Send()
		}
	}()

	<-ctx.Done()
	log.Info().Err(err).Msg("Shutdown request error")

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctxShutDown); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}
}

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
	"context"
	"flag"
	"fmt"
	"frodo/cnf"
	"frodo/db/mysql"
	"frodo/ujc/lex"
	"frodo/ujc/lex/assc"
	"frodo/ujc/lex/ijp"
	"frodo/ujc/lex/ssc"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
)

type cmdAction string

const (
	cmdActionImport cmdAction = "import"
	cmdActionUpdate cmdAction = "update"
)

// Import subcommand flags
type importArgs struct {
	configPath  string
	inputFile   string
	serviceType string
	dryRun      bool
}

// Update subcommand flags
type updateArgs struct {
	targetID    string
	serviceType string
	force       bool
}

func runIjpImport(args importArgs) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	conf := cnf.LoadConfig(args.configPath)
	db, err := mysql.OpenDB(*conf.LiveAttrs.DB)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to import data")
	}

	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to import data")
	}

	log.Info().Msg("Pruning old IJP data")
	if err := lex.PruneData(ctx, tx, lex.SourceIJP); err != nil {
		log.Fatal().Err(err).Msg("failed to import data")
	}

	log.Info().Msg("Reading IJP data from TSV file")
	data, err := ijp.ReadTSV(ctx, args.inputFile)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to import data")
	}

	for chunk := range data {
		if chunk.Error != nil {
			if chunk.NotFatal {
				log.Warn().Err(chunk.Error).Msg("skipping line")
				continue
			}
			log.Fatal().Err(chunk.Error).Msg("failed to import data")
		}
		if err := ijp.InsertDictChunk(ctx, tx, chunk.Items); err != nil {
			log.Fatal().Err(err).Msg("failed to import data")
		}
	}
	tx.Commit()

}

func runIjpUpdate(args updateArgs) error {
	fmt.Printf("Running IJP update: targetID=%s, force=%v\n", args.targetID, args.force)
	// TODO: implement update logic
	return nil
}

func runAsscImport(args importArgs) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	conf := cnf.LoadConfig(args.configPath)
	db, err := mysql.OpenDB(*conf.LiveAttrs.DB)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to import data")
	}

	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to import data")
	}

	log.Info().Msg("Pruning old ASSC data")
	if err := lex.PruneData(ctx, tx, lex.SourceASSC); err != nil {
		log.Fatal().Err(err).Msg("failed to import data")
	}

	log.Info().Msg("Reading ASSC data from TSV file")
	data, err := assc.ReadTSV(ctx, args.inputFile)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to import data")
	}

	for chunk := range data {
		if chunk.Error != nil {
			if chunk.NotFatal {
				log.Warn().Err(chunk.Error).Msg("skipping line")
				continue
			}
			log.Fatal().Err(chunk.Error).Msg("failed to import data")
		}
		if err := assc.InsertDictChunk(ctx, tx, chunk.Items); err != nil {
			log.Fatal().Err(err).Msg("failed to import data")
		}
	}
	tx.Commit()

}

func runAsscUpdate(args updateArgs) error {
	fmt.Printf("Running ASSC update: targetID=%s, force=%v\n", args.targetID, args.force)
	// TODO: implement update logic
	return nil
}

func runSscImport(args importArgs) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	conf := cnf.LoadConfig(args.configPath)
	db, err := mysql.OpenDB(*conf.LiveAttrs.DB)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to import data")
	}

	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to import data")
	}

	log.Info().Msg("Pruning old SSC data")
	if err := lex.PruneData(ctx, tx, lex.SourceSSC); err != nil {
		log.Fatal().Err(err).Msg("failed to import data")
	}

	log.Info().Msg("Reading SSC data from TSV file")
	data, err := ssc.ReadTSV(ctx, args.inputFile)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to import data")
	}

	for chunk := range data {
		if chunk.Error != nil {
			if chunk.NotFatal {
				log.Warn().Err(chunk.Error).Msg("skipping line")
				continue
			}
			log.Fatal().Err(chunk.Error).Msg("failed to import data")
		}
		if err := ssc.InsertDictChunk(ctx, tx, chunk.Items); err != nil {
			log.Fatal().Err(err).Msg("failed to import data")
		}
	}
	tx.Commit()

}

func runSscUpdate(args updateArgs) error {
	fmt.Printf("Running SSC update: targetID=%s, force=%v\n", args.targetID, args.force)
	// TODO: implement update logic
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Commands: %s, %s\n", cmdActionImport, cmdActionUpdate)
		os.Exit(1)
	}

	// Create subcommand flag sets
	importCmd := flag.NewFlagSet(string(cmdActionImport), flag.ExitOnError)
	updateCmd := flag.NewFlagSet(string(cmdActionUpdate), flag.ExitOnError)

	// Define flags for import subcommand
	var importOpts importArgs
	importCmd.StringVar(&importOpts.serviceType, "service", "", "type of service data to import (required)")
	importCmd.BoolVar(&importOpts.dryRun, "dry-run", false, "perform a dry run without making changes")

	// Define flags for update subcommand
	var updateOpts updateArgs
	updateCmd.StringVar(&updateOpts.serviceType, "service", "", "type of service data to import (required)")
	updateCmd.StringVar(&updateOpts.targetID, "id", "", "target ID to update")
	updateCmd.BoolVar(&updateOpts.force, "force", false, "force update even if conflicts exist")

	// Parse based on subcommand
	switch cmdAction(os.Args[1]) {
	case cmdActionImport:
		if err := importCmd.Parse(os.Args[2:]); err != nil {
			os.Exit(1)
		}
		importOpts.configPath = importCmd.Arg(0)
		importOpts.inputFile = importCmd.Arg(1)
		if importOpts.serviceType == "" {
			fmt.Fprintf(os.Stderr, "missing required flag -service\n")
			importCmd.Usage()
			os.Exit(1)
		}

		switch importOpts.serviceType {
		case "ijp":
			runIjpImport(importOpts)
		case "assc":
			runAsscImport(importOpts)
		case "ssc":
			runSscImport(importOpts)
		default:
			fmt.Fprintf(os.Stderr, "Unknown service type: %s\n", importOpts.serviceType)
			os.Exit(1)
		}

	case cmdActionUpdate:
		err := updateCmd.Parse(os.Args[2:])
		if err != nil {
			os.Exit(1)
		}

		switch updateOpts.serviceType {
		case "ijp":
			err = runIjpUpdate(updateOpts)
		case "assc":
			err = runAsscUpdate(updateOpts)
		case "ssc":
			err = runSscUpdate(updateOpts)
		default:
			fmt.Fprintf(os.Stderr, "Unknown service type: %s\n", updateOpts.serviceType)
			os.Exit(1)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "update failed: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		fmt.Fprintf(os.Stderr, "Commands: %s, %s\n", cmdActionImport, cmdActionUpdate)
		os.Exit(1)
	}
}

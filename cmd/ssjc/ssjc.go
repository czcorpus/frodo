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
	"frodo/ujc/ssjc"
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
	configPath string
	inputFile  string
	dryRun     bool
}

// Update subcommand flags
type updateArgs struct {
	targetID string
	force    bool
}

type ssjcFileRow struct {
	ID           string
	ParentID     string
	HeadWord     string
	HeadWordType string
	PoS          string
	Gender       string
	Aspect       string
}

func runImport(args importArgs) {
	log.Info().Str("file", args.inputFile).Msgf("Running import")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	conf := cnf.LoadConfig(args.configPath)
	db, err := mysql.OpenDB(*conf.LiveAttrs.DB)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to import data")
	}

	tx, err := ssjc.CreateTables(ctx, db.DB())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to import data")
	}

	data, err := ReadTSV(ctx, args.inputFile, recTypeParent)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to import data")
	}

	for chunk := range data {
		if chunk.Error != nil {
			log.Fatal().Err(chunk.Error).Msg("failed to import data")
		}
		if err := ssjc.InsertDictChunk(ctx, tx, chunk.Items); err != nil {
			log.Fatal().Err(err).Msg("failed to import data")
		}
	}

	data, err = ReadTSV(ctx, args.inputFile, recTypeChild)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to import data")
	}
	for chunk := range data {
		if chunk.Error != nil {
			log.Fatal().Err(chunk.Error).Msg("failed to import data")
		}
		if err := ssjc.InsertDictChunk(ctx, tx, chunk.Items); err != nil {
			log.Fatal().Err(err).Msg("failed to import data")
		}
	}
	tx.Commit()

}

func runUpdate(args updateArgs) error {
	fmt.Printf("Running update: targetID=%s, force=%v\n", args.targetID, args.force)
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
	importCmd.BoolVar(&importOpts.dryRun, "dry-run", false, "perform a dry run without making changes")

	// Define flags for update subcommand
	var updateOpts updateArgs
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
		runImport(importOpts)

	case cmdActionUpdate:
		if err := updateCmd.Parse(os.Args[2:]); err != nil {
			os.Exit(1)
		}
		if err := runUpdate(updateOpts); err != nil {
			fmt.Fprintf(os.Stderr, "update failed: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		fmt.Fprintf(os.Stderr, "Commands: %s, %s\n", cmdActionImport, cmdActionUpdate)
		os.Exit(1)
	}
}

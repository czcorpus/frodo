package keywords

import (
	"context"
	"fmt"
	"frodo/jobs"

	"github.com/czcorpus/vert-tagextract/v3/proc"
	"github.com/czcorpus/vert-tagextract/v3/ptcount"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/tomachalek/vertigo/v6"
)

func generateKeywordsSync(
	ctx context.Context,
	args KeywordsBuildArgs,
	jobStatus chan<- keywordsBuildStatus,
) {
	status := keywordsBuildStatus{}
	wordDict := ptcount.NewWordDict()
	wordDict.Add("")

	parserConf := &vertigo.ParserConf{
		StructAttrAccumulator: "nil",
		Encoding:              "utf-8",
		LogProgressEachNth:    1000000, // TODO
	}

	// count tokens in 1 (ref)

	tc1 := &TokenCounter{}
	vertScanner1, err := proc.NewMultiFileScanner(args.ReferenceVerticals...)
	if err != nil {
		status.Error = fmt.Errorf("failed to run TTExtractor: %w", err)
		jobStatus <- status
		return
	}
	if err := vertigo.ParseVerticalFromScanner(ctx, vertScanner1, parserConf, tc1); err != nil {
		status.Error = err
		jobStatus <- status
	}
	vertScanner1.Close()

	// find keywords in 1 (ref)

	vertScanner1, err = proc.NewMultiFileScanner(args.ReferenceVerticals...)
	if err != nil {
		status.Error = fmt.Errorf("failed to run TTExtractor: %w", err)
		jobStatus <- status
		return
	}
	processor1 := NewNgramExtractor(ctx, args, wordDict, tc1.NumTokens, jobStatus)
	if err := vertigo.ParseVerticalFromScanner(ctx, vertScanner1, parserConf, processor1); err != nil {
		status.Error = err
		jobStatus <- status
	}
	vertScanner1.Close()
	processor1.Preview()

	// count tokens in 2 (foc)

	tc2 := &TokenCounter{}
	vertScanner2, err := proc.NewMultiFileScanner(args.FocusVerticals...)
	if err != nil {
		status.Error = fmt.Errorf("failed to run TTExtractor: %w", err)
		jobStatus <- status
		return
	}
	if err := vertigo.ParseVerticalFromScanner(ctx, vertScanner2, parserConf, tc2); err != nil {
		status.Error = err
		jobStatus <- status
	}
	vertScanner2.Close()

	vertScanner2, err = proc.NewMultiFileScanner(args.FocusVerticals...)
	if err != nil {
		status.Error = fmt.Errorf("failed to run TTExtractor: %w", err)
		jobStatus <- status
		return
	}
	processor2 := NewNgramExtractor(ctx, args, wordDict, tc2.NumTokens, jobStatus)
	if err := vertigo.ParseVerticalFromScanner(ctx, vertScanner2, parserConf, processor2); err != nil {
		status.Error = err
		jobStatus <- status
	}
	vertScanner2.Close()
	processor2.Preview()

	ans := FindKeywords(processor1.colCounts2, processor2.colCounts2, wordDict, 2)
	for i, v := range ans {
		fmt.Printf("[%d]: %s (%.2f)\n", i+1, v.Lemma, v.EffectSize)
	}

	ans = FindKeywords(processor1.colCounts1, processor2.colCounts1, wordDict, 2)
	for i, v := range ans {
		fmt.Printf("[%d]: %s (%.2f)\n", i+1, v.Lemma, v.EffectSize)
	}
}

func RunJob(datasetID string, args KeywordsBuildArgs, jobActions *jobs.Actions) (KeywordsBuildJob, error) {
	jobID, err := uuid.NewUUID()
	if err != nil {
		return KeywordsBuildJob{}, err
	}
	jobStatus := KeywordsBuildJob{
		ID:       jobID.String(),
		Type:     "ngram-generating",
		CorpusID: datasetID,
		Start:    jobs.CurrentDatetime(),
		Update:   jobs.CurrentDatetime(),
		Finished: false,
		Args:     args,
	}
	fn := func(updateJobChan chan<- jobs.GeneralJobInfo) {
		statusChan := make(chan keywordsBuildStatus)
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		go func(runStatus KeywordsBuildJob) {
			defer close(updateJobChan)
			for statUpd := range statusChan {
				if statUpd.ClientWarn != "" {
					log.Warn().
						Str("corpusId", statUpd.CorpusID).
						Int("totalLines", statUpd.TotalLines).
						Int("numProcLines", statUpd.NumProcLines).
						Msg(statUpd.ClientWarn)

				} else if statUpd.Error != nil {
					log.Error().
						Str("corpusId", statUpd.CorpusID).
						Int("totalLines", statUpd.TotalLines).
						Int("numProcLines", statUpd.NumProcLines).
						Err(statUpd.Error).
						Msg("failed to process ngram job")

				} else {
					log.Info().
						Str("corpusId", statUpd.CorpusID).
						Int("totalLines", statUpd.TotalLines).
						Int("numProcLines", statUpd.NumProcLines).
						Err(statUpd.Error).
						Msg("reporting job status")
				}

				runStatus.Result = statUpd
				runStatus.Error = statUpd.Error
				runStatus.Update = jobs.CurrentDatetime()
				updateJobChan <- runStatus
				if runStatus.Error != nil {
					runStatus.Finished = true
					cancel()
				}
			}
			runStatus.Update = jobs.CurrentDatetime()
			runStatus.Finished = true
			fmt.Println("updateJobChan <- runStatus")
			updateJobChan <- runStatus
			fmt.Println("  ... done")
		}(jobStatus)
		generateKeywordsSync(ctx, args, statusChan)
		close(statusChan)
	}
	jobActions.EnqueueJob(&fn, &jobStatus)
	return jobStatus, nil
}

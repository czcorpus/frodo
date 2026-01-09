package keywords

import (
	"context"
	"fmt"
	"frodo/db/mysql"
	"frodo/jobs"
	"strings"

	"github.com/czcorpus/vert-tagextract/v3/proc"
	"github.com/czcorpus/vert-tagextract/v3/ptcount"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/tomachalek/vertigo/v6"
)

func reduceNestedValues(ng1, ng2, ng3 []Keyword) ([]Keyword, []Keyword) {
	ng1New := make([]Keyword, 0, len(ng1))
	for _, v1 := range ng1 {
		var isInV2 bool
		for _, v2 := range ng2 {
			if strings.Contains(v2.Lemma, v1.Lemma) {
				isInV2 = true
				break
			}
		}
		if !isInV2 {
			ng1New = append(ng1New, v1)
		}
	}
	ng2New := make([]Keyword, 0, len(ng2))
	for _, v2 := range ng2 {
		var isInV3 bool
		for _, v3 := range ng3 {
			if strings.Contains(v3.Lemma, v2.Lemma) {
				isInV3 = true
				break
			}
		}
		if !isInV3 {
			ng2New = append(ng2New, v2)
		}
	}
	return ng1New, ng2New
}

func generateKeywordsSync(
	ctx context.Context,
	db *mysql.Adapter,
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
	defer vertScanner1.Close()
	if err := vertigo.ParseVerticalFromScanner(ctx, vertScanner1, parserConf, tc1); err != nil {
		status.Error = err
		jobStatus <- status
	}

	// find keywords in 1 (ref)

	vertScanner1b, err := proc.NewMultiFileScanner(args.ReferenceVerticals...)
	if err != nil {
		status.Error = fmt.Errorf("failed to run TTExtractor: %w", err)
		jobStatus <- status
		return
	}
	defer vertScanner1b.Close()
	processor1 := NewNgramExtractor(ctx, args, wordDict, tc1.NumTokens, jobStatus)
	if err := vertigo.ParseVerticalFromScanner(ctx, vertScanner1b, parserConf, processor1); err != nil {
		status.Error = err
		jobStatus <- status
	}
	processor1.Preview()

	// count tokens in 2 (foc)

	tc2 := &TokenCounter{}
	vertScanner2, err := proc.NewMultiFileScanner(args.FocusVerticals...)
	if err != nil {
		status.Error = fmt.Errorf("failed to run TTExtractor: %w", err)
		jobStatus <- status
		return
	}
	defer vertScanner2.Close()
	if err := vertigo.ParseVerticalFromScanner(ctx, vertScanner2, parserConf, tc2); err != nil {
		status.Error = err
		jobStatus <- status
		return
	}

	vertScanner2b, err := proc.NewMultiFileScanner(args.FocusVerticals...)
	if err != nil {
		status.Error = fmt.Errorf("failed to run TTExtractor: %w", err)
		jobStatus <- status
		return
	}
	defer vertScanner2b.Close()
	processor2 := NewNgramExtractor(ctx, args, wordDict, tc2.NumTokens, jobStatus)
	if err := vertigo.ParseVerticalFromScanner(ctx, vertScanner2b, parserConf, processor2); err != nil {
		status.Error = err
		jobStatus <- status
		return
	}

	processor2.Preview()

	ans1 := FindKeywords(processor1.colCounts1, processor2.colCounts1, wordDict, 2)
	ans2 := FindKeywords(processor1.colCounts2, processor2.colCounts2, wordDict, 2)
	ans3 := FindKeywords(processor1.colCounts3, processor2.colCounts3, wordDict, 2)
	ans1, ans2 = reduceNestedValues(ans1, ans2, ans3)
	allAns := append(ans1, ans2...)
	allAns = append(allAns, ans3...)

	err = StoreKeywords(ctx, db.DB(), args, allAns)
	if err != nil {
		status.Error = err
		jobStatus <- status
		return
	}
}

func RunJob(db *mysql.Adapter, datasetID string, args KeywordsBuildArgs, jobActions *jobs.Actions) (KeywordsBuildJob, error) {
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
		generateKeywordsSync(ctx, db, args, statusChan)
		close(statusChan)
	}
	jobActions.EnqueueJob(&fn, &jobStatus)
	return jobStatus, nil
}

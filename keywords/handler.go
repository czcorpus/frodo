package keywords

import (
	"context"
	"errors"
	"frodo/corpus"
	"frodo/db/mysql"
	"frodo/jobs"
	"net/http"
	"path/filepath"
	"time"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// GetWeekAndReferenceVerticals returns two lists of date strings:
// 1. Target period: focusDays days ending at `now - daysBack - 1` (as the current day data are available the next day)
// 2. Reference period: 30 days before the start of the target period
func GetWeekAndReferenceVerticals(now time.Time, daysBack, focusDays int, pathPrefix string) (targetWeek []string, reference []string) {
	// Calculate the end date of the target period
	targetEnd := now.AddDate(0, 0, -daysBack-1)

	// Target period: focusDays days ending at targetEnd (inclusive)
	targetStart := targetEnd.AddDate(0, 0, -(focusDays - 1))

	targetWeek = make([]string, 0, focusDays)
	for d := targetStart; !d.After(targetEnd); d = d.AddDate(0, 0, 1) {
		p := filepath.Join(pathPrefix, d.Format("2006-01-02")+".vrt")
		targetWeek = append(targetWeek, p)
	}

	// Reference period: 30 days before target period
	referenceEnd := targetStart.AddDate(0, 0, -1)     // day before target starts
	referenceStart := referenceEnd.AddDate(0, 0, -29) // 29 days back = 30 days total

	reference = make([]string, 0, 30)
	for d := referenceStart; !d.After(referenceEnd); d = d.AddDate(0, 0, 1) {
		p := filepath.Join(pathPrefix, d.Format("2006-01-02")+".vrt")
		reference = append(reference, p)
	}
	return
}

func filterNonExistingFiles(flist []string) []string {
	ans := make([]string, 0, len(flist))
	for _, v := range flist {
		tst, err := fs.IsFile(v)
		if err != nil {
			log.Error().Err(err).Str("path", v).Msg("path does not refer to a vertical file, skipping")
			tst = false
		}
		if tst {
			ans = append(ans, v)
		}
	}
	return ans
}

type procArgs struct {
}

type ActionHandler struct {
	ctx context.Context

	jobActions *jobs.Actions

	jobCancel map[string]context.CancelFunc

	laDB *mysql.Adapter

	datasets corpus.MonitoringDatasets
}

func (handler *ActionHandler) ProcessKWOFWeek(ctx *gin.Context) {
	dataset := handler.datasets.GetByID(ctx.Param("datasetId"))
	if dataset.IsZero() {
		uniresp.RespondWithErrorJSON(ctx, errors.New("unknown dataset"), http.StatusNotFound)
		return
	}

	daysBack, ok := unireq.GetURLIntArgOrFail(ctx, "daysBack", 0)
	if !ok {
		return
	}

	focusDays, ok := unireq.GetURLIntArgOrFail(ctx, "focusDays", 7)
	if !ok {
		return
	}

	now := time.Now() // TODO timezone
	currWeek, refDays := GetWeekAndReferenceVerticals(now, daysBack, focusDays, dataset.VerticalsDir)
	currWeek = filterNonExistingFiles(currWeek)
	refDays = filterNonExistingFiles(refDays)
	args := KeywordsBuildArgs{
		ReferenceVerticals: refDays,
		FocusVerticals:     currWeek,
		WordColIdx:         dataset.WordColIdx,
		LemmaColIdx:        dataset.LemmaColIdx,
		TagColIdx:          dataset.TagColIdx,
		NgramSize:          dataset.NgramSize,
		SentenceStruct:     dataset.SentenceStruct,
		Metadata: KeywordsMetadata{
			DatasetID:   dataset.Ident,
			FocusDays:   focusDays,
			LastDayDate: now.Format("2006-01-02"),
		},
	}

	job, err := RunJob(handler.laDB, dataset.Ident, args, handler.jobActions)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, job)

}

func (handler *ActionHandler) Process(ctx *gin.Context) {
	datasetID := ctx.Param("datasetId")
	var args KeywordsBuildArgs
	if err := ctx.BindJSON(&args); err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadRequest)
		return
	}

	job, err := RunJob(handler.laDB, datasetID, args, handler.jobActions)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, job)

}

func (handler *ActionHandler) GetKWOFWeek(ctx *gin.Context) {
	datasetID := ctx.Param("datasetId")

	kws, err := LoadKeywords(ctx, handler.laDB.DB(), datasetID)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError) // TODO
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, kws)
}

func NewActionHandler(laDB *mysql.Adapter, datasets corpus.MonitoringDatasets, jobActions *jobs.Actions) *ActionHandler {
	return &ActionHandler{
		jobActions: jobActions,
		datasets:   datasets,
		laDB:       laDB,
	}
}

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
// 1. Days of the current week (Monday through today, or full week if after Sunday)
// 2. 30 days before the start of current week
func GetWeekAndReferenceVerticals(now time.Time, weeksBack int, pathPrefix string) (targetWeek []string, reference []string) {
	// Find the Monday of the current week
	weekday := now.Weekday()

	// Convert Sunday (0) to 7 for easier calculation
	daysFromMonday := int(weekday)
	if weekday == time.Sunday {
		daysFromMonday = 6
	} else {
		daysFromMonday = daysFromMonday - 1
	}

	currentMonday := now.AddDate(0, 0, -daysFromMonday)

	// Calculate the target week's Monday
	targetMonday := currentMonday.AddDate(0, 0, -7*weeksBack)

	// Generate target week dates
	targetWeek = make([]string, 0)

	if weeksBack == 0 {
		// Current week: Monday through today
		for d := targetMonday; !d.After(now); d = d.AddDate(0, 0, 1) {
			p := filepath.Join(pathPrefix, d.Format("2006-01-02")+".vrt")
			targetWeek = append(targetWeek, p)
		}
	} else {
		// Past weeks: full week (Monday through Sunday)
		sunday := targetMonday.AddDate(0, 0, 6)
		for d := targetMonday; !d.After(sunday); d = d.AddDate(0, 0, 1) {
			p := filepath.Join(pathPrefix, d.Format("2006-01-02")+".vrt")
			targetWeek = append(targetWeek, p)
		}
	}

	// Generate reference period: 30 days before target Monday
	referenceStart := targetMonday.AddDate(0, 0, -30)
	referenceEnd := targetMonday.AddDate(0, 0, -1) // day before Monday

	reference = make([]string, 0)
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

	weeksBack, ok := unireq.GetURLIntArgOrFail(ctx, "weeksBack", 0)
	if !ok {
		return
	}

	now := time.Now() // TODO timezone
	currWeek, refDays := GetWeekAndReferenceVerticals(now, weeksBack, dataset.VerticalsDir)
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
			FocusDays:   7,
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

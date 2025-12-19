package keywords

import (
	"frodo/jobs"
	"time"
)

type KeywordsBuildArgs struct {
	ReferenceVerticals []string `json:"referenceVerticals"`
	FocusVerticals     []string `json:"focusVerticals"`
	WordColIdx         int      `json:"wordColIdx"`
	LemmaColIdx        int      `json:"lemmaColIdx"`
	TagColIdx          int      `json:"tagColIdx"`
	NgramSize          int      `json:"ngramSize"`
	SentenceStruct     string   `json:"sentenceStruct"`
}

type keywordsBuildStatus struct {
	Datetime     time.Time
	CorpusID     string
	TablesReady  bool
	TotalLines   int
	NumProcLines int
	NumStopWords int
	Error        error
	ClientWarn   string
}

type KeywordsBuildJob struct {
	ID          string              `json:"id"`
	Type        string              `json:"type"`
	CorpusID    string              `json:"corpusId"`
	Start       jobs.JSONTime       `json:"start"`
	Update      jobs.JSONTime       `json:"update"`
	Finished    bool                `json:"finished"`
	Error       error               `json:"error,omitempty"`
	NumRestarts int                 `json:"numRestarts"`
	Args        KeywordsBuildArgs   `json:"args"`
	Result      keywordsBuildStatus `json:"result"`
}

func (j KeywordsBuildJob) GetID() string {
	return j.ID
}

func (j KeywordsBuildJob) GetType() string {
	return j.Type
}

func (j KeywordsBuildJob) GetStartDT() jobs.JSONTime {
	return j.Start
}

func (j KeywordsBuildJob) GetNumRestarts() int {
	return j.NumRestarts
}

func (j KeywordsBuildJob) GetCorpus() string {
	return j.CorpusID
}

func (j KeywordsBuildJob) GetDatasetID() string {
	return j.CorpusID
}

func (j KeywordsBuildJob) AsFinished() jobs.GeneralJobInfo {
	j.Update = jobs.CurrentDatetime()
	j.Finished = true
	return j
}

func (j KeywordsBuildJob) IsFinished() bool {
	return j.Finished
}

func (j KeywordsBuildJob) FullInfo() any {
	return struct {
		ID          string              `json:"id"`
		Type        string              `json:"type"`
		CorpusID    string              `json:"corpusId"`
		Start       jobs.JSONTime       `json:"start"`
		Update      jobs.JSONTime       `json:"update"`
		Finished    bool                `json:"finished"`
		Error       string              `json:"error,omitempty"`
		OK          bool                `json:"ok"`
		NumRestarts int                 `json:"numRestarts"`
		Args        KeywordsBuildArgs   `json:"args"`
		Result      keywordsBuildStatus `json:"result"`
	}{
		ID:          j.ID,
		Type:        j.Type,
		CorpusID:    j.CorpusID,
		Start:       j.Start,
		Update:      j.Update,
		Finished:    j.Finished,
		Error:       jobs.ErrorToString(j.Error),
		OK:          j.Error == nil,
		NumRestarts: j.NumRestarts,
		Args:        j.Args,
		Result:      j.Result,
	}
}

func (j KeywordsBuildJob) CompactVersion() jobs.JobInfoCompact {
	item := jobs.JobInfoCompact{
		ID:       j.ID,
		Type:     j.Type,
		CorpusID: j.CorpusID,
		Start:    j.Start,
		Update:   j.Update,
		Finished: j.Finished,
		OK:       true,
	}
	item.OK = j.Error == nil
	return item
}

func (j KeywordsBuildJob) GetError() error {
	return j.Error
}

func (j KeywordsBuildJob) WithError(err error) jobs.GeneralJobInfo {
	return &KeywordsBuildJob{
		ID:          j.ID,
		Type:        j.Type,
		CorpusID:    j.CorpusID,
		Start:       j.Start,
		Update:      jobs.JSONTime(time.Now()),
		Finished:    true,
		Error:       err,
		Result:      j.Result,
		NumRestarts: j.NumRestarts,
	}
}

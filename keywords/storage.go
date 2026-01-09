package keywords

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

/*

CREATE TABLE keyword_of_the_week (
	id INT NOT NULL auto_increment,
	dataset_id VARCHAR(100) NOT NULL,
	value VARCHAR(200) NOT NULL,
	score FLOAT NOT NULL,
	ngram_size INT NOT NULL,
	num_focus_days INT NOT NULL,
	last_day DATE NOT NULL,
	ref_verticals TEXT NOT NULL,
	focus_verticals TEXT NOT NULL,
    PRIMARY KEY (id)
);


*/

func StoreKeywords(ctx context.Context, db *sql.DB, args KeywordsBuildArgs, kws []Keyword) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to store keywords: %w", err)
	}
	for _, kw := range kws {
		_, err = tx.ExecContext(
			ctx,
			"INSERT INTO keyword_of_the_week "+
				"(dataset_id, value, score, ngram_size, num_focus_days, last_day, ref_verticals, focus_verticals) "+
				"VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			args.Metadata.DatasetID,
			kw.Lemma,
			kw.EffectSize,
			kw.NgramSize,
			args.Metadata.FocusDays,
			args.Metadata.LastDayDate,
			strings.Join(args.ReferenceVerticals, ", "),
			strings.Join(args.FocusVerticals, ", "),
		)
		if err != nil {
			if err := tx.Rollback(); err != nil {
				log.Error().Err(err).Msg("StoreKeywords - failed to rollback a transaction")
			}
			return fmt.Errorf("failed to store keywords: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to store keywords: %w", err)
	}
	return nil
}

func LoadKeywords(ctx context.Context, db *sql.DB, datasetID string) ([]Keyword, error) {
	now := time.Now()
	rows, err := db.QueryContext(
		ctx,
		"SELECT value, score, ngram_size "+
			"FROM keyword_of_the_week "+
			"WHERE last_day = ( "+
			"  SELECT last_day "+
			"  FROM keyword_of_the_week "+
			"  WHERE last_day <= ? "+
			"  ORDER BY last_day DESC "+
			"  LIMIT 1 "+
			") "+
			"ORDER BY score DESC LIMIT 15",
		now.Format("2006-01-02"),
	)
	if err != nil {
		return []Keyword{}, fmt.Errorf("failed to get keywords: %w", err)
	}
	defer rows.Close()
	ans := make([]Keyword, 0, 30)
	for rows.Next() {
		var kw Keyword
		if err := rows.Scan(&kw.Lemma, &kw.EffectSize, &kw.NgramSize); err != nil {
			return []Keyword{}, fmt.Errorf("failed to attach kw values from db: %w", err)
		}
		ans = append(ans, kw)
	}
	return ans, nil
}

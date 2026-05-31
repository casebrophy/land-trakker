package sourcedb

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/source"
	"github.com/cbrophy/land_trakker/storage/db"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, queries: db.New(pool)}
}

func (s *Store) CreateSource(ctx context.Context, src source.Source) (source.Source, error) {
	row, err := s.queries.CreateSource(ctx, db.CreateSourceParams{
		ID:                             src.ID,
		DisplayName:                    src.DisplayName,
		BaseUrl:                        src.BaseURL,
		Enabled:                        src.Enabled,
		RateLimitMs:                    int32(src.RateLimitMS),
		Concurrency:                    int32(src.Concurrency),
		UserAgent:                      src.UserAgent,
		RespectRobots:                  src.RespectRobots,
		AbsenceDaysBeforeStale:         int32(src.AbsenceDaysBeforeStale),
		AbsenceDaysBeforeInactive:      int32(src.AbsenceDaysBeforeInactive),
		ConsecutiveMissedRunsThreshold: int32(src.ConsecutiveMissedRunsThreshold),
		MinResultRatioForInactivation:  float64ToNumeric(src.MinResultRatioForInactivation),
		LastRunAt:                      timePtrToTZ(src.LastRunAt),
		NextRunAt:                      timePtrToTZ(src.NextRunAt),
		Notes:                          strPtrToText(src.Notes),
	})
	if err != nil {
		return source.Source{}, fmt.Errorf("sourcedb.CreateSource: %w", err)
	}
	return rowToSource(row), nil
}

func (s *Store) UpdateSource(ctx context.Context, src source.Source) error {
	err := s.queries.UpdateSource(ctx, db.UpdateSourceParams{
		ID:                             src.ID,
		DisplayName:                    src.DisplayName,
		BaseUrl:                        src.BaseURL,
		Enabled:                        src.Enabled,
		RateLimitMs:                    int32(src.RateLimitMS),
		Concurrency:                    int32(src.Concurrency),
		UserAgent:                      src.UserAgent,
		RespectRobots:                  src.RespectRobots,
		AbsenceDaysBeforeStale:         int32(src.AbsenceDaysBeforeStale),
		AbsenceDaysBeforeInactive:      int32(src.AbsenceDaysBeforeInactive),
		ConsecutiveMissedRunsThreshold: int32(src.ConsecutiveMissedRunsThreshold),
		MinResultRatioForInactivation:  float64ToNumeric(src.MinResultRatioForInactivation),
		LastRunAt:                      timePtrToTZ(src.LastRunAt),
		NextRunAt:                      timePtrToTZ(src.NextRunAt),
		Notes:                          strPtrToText(src.Notes),
	})
	if err != nil {
		return fmt.Errorf("sourcedb.UpdateSource: %w", err)
	}
	return nil
}

func (s *Store) QuerySourceByID(ctx context.Context, id string) (source.Source, error) {
	row, err := s.queries.GetSourceByID(ctx, id)
	if err != nil {
		return source.Source{}, fmt.Errorf("sourcedb.QuerySourceByID: %w", err)
	}
	return rowToSource(row), nil
}

func (s *Store) QuerySources(ctx context.Context) ([]source.Source, error) {
	rows, err := s.queries.ListSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("sourcedb.QuerySources: %w", err)
	}
	out := make([]source.Source, len(rows))
	for i, r := range rows {
		out[i] = rowToSource(r)
	}
	return out, nil
}

func (s *Store) CreateRun(ctx context.Context, run source.ScrapeRun) (source.ScrapeRun, error) {
	row, err := s.queries.CreateScrapeRun(ctx, db.CreateScrapeRunParams{
		SourceID:        run.SourceID,
		StartedAt:       timeToTZ(run.StartedAt),
		FinishedAt:      timePtrToTZ(run.FinishedAt),
		Status:          string(run.Status),
		DiscoveredCount: intPtrToInt4(run.DiscoveredCount),
		FetchedCount:    intPtrToInt4(run.FetchedCount),
		ParsedCount:     intPtrToInt4(run.ParsedCount),
		ErrorCount:      intPtrToInt4(run.ErrorCount),
		ErrorSample:     strPtrToText(run.ErrorSample),
		Notes:           strPtrToText(run.Notes),
	})
	if err != nil {
		return source.ScrapeRun{}, fmt.Errorf("sourcedb.CreateRun: %w", err)
	}
	return rowToRun(row), nil
}

func (s *Store) UpdateRun(ctx context.Context, run source.ScrapeRun) error {
	err := s.queries.UpdateScrapeRun(ctx, db.UpdateScrapeRunParams{
		ID:              run.ID,
		FinishedAt:      timePtrToTZ(run.FinishedAt),
		Status:          string(run.Status),
		DiscoveredCount: intPtrToInt4(run.DiscoveredCount),
		FetchedCount:    intPtrToInt4(run.FetchedCount),
		ParsedCount:     intPtrToInt4(run.ParsedCount),
		ErrorCount:      intPtrToInt4(run.ErrorCount),
		ErrorSample:     strPtrToText(run.ErrorSample),
		Notes:           strPtrToText(run.Notes),
	})
	if err != nil {
		return fmt.Errorf("sourcedb.UpdateRun: %w", err)
	}
	return nil
}

func (s *Store) QueryRunByID(ctx context.Context, id int64) (source.ScrapeRun, error) {
	row, err := s.queries.GetScrapeRunByID(ctx, id)
	if err != nil {
		return source.ScrapeRun{}, fmt.Errorf("sourcedb.QueryRunByID: %w", err)
	}
	return rowToRun(row), nil
}

func (s *Store) QueryRunsBySource(ctx context.Context, sourceID string, limit int) ([]source.ScrapeRun, error) {
	rows, err := s.queries.ListScrapeRunsBySource(ctx, db.ListScrapeRunsBySourceParams{
		SourceID: sourceID,
		Lim:      int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("sourcedb.QueryRunsBySource: %w", err)
	}
	out := make([]source.ScrapeRun, len(rows))
	for i, r := range rows {
		out[i] = rowToRun(r)
	}
	return out, nil
}

func (s *Store) CreateRawFetch(ctx context.Context, rf source.RawFetch) (source.RawFetch, error) {
	row, err := s.queries.CreateRawFetch(ctx, db.CreateRawFetchParams{
		SourceID:        rf.SourceID,
		SourceListingID: rf.SourceListingID,
		ScrapeRunID:     int64PtrToInt8(rf.ScrapeRunID),
		Url:             rf.URL,
		FetchedAt:       timeToTZ(rf.FetchedAt),
		StatusCode:      int32(rf.StatusCode),
		ContentType:     strPtrToText(rf.ContentType),
		Body:            rf.Body,
		BodySha256:      rf.BodySHA256,
		HeadersJson:     rf.HeadersJSON,
	})
	if err != nil {
		return source.RawFetch{}, fmt.Errorf("sourcedb.CreateRawFetch: %w", err)
	}
	return rowToRawFetch(row), nil
}

func (s *Store) QueryRawFetchByID(ctx context.Context, id int64) (source.RawFetch, error) {
	row, err := s.queries.GetRawFetchByID(ctx, id)
	if err != nil {
		return source.RawFetch{}, fmt.Errorf("sourcedb.QueryRawFetchByID: %w", err)
	}
	return rowToRawFetch(row), nil
}

func (s *Store) QueryRawFetchesByListing(ctx context.Context, sourceID, sourceListingID string) ([]source.RawFetch, error) {
	rows, err := s.queries.ListRawFetchesByListing(ctx, db.ListRawFetchesByListingParams{
		SourceID:        sourceID,
		SourceListingID: sourceListingID,
	})
	if err != nil {
		return nil, fmt.Errorf("sourcedb.QueryRawFetchesByListing: %w", err)
	}
	out := make([]source.RawFetch, len(rows))
	for i, r := range rows {
		out[i] = rowToRawFetch(r)
	}
	return out, nil
}

// -- conversion helpers --

func rowToSource(r db.Source) source.Source {
	return source.Source{
		ID:                             r.ID,
		DisplayName:                    r.DisplayName,
		BaseURL:                        r.BaseUrl,
		Enabled:                        r.Enabled,
		RateLimitMS:                    int(r.RateLimitMs),
		Concurrency:                    int(r.Concurrency),
		UserAgent:                      r.UserAgent,
		RespectRobots:                  r.RespectRobots,
		AbsenceDaysBeforeStale:         int(r.AbsenceDaysBeforeStale),
		AbsenceDaysBeforeInactive:      int(r.AbsenceDaysBeforeInactive),
		ConsecutiveMissedRunsThreshold: int(r.ConsecutiveMissedRunsThreshold),
		MinResultRatioForInactivation:  numericToFloat64(r.MinResultRatioForInactivation),
		LastRunAt:                      tzToTimePtr(r.LastRunAt),
		NextRunAt:                      tzToTimePtr(r.NextRunAt),
		Notes:                          textToStrPtr(r.Notes),
	}
}

func rowToRun(r db.ScrapeRun) source.ScrapeRun {
	return source.ScrapeRun{
		ID:              r.ID,
		SourceID:        r.SourceID,
		StartedAt:       r.StartedAt.Time,
		FinishedAt:      tzToTimePtr(r.FinishedAt),
		Status:          source.RunStatus(r.Status),
		DiscoveredCount: int4ToIntPtr(r.DiscoveredCount),
		FetchedCount:    int4ToIntPtr(r.FetchedCount),
		ParsedCount:     int4ToIntPtr(r.ParsedCount),
		ErrorCount:      int4ToIntPtr(r.ErrorCount),
		ErrorSample:     textToStrPtr(r.ErrorSample),
		Notes:           textToStrPtr(r.Notes),
	}
}

func rowToRawFetch(r db.RawFetch) source.RawFetch {
	return source.RawFetch{
		ID:              r.ID,
		SourceID:        r.SourceID,
		SourceListingID: r.SourceListingID,
		ScrapeRunID:     int8ToInt64Ptr(r.ScrapeRunID),
		URL:             r.Url,
		FetchedAt:       r.FetchedAt.Time,
		StatusCode:      int(r.StatusCode),
		ContentType:     textToStrPtr(r.ContentType),
		Body:            r.Body,
		BodySHA256:      r.BodySha256,
		HeadersJSON:     r.HeadersJson,
	}
}

func timeToTZ(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func timePtrToTZ(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func tzToTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func strPtrToText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func textToStrPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	v := t.String
	return &v
}

func intPtrToInt4(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

func int4ToIntPtr(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int32)
	return &i
}

func int64PtrToInt8(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func int8ToInt64Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	i := v.Int64
	return &i
}

func float64ToNumeric(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(f, 'f', -1, 64))
	return n
}

func numericToFloat64(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, _ := strconv.ParseFloat(n.Int.String(), 64)
	if n.Exp != 0 {
		exp := float64(n.Exp)
		if exp > 0 {
			for range int(exp) {
				f *= 10
			}
		} else {
			for range int(-exp) {
				f /= 10
			}
		}
	}
	return f
}

func uuidToStr(b [16]byte) string {
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}

func strToUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

var _ source.Storer = (*Store)(nil)

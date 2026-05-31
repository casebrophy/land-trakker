package listingdb

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
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

func (s *Store) CreateListing(ctx context.Context, l listing.Listing) (listing.Listing, error) {
	attrsExtra, err := jsonMarshal(l.AttrsExtra)
	if err != nil {
		return listing.Listing{}, fmt.Errorf("listingdb.CreateListing: attrs_extra: %w", err)
	}
	attrsExtraction, err := jsonMarshal(l.AttrsExtraction)
	if err != nil {
		return listing.Listing{}, fmt.Errorf("listingdb.CreateListing: attrs_extraction: %w", err)
	}
	photos := l.Photos
	if photos == nil {
		photos = []string{}
	}
	row, err := s.queries.CreateListing(ctx, db.CreateListingParams{
		SourceID:          l.SourceID,
		SourceListingID:   l.SourceListingID,
		Url:               l.URL,
		FirstSeenAt:       timeToTZ(l.FirstSeenAt),
		LastSeenAt:        timeToTZ(l.LastSeenAt),
		Status:            string(l.Status),
		ConsecutiveMisses: int32(l.ConsecutiveMisses),
		Dismissed:         l.Dismissed,
		DismissedReason:   strPtrToText(l.DismissedReason),
		Saved:             l.Saved,
		Title:             strPtrToText(l.Title),
		Description:       strPtrToText(l.Description),
		PriceCents:        int64PtrToInt8(l.PriceCents),
		Acres:             float64PtrToNumeric(l.Acres),
		AddressLine:       strPtrToText(l.AddressLine),
		City:              strPtrToText(l.City),
		County:            strPtrToText(l.County),
		State:             strPtrToText(l.State),
		PostalCode:        strPtrToText(l.PostalCode),
		GeomWkt:           pointToWKT(l.Geom),
		Photos:            photos,
		BrokerName:        strPtrToText(l.BrokerName),
		BrokerPhone:       strPtrToText(l.BrokerPhone),
		BrokerEmail:       strPtrToText(l.BrokerEmail),
		PostedAt:          timePtrToTZ(l.PostedAt),
		SourceUpdatedAt:   timePtrToTZ(l.SourceUpdatedAt),
		AttrWaterFrontage: boolPtrToBool(l.AttrWaterFrontage),
		AttrOffGrid:       boolPtrToBool(l.AttrOffGrid),
		AttrRoadAccess:    strPtrToText(l.AttrRoadAccess),
		AttrPower:         boolPtrToBool(l.AttrPower),
		AttrWell:          boolPtrToBool(l.AttrWell),
		AttrSeptic:        boolPtrToBool(l.AttrSeptic),
		AttrPropertyType:  strPtrToText(l.AttrPropertyType),
		AttrsExtra:        attrsExtra,
		AttrsExtraction:   attrsExtraction,
	})
	if err != nil {
		return listing.Listing{}, fmt.Errorf("listingdb.CreateListing: %w", err)
	}
	return createRowToListing(row)
}

func (s *Store) UpdateListing(ctx context.Context, l listing.Listing) error {
	attrsExtra, err := jsonMarshal(l.AttrsExtra)
	if err != nil {
		return fmt.Errorf("listingdb.UpdateListing: attrs_extra: %w", err)
	}
	attrsExtraction, err := jsonMarshal(l.AttrsExtraction)
	if err != nil {
		return fmt.Errorf("listingdb.UpdateListing: attrs_extraction: %w", err)
	}
	photos := l.Photos
	if photos == nil {
		photos = []string{}
	}
	err = s.queries.UpdateListing(ctx, db.UpdateListingParams{
		ID:                strToUUID(l.ID),
		Url:               l.URL,
		LastSeenAt:        timeToTZ(l.LastSeenAt),
		Status:            string(l.Status),
		ConsecutiveMisses: int32(l.ConsecutiveMisses),
		Dismissed:         l.Dismissed,
		DismissedReason:   strPtrToText(l.DismissedReason),
		Saved:             l.Saved,
		Title:             strPtrToText(l.Title),
		Description:       strPtrToText(l.Description),
		PriceCents:        int64PtrToInt8(l.PriceCents),
		Acres:             float64PtrToNumeric(l.Acres),
		AddressLine:       strPtrToText(l.AddressLine),
		City:              strPtrToText(l.City),
		County:            strPtrToText(l.County),
		State:             strPtrToText(l.State),
		PostalCode:        strPtrToText(l.PostalCode),
		GeomWkt:           pointToWKT(l.Geom),
		Photos:            photos,
		BrokerName:        strPtrToText(l.BrokerName),
		BrokerPhone:       strPtrToText(l.BrokerPhone),
		BrokerEmail:       strPtrToText(l.BrokerEmail),
		PostedAt:          timePtrToTZ(l.PostedAt),
		SourceUpdatedAt:   timePtrToTZ(l.SourceUpdatedAt),
		AttrWaterFrontage: boolPtrToBool(l.AttrWaterFrontage),
		AttrOffGrid:       boolPtrToBool(l.AttrOffGrid),
		AttrRoadAccess:    strPtrToText(l.AttrRoadAccess),
		AttrPower:         boolPtrToBool(l.AttrPower),
		AttrWell:          boolPtrToBool(l.AttrWell),
		AttrSeptic:        boolPtrToBool(l.AttrSeptic),
		AttrPropertyType:  strPtrToText(l.AttrPropertyType),
		AttrsExtra:        attrsExtra,
		AttrsExtraction:   attrsExtraction,
	})
	if err != nil {
		return fmt.Errorf("listingdb.UpdateListing: %w", err)
	}
	return nil
}

func (s *Store) QueryListingByID(ctx context.Context, id string) (listing.Listing, error) {
	row, err := s.queries.GetListingByID(ctx, strToUUID(id))
	if err != nil {
		return listing.Listing{}, fmt.Errorf("listingdb.QueryListingByID: %w", err)
	}
	return getByIDRowToListing(row)
}

func (s *Store) QueryListingBySource(ctx context.Context, sourceID, sourceListingID string) (listing.Listing, error) {
	row, err := s.queries.GetListingBySource(ctx, db.GetListingBySourceParams{
		SourceID:        sourceID,
		SourceListingID: sourceListingID,
	})
	if err != nil {
		return listing.Listing{}, fmt.Errorf("listingdb.QueryListingBySource: %w", err)
	}
	return getBySourceRowToListing(row)
}

func (s *Store) CreateSnapshot(ctx context.Context, snap listing.ListingSnapshot) (listing.ListingSnapshot, error) {
	structuredAttrs, err := jsonMarshal(snap.StructuredAttrs)
	if err != nil {
		return listing.ListingSnapshot{}, fmt.Errorf("listingdb.CreateSnapshot: structured_attrs: %w", err)
	}
	diff, err := jsonMarshal(snap.Diff)
	if err != nil {
		return listing.ListingSnapshot{}, fmt.Errorf("listingdb.CreateSnapshot: diff: %w", err)
	}
	row, err := s.queries.CreateListingSnapshot(ctx, db.CreateListingSnapshotParams{
		ListingID:       strToUUID(snap.ListingID),
		RawFetchID:      snap.RawFetchID,
		CapturedAt:      timeToTZ(snap.CapturedAt),
		PriceCents:      int64PtrToInt8(snap.PriceCents),
		Acres:           float64PtrToNumeric(snap.Acres),
		Status:          strPtrToText(snap.Status),
		Title:           strPtrToText(snap.Title),
		Description:     strPtrToText(snap.Description),
		StructuredAttrs: structuredAttrs,
		Diff:            diff,
	})
	if err != nil {
		return listing.ListingSnapshot{}, fmt.Errorf("listingdb.CreateSnapshot: %w", err)
	}
	return rowToSnapshot(row)
}

func (s *Store) QuerySnapshotsByListing(ctx context.Context, listingID string) ([]listing.ListingSnapshot, error) {
	rows, err := s.queries.ListSnapshotsByListing(ctx, strToUUID(listingID))
	if err != nil {
		return nil, fmt.Errorf("listingdb.QuerySnapshotsByListing: %w", err)
	}
	out := make([]listing.ListingSnapshot, len(rows))
	for i, r := range rows {
		snap, err := rowToSnapshot(r)
		if err != nil {
			return nil, fmt.Errorf("listingdb.QuerySnapshotsByListing: %w", err)
		}
		out[i] = snap
	}
	return out, nil
}

func (s *Store) CreatePriceChange(ctx context.Context, pc listing.PriceChange) (listing.PriceChange, error) {
	row, err := s.queries.CreatePriceChange(ctx, db.CreatePriceChangeParams{
		ListingID:     strToUUID(pc.ListingID),
		ChangedAt:     timeToTZ(pc.ChangedAt),
		OldPriceCents: int64PtrToInt8(pc.OldPriceCents),
		NewPriceCents: pc.NewPriceCents,
		SnapshotID:    int64PtrToInt8(pc.SnapshotID),
	})
	if err != nil {
		return listing.PriceChange{}, fmt.Errorf("listingdb.CreatePriceChange: %w", err)
	}
	return rowToPriceChange(row), nil
}

func (s *Store) QueryPriceChangesByListing(ctx context.Context, listingID string) ([]listing.PriceChange, error) {
	rows, err := s.queries.ListPriceChangesByListing(ctx, strToUUID(listingID))
	if err != nil {
		return nil, fmt.Errorf("listingdb.QueryPriceChangesByListing: %w", err)
	}
	out := make([]listing.PriceChange, len(rows))
	for i, r := range rows {
		out[i] = rowToPriceChange(r)
	}
	return out, nil
}

func (s *Store) CreateParseAttempt(ctx context.Context, pa listing.ParseAttempt) (listing.ParseAttempt, error) {
	row, err := s.queries.CreateParseAttempt(ctx, db.CreateParseAttemptParams{
		RawFetchID:    pa.RawFetchID,
		ParserVersion: pa.ParserVersion,
		AttemptedAt:   timeToTZ(pa.AttemptedAt),
		Outcome:       string(pa.Outcome),
		ErrorMessage:  strPtrToText(pa.ErrorMessage),
		SnapshotID:    int64PtrToInt8(pa.SnapshotID),
	})
	if err != nil {
		return listing.ParseAttempt{}, fmt.Errorf("listingdb.CreateParseAttempt: %w", err)
	}
	return rowToParseAttempt(row), nil
}

func (s *Store) QueryEligibleRawFetchIDs(ctx context.Context, sourceID string, parserVersion string) ([]int64, error) {
	const q = `
		SELECT rf.id
		FROM raw_fetches rf
		LEFT JOIN LATERAL (
			SELECT parser_version, outcome
			FROM parse_attempts
			WHERE raw_fetch_id = rf.id
			ORDER BY attempted_at DESC
			LIMIT 1
		) latest ON true
		WHERE rf.source_id = $1
		  AND (
		        latest.parser_version IS NULL
		     OR latest.outcome = 'parser_error'
		     OR (latest.outcome IN ('success','partial')
		         AND latest.parser_version <> $2)
		  )`
	rows, err := s.pool.Query(ctx, q, sourceID, parserVersion)
	if err != nil {
		return nil, fmt.Errorf("listingdb.QueryEligibleRawFetchIDs: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("listingdb.QueryEligibleRawFetchIDs: scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// -- row conversion helpers --

func createRowToListing(r db.CreateListingRow) (listing.Listing, error) {
	attrsExtra, err := jsonUnmarshal(r.AttrsExtra)
	if err != nil {
		return listing.Listing{}, err
	}
	attrsExtraction, err := jsonUnmarshal(r.AttrsExtraction)
	if err != nil {
		return listing.Listing{}, err
	}
	return listing.Listing{
		ID:                uuidToStr(r.ID.Bytes),
		SourceID:          r.SourceID,
		SourceListingID:   r.SourceListingID,
		URL:               r.Url,
		FirstSeenAt:       r.FirstSeenAt.Time,
		LastSeenAt:        r.LastSeenAt.Time,
		Status:            listing.ListingStatus(r.Status),
		ConsecutiveMisses: int(r.ConsecutiveMisses),
		Dismissed:         r.Dismissed,
		DismissedReason:   textToStrPtr(r.DismissedReason),
		Saved:             r.Saved,
		Title:             textToStrPtr(r.Title),
		Description:       textToStrPtr(r.Description),
		PriceCents:        int8ToInt64Ptr(r.PriceCents),
		Acres:             numericToFloat64Ptr(r.Acres),
		PricePerAcreCents: int8ToInt64Ptr(r.PricePerAcreCents),
		AddressLine:       textToStrPtr(r.AddressLine),
		City:              textToStrPtr(r.City),
		County:            textToStrPtr(r.County),
		State:             textToStrPtr(r.State),
		PostalCode:        textToStrPtr(r.PostalCode),
		Geom:              ifaceToPoint(r.GeomWkt),
		Photos:            r.Photos,
		BrokerName:        textToStrPtr(r.BrokerName),
		BrokerPhone:       textToStrPtr(r.BrokerPhone),
		BrokerEmail:       textToStrPtr(r.BrokerEmail),
		PostedAt:          tzToTimePtr(r.PostedAt),
		SourceUpdatedAt:   tzToTimePtr(r.SourceUpdatedAt),
		AttrWaterFrontage: boolToPtr(r.AttrWaterFrontage),
		AttrOffGrid:       boolToPtr(r.AttrOffGrid),
		AttrRoadAccess:    textToStrPtr(r.AttrRoadAccess),
		AttrPower:         boolToPtr(r.AttrPower),
		AttrWell:          boolToPtr(r.AttrWell),
		AttrSeptic:        boolToPtr(r.AttrSeptic),
		AttrPropertyType:  textToStrPtr(r.AttrPropertyType),
		AttrsExtra:        attrsExtra,
		AttrsExtraction:   attrsExtraction,
	}, nil
}

func getByIDRowToListing(r db.GetListingByIDRow) (listing.Listing, error) {
	attrsExtra, err := jsonUnmarshal(r.AttrsExtra)
	if err != nil {
		return listing.Listing{}, err
	}
	attrsExtraction, err := jsonUnmarshal(r.AttrsExtraction)
	if err != nil {
		return listing.Listing{}, err
	}
	return listing.Listing{
		ID:                uuidToStr(r.ID.Bytes),
		SourceID:          r.SourceID,
		SourceListingID:   r.SourceListingID,
		URL:               r.Url,
		FirstSeenAt:       r.FirstSeenAt.Time,
		LastSeenAt:        r.LastSeenAt.Time,
		Status:            listing.ListingStatus(r.Status),
		ConsecutiveMisses: int(r.ConsecutiveMisses),
		Dismissed:         r.Dismissed,
		DismissedReason:   textToStrPtr(r.DismissedReason),
		Saved:             r.Saved,
		Title:             textToStrPtr(r.Title),
		Description:       textToStrPtr(r.Description),
		PriceCents:        int8ToInt64Ptr(r.PriceCents),
		Acres:             numericToFloat64Ptr(r.Acres),
		PricePerAcreCents: int8ToInt64Ptr(r.PricePerAcreCents),
		AddressLine:       textToStrPtr(r.AddressLine),
		City:              textToStrPtr(r.City),
		County:            textToStrPtr(r.County),
		State:             textToStrPtr(r.State),
		PostalCode:        textToStrPtr(r.PostalCode),
		Geom:              ifaceToPoint(r.GeomWkt),
		Photos:            r.Photos,
		BrokerName:        textToStrPtr(r.BrokerName),
		BrokerPhone:       textToStrPtr(r.BrokerPhone),
		BrokerEmail:       textToStrPtr(r.BrokerEmail),
		PostedAt:          tzToTimePtr(r.PostedAt),
		SourceUpdatedAt:   tzToTimePtr(r.SourceUpdatedAt),
		AttrWaterFrontage: boolToPtr(r.AttrWaterFrontage),
		AttrOffGrid:       boolToPtr(r.AttrOffGrid),
		AttrRoadAccess:    textToStrPtr(r.AttrRoadAccess),
		AttrPower:         boolToPtr(r.AttrPower),
		AttrWell:          boolToPtr(r.AttrWell),
		AttrSeptic:        boolToPtr(r.AttrSeptic),
		AttrPropertyType:  textToStrPtr(r.AttrPropertyType),
		AttrsExtra:        attrsExtra,
		AttrsExtraction:   attrsExtraction,
	}, nil
}

func getBySourceRowToListing(r db.GetListingBySourceRow) (listing.Listing, error) {
	attrsExtra, err := jsonUnmarshal(r.AttrsExtra)
	if err != nil {
		return listing.Listing{}, err
	}
	attrsExtraction, err := jsonUnmarshal(r.AttrsExtraction)
	if err != nil {
		return listing.Listing{}, err
	}
	return listing.Listing{
		ID:                uuidToStr(r.ID.Bytes),
		SourceID:          r.SourceID,
		SourceListingID:   r.SourceListingID,
		URL:               r.Url,
		FirstSeenAt:       r.FirstSeenAt.Time,
		LastSeenAt:        r.LastSeenAt.Time,
		Status:            listing.ListingStatus(r.Status),
		ConsecutiveMisses: int(r.ConsecutiveMisses),
		Dismissed:         r.Dismissed,
		DismissedReason:   textToStrPtr(r.DismissedReason),
		Saved:             r.Saved,
		Title:             textToStrPtr(r.Title),
		Description:       textToStrPtr(r.Description),
		PriceCents:        int8ToInt64Ptr(r.PriceCents),
		Acres:             numericToFloat64Ptr(r.Acres),
		PricePerAcreCents: int8ToInt64Ptr(r.PricePerAcreCents),
		AddressLine:       textToStrPtr(r.AddressLine),
		City:              textToStrPtr(r.City),
		County:            textToStrPtr(r.County),
		State:             textToStrPtr(r.State),
		PostalCode:        textToStrPtr(r.PostalCode),
		Geom:              ifaceToPoint(r.GeomWkt),
		Photos:            r.Photos,
		BrokerName:        textToStrPtr(r.BrokerName),
		BrokerPhone:       textToStrPtr(r.BrokerPhone),
		BrokerEmail:       textToStrPtr(r.BrokerEmail),
		PostedAt:          tzToTimePtr(r.PostedAt),
		SourceUpdatedAt:   tzToTimePtr(r.SourceUpdatedAt),
		AttrWaterFrontage: boolToPtr(r.AttrWaterFrontage),
		AttrOffGrid:       boolToPtr(r.AttrOffGrid),
		AttrRoadAccess:    textToStrPtr(r.AttrRoadAccess),
		AttrPower:         boolToPtr(r.AttrPower),
		AttrWell:          boolToPtr(r.AttrWell),
		AttrSeptic:        boolToPtr(r.AttrSeptic),
		AttrPropertyType:  textToStrPtr(r.AttrPropertyType),
		AttrsExtra:        attrsExtra,
		AttrsExtraction:   attrsExtraction,
	}, nil
}

func rowToSnapshot(r db.ListingSnapshot) (listing.ListingSnapshot, error) {
	structuredAttrs, err := jsonUnmarshal(r.StructuredAttrs)
	if err != nil {
		return listing.ListingSnapshot{}, err
	}
	diff, err := jsonUnmarshal(r.Diff)
	if err != nil {
		return listing.ListingSnapshot{}, err
	}
	return listing.ListingSnapshot{
		ID:              r.ID,
		ListingID:       uuidToStr(r.ListingID.Bytes),
		RawFetchID:      r.RawFetchID,
		CapturedAt:      r.CapturedAt.Time,
		PriceCents:      int8ToInt64Ptr(r.PriceCents),
		Acres:           numericToFloat64Ptr(r.Acres),
		Status:          textToStrPtr(r.Status),
		Title:           textToStrPtr(r.Title),
		Description:     textToStrPtr(r.Description),
		StructuredAttrs: structuredAttrs,
		Diff:            diff,
	}, nil
}

func rowToParseAttempt(r db.ParseAttempt) listing.ParseAttempt {
	return listing.ParseAttempt{
		ID:            r.ID,
		RawFetchID:    r.RawFetchID,
		ParserVersion: r.ParserVersion,
		AttemptedAt:   r.AttemptedAt.Time,
		Outcome:       listing.ParseAttemptOutcome(r.Outcome),
		ErrorMessage:  textToStrPtr(r.ErrorMessage),
		SnapshotID:    int8ToInt64Ptr(r.SnapshotID),
	}
}

func rowToPriceChange(r db.PriceChange) listing.PriceChange {
	return listing.PriceChange{
		ID:            r.ID,
		ListingID:     uuidToStr(r.ListingID.Bytes),
		ChangedAt:     r.ChangedAt.Time,
		OldPriceCents: int8ToInt64Ptr(r.OldPriceCents),
		NewPriceCents: r.NewPriceCents,
		DeltaCents:    r.DeltaCents.Int64,
		SnapshotID:    int8ToInt64Ptr(r.SnapshotID),
	}
}

// -- type conversion helpers --

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

func boolPtrToBool(v *bool) pgtype.Bool {
	if v == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *v, Valid: true}
}

func boolToPtr(v pgtype.Bool) *bool {
	if !v.Valid {
		return nil
	}
	b := v.Bool
	return &b
}

func float64PtrToNumeric(v *float64) pgtype.Numeric {
	if v == nil {
		return pgtype.Numeric{}
	}
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(*v, 'f', -1, 64))
	return n
}

func numericToFloat64Ptr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
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
	return &f
}

func pointToWKT(p *listing.Point) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{
		String: fmt.Sprintf("POINT(%f %f)", p.Lng, p.Lat),
		Valid:  true,
	}
}

func ifaceToPoint(v interface{}) *listing.Point {
	if v == nil {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return nil
	}
	var lng, lat float64
	if _, err := fmt.Sscanf(s, "POINT(%f %f)", &lng, &lat); err != nil {
		return nil
	}
	return &listing.Point{Lat: lat, Lng: lng}
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

func jsonMarshal(m map[string]any) ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

func jsonUnmarshal(b []byte) (map[string]any, error) {
	if len(b) == 0 || string(b) == "null" {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

var _ listing.Storer = (*Store)(nil)

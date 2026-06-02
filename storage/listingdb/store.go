package listingdb

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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
	result, err := createRowToListing(row)
	if err != nil {
		return listing.Listing{}, err
	}

	// Populate auction fields if they exist
	if l.AuctionEndDate != nil || l.AuctionCurrentBid != nil || l.AuctionReserve != nil {
		if err := s.CreateAuctionExt(ctx, result.ID, l.AuctionEndDate, l.AuctionCurrentBid, l.AuctionReserve); err != nil {
			return listing.Listing{}, err
		}
		result.AuctionEndDate = l.AuctionEndDate
		result.AuctionCurrentBid = l.AuctionCurrentBid
		result.AuctionReserve = l.AuctionReserve
	}

	return result, nil
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

	// Update auction fields if they exist; otherwise, leave as-is
	if l.AuctionEndDate != nil || l.AuctionCurrentBid != nil || l.AuctionReserve != nil {
		// Check if auction extension exists
		existing, err := s.getAuctionExtByListingID(ctx, l.ID)
		if err != nil {
			return err
		}
		if existing != nil {
			// Update existing
			if err := s.UpdateAuctionExt(ctx, l.ID, l.AuctionEndDate, l.AuctionCurrentBid, l.AuctionReserve); err != nil {
				return err
			}
		} else {
			// Create new
			if err := s.CreateAuctionExt(ctx, l.ID, l.AuctionEndDate, l.AuctionCurrentBid, l.AuctionReserve); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Store) QueryListingByID(ctx context.Context, id string) (listing.Listing, error) {
	row, err := s.queries.GetListingByID(ctx, strToUUID(id))
	if err != nil {
		return listing.Listing{}, fmt.Errorf("listingdb.QueryListingByID: %w", err)
	}
	result, err := getByIDRowToListing(row)
	if err != nil {
		return listing.Listing{}, err
	}

	// Populate auction fields if they exist
	auctionExt, err := s.getAuctionExtByListingID(ctx, id)
	if err != nil {
		return listing.Listing{}, err
	}
	if auctionExt != nil {
		result.AuctionEndDate = auctionExt.endDate
		result.AuctionCurrentBid = auctionExt.currentBid
		result.AuctionReserve = auctionExt.reserve
	}

	return result, nil
}

func (s *Store) QueryListingBySource(ctx context.Context, sourceID, sourceListingID string) (listing.Listing, error) {
	row, err := s.queries.GetListingBySource(ctx, db.GetListingBySourceParams{
		SourceID:        sourceID,
		SourceListingID: sourceListingID,
	})
	if err != nil {
		return listing.Listing{}, fmt.Errorf("listingdb.QueryListingBySource: %w", err)
	}
	result, err := getBySourceRowToListing(row)
	if err != nil {
		return listing.Listing{}, err
	}

	// Populate auction fields if they exist
	auctionExt, err := s.getAuctionExtByListingID(ctx, result.ID)
	if err != nil {
		return listing.Listing{}, err
	}
	if auctionExt != nil {
		result.AuctionEndDate = auctionExt.endDate
		result.AuctionCurrentBid = auctionExt.currentBid
		result.AuctionReserve = auctionExt.reserve
	}

	return result, nil
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

func (s *Store) QueryListings(ctx context.Context, limit, offset int) ([]listing.Listing, error) {
	const q = `
		SELECT
		    id, source_id, source_listing_id, url, first_seen_at, last_seen_at,
		    status, consecutive_misses, dismissed, dismissed_reason, saved,
		    title, description, price_cents, acres, price_per_acre_cents,
		    address_line, city, county, state, postal_code,
		    ST_AsText(geom) AS geom_wkt,
		    photos, broker_name, broker_phone, broker_email, posted_at, source_updated_at,
		    attr_water_frontage, attr_off_grid, attr_road_access, attr_power, attr_well, attr_septic,
		    attr_property_type, attrs_extra, attrs_extraction
		FROM listings
		ORDER BY first_seen_at DESC
		LIMIT $1 OFFSET $2`
	rows, err := s.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listingdb.QueryListings: %w", err)
	}
	defer rows.Close()
	var out []listing.Listing
	var listingIDs []string
	for rows.Next() {
		var r db.GetListingByIDRow
		if err := rows.Scan(
			&r.ID,
			&r.SourceID,
			&r.SourceListingID,
			&r.Url,
			&r.FirstSeenAt,
			&r.LastSeenAt,
			&r.Status,
			&r.ConsecutiveMisses,
			&r.Dismissed,
			&r.DismissedReason,
			&r.Saved,
			&r.Title,
			&r.Description,
			&r.PriceCents,
			&r.Acres,
			&r.PricePerAcreCents,
			&r.AddressLine,
			&r.City,
			&r.County,
			&r.State,
			&r.PostalCode,
			&r.GeomWkt,
			&r.Photos,
			&r.BrokerName,
			&r.BrokerPhone,
			&r.BrokerEmail,
			&r.PostedAt,
			&r.SourceUpdatedAt,
			&r.AttrWaterFrontage,
			&r.AttrOffGrid,
			&r.AttrRoadAccess,
			&r.AttrPower,
			&r.AttrWell,
			&r.AttrSeptic,
			&r.AttrPropertyType,
			&r.AttrsExtra,
			&r.AttrsExtraction,
		); err != nil {
			return nil, fmt.Errorf("listingdb.QueryListings: scan: %w", err)
		}
		l, err := getByIDRowToListing(r)
		if err != nil {
			return nil, fmt.Errorf("listingdb.QueryListings: convert: %w", err)
		}
		out = append(out, l)
		listingIDs = append(listingIDs, l.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Batch-load auction extensions
	if len(listingIDs) > 0 {
		auctionExts, err := s.getAuctionExtByListingIDs(ctx, listingIDs)
		if err != nil {
			return nil, err
		}
		for i := range out {
			if auctionExt, ok := auctionExts[out[i].ID]; ok {
				out[i].AuctionEndDate = auctionExt.endDate
				out[i].AuctionCurrentBid = auctionExt.currentBid
				out[i].AuctionReserve = auctionExt.reserve
			}
		}
	}

	return out, nil
}

func (s *Store) QueryListingsFilter(ctx context.Context, f listing.ListingFilter, limit, offset int) ([]listing.Listing, error) {
	const baseQ = `
		SELECT
		    id, source_id, source_listing_id, url, first_seen_at, last_seen_at,
		    status, consecutive_misses, dismissed, dismissed_reason, saved,
		    title, description, price_cents, acres, price_per_acre_cents,
		    address_line, city, county, state, postal_code,
		    ST_AsText(geom) AS geom_wkt,
		    photos, broker_name, broker_phone, broker_email, posted_at, source_updated_at,
		    attr_water_frontage, attr_off_grid, attr_road_access, attr_power, attr_well, attr_septic,
		    attr_property_type, attrs_extra, attrs_extraction
		FROM listings`

	var sb strings.Builder
	sb.WriteString(baseQ)

	var args []any
	argN := 1

	addCond := func(cond string, val any) {
		if argN == 1 {
			sb.WriteString("\n\t\tWHERE ")
		} else {
			sb.WriteString("\n\t\t  AND ")
		}
		sb.WriteString(cond)
		args = append(args, val)
		argN++
	}

	if f.AcresMin != nil {
		addCond(fmt.Sprintf("acres >= $%d", argN), *f.AcresMin)
	}
	if f.AcresMax != nil {
		addCond(fmt.Sprintf("acres <= $%d", argN), *f.AcresMax)
	}
	if f.PriceMin != nil {
		addCond(fmt.Sprintf("price_cents >= $%d", argN), *f.PriceMin)
	}
	if f.PriceMax != nil {
		addCond(fmt.Sprintf("price_cents <= $%d", argN), *f.PriceMax)
	}
	if len(f.Counties) > 0 {
		addCond(fmt.Sprintf("county = ANY($%d)", argN), f.Counties)
	}
	if f.PPAMin != nil {
		addCond(fmt.Sprintf("price_per_acre_cents >= $%d", argN), *f.PPAMin)
	}
	if f.PPAMax != nil {
		addCond(fmt.Sprintf("price_per_acre_cents <= $%d", argN), *f.PPAMax)
	}
	if f.PropertyType != nil {
		addCond(fmt.Sprintf("attr_property_type = $%d", argN), *f.PropertyType)
	}
	if f.AttrWaterFrontage != nil {
		addCond(fmt.Sprintf("attr_water_frontage = $%d", argN), *f.AttrWaterFrontage)
	}
	if f.AttrOffGrid != nil {
		addCond(fmt.Sprintf("attr_off_grid = $%d", argN), *f.AttrOffGrid)
	}
	if f.AttrPower != nil {
		addCond(fmt.Sprintf("attr_power = $%d", argN), *f.AttrPower)
	}
	if f.AttrWell != nil {
		addCond(fmt.Sprintf("attr_well = $%d", argN), *f.AttrWell)
	}
	if f.AttrSeptic != nil {
		addCond(fmt.Sprintf("attr_septic = $%d", argN), *f.AttrSeptic)
	}
	if f.FullText != nil {
		addCond(fmt.Sprintf("to_tsvector('english', coalesce(title,'') || ' ' || coalesce(description,'')) @@ plainto_tsquery('english', $%d)", argN), *f.FullText)
	}

	sb.WriteString(fmt.Sprintf("\n\t\tORDER BY first_seen_at DESC LIMIT $%d OFFSET $%d", argN, argN+1))
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("listingdb.QueryListingsFilter: %w", err)
	}
	defer rows.Close()
	var out []listing.Listing
	var listingIDs []string
	for rows.Next() {
		var r db.GetListingByIDRow
		if err := rows.Scan(
			&r.ID,
			&r.SourceID,
			&r.SourceListingID,
			&r.Url,
			&r.FirstSeenAt,
			&r.LastSeenAt,
			&r.Status,
			&r.ConsecutiveMisses,
			&r.Dismissed,
			&r.DismissedReason,
			&r.Saved,
			&r.Title,
			&r.Description,
			&r.PriceCents,
			&r.Acres,
			&r.PricePerAcreCents,
			&r.AddressLine,
			&r.City,
			&r.County,
			&r.State,
			&r.PostalCode,
			&r.GeomWkt,
			&r.Photos,
			&r.BrokerName,
			&r.BrokerPhone,
			&r.BrokerEmail,
			&r.PostedAt,
			&r.SourceUpdatedAt,
			&r.AttrWaterFrontage,
			&r.AttrOffGrid,
			&r.AttrRoadAccess,
			&r.AttrPower,
			&r.AttrWell,
			&r.AttrSeptic,
			&r.AttrPropertyType,
			&r.AttrsExtra,
			&r.AttrsExtraction,
		); err != nil {
			return nil, fmt.Errorf("listingdb.QueryListingsFilter: scan: %w", err)
		}
		l, err := getByIDRowToListing(r)
		if err != nil {
			return nil, fmt.Errorf("listingdb.QueryListingsFilter: convert: %w", err)
		}
		out = append(out, l)
		listingIDs = append(listingIDs, l.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Batch-load auction extensions
	if len(listingIDs) > 0 {
		auctionExts, err := s.getAuctionExtByListingIDs(ctx, listingIDs)
		if err != nil {
			return nil, err
		}
		for i := range out {
			if auctionExt, ok := auctionExts[out[i].ID]; ok {
				out[i].AuctionEndDate = auctionExt.endDate
				out[i].AuctionCurrentBid = auctionExt.currentBid
				out[i].AuctionReserve = auctionExt.reserve
			}
		}
	}

	return out, nil
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

func (s *Store) QueryListingsForDedup(ctx context.Context) ([]listing.Listing, error) {
	const q = `
		SELECT
		    id, source_id, source_listing_id, url, first_seen_at, last_seen_at,
		    status, consecutive_misses, dismissed, dismissed_reason, saved,
		    title, description, price_cents, acres, price_per_acre_cents,
		    address_line, city, county, state, postal_code,
		    ST_AsText(geom) AS geom_wkt,
		    photos, broker_name, broker_phone, broker_email, posted_at, source_updated_at,
		    attr_water_frontage, attr_off_grid, attr_road_access, attr_power, attr_well, attr_septic,
		    attr_property_type, attrs_extra, attrs_extraction
		FROM listings
		WHERE status IN ('active','stale')
		ORDER BY id`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("listingdb.QueryListingsForDedup: %w", err)
	}
	defer rows.Close()
	var out []listing.Listing
	var listingIDs []string
	for rows.Next() {
		var r db.GetListingByIDRow
		if err := rows.Scan(
			&r.ID,
			&r.SourceID,
			&r.SourceListingID,
			&r.Url,
			&r.FirstSeenAt,
			&r.LastSeenAt,
			&r.Status,
			&r.ConsecutiveMisses,
			&r.Dismissed,
			&r.DismissedReason,
			&r.Saved,
			&r.Title,
			&r.Description,
			&r.PriceCents,
			&r.Acres,
			&r.PricePerAcreCents,
			&r.AddressLine,
			&r.City,
			&r.County,
			&r.State,
			&r.PostalCode,
			&r.GeomWkt,
			&r.Photos,
			&r.BrokerName,
			&r.BrokerPhone,
			&r.BrokerEmail,
			&r.PostedAt,
			&r.SourceUpdatedAt,
			&r.AttrWaterFrontage,
			&r.AttrOffGrid,
			&r.AttrRoadAccess,
			&r.AttrPower,
			&r.AttrWell,
			&r.AttrSeptic,
			&r.AttrPropertyType,
			&r.AttrsExtra,
			&r.AttrsExtraction,
		); err != nil {
			return nil, fmt.Errorf("listingdb.QueryListingsForDedup: scan: %w", err)
		}
		l, err := getByIDRowToListing(r)
		if err != nil {
			return nil, fmt.Errorf("listingdb.QueryListingsForDedup: convert: %w", err)
		}
		out = append(out, l)
		listingIDs = append(listingIDs, l.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Batch-load auction extensions
	if len(listingIDs) > 0 {
		auctionExts, err := s.getAuctionExtByListingIDs(ctx, listingIDs)
		if err != nil {
			return nil, err
		}
		for i := range out {
			if auctionExt, ok := auctionExts[out[i].ID]; ok {
				out[i].AuctionEndDate = auctionExt.endDate
				out[i].AuctionCurrentBid = auctionExt.currentBid
				out[i].AuctionReserve = auctionExt.reserve
			}
		}
	}

	return out, nil
}

func (s *Store) UpsertPossibleDuplicate(ctx context.Context, pd listing.PossibleDuplicate) error {
	const q = `
		INSERT INTO possible_duplicates (listing_a_id, listing_b_id, score, reasons, detected_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5)
		ON CONFLICT (listing_a_id, listing_b_id) DO UPDATE SET
		    score = EXCLUDED.score,
		    reasons = EXCLUDED.reasons,
		    detected_at = EXCLUDED.detected_at`
	_, err := s.pool.Exec(ctx, q, pd.ListingAID, pd.ListingBID, pd.Score, pd.Reasons, timeToTZ(pd.DetectedAt))
	if err != nil {
		return fmt.Errorf("listingdb.UpsertPossibleDuplicate: %w", err)
	}
	return nil
}

func (s *Store) QueryPossibleDuplicates(ctx context.Context, decision *string) ([]listing.PossibleDuplicate, error) {
	var q string
	var args []any
	if decision == nil {
		q = `SELECT listing_a_id, listing_b_id, score, reasons, detected_at, user_decision
		     FROM possible_duplicates
		     WHERE user_decision IS NULL
		     ORDER BY score DESC`
	} else {
		q = `SELECT listing_a_id, listing_b_id, score, reasons, detected_at, user_decision
		     FROM possible_duplicates
		     WHERE user_decision = $1
		     ORDER BY score DESC`
		args = append(args, *decision)
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listingdb.QueryPossibleDuplicates: %w", err)
	}
	defer rows.Close()
	var out []listing.PossibleDuplicate
	for rows.Next() {
		var aID, bID pgtype.UUID
		var score pgtype.Numeric
		var reasons []string
		var detectedAt pgtype.Timestamptz
		var userDecision pgtype.Text
		if err := rows.Scan(&aID, &bID, &score, &reasons, &detectedAt, &userDecision); err != nil {
			return nil, fmt.Errorf("listingdb.QueryPossibleDuplicates: scan: %w", err)
		}
		scoreF := numericToFloat64Ptr(score)
		var scoreVal float64
		if scoreF != nil {
			scoreVal = *scoreF
		}
		pd := listing.PossibleDuplicate{
			ListingAID:   uuidToStr(aID.Bytes),
			ListingBID:   uuidToStr(bID.Bytes),
			Score:        scoreVal,
			Reasons:      reasons,
			DetectedAt:   detectedAt.Time,
			UserDecision: textToStrPtr(userDecision),
		}
		out = append(out, pd)
	}
	return out, rows.Err()
}

func (s *Store) UpdateDuplicateDecision(ctx context.Context, aID, bID string, decision string) error {
	const q = `UPDATE possible_duplicates SET user_decision = $1 WHERE listing_a_id = $2::uuid AND listing_b_id = $3::uuid`
	_, err := s.pool.Exec(ctx, q, decision, aID, bID)
	if err != nil {
		return fmt.Errorf("listingdb.UpdateDuplicateDecision: %w", err)
	}
	return nil
}

// CreateAuctionExt creates an auction_extension record for a listing.
func (s *Store) CreateAuctionExt(ctx context.Context, listingID string, endDate *time.Time, currentBid, reserve *int64) error {
	err := s.queries.CreateAuctionExt(ctx, db.CreateAuctionExtParams{
		ListingID:         strToUUID(listingID),
		AuctionEndDate:    timePtrToTZ(endDate),
		AuctionCurrentBid: int64PtrToInt8(currentBid),
		AuctionReserve:    int64PtrToInt8(reserve),
	})
	if err != nil {
		return fmt.Errorf("listingdb.CreateAuctionExt: %w", err)
	}
	return nil
}

// UpdateAuctionExt updates an auction_extension record for a listing.
func (s *Store) UpdateAuctionExt(ctx context.Context, listingID string, endDate *time.Time, currentBid, reserve *int64) error {
	err := s.queries.UpdateAuctionExt(ctx, db.UpdateAuctionExtParams{
		ListingID:         strToUUID(listingID),
		AuctionEndDate:    timePtrToTZ(endDate),
		AuctionCurrentBid: int64PtrToInt8(currentBid),
		AuctionReserve:    int64PtrToInt8(reserve),
	})
	if err != nil {
		return fmt.Errorf("listingdb.UpdateAuctionExt: %w", err)
	}
	return nil
}

// getAuctionExtByListingID retrieves auction_extension data for a listing.
// Returns nil if no row found (not an error).
func (s *Store) getAuctionExtByListingID(ctx context.Context, listingID string) (*struct {
	endDate   *time.Time
	currentBid *int64
	reserve   *int64
}, error) {
	row, err := s.queries.GetAuctionExt(ctx, strToUUID(listingID))
	if err != nil {
		// No auction extension found; return nil, not an error
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("listingdb.getAuctionExtByListingID: %w", err)
	}
	return &struct {
		endDate   *time.Time
		currentBid *int64
		reserve   *int64
	}{
		endDate:    tzToTimePtr(row.AuctionEndDate),
		currentBid: int8ToInt64Ptr(row.AuctionCurrentBid),
		reserve:    int8ToInt64Ptr(row.AuctionReserve),
	}, nil
}

// getAuctionExtByListingIDs batch-loads auction_extension data for multiple listing IDs.
// Returns a map of listing ID to auction data.
func (s *Store) getAuctionExtByListingIDs(ctx context.Context, listingIDs []string) (map[string]struct {
	endDate   *time.Time
	currentBid *int64
	reserve   *int64
}, error) {
	if len(listingIDs) == 0 {
		return map[string]struct {
			endDate   *time.Time
			currentBid *int64
			reserve   *int64
		}{}, nil
	}

	const q = `
		SELECT listing_id, auction_end_date, auction_current_bid, auction_reserve
		FROM auction_extension
		WHERE listing_id = ANY($1::uuid[])`

	var uuids []string
	for _, id := range listingIDs {
		uuids = append(uuids, id)
	}

	rows, err := s.pool.Query(ctx, q, uuids)
	if err != nil {
		return nil, fmt.Errorf("listingdb.getAuctionExtByListingIDs: %w", err)
	}
	defer rows.Close()

	result := make(map[string]struct {
		endDate   *time.Time
		currentBid *int64
		reserve   *int64
	})

	for rows.Next() {
		var listingID pgtype.UUID
		var auctionEndDate pgtype.Timestamptz
		var auctionCurrentBid pgtype.Int8
		var auctionReserve pgtype.Int8
		if err := rows.Scan(&listingID, &auctionEndDate, &auctionCurrentBid, &auctionReserve); err != nil {
			return nil, fmt.Errorf("listingdb.getAuctionExtByListingIDs: scan: %w", err)
		}
		id := uuidToStr(listingID.Bytes)
		result[id] = struct {
			endDate   *time.Time
			currentBid *int64
			reserve   *int64
		}{
			endDate:    tzToTimePtr(auctionEndDate),
			currentBid: int8ToInt64Ptr(auctionCurrentBid),
			reserve:    int8ToInt64Ptr(auctionReserve),
		}
	}
	return result, rows.Err()
}

var _ listing.Storer = (*Store)(nil)

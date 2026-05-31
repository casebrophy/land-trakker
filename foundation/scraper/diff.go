package scraper

// DiffRefs computes the difference between two sets of source listing IDs.
// prev contains the IDs from the last discovery run; curr contains the IDs
// from the current Discover() call.  Returns added (new), kept (present in
// both), and removed (disappeared) slices.  Order within each slice is
// unspecified.
func DiffRefs(prev, curr []string) (added, kept, removed []string) {
	prevSet := make(map[string]struct{}, len(prev))
	for _, id := range prev {
		prevSet[id] = struct{}{}
	}
	currSet := make(map[string]struct{}, len(curr))
	for _, id := range curr {
		currSet[id] = struct{}{}
	}
	for _, id := range curr {
		if _, ok := prevSet[id]; ok {
			kept = append(kept, id)
		} else {
			added = append(added, id)
		}
	}
	for _, id := range prev {
		if _, ok := currSet[id]; !ok {
			removed = append(removed, id)
		}
	}
	return
}

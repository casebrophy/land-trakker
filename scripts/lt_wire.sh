#!/bin/bash
# bash 3.2 compatible. No associative arrays, no mapfile. Sequential, no races.
cd /Users/casebrophy/personal/land_trakker || exit 1

MAP=/tmp/lt_map.tsv
bd list --all --json 2>/dev/null | jq -r '.[] | "\(.id)\t\(.title)"' > "$MAP"
N=$(wc -l < "$MAP" | tr -d ' ')
echo "MAP_SIZE=$N"
[ "$N" -lt 60 ] && { echo "FATAL: map too small"; exit 1; }

# exact title -> id
tid() { awk -F'\t' -v t="$1" '$2==t{print $1; exit}' "$MAP"; }
# prefix title -> id (for long gate titles)
pid() { awk -F'\t' -v t="$1" 'index($2,t)==1{print $1; exit}' "$MAP"; }

GSEC=$(pid "gate-secrets"); GVPS=$(pid "gate-vps"); GFIX=$(pid "gate-fixtures")
echo "GATES sec=$GSEC vps=$GVPS fix=$GFIX"
[ -z "$GSEC" ] || [ -z "$GVPS" ] || [ -z "$GFIX" ] && { echo "FATAL: gate id missing"; exit 1; }

OK=0; DUP=0; BAD=0
add() { # add <child-title> <prereq-title|id>
  c=$(tid "$1")
  case "$2" in land_trakker-*) p="$2";; *) p=$(tid "$2");; esac
  if [ -z "$c" ] || [ -z "$p" ]; then echo "BAD: $1($c) -> $2($p)"; BAD=$((BAD+1)); return; fi
  if bd dep add "$c" "$p" >/dev/null 2>&1; then OK=$((OK+1)); else DUP=$((DUP+1)); fi
}

# Phase 0
add config-toml bootstrap-module
add migrations-core bootstrap-module
add domain-types bootstrap-module
add parser-helpers bootstrap-module
add scraper-iface bootstrap-module
add rate-limiter scraper-iface
add storage-impls migrations-core
add storage-impls domain-types
add listingbus domain-types
add backfill-query storage-impls
add backfill-query listingbus
add orchestrator rate-limiter
add orchestrator listingbus
add orchestrator storage-impls
add fakebroker scraper-iface
add cmd-scraperd orchestrator
add cmd-scraperd fakebroker
add cmd-scrape-once orchestrator
add cmd-scrape-once fakebroker
add web-shell config-toml
add web-listings web-shell
add web-listings listingbus
add p0-e2e-tests cmd-scrape-once
add p0-e2e-tests backfill-query
add p0-e2e-tests fakebroker
# Phase 1
add geocode-client migrations-core
add geocode-client domain-types
add normalize-geocode geocode-client
add normalize-geocode orchestrator
add search-filters listingbus
add search-filters storage-impls
add web-search-map web-listings
add web-search-map search-filters
add price-history-ui web-listings
add price-history-ui listingbus
add scraper-fixture-harness parser-helpers
add scraper-fixture-harness scraper-iface
# Phase 2
add saved-searches search-filters
add web-saved-searches web-search-map
add web-saved-searches saved-searches
add dedup-job listingbus
add dedup-job geocode-client
add web-duplicates web-listings
add web-duplicates dedup-job
add auction-ext domain-types
add auction-ext migrations-core
# Phase 3
add attr-extractors parser-helpers
add attr-wire attr-extractors
add attr-wire orchestrator
add fulltext-search search-filters
add fulltext-search web-search-map
add health-dashboard web-shell
add health-dashboard listingbus
add logging-tail web-shell
add web-admin-sources health-dashboard
add web-admin-sources backfill-query
# Phase 4
add llm-client config-toml
add llm-attr-fallback llm-client
add llm-attr-fallback attr-extractors
add attr-filters-ui web-search-map
add attr-filters-ui attr-wire
add parcels-schema migrations-core
add parcels-schema domain-types
add csv-import parcels-schema
add parcel-matching parcels-schema
add parcel-matching geocode-client
add web-parcel-panel web-listings
add web-parcel-panel parcel-matching
# Phase 5
add comps-query parcels-schema
add comps-query parcel-matching
add web-comps-panel web-parcel-panel
add web-comps-panel comps-query
add montana-import parcels-schema
# Deferred
add ops-deploy "$GVPS"
add ops-backup "$GVPS"
add secrets-config "$GSEC"
add secrets-config geocode-client
add secrets-config llm-client
add scraper-knipe "$GFIX"
add scraper-knipe scraper-fixture-harness
add scraper-idaho-country "$GFIX"
add scraper-idaho-country scraper-fixture-harness
add scraper-fay "$GFIX"
add scraper-fay scraper-fixture-harness
add scraper-whitetail "$GFIX"
add scraper-whitetail scraper-fixture-harness
add scraper-hall "$GFIX"
add scraper-hall scraper-fixture-harness
add scraper-livewater "$GFIX"
add scraper-livewater scraper-fixture-harness
add scraper-acretrader "$GFIX"
add scraper-acretrader auction-ext
add scraper-landwatch "$GFIX"
add scraper-landsofamerica "$GFIX"
add parcel-scraper-ada "$GFIX"
add parcel-scraper-ada parcels-schema
add sales-idaho-beacon "$GFIX"
add sales-idaho-beacon parcels-schema
add sales-idaho-beacon comps-query
add montana-listings "$GFIX"

echo "RESULT ok=$OK dup=$DUP bad=$BAD"
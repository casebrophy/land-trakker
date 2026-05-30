#!/usr/bin/env bash
# Wire all dependency edges. Tolerates duplicates. No set -e.
cd /Users/casebrophy/personal/land_trakker

declare -A ID
while IFS=$'\t' read -r id title; do
  ID["$title"]="$id"
done < /tmp/lt_issues.tsv

# exact-title lookup
t() { local v="${ID[$1]}"; if [[ -z "$v" ]]; then echo "MISSING:$1" >&2; fi; echo "$v"; }
# prefix lookup (for long gate titles)
p() {
  local key id
  while IFS=$'\t' read -r id key; do
    if [[ "$key" == "$1"* ]]; then echo "$id"; return; fi
  done < /tmp/lt_issues.tsv
  echo "MISSING_PREFIX:$1" >&2
}

GSEC=$(p "gate-secrets"); GVPS=$(p "gate-vps"); GFIX=$(p "gate-fixtures")

# dep: child depends on prereq  ->  bd dep add <child> <prereq>
dep() { bd dep add "$1" "$2" 2>&1 | grep -vE '^(✓|Error: dependency already exists)' || true; }

add() { # add <child-title> <prereq-id>
  local c; c=$(t "$1")
  [[ -z "$c" || "$c" == MISSING* || -z "$2" || "$2" == MISSING* ]] && { echo "SKIP $1 -> $2"; return; }
  bd dep add "$c" "$2" >/dev/null 2>&1 && echo "ok: $1 -> $2" || echo "warn(dup?): $1 -> $2"
}

# Phase 0
add "config-toml"        "$(t bootstrap-module)"
add "migrations-core"    "$(t bootstrap-module)"
add "domain-types"       "$(t bootstrap-module)"
add "parser-helpers"     "$(t bootstrap-module)"
add "scraper-iface"      "$(t bootstrap-module)"
add "rate-limiter"       "$(t scraper-iface)"
add "storage-impls"      "$(t migrations-core)"
add "storage-impls"      "$(t domain-types)"
add "listingbus"         "$(t domain-types)"
add "backfill-query"     "$(t storage-impls)"
add "backfill-query"     "$(t listingbus)"
add "orchestrator"       "$(t rate-limiter)"
add "orchestrator"       "$(t listingbus)"
add "orchestrator"       "$(t storage-impls)"
add "fakebroker"         "$(t scraper-iface)"
add "cmd-scraperd"       "$(t orchestrator)"
add "cmd-scraperd"       "$(t fakebroker)"
add "cmd-scrape-once"    "$(t orchestrator)"
add "cmd-scrape-once"    "$(t fakebroker)"
add "web-shell"          "$(t config-toml)"
add "web-listings"       "$(t web-shell)"
add "web-listings"       "$(t listingbus)"
add "p0-e2e-tests"       "$(t cmd-scrape-once)"
add "p0-e2e-tests"       "$(t backfill-query)"
add "p0-e2e-tests"       "$(t fakebroker)"
# Phase 1
add "geocode-client"          "$(t migrations-core)"
add "geocode-client"          "$(t domain-types)"
add "normalize-geocode"       "$(t geocode-client)"
add "normalize-geocode"       "$(t orchestrator)"
add "search-filters"          "$(t listingbus)"
add "search-filters"          "$(t storage-impls)"
add "web-search-map"          "$(t web-listings)"
add "web-search-map"          "$(t search-filters)"
add "price-history-ui"        "$(t web-listings)"
add "price-history-ui"        "$(t listingbus)"
add "scraper-fixture-harness" "$(t parser-helpers)"
add "scraper-fixture-harness" "$(t scraper-iface)"
# Phase 2
add "saved-searches"      "$(t search-filters)"
add "web-saved-searches"  "$(t web-search-map)"
add "web-saved-searches"  "$(t saved-searches)"
add "dedup-job"           "$(t listingbus)"
add "dedup-job"           "$(t geocode-client)"
add "web-duplicates"      "$(t web-listings)"
add "web-duplicates"      "$(t dedup-job)"
add "auction-ext"         "$(t domain-types)"
add "auction-ext"         "$(t migrations-core)"
# Phase 3
add "attr-extractors"     "$(t parser-helpers)"
add "attr-wire"           "$(t attr-extractors)"
add "attr-wire"           "$(t orchestrator)"
add "fulltext-search"     "$(t search-filters)"
add "fulltext-search"     "$(t web-search-map)"
add "health-dashboard"    "$(t web-shell)"
add "health-dashboard"    "$(t listingbus)"
add "logging-tail"        "$(t web-shell)"
add "web-admin-sources"   "$(t health-dashboard)"
add "web-admin-sources"   "$(t backfill-query)"
# Phase 4
add "llm-client"          "$(t config-toml)"
add "llm-attr-fallback"   "$(t llm-client)"
add "llm-attr-fallback"   "$(t attr-extractors)"
add "attr-filters-ui"     "$(t web-search-map)"
add "attr-filters-ui"     "$(t attr-wire)"
add "parcels-schema"      "$(t migrations-core)"
add "parcels-schema"      "$(t domain-types)"
add "csv-import"          "$(t parcels-schema)"
add "parcel-matching"     "$(t parcels-schema)"
add "parcel-matching"     "$(t geocode-client)"
add "web-parcel-panel"    "$(t web-listings)"
add "web-parcel-panel"    "$(t parcel-matching)"
# Phase 5
add "comps-query"         "$(t parcels-schema)"
add "comps-query"         "$(t parcel-matching)"
add "web-comps-panel"     "$(t web-parcel-panel)"
add "web-comps-panel"     "$(t comps-query)"
add "montana-import"      "$(t parcels-schema)"
# Deferred -> gates + build prereqs
add "ops-deploy"            "$GVPS"
add "ops-backup"            "$GVPS"
add "secrets-config"        "$GSEC"
add "secrets-config"        "$(t geocode-client)"
add "secrets-config"        "$(t llm-client)"
add "scraper-knipe"         "$GFIX"
add "scraper-knipe"         "$(t scraper-fixture-harness)"
add "scraper-idaho-country" "$GFIX"
add "scraper-idaho-country" "$(t scraper-fixture-harness)"
add "scraper-fay"           "$GFIX"
add "scraper-fay"           "$(t scraper-fixture-harness)"
add "scraper-whitetail"     "$GFIX"
add "scraper-whitetail"     "$(t scraper-fixture-harness)"
add "scraper-hall"          "$GFIX"
add "scraper-hall"          "$(t scraper-fixture-harness)"
add "scraper-livewater"     "$GFIX"
add "scraper-livewater"     "$(t scraper-fixture-harness)"
add "scraper-acretrader"    "$GFIX"
add "scraper-acretrader"    "$(t auction-ext)"
add "scraper-landwatch"     "$GFIX"
add "scraper-landsofamerica" "$GFIX"
add "parcel-scraper-ada"    "$GFIX"
add "parcel-scraper-ada"    "$(t parcels-schema)"
add "sales-idaho-beacon"    "$GFIX"
add "sales-idaho-beacon"    "$(t parcels-schema)"
add "sales-idaho-beacon"    "$(t comps-query)"
add "montana-listings"      "$GFIX"
echo "ALLDONE"
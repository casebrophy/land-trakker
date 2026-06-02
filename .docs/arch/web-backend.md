# Web Backend Architecture

## Foundation Layer: `foundation/web/`

### Auth & Session Management

**Session** (`session.go`) — HMAC-SHA256 signed token in HttpOnly cookie (name: `lt_session`). Signature uses a shared secret; no session store.

**IsAuthenticated(r, secret)** — validates request's session cookie against `signedToken(secret)`. Returns bool.

**SetSession(w, secret)** → creates signed cookie with SameSite=Lax, HttpOnly=true. No expiry (browser session).

**ClearSession(w)** → sets MaxAge=-1 to delete cookie.

**RequireAuth(secret)** — chi middleware that redirects unauthenticated requests to /login with 303 SeeOther.

### Login Handlers

**LoginHandler(passwordHash, secret)** → GET renders embedded login.html template; POST validates bcrypt password hash against form field, calls SetSession on success with 303 redirect to /, or returns 401 + form on invalid password.

**LogoutHandler()** → calls ClearSession and redirects to /login with 303.

### Listing Handlers

**ListingsQuerier interface** — minimal contract: `QueryListings(ctx, limit, offset) []Listing`, `QueryListingByID(ctx, id) Listing`, `QuerySnapshotsByListing(ctx, id) []ListingSnapshot`, `QueryListingsFilter(ctx, f listing.ListingFilter, limit, offset) []Listing`. Callers inject via dependency.

**ListingsHandler(q)** → GET only. Supports HTMX partial requests (HX-Request header). Parses:
  - Pagination: limit (default 50, max 200), offset query params
  - Filter form: acres_min/acres_max (float), price_min/price_max (int, converted to cents), counties (comma-separated), property_type, attr_water/attr_off_grid/attr_power/attr_well/attr_septic (checkboxes)
  
  Queries paginated listings with parseFilter (empty filter → QueryListings; non-empty → QueryListingsFilter). Builds listingRow objects (ID, Title, Status, PricePerAcre formatted as $X/acre, Acres, Location as City+State, FirstSeenDate formatted YYYY-MM-DD) and mapMarker objects from Geom (Lat, Lng, Title, ID) for Leaflet. Renders listings.html (full page) on initial load or listings_results.html (partial template "results_content") on HTMX requests. Returns 503 if q is nil, 500 on query error.

**ListingDetailHandler(q)** → GET with `{id}` chi URL param. Queries single listing by ID (404 if "no rows"), queries snapshots by ID (500 on error). Builds timelineData from snapshots (date, price, acres) for Chart.js dual-axis timeline. Maps listing fields to HTML (Title, URL, Status, Price formatted $X, Acres, Address as line+city+state). Maps snapshots to rows (CapturedAt formatted YYYY-MM-DD HH:MM, Status, Price, Acres, Diff as comma-joined keys from s.Diff map). Renders listing_detail.html with timeline JSON (serialized as template.JS). Returns 500 on query errors.

### Helpers

**formatCents(cents)** → converts int64 cents to string "$X,XXX" format (dollars with commas, no cents precision).

**addCommas(s)** — utility to insert thousands separators in digit strings.

### Templates

Embedded via `//go:embed templates/`. All templates parsed at package init:
- `login.html` — form with password input, displays {{.Error}} on POST failure
- `listings.html` — full page: search filter form (acres range, price range, counties, property type, 5 boolean attributes) with HTMX; Leaflet map (OSM tile layer, center 44.5/-114.0 zoom 6); results div (swapped by HTMX)
- `listings_results.html` — define "results_content": table of {{.Rows}} with HTMX pagination links; embedded script calls updateMapMarkers({{.Markers}}) to refresh map after swap
- `listing_detail.html` — detail view with {{.ID}}, {{.Title}}, {{.URL}}, {{.Status}}, {{.Price}}, {{.Acres}}, {{.Address}}; Chart.js dual-axis timeline ({{.Timeline}} JSON: points array with date, price, acres); snapshot history table (CapturedAt, Status, Price, Acres, Diff)

---

## API Layer: `cmd/api/`

### Server Setup (`main.go`)

**Config** — loaded from $CONFIG_PATH or land_trakker.toml (defaults: listen ":8080"). Contains Server.Listen, Server.AdminPasswordHash, Server.SessionSecret.

**HTTP Server** — chi router from `newRouter(cfg, q)` with middleware: RealIP, Recoverer. Timeouts: read 15s, write 30s, idle 60s. Graceful shutdown waits 10s on SIGINT/SIGTERM.

**Dependency Injection** — `newRouter` takes cfg and q (ListingsQuerier). q is nil in main.go (intent: injected upstream by harness).

### Route Wiring (`routes.go`)

```
GET  /health                  → HealthHandler()
GET  /login, POST /login      → LoginHandler(cfg.Server.AdminPasswordHash, cfg.Server.SessionSecret)
GET  /logout                  → LogoutHandler()

Group {
  Middleware: RequireAuth(cfg.Server.SessionSecret)
  GET  /                      → ListingsHandler(q)
  GET  /listings/{id}         → ListingDetailHandler(q)
}
```

Public: /health, /login routes. Authenticated group protects / and /listings/{id}.

---

## Data Flow

1. **Unauthenticated request** → RequireAuth redirects to /login
2. **Login POST** → bcrypt compare; on success, SetSession + redirect to /
3. **GET /** (initial load) → ListingsHandler detects no HTMX header; queries listings (empty filter); builds rows + markers; renders full listings.html (map + filter form + results div)
4. **GET /** (HTMX filter/paginate) → ListingsHandler detects HX-Request header; queries with parseFilter(r); builds rows + markers; renders listings_results.html partial "results_content"; browser runs embedded updateMapMarkers(markers) script via htmx:afterSettle event
5. **GET /listings/{id}** → ListingDetailHandler queries Listing + ListingSnapshot history; builds timelineData (points array: date, price, acres); renders listing_detail.html; Chart.js initializes on page load with dual-axis line chart (price on left Y, acres on right Y)
6. **Logout** → ClearSession + redirect to /login

---

## Key Behaviors

1. **Session** is stateless signed token (no DB lookup per request). Secret is shared credential.
2. **Password** only checked at POST /login; subsequent requests validated by cookie signature alone.
3. **Pagination** defaults to 50, max 200 per query param. HTMX requests preserve filter state in URLs (hx-push-url="true").
4. **Filtering** — parseFilter converts URL query params to listing.ListingFilter struct (price in dollars, converted to cents for DB query; acres as float; counties as comma-separated string parsed to []string; booleans set only if "true"). isFilterEmpty checks all fields to decide QueryListings vs. QueryListingsFilter.
5. **Map** — Leaflet with OSM tiles; markers from listing Geom fields (Lat, Lng); HTMX swap event triggers updateMapMarkers(data) to refresh pins and fitBounds around visible listings.
6. **Timeline** — Chart.js dual-axis: price (left Y, blue line) and acres (right Y, green line) over snapshot dates. Price values formatted as $M/$K in ticks; null/zero prices filtered out per snapshot.
7. **Formatting** — prices in cents stored in domain, rendered as "$X,XXX" in UI; acres as float 2 decimals; dates as "2006-01-02" or "2006-01-02 15:04".
8. **Snapshots** show field-level Diff map (e.g., `{"price_cents": {"old": 500000, "new": 490000}}`); UI renders keys as comma-joined string.
9. **Nil checks** on optional fields (Title, City, State, Price, Acres, Address, Geom) before use; safe fallbacks ("n/a", "(untitled)").
10. **HTMX integration** — filter form uses hx-get + hx-target="#results" + hx-push-url="true"; pagination links also HTMX-enabled; htmx:afterSettle event handler runs embedded script to update map markers.

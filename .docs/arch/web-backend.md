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

**ListingsQuerier interface** — minimal contract: `QueryListings(ctx, limit, offset) []Listing`, `QueryListingByID(ctx, id) Listing`, `QuerySnapshotsByListing(ctx, id) []ListingSnapshot`. Callers inject via dependency.

**ListingsHandler(q)** → GET only. Parses limit (default 50, max 200) and offset query params. Queries paginated listings, maps to HTML rows (ID, Title, Status, PricePerAcre formatted as $X/acre, Acres, Location as City+State, FirstSeenDate formatted YYYY-MM-DD). Renders listings.html. Returns 503 if q is nil, 500 on query error.

**ListingDetailHandler(q)** → GET with `{id}` chi URL param. Queries single listing by ID (404 if "no rows"), queries snapshots by ID (500 on error). Maps listing fields to HTML (Title, URL, Status, Price formatted $X, Acres, Address as line+city+state). Maps snapshots to rows (CapturedAt formatted YYYY-MM-DD HH:MM, Status, Price, Acres, Diff as comma-joined keys). Renders listing_detail.html. Returns 500 on query errors.

### Helpers

**formatCents(cents)** → converts int64 cents to string "$X,XXX" format (dollars with commas, no cents precision).

**addCommas(s)** — utility to insert thousands separators in digit strings.

### Templates

Embedded via `//go:embed templates/`. All templates parsed at package init:
- `login.html` — form with password input, displays {{.Error}} on POST failure
- `listings.html` — table of {{.Rows}}, each row: ID (link to detail), Title, Status, PricePerAcre, Acres, Location, FirstSeenDate
- `listing_detail.html` — detail view with {{.ID}}, {{.Title}}, {{.URL}}, {{.Status}}, {{.Price}}, {{.Acres}}, {{.Address}}, table of {{.Snaps}} with columns: CapturedAt, Status, Price, Acres, Diff

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
3. **GET /** → ListingsHandler queries business/sdk/listingbus (Storer-backed) for paginated Listing rows; formats and renders
4. **GET /listings/{id}** → ListingDetailHandler queries Listing + ListingSnapshot history; formats with price diffs; renders detail template
5. **Logout** → ClearSession + redirect to /login

---

## Key Behaviors

1. **Session** is stateless signed token (no DB lookup per request). Secret is shared credential.
2. **Password** only checked at POST /login; subsequent requests validated by cookie signature alone.
3. **Pagination** defaults to 50, max 200 per query param.
4. **Formatting** — prices in cents stored in domain, rendered as "$X,XXX" in UI; acres as float 2 decimals.
5. **Snapshots** show field-level Diff map (e.g., `{"price_cents": {"old": 500000, "new": 490000}}`); UI renders as comma-joined keys.
6. **Nil checks** on optional fields (Title, City, State, Price, Acres) before use; safe fallbacks ("n/a", "(untitled)").

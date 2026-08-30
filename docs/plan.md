# Habit Tracker - Implementation Plan

## 1. Goal
Logiciel de suivi d'habitudes mono-utilisateur, desktop + mobile + PWA, Go + SQLite (no CGo) backend, React frontend, import/export CSV, table 3 derniers jours, calendrier mensuel heatmap, édition inline.

Workspace: `/home/loris/Téléchargements/habit-go` (currently empty, greenfield)

## 2. Decisions & Constraints (validated with user)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Frontend | **Vite + React + TypeScript + React Router + Tailwind + vite-plugin-pwa** | User choice. SPA embedded in Go binary via `go:embed`. `vite-plugin-pwa` with Workbox for shell offline. |
| Backend | Go `net/http` + `modernc.org/sqlite` (pure Go, no CGo) + `mattn` is prohibited | No CGo constraint |
| Auth | Mono-utilisateur sans auth, DB locale unique | V1 simplification |
| Negative habit | `is_negative bool`; success = `value <= goal_value` (seuil max). Positive: `value >= goal_value`. Color inversion. | User confirmed |
| Goal period | Daily / Weekly / Monthly. **Fenêtre glissante**: daily = jour J, weekly = 7 derniers jours, monthly = 30 derniers jours. Cumul: count `yes` for `boolean`, sum for `numerical`. | User choice glissante vs ISO |
| Main table interaction | Click = toggle yes/no OR +1 numerical, long-press / right-click = -1 (clamped >=0), frontend `onContextMenu` + `onLongPress` (500ms timer). Edit fine via habit page if needed. | User confirmed |
| Calendar | Heatmap couleur: intensity based on `value/goal`, vert si success else rouge/orange, gris si no entry, inversé si `is_negative`. Tooltip value. Month nav `< >` + Today. | User confirmed |
| CSV | 2 files separés: `habits.csv` + `entries.csv`, upsert idempotent on `habit_name/date` | User confirmed |
| Groups | Free text `habit.group`, frontend groups alpha, habits alpha inside, collapsible sections | User confirmed |
| PWA | Shell offline-first (Workbox `precache` + `NetworkFirst` for `/api/*`), data requires network, offline banner | User confirmed |
| Delete | **Soft delete (archiver)** `archived_at` nullable, filter `archived=0` default, toggle archive/restore, entries kept | User chose soft vs hard |
| Dates | `YYYY-MM-DD` locale navigateur, stored TEXT, backend treats as opaque date string, frontend sends `toLocaleDateString('en-CA')` | Inferred, to confirm |
| Port / serving | Backend `:8080`, `GET /` serves `frontend/dist`, `GET /api/*` JSON, fallback `index.html` for SPA router | Convention |

## 3. Architecture

```
habit-go/
├── go.mod
├── main.go              // http server, embed.FS for frontend/dist, sqlite init
├── internal/
│   ├── db/sqlite.go     // open, migrate
│   ├── habit/           // model, repository, handler
│   ├── entry/           // model, repository, handler
│   └── csv/             // import/export logic
├── frontend/
│   ├── index.html
│   ├── vite.config.ts   // vite-plugin-pwa config
│   ├── tailwind.config.js
│   ├── src/
│   │   ├── main.tsx, App.tsx, router.tsx
│   │   ├── pages/MainPage.tsx, HabitPage.tsx, HabitFormPage.tsx
│   │   ├── components/HabitTable.tsx, Calendar.tsx, HeatmapCell.tsx
│   │   ├── hooks/useHabits.ts, api.ts
│   │   └── pwa.ts
│   └── public/manifest, icons
└── data/habits.db       // sqlite file (gitignored)
```

Single binary deployment: `go build -o habit-go` embeds `frontend/dist`.

## 4. Data Model (SQLite)

**habits**
```sql
CREATE TABLE habits (
  id TEXT PRIMARY KEY, -- uuid v4
  name TEXT NOT NULL,
  group_name TEXT NOT NULL DEFAULT '',
  type TEXT NOT NULL CHECK(type IN ('boolean','numerical')),
  goal_value REAL NOT NULL,
  goal_period TEXT NOT NULL CHECK(goal_period IN ('daily','weekly','monthly')),
  is_negative INTEGER NOT NULL DEFAULT 0,
  archived_at TEXT, -- ISO8601 nullable, soft delete
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  updated_at TEXT NOT NULL
);
CREATE INDEX idx_habits_archived ON habits(archived_at);
CREATE INDEX idx_habits_group ON habits(group_name);
```

**entries**
```sql
CREATE TABLE entries (
  id TEXT PRIMARY KEY,
  habit_id TEXT NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
  date TEXT NOT NULL, -- YYYY-MM-DD locale
  value REAL NOT NULL, -- 0/1 for boolean, number for numerical
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(habit_id, date)
);
CREATE INDEX idx_entries_habit_date ON entries(habit_id, date);
CREATE INDEX idx_entries_date ON entries(date);
```

Note: `value` REAL to support numerical; boolean uses 0/1.

Migrations: `golang-migrate` style or simple `CREATE TABLE IF NOT EXISTS` + version table.

## 5. API Design (JSON)

Base `/api`

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/habits?include_archived=false | List, grouped frontend |
| POST | /api/habits | Create |
| GET | /api/habits/:id | Get one + computed progress (weekly/monthly rolling) |
| PUT | /api/habits/:id | Update (name, group, type, goal, is_negative) |
| POST | /api/habits/:id/archive | Soft delete (set archived_at) |
| POST | /api/habits/:id/restore | Unarchive |
| DELETE | /api/habits/:id?hard=true | Optional hard delete (cascade) admin |
| GET | /api/habits/:id/entries?from=YYYY-MM-DD&to=YYYY-MM-DD | Range query for calendar |
| PUT | /api/habits/:id/entries/:date | Upsert entry `{"value": number}` |
| DELETE | /api/habits/:id/entries/:date | Clear entry |
| GET | /api/entries?from=&to=&habit_id= | Bulk for main page last 3 days |
| GET | /api/export/habits | CSV download |
| GET | /api/export/entries | CSV download |
| POST | /api/import/habits | multipart csv upload |
| POST | /api/import/entries | multipart csv upload |
| GET | /api/health | liveness |

**Main page optimization**: `GET /api/habits?with_entries=3` returns habits with `recent_entries: [{date,value},...last 3 days]` to avoid N+1.

**Entry upsert logic**: `INSERT OR REPLACE` with txn.

**Validation**: `name` required, `group_name` allow empty => displayed "Sans groupe", `goal_value` >0, `type` immutable after creation? Allow but warn (would convert entries).

## 6. Frontend Spec

**Routes**
- `/` MainPage
- `/habits/new` HabitFormPage
- `/habits/:id` HabitPage (calendar + edit button)
- `/habits/:id/edit` HabitFormPage (edit mode)

**MainPage (`pages/MainPage.tsx`)**
- Fetch habits + last 3 days entries (today = local date)
- Group by `group_name` (empty => "Autres"), sort alpha groups, alpha habits
- Table: columns `Habit | Aujourd'hui | Hier | J-2`, header sticky
- Cell: for boolean: 0= `-` gris, 1= `✓` vert (rouge if negative & 1>goal?), for numerical: show number
- Click handler: `PUT /api/habits/:id/entries/:date` toggle or +1; optimistic update
- Long-press: -1 clamped 0, `preventDefault` on contextMenu
- Row click habit name => navigate `/habits/:id`
- Button "Nouvelle habitude" => `/habits/new`
- Import/Export buttons toolbar (download csv, upload input)
- Filter: show archived toggle

**HabitFormPage**
- Fields: name text, group text (datalist existing groups), type select boolean/numerical, goal_value number, goal_period select, is_negative checkbox
- Validation zod/react-hook-form
- Submit POST/PUT then navigate /

**HabitPage**
- Header habit name + group + goal summary + Edit button + Archive button
- Stats: rolling progress for goal_period (e.g., "5/7 cette semaine" or "12/30 ce mois")
- Calendar: grid 7x5/6, month nav, cells colored heatmap:
  - Compute `intensity = min(value/goal,1)` positive, `value<=goal ? 0.3 : 1` negative
  - Colors: `bg-gray-100` empty, `bg-green-200..500` success, `bg-red-200..500` fail/negative exceed
- Click date => inline popover/modal `EntryEditor`: for boolean toggle switch, for numerical input number + +/- buttons, Save => PUT, Delete => clear
- Fetch `GET /api/habits/:id/entries?from=YYYY-MM-01&to=YYYY-MM-31`

**PWA**
- `vite-plugin-pwa` config: `registerType: 'autoUpdate'`, manifest name `Habit Tracker`, themeColor, icons 192/512, display standalone
- Workbox runtimeCaching: `NetworkFirst` for `/api/*` with 10s timeout, `CacheFirst` for assets
- Offline banner component checking `navigator.onLine` + fetch failure

**Styling**: Tailwind, responsive table scroll horizontal on mobile, bottom nav.

## 7. CSV Spec

**habits.csv header**: `id,name,group_name,type,goal_value,goal_period,is_negative,archived_at,created_at`
Export all (including archived). Import: if `id` exists update, else if `name` matches existing -> update, else create new uuid if empty. Upsert.

**entries.csv header**: `habit_id,habit_name,date,value`
`habit_id` may be empty if `habit_name` provided. Resolver: prefer `habit_id` else lookup by `name`. Date `YYYY-MM-DD`, value numeric. Import upsert `UNIQUE(habit_id,date)`, skip invalid dates.

Endpoints set `Content-Disposition: attachment; filename=*.csv`, `Content-Type: text/csv`.

## 8. Implementation Task List (ordered)

1. **Init Go project**: `go mod init habit-go`, add `modernc.org/sqlite` v1.3x, `github.com/google/uuid`, setup `main.go` with `database/sql`, migrations, `http.ServeMux`, CORS, logging, `go:embed frontend/dist`.
2. **DB layer**: `internal/db/db.go` open `data/habits.db`, WAL mode, foreign_keys=on, migrate 2 tables.
3. **Habit domain**: model struct, repository (List/Create/Get/Update/Archive/Restore), handler, validation, routes. Unit tests for repository with temp sqlite.
4. **Entry domain**: model, repo Upsert/GetRange/Delete/BulkLast3Days, handler, route `PUT /api/habits/:id/entries/:date`. Handle boolean 0/1 coercion.
5. **Main page API optimization**: `with_entries` query param join, return habits + recent_entries in one call.
6. **CSV import/export**: handlers streaming `encoding/csv`, export writer, import reader with txn, upsert logic, error report JSON.
7. **Static serving & SPA fallback**: embed FS, serve `/` and `/assets/*`, fallback to index.html, 404 for `/api`.
8. **Init frontend**: `npm create vite@latest frontend -- --template react-ts`, add `tailwind`, `react-router-dom`, `vite-plugin-pwa`, `date-fns`, `clsx`.
9. **API client**: `src/api.ts` fetch wrapper, types `Habit`, `Entry`, error handling.
10. **MainPage + HabitTable**: implement grouping, sorting, 3-day columns, click +1 / toggle, long-press -1, optimistic updates, loading states.
11. **HabitFormPage**: form with validation, create/edit, group datalist, negative toggle helper text "Seuil max à ne pas dépasser".
12. **HabitPage + Calendar**: month navigation, heatmap cells, intensity logic, entry editor modal, rolling progress calc (7d/30d window via frontend calc from fetched entries or backend computed field).
13. **PWA**: configure `vite-plugin-pwa` manifest/icons, workbox, test `npm run build` + `go:embed`, lighthouse check, service worker autoUpdate.
14. **Responsive & polish**: mobile table horizontal scroll, collapsible groups, archived filter, confirm dialogs, toasts, offline banner, empty states.
15. **Build & run scripts**: `Makefile` with `make dev` (run go + vite proxy), `make build` (vite build then go build), `make test`, README with `go run .` usage, sqlite location env `DB_PATH`/`PORT`.
16. **Validation**: e2e sanity: create boolean + numerical habits, test today/yesterday edits, archive, csv roundtrip, calendar month nav, PWA install, mobile viewport.

## 9. Risks & Mitigations

- **modernc.org/sqlite size/performance**: large pure-Go binary (~20MB), slower than CGo but acceptable for local single-user; mitigate with WAL + indexes, limit N+1 via bulk endpoint.
- **Date locale vs server**: storing `YYYY-MM-DD` locale avoids TZ shifts; frontend must use `en-CA` local date, never `toISOString()`; backend treats dates as strings, no conversion.
- **Numerical long-press discoverability**: add tooltip "Clic +1, appui long -1" and visible +/- in HabitPage editor; test on touch devices.
- **Type change habit**: changing `boolean` <-> `numerical` leaves legacy entries type-mismatch; either forbid type change after creation or coerce values on read.
- **CSV habit_name collision**: duplicate names not enforced unique; import will lookup first match; mitigate by preferring `id` and warning on duplicate names.
- **Soft delete accumulation**: archived habits hidden but still exported; need "Voir archivées" toggle and eventual hard purge option.
- **PWA cache stale API**: `NetworkFirst` prevents serving stale entries; shell `CacheFirst` ensures installable offline but data still needs network (as agreed).

## 10. Validation Plan

- **Backend unit**: `go test ./...` for habit/entry repo (crud, unique date, soft delete filter, rolling sum calc)
- **API manual**: curl tests for all endpoints, csv curl download/upload roundtrip
- **Frontend**: Vite build succeeds, `vite preview` PWA, Lighthouse PWA score >90, responsive Chrome devtools 360px & 1280px
- **Functional checklist**:
  - Create habit boolean positive daily 1, numerical negative weekly 10
  - Main page shows 3 columns, click toggle/+1 works, long-press -1 works, page refresh persists
  - Group sections sorted alpha, new group appears
  - HabitPage calendar heatmap reflects values, month nav, click date edit saves
  - Edit habit changes goal/negative and heatmap colors update
  - Archive restores/hides
  - Export both csv, delete DB, import restores
  - PWA install prompt, offline shell loads, online banner when fetch fails

## 11. Out of Scope (V1)

- Auth/multi-user, JWT, per-user isolation
- Notifications/reminders push
- Charts/graphs beyond heatmap
- Full offline write queue + Background Sync + IndexedDB
- Drag & drop ordering, custom group order
- i18n (French UI assumed), dark mode
- Docker image (could be added later)

## 12. Open Questions for Implementation

- Confirm date locale handling: store `YYYY-MM-DD` local (browser) acceptable vs UTC? (reco local)
- Forbid `type` change on edit if entries exist? (reco forbid or warn)
- Need `archived_at` filter default `false` and settings page to show archived?

---
Plan ready for build agent. No source changes made per plan mode.

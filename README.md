# Habit Tracker — habit-go

Mono-utilisateur, Go + SQLite (sans CGo) + React + PWA. Suivi d'habitudes avec table 3 derniers jours, calendrier heatmap, import/export CSV, archivage et i18n FR/EN.

![logo](frontend/public/logo.svg)

## Stack
- Backend: Go `net/http`, `modernc.org/sqlite` (pure Go), `go:embed` pour servir le frontend, `time.Time` en `2006-01-02`
- Frontend: Vite + React + TypeScript + React Router + Tailwind + `vite-plugin-pwa` (Workbox)
- DB: SQLite `data/habits.db` (WAL, `foreign_keys=ON`), deux tables `habits` / `entries` avec index `archived_at` et `UNIQUE(habit_id,date)`
- Logo: `frontend/public/logo.svg` (dégradé `#22c55e→#16a34a`, coche + pastille) décliné en `pwa-512x512.png` / `pwa-192x192.png` / `apple-touch-icon.png` via ImageMagick (fidélité gradient+shadow), `favicon-32.png`

## Prérequis
- Go 1.27+ (module `habit-go`, `GOTOOLCHAIN=auto`)
- Node 24+ / npm 10+ (Vite 8, Tailwind 4, TypeScript 7)

## Lancement rapide

```bash
# 1. Installer
make install

# 2. Dev (backend :8080 + frontend :5173 avec proxy /api)
make dev
#   => http://localhost:5173  (Vite)
#   => http://localhost:8080/api/health  (API)

# 3. Build prod (Vite -> frontend/dist puis binaire Go avec embed)
make build
# 4. Run prod binaire seul (sert le frontend intégré)
PORT=8080 DB_PATH=data/habits.db ./habit-go
#   => http://localhost:8080
```

Variables d'environnement:
- `PORT` (défaut `8080`)
- `DB_PATH` (défaut `data/habits.db`)
- `PASSWORD` / `APP_PASSWORD` / `HABIT_PASSWORD` (si défini, protège `/api/*` par `sha256` + cookie `auth_token` + header `Authorization: Bearer`)

## API
Base `/api`:

| Méthode | Path | Description |
|---------|------|-------------|
| GET | /api/health | liveness |
| GET | /api/habits?include_archived=&with_entries=3&today=YYYY-MM-DD | liste + `recent_entries` optimisé + `progress` rolling |
| POST | /api/habits | création |
| GET | /api/habits/:id?today=YYYY-MM-DD | détail + `progress` rolling |
| PUT | /api/habits/:id | update |
| POST | /api/habits/:id/archive | soft delete |
| POST | /api/habits/:id/restore | unarchive |
| DELETE | /api/habits/:id?hard=true | hard delete |
| GET | /api/habits/:id/entries?from=&to= | range calendrier |
| PUT | /api/habits/:id/entries/:date | upsert `{"value": number}` |
| DELETE | /api/habits/:id/entries/:date | clear |
| GET | /api/entries?from=&to=&habit_id= | bulk |
| POST | /api/login / /api/logout / /api/auth/status | auth |
| GET | /api/export/habits | habits.csv |
| GET | /api/export/entries | entries.csv |
| POST | /api/import/habits | multipart csv |
| POST | /api/import/entries | multipart csv |

Dates `YYYY-MM-DD` en locale navigateur (`localDateStr` DST-safe `12h`), stockées TEXT opaque. **`?today`** envoyé par le frontend (`localDateStr(new Date())`) évite divergence TZ serveur vs navigateur pour `with_entries` et `progress`. Frontend fallback `time.Now()` si param absent.

`goal_period` glissante: `daily`=1j, `weekly`=7j, `monthly`=30j. Cumul: `count(yes)` pour `boolean`, `sum` pour `numerical`. `is_negative`: succès si `value <= goal`. `is_negative+boolean` spécialisé: `0`=vert `bg-green-200`, `>=1`=rouge `bg-red-500` (au lieu de `value<=goal` toujours vert). `percentage = current/goal*100` affiché `100% (+X%)` si overflow >100%, barre claquée à 100%.

## Frontend

### Routes
- `/` MainPage: groupes alpha, habits alpha, colonnes Aujourd'hui/Hier/J-2, **cases remplies** via `getHabitIntensity()` (même `bg-green-500/300/200/100` gradué que Calendar, rouge si `is_negative` dépassé), `ring-2 ring-green-400` sur `today`, clic toggle/+1, appui long 500ms / clic droit / clavier `Enter/Space` + `+/-` / `ArrowUp/Down` (-1), `role=grid` + `aria-pressed` + `focus-visible:ring`, **progression globale en 3 cartes** `Quotidienne (Daily)` / `Hebdomadaire (Weekly)` / `Mensuelle (Monthly)` (`successRate` + `avg %` + `cumul current/target`), état vide `opacity-60` + CTA `Créer` avec contraste dark mode
- `/habits/new` & `/habits/:id/edit` HabitFormPage: datalist groupes, validation `name`/`goal_value>0`, `t('dailyLong')` etc.
- `/habits/:id` HabitPage: header + progression rolling (`100% (+X%)` + barre), Calendar heatmap (7 colonnes Lun→Dim, `border` + `hover`), modal édition, fetch unique wide `60j back / 30j fwd` avec `cancelled` guard (fix double fetch)
- `/admin` AdminPage: **sélecteur langue** `FR/EN` (`localStorage app_lang` + `document.documentElement.lang`), Export `habits.csv`/`entries.csv` (`downloadCsv` blob), Import multipart 10MiB, `errors: string[]` (plus de `join` côté `api.ts`), previews `details`
- `LoginPage` + `OfflineBanner` traduits

### i18n
`frontend/src/i18n.tsx` `LanguageProvider` (`fr` défaut, `en` alternative). Dictionnaires `fr`/`en` complets (`habit`, `today`, `admin`, `negativeHabit`…), `t(key)` fallback `fr` + warn `DEV` si manquant. `formatMonth(d, lang)` et `Calendar` `toLocaleDateString(lang==='en'?'en-US':'fr-FR')`. `habit.goal_period` et `habit.progress.period` traduits via `t(daily|weekly|monthly)`. `<html lang>` synchronisé.

### Intensité & progression
`frontend/src/lib/habitIntensity.ts` **helper partagé** `getHabitIntensity(habit,value)` + `formatProgressPct(p)` utilisé par `Calendar` et `HabitTable` (DRY). `MainPage`/`HabitTable`/`HabitPage` clamp `percentage` à `100% (+X%)` avec `title` valeur réelle.

### HabitTable
`HabitTable.tsx` : `Link block` + `mt-1.5` badge seul `current ≤/ target ✓/✗ • pct` (`bg-green-100`/`bg-red-100` + `dark:`), suppression ligne `goal_value / period` (ne garde que badge), `sortedGroups` `useMemo`, `CellButton` `getHabitIntensity` + `ring` today, `dark:bg-gray-800` sticky.

### PWA
`vite-plugin-pwa` `autoUpdate`, manifest `Habit Tracker` `short_name Habits`, `theme_color #22c55e`, icons `pwa-192/512` + `logo.svg` `any`, `includeAssets` `logo.svg`/`favicon-32.png`/`apple-touch-icon.png`, `NetworkFirst` `/api/*` 10s, `CacheFirst` assets 30j, banner offline `t('offline')`. `index.html` `theme-color`, `description`, `icon` `logo.svg` + `favicon-32.png` + `apple-touch-icon`.

### Perf
`HabitTable` `useMemo` groupes, `MainPage` `useEffect` `cancelled` guard (évite race `optimisticUpdate` + `refreshSilent`), `HabitPage` single `useEffect` wide range au lieu de double fetch month+wide.

## CSV
- `habits.csv`: `id,name,group_name,type,goal_value,goal_period,is_negative,unit,archived_at,created_at` — upsert par `id` sinon `name` (unit optionnel, ex: `km`, `L`, `pages`)
- `entries.csv`: `habit_id,habit_name,date,value` — résout `habit_id` ou lookup `habit_name`, upsert `UNIQUE(habit_id,date)`

## Scripts Make
```
make install  # npm install + go mod download
make dev      # go run + vite dev (concurrent)
make build    # frontend build (tsc -b && vite build) + go build -o habit-go (embed dist)
make run      # build puis ./habit-go
make test     # go test + go vet + vite build
make clean    # rm binaire + frontend/dist + data/*.db*
make vet / fmt
```

## Tests
```bash
go test ./...      # repos habit/entry avec sqlite temp
go vet ./...
curl http://localhost:8080/api/health
curl "http://localhost:8080/api/habits?with_entries=3&today=$(date +%F)"
```

Checklist fonctionnelle (plan §10): booléen/numérique + positif/négatif + daily/weekly/monthly, toggle +1/-1 persistant (optimistic + revert), groupes triés, heatmap couleurs (is_negative inversé, boolean fix), édition goal, archive/restore, export/import roundtrip (`errors[]`), progression globale 3 périodes + badge unique, cases remplies, i18n FR/EN persistant, PWA `vite preview` Lighthouse >90, mobile 360px / desktop 1280px, a11y `gridcell` + clavier.

## Déploiement
Binaire unique `habit-go` (~16MB, pure Go) embarque `frontend/dist` via `embed.FS`. Pas de CGo, `data/` gitignoré.

## Docker
```bash
# Build & run avec Docker (multi-stage cache: go mod download + npm ci)
docker build -t habit-go .
docker run -p 8080:8080 -v ./data:/app/data -e PASSWORD=test habit-go

# ou avec compose (prod)
PASSWORD=test PORT=8080 docker compose up --build -d
# => http://localhost:8080
# logs
docker compose logs -f
docker compose down
```
Compose monte `./data` en volume, expose `8080`, lit `PASSWORD`/`PORT`/`DB_PATH` depuis l'env. Healthcheck `wget --spider http://localhost:${PORT:-8080}/api/health`. Frontend builder `node:24-alpine` (`npm ci` → `npm run build`), Go builder `golang:1.27-alpine` (`CGO_ENABLED=0`).

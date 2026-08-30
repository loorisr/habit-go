# Habit Tracker — habit-go

Mono-utilisateur, Go + SQLite (sans CGo) + React + PWA. Suivi d'habitudes avec table 3 derniers jours, calendrier heatmap, import/export CSV et archivage.

## Stack
- Backend: Go `net/http`, `modernc.org/sqlite` (pure Go), `go:embed` pour servir le frontend
- Frontend: Vite + React + TypeScript + React Router + Tailwind + `vite-plugin-pwa` (Workbox)
- DB: SQLite `data/habits.db` (WAL, `foreign_keys=ON`), deux tables `habits` / `entries`

## Prérequis
- Go 1.21+ (module `habit-go`)
- Node 18+ / npm 9+

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

## API
Base `/api`:

| Méthode | Path | Description |
|---------|------|-------------|
| GET | /api/health | liveness |
| GET | /api/habits?include_archived=&with_entries=3 | liste + `recent_entries` optimisé |
| POST | /api/habits | création |
| GET | /api/habits/:id | détail + `progress` rolling |
| PUT | /api/habits/:id | update |
| POST | /api/habits/:id/archive | soft delete |
| POST | /api/habits/:id/restore | unarchive |
| DELETE | /api/habits/:id?hard=true | hard delete |
| GET | /api/habits/:id/entries?from=&to= | range calendrier |
| PUT | /api/habits/:id/entries/:date | upsert `{"value": number}` |
| DELETE | /api/habits/:id/entries/:date | clear |
| GET | /api/entries?from=&to=&habit_id= | bulk |
| GET | /api/export/habits | habits.csv |
| GET | /api/export/entries | entries.csv |
| POST | /api/import/habits | multipart csv |
| POST | /api/import/entries | multipart csv |

Dates `YYYY-MM-DD` en locale navigateur (`toLocaleDateString('en-CA')`), stockées TEXT opaque côté backend.

`goal_period` glissante: `daily`=J, `weekly`=7j, `monthly`=30j. Cumul: `count(yes)` pour `boolean`, `sum` pour `numerical`. `is_negative`: succès si `value <= goal`.

## Frontend
- `/` MainPage: groupes alpha, habits alpha, colonnes Aujourd'hui/Hier/J-2, clic toggle/+1, appui long / clic droit -1, import/export CSV, filtre archivées
- `/habits/new` & `/habits/:id/edit` HabitFormPage: datalist groupes, validation `name`/`goal_value>0`
- `/habits/:id` HabitPage: header + progression rolling, Calendar heatmap (vert succès / rouge échec / gris vide, inversé si négative), modal édition

PWA: `vite-plugin-pwa` `autoUpdate`, manifest `Habit Tracker`, `NetworkFirst` pour `/api/*` (10s), `CacheFirst` pour assets, banner offline si `navigator.onLine === false`.

## CSV
- `habits.csv`: `id,name,group_name,type,goal_value,goal_period,is_negative,unit,archived_at,created_at` — upsert par `id` sinon `name` (unit optionnel, ex: `km`, `L`, `pages`)
- `entries.csv`: `habit_id,habit_name,date,value` — résout `habit_id` ou lookup `habit_name`, upsert `UNIQUE(habit_id,date)`

## Scripts Make
```
make install  # npm install + go mod download
make dev      # go run + vite dev (concurrent)
make build    # frontend build + go build -o habit-go
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
curl http://localhost:8080/api/habits?with_entries=3
```

Checklist fonctionnelle (plan §10): booléen/numerique + positif/négatif + daily/weekly/monthly, toggle +1/-1 persistant, groupes triés, heatmap couleurs, édition goal, archive/restore, export/import roundtrip, PWA `vite preview` Lighthouse >90, mobile 360px / desktop 1280px.

## Déploiement
Binaire unique `habit-go` (~16MB, pure Go) embarque `frontend/dist` via `embed.FS`. Pas de CGo, `data/` gitignoré.

## Docker
```bash
# Build & run avec Docker
docker build -t habit-go .
docker run -p 8080:8080 -v ./data:/app/data -e PASSWORD=test habit-go

# ou avec compose (prod)
PASSWORD=test PORT=8080 docker compose up --build -d
# => http://localhost:8080
# logs
docker compose logs -f
docker compose down
```
Compose monte `./data` en volume, expose `8080`, lit `PASSWORD`/`PORT`/`DB_PATH` depuis l'env. Healthcheck sur `/api/health`.

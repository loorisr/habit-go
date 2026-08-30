# Format d'import CSV — habit-go

Ce document décrit le format attendu pour les deux fichiers d'import : `habits.csv` et `entries.csv`.
Les exports (`GET /api/export/habits` et `GET /api/export/entries`) génèrent exactement ce format, ce qui permet un roundtrip export → édition → ré-import.

> Encodage : UTF-8, séparateur `,` (RFC4180 via `encoding/csv` Go). Ligne d'en-tête obligatoire. L'ordre des colonnes n'est pas important (détection par nom insensible à la casse, `strings.TrimSpace`). Les colonnes inconnues sont ignorées.

---

## 1. `habits.csv` — Habitudes

### En-tête

```
id,name,group_name,type,goal_value,goal_period,is_negative,unit,archived_at,created_at
```

| Colonne | Requis | Type / Valeurs | Défaut | Description |
|---------|--------|----------------|--------|-------------|
| `id` | non | UUID `string` (ex: `550e8400-e29b-41d4-a716-446655440000`) | auto `uuid.NewString()` si vide | Identifiant primaire. Si fourni et trouvé en DB → **update**. Sinon lookup par `name`. |
| `name` | **oui** | `string` non vide, unique recommandé | — | Nom de l'habitude. Si `id` absent ou inconnu et un habit avec ce `name` existe → **update** de ce habit (upsert par nom). |
| `group_name` | non | `string` | `""` → affiché « Sans groupe » | Groupe libre, tri alpha côté frontend. |
| `type` | non | `boolean` ou `numerical` | `boolean` | `boolean` = oui/non (valeur 0/1), `numerical` = compteur. |
| `goal_value` | non | `float >0` | `1` | Objectif à atteindre. Doit être >0 côté UI mais l'import accepte `0` et le remplace par `1` si parse échoue. |
| `goal_period` | non | `daily` / `weekly` / `monthly` | `daily` | Fenêtre glissante : `daily`=jour J, `weekly`=7 derniers jours, `monthly`=30 derniers jours. |
| `is_negative` | non | `0`/`1` ou `true`/`false` (insensible casse) | `0` (`false`) | `1`/`true` = habitude négative : succès si `valeur ≤ objectif` (seuil max). Ex: cigarettes ≤2/jour. `0` = succès si `valeur ≥ objectif`. |
| `unit` | non | `string` | `""` | Unité pour habitudes `numerical` (ex: `km`, `pages`, `L`, `verres`, `min`). Vide pour `boolean`. Affichée sur main page et page détail. |
| `archived_at` | non | `RFC3339` ou vide | `NULL` | Si non vide, l'habitude est importée archivée. Vide = active. Ex: `2025-08-10T14:00:00Z`. |
| `created_at` | non | `RFC3339` | `now()` UTC | Date de création. Si vide, `time.Now().UTC().Format(time.RFC3339)`. |
| `updated_at` | — | auto | — | Non lu à l'import : toujours écrasé par `now()` UTC. |

### Règle d'upsert

```
1. Si id != "" et id existe en DB → UPDATE WHERE id=?
2. Sinon si name existe en DB → UPDATE WHERE name=? (premier match)
3. Sinon → INSERT avec id fourni ou nouveau UUID
```

> Attention : si plusieurs habitudes ont le même `name`, l'import par `name` met à jour la première trouvée. Préférez toujours exporter puis ré-importer avec les `id`.

### Exemple `habits.csv`

```csv
id,name,group_name,type,goal_value,goal_period,is_negative,unit,archived_at,created_at
550e8400-e29b-41d4-a716-446655440000,Méditation,Bien-être,boolean,1,daily,0,,,2025-01-01T08:00:00Z
,,Lecture,numerical,20,daily,0,pages,,2025-01-02T08:00:00Z
abc123,Sport,Santé,numerical,3,weekly,0,km,,2025-01-03T08:00:00Z
def456,Cigarettes,Santé,numerical,2,daily,1,verres,,2025-01-03T08:00:00Z
def789,Eau,Santé,numerical,2,daily,0,L,,2025-01-03T08:00:00Z
```

- Ligne 1 : update/création avec id fixe.
- Ligne 2 : id vide → nouvel UUID généré, groupe `Lecture`.
- Ligne 4 : habitude négative (`is_negative=1`) : objectif max 2/jour → progression affichée `≤` (ex: `1 ≤ 2 ✓`).

### Import via API

```bash
# avec token si protection par mot de passe activée
curl -X POST http://localhost:8080/api/import/habits \
  -H "Authorization: Bearer <token>" \
  -F file=@habits.csv

# réponse JSON
{"imported":4,"errors":""}
```

Formats acceptés : `multipart/form-data` champ `file` **ou** body brut `text/csv` (fallback `r.Body`).

---

## 2. `entries.csv` — Entrées journalières

### En-tête

```
habit_id,habit_name,date,value
```

| Colonne | Requis | Type / Valeurs | Description |
|---------|--------|----------------|-------------|
| `habit_id` | **conditionnel** | UUID `string` | Id de l'habitude. Si vide, résolution par `habit_name`. Au moins un des deux doit être fourni. |
| `habit_name` | **conditionnel** | `string` | Nom de l'habitude (lookup `SELECT id FROM habits WHERE name=?`). Utilisé si `habit_id` vide. |
| `date` | **oui** | `YYYY-MM-DD` (10 caractères, ex: `2025-08-30`) | Date locale navigateur (`toLocaleDateString('en-CA')`). Backend la stocke en TEXT opaque, aucune conversion TZ. Invalide si longueur ≠10. |
| `value` | non | `float` | `boolean` : `0` ou `1` (mais tout nombre accepté, `≥1` compte comme fait). `numerical` : somme journalière. Vide → `0`. Écrasé sur conflit. |

### Règle d'upsert

```
1. Résoudre habit_id : si habit_id == "" et habit_name != "" → SELECT id WHERE name=?
   sinon habit_id doit exister → erreur "habit not found"
2. Vérifier existence habitude
3. UPSERT : INSERT ... ON CONFLICT(habit_id, date) DO UPDATE SET value=excluded.value
```

> Contrainte DB : `UNIQUE(habit_id, date)` — une seule valeur par habitude et par jour.

### Exemple `entries.csv`

```csv
habit_id,habit_name,date,value
550e8400-e29b-41d4-a716-446655440000,,2025-08-30,1
, Méditation,2025-08-29,1
abc123,,2025-08-30,2
def456,,2025-08-30,1
```

- Ligne 1 : par `habit_id` direct
- Ligne 2 : par `habit_name` (lookup), note l'espace trimé
- Valeur `1` pour boolean, `2` pour numerical

### Import via API

```bash
curl -X POST http://localhost:8080/api/import/entries \
  -H "Authorization: Bearer <token>" \
  -F file=@entries.csv

# réponse JSON
{"imported":4,"errors":""}
# en cas d'erreurs partielles : {"imported":2,"errors":"habit not found: Inconnu; invalid date 2025/08/30"}
```

Même fallback `multipart` vs body brut que pour habits.

---

## 3. Erreurs et retours

Chaque handler renvoie `{"imported": <n>, "errors": "<msg1; msg2>"}` en `200` même si certaines lignes échouent (commit partiel en transaction). Les lignes en erreur sont ignorées, les autres sont commitées.

**Cas d'erreur fréquents :**
- `missing name` → `name` vide dans `habits.csv`
- `invalid date 2025/08/30` → format doit être `YYYY-MM-DD`
- `habit not found: X` → `habit_name` inconnu
- `missing habit_id and habit_name` → les deux vides
- `invalid value abc` → `value` non numérique

Consultez toujours le champ `errors` après import.

---

## 4. Roundtrip recommandé

```bash
# export
curl -H "Authorization: Bearer <token>" http://localhost:8080/api/export/habits > habits.csv
curl -H "Authorization: Bearer <token>" http://localhost:8080/api/export/entries > entries.csv
# édition dans tableur / script
# ré-import
curl -X POST -H "Authorization: Bearer <token>" -F file=@habits.csv http://localhost:8080/api/import/habits
curl -X POST -H "Authorization: Bearer <token>" -F file=@entries.csv http://localhost:8080/api/import/entries
```

Pour un reset complet : supprimer `data/habits.db*` puis ré-importer `habits.csv` avant `entries.csv` (les entries dépendent des habits).

---

## 5. Notes

- Séparateur `,` uniquement (pas `;`). Si votre tableur exporte en `;`, convertissez ou utilisez un éditeur qui respecte RFC4180.
- UTF-8 sans BOM recommandé.
- Les dates sont stockées en TEXT, tri lexical = tri chronologique si format `YYYY-MM-DD` respecté.
- `is_negative` inverse la couleur heatmap (vert ↔ rouge) et l'affichage progression `≤` au lieu de `/`.

Voir aussi `internal/csv/handler.go:26` (export) et `78`/`198` (import) pour la logique exacte.

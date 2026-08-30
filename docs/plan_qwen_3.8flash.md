# Revue de code habit-go — bugs & améliorations

Source : analyse statique de `main.go`, `internal/db`, `internal/habit`, `internal/entry`, `internal/csv`, `frontend/src/api.ts` (2026-08-30).

## Vrais bugs

1. **Cookie de login ignore "remember"** — `main.go:266`
   - Le handler `loginHandler` pose toujours un cookie HttpOnly `auth_token` avec `MaxAge: 30j`, quelle que soit la valeur `remember` du frontend.
   - Le frontend (`frontend/src/api.ts:43-55`) stocke le token en `sessionStorage` quand `remember=false` et ne transmet jamais `remember` au serveur.
   - Conséquence : fermer l'onglet semble déconnecter côté SPA mais le cookie persiste 30 jours — la session ne peut pas vraiment expirer à la fermeture.
   - Fix : accepter `{ password, remember }` au login et ne poser le cookie persistant (`MaxAge>0`) que si `remember=true` (sinon cookie de session `MaxAge=0` ou pas de cookie).

2. **Décalage de fuseau horaire sur "today"** — `internal/habit/handler.go:255` (`lastNDates`)
   - Les entrées sont créées avec des dates locales navigateur (`frontend/src/api.ts:205` DST-safe), mais `lastNDates` utilise `time.Now()` côté serveur pour calculer `today` et les fenêtres glissantes (daily/weekly/monthly) et `computeProgress`.
   - Si le conteneur est en UTC et l'utilisateur en UTC+2, la progression "daily" est fausse entre 00:00 et 02:00 locales.
   - Fix : accepter un paramètre `today` (ou `from`/`to` explicites) envoyé par le client, ou uniformiser en UTC des deux côtés.

3. **Valeur d'entrée tronquée silencieusement à 0** — `internal/entry/handler.go:30-32`
   - L'API `Upsert` force `value<0 → 0` sans erreur, alors que l'import CSV (`internal/csv/handler.go:384`) autorise les valeurs négatives.
   - Incohérence + perte silencieuse pour des habitudes numériques (ex. variations). Fix : renvoyer 400 sur valeur invalide, ou autoriser les négatifs partout.

4. **`sql.ErrNoRows` détourné comme erreur de validation** — `internal/entry/repository.go:21`
   - `Upsert` renvoie `sql.ErrNoRows` pour une date invalide (`len!=10`), que les appelants interprètent comme "not found". Utiliser `fmt.Errorf("invalid date")` et une erreur sentinelle.

5. **Taille multipart incohérente** — `main.go:340` vs `internal/csv/handler.go:144`
   - `maxBytesMiddleware` limite les imports à `10<<20` octets, mais `ParseMultipartForm(10<<20)` autorise un fichier de 10 Mo. Avec l'overhead multipart/boundary un fichier proche de 10 Mo dépasse la limite et échoue en `400 invalid multipart`.
   - Fix : marge côté middleware (ex. 11 MiB) ou limiter côté CSV à 9 MiB.

6. **Export CSV renvoie un fichier partiel en 200 OK** — `internal/csv/handler.go:50,79,125`
   - Sur erreur de `rows.Scan`, le code fait `continue` et renvoie un CSV tronqué avec `200`. De plus `http.Error` après l'envoi des en-têtes `text/csv` produit un corps corrompu.
   - Fix : interrompre et renvoyer une erreur avant d'écrire les en-têtes, ou accumuler puis écrire atomiquement.

## Sécurité

7. **Token = `sha256(password)`** — `main.go:63`
   - Token statique, sans expiration ni révocation, stocké en `localStorage`/`sessionStorage` (volable par XSS). Comme c'est directement le hash du mot de passe, une fuite permet un crack offline.
   - Amélioration : token de session aléatoire côté serveur (ou HMAC avec secret serveur), expiration, révocation au logout, voire `bcrypt` pour le stockage du mot de passe.

8. **Pas de limitation de tentatives au login** — `main.go:237` (`loginHandler`)
   - Brute-force possible sur un port exposé. Ajouter rate limiting (ex. 5 essais/min/IP) et délai exponentiel.

9. **CORS renvoie n'importe quelle origine** — `main.go:315-321`
   - `Access-Control-Allow-Origin: <Origin>` reflété sans `Allow-Credentials`. Inutile puisque le frontend est embarqué same-origin ; préférer `*` quand non protégé ou une allowlist configurable.

10. **Cookie sans flag `Secure`** — `main.go:266-273`
    - À ajouter quand déployé derrière HTTPS (`Secure: true`).

11. **Pas de `Access-Control-Allow-Credentials`** — cohérent avec le point 9 : le cookie HttpOnly n'est pas utilisable cross-origin sans ce header ; à clarifier selon le modèle d'auth choisi.

## Code mort / fragile (maintenabilité)

12. **Fallback "vieille DB sans colonne `unit`" mort** — `internal/db/db.go:96` fait toujours `ALTER TABLE ... ADD COLUMN unit` dans `Migrate`, donc tout le code de repli est inatteignable :
    - `internal/habit/repository.go:56-146` (`listFallback`, branches `isNoSuchColumn` dans `List`/`Get`/`Create`/`Update`/`FindByName`), `internal/csv/handler.go` (branches sans `unit`).
    - ~150 lignes supprimables. `containsStr` (`repository.go:92`) duplique `strings.Contains`.

13. **Blocs `if` vides** — `internal/csv/handler.go:191-193` et `269-272` (no-ops à supprimer).

14. **Branches `sql.ErrNoRows` inatteignables** — `internal/habit/repository.go:136` (après gestion d'erreur) et `internal/habit/handler.go:226` (`GetRange` ne renvoie jamais `ErrNoRows`, seulement un slice vide).

15. **Mapping erreurs → statuts HTTP via `strings.Contains(err.Error(), ...)`** — `internal/habit/handler.go:100,144,161` fragile (casse au moindre changement de message). Utiliser des erreurs sentinelles (`var ErrNotFound = errors.New(...)`) et `errors.Is`.

16. **Comparaison constant-time qui fuit la longueur** — `main.go:259` (`ConstantTimeCompare` sur mots de passe de longueurs différentes retourne immédiatement 0). Négligeable mais à noter ; préférer comparer des hashes.

17. **Vérifications de méthode mortes** — `loginHandler` re-teste `r.Method != POST` alors que la route mux est déjà `POST /api/login` (`main.go:86,239`).

18. **Code dupliqué d'export CSV** — `ExportHabits` duplique la boucle avec/sans `unit`; factorisable après suppression du fallback.

## Performance & robustesse

19. **Requêtes N+1 sur `List`** — `internal/habit/handler.go:64,81` fait un `BulkGet` puis `computeProgress` (1 requête/habitude). Avec `SetMaxOpenConns(1)` (`internal/db/db.go:26`) tout est sérialisé. Calculer la progression depuis le résultat bulk au lieu de requêter par habitude.

20. **Aucun `context.Context` propagé** — aucun appel DB n'utilise `r.Context()` ; les requêtes lentes ne sont pas annulables à la déconnexion client. Passer à `QueryContext`/`ExecContext`.

21. **Pas d'arrêt gracieux** — `main.go:174` (`ListenAndServe` sans `signal.NotifyContext` + `srv.Shutdown`) ; `defer database.Close()` ne s'exécute jamais sur arrêt par signal.

22. **Import long bloque toute l'app** — `SetMaxOpenConns(1)` + transaction d'import (`internal/csv/handler.go:175,358`) monopolise la seule connexion ; acceptable pour usage perso mais à documenter.

23. **`OPTIONS` contournant l'auth en code mort** — `main.go:223` (`authMiddleware` laisse passer `OPTIONS`) n'est jamais atteint car `corsMiddleware` (extérieur) répond `204` avant (`main.go:161,326`). Supprimer la branche morte ou inverser l'ordre documenté.

24. **Fuite de fichiers temporaires multipart non nettoyés explicitement** — `r.ParseMultipartForm` crée des fichiers temp ; Go les nettoie à la fin de requête mais un `defer r.MultipartForm.RemoveAll()` explicite est plus sûr sur imports volumineux.

## Frontend

25. **Incohérence `remember` non transmise** — `frontend/src/api.ts:149` (`login`) n'envoie jamais `remember` au backend (lié au bug 1).

26. **Gestion d'erreurs d'import normalisée en string** — `frontend/src/api.ts:141` joint les erreurs en une seule chaîne ; perte de granularité pour l'UI. Garder le tableau côté backend/frontend.

## Améliorations mineures suggérées

- Remplacer `PRAGMA foreign_keys=ON` exécuté via `Exec` par paramètre DSN `?_pragma=foreign_keys(1)` (plus robuste que dépendre de `MaxOpenConns(1)`).
- Ajouter pagination à `GET /api/entries` sans filtre (dump complet).
- Capping `errorsArr` en import pour éviter une réponse JSON géante.
- Protection CSV injection (`=`, `+`, `-`, `@` en début de champ) à l'export si ouverture dans un tableur.

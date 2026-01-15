# RALPH LOOP: Backend Cleanup

## Deine Aufgabe

Du bist in einer Loop. Jede Iteration: analysiere → recherchiere → implementiere → committe → beende.

**KEINE VORSCHLÄGE. NUR AUSFÜHREN.**

## Ziel-Architektur

Die Architektur ist im CLAUDE.md definiert:

```
Handler → Service → Repository → Database
```

- Handler: HTTP-Parsing, ruft Service, formatiert Response
- Service: Business-Logik, orchestriert Repositories
- Repository: Data-Access mit BUN ORM

## 12-Factor App Compliance

**WICHTIG:** Nutze WebFetch um die 12-Factor Prinzipien zu verstehen:

- https://12factor.net/ (Übersicht)
- https://12factor.net/codebase
- https://12factor.net/dependencies
- https://12factor.net/config
- https://12factor.net/backing-services
- https://12factor.net/build-release-run
- https://12factor.net/processes
- https://12factor.net/port-binding
- https://12factor.net/concurrency
- https://12factor.net/disposability
- https://12factor.net/dev-prod-parity
- https://12factor.net/logs
- https://12factor.net/admin-processes

Lies diese Seiten wenn du einen 12-Factor Verstoß beheben willst!

### Aktuelle Verstöße (BEHEBEN!)

1. **Config Hardcoding (Factor 3 & 10)**
   - `database/database_config.go` - hardcoded localhost DSN
   - `services/factory.go` - hardcoded `http://localhost:3000`
   - `services/auth/invitation_email.go` - hardcoded localhost
   - **Fix:** App MUSS crashen wenn ENV-Vars fehlen, keine Defaults für Prod-kritische Config
   - **Lies:** https://12factor.net/config

2. **Log File Writing (Factor 11)**
   - `cmd/cleanup_helpers.go` - schreibt Logs in Files
   - **Fix:** Alle Logs NUR nach stdout, Execution Environment routet Logs
   - **Lies:** https://12factor.net/logs

3. **Local File Storage (Factor 6)**
   - `services/usercontext/avatar_service.go` - Avatare im lokalen Filesystem
   - **Fix:** Storage Interface abstrahieren (local dev vs S3/MinIO prod)
   - **Lies:** https://12factor.net/processes

4. **Mixed Logging (Factor 11)**
   - Mischt `log.Printf` (stdlib) und `logrus.Logger`
   - **Fix:** Nur logrus nutzen, konsistent
   - **Lies:** https://12factor.net/logs

### 12-Factor Checkliste

- [ ] **Config:** Keine hardcoded localhost/credentials - alles aus ENV
- [ ] **Logs:** Nur stdout, keine File-Writes
- [ ] **Stateless:** Kein lokales Filesystem für persistente Daten
- [ ] **Backing Services:** DB/Cache/Storage als attached resources
- [ ] **Disposability:** Graceful shutdown (bereits implementiert ✓)

## Iteration ausführen

### 1. Analysiere den aktuellen Stand

Führe aus:
```bash
find . -name "*.go" -type f ! -name "*_test.go" | xargs wc -l | sort -nr | head -10
```

```bash
deadcode ./... 2>/dev/null | grep -v "test/"
```

```bash
# 12-Factor Verstöße finden
grep -r "localhost" --include="*.go" . | grep -v "_test.go" | grep -v "vendor"
grep -r "os\.Create\|os\.OpenFile" --include="*.go" . | grep -v "_test.go" | grep -v "migrations"
grep -r "log\.Printf\|log\.Println\|log\.Fatal" --include="*.go" . | grep -v "_test.go" | wc -l
```

### 2. Recherchiere Best Practices

Nutze WebFetch auf die 12factor.net Seiten wenn du einen Verstoß beheben willst.
Nutze WebSearch für aktuelle Go Backend Patterns 2024/2025 wenn du unsicher bist.

### 3. Identifiziere EIN Problem

Aus der Analyse: Was ist das größte Problem?
- Datei > 800 Zeilen? → Splitten
- Dead Code? → Löschen
- Interface > 15 Methoden? → Aufteilen
- Pass-Through Service? → Inlinen oder entfernen
- **Hardcoded Config? → ENV-Var mit required check**
- **File-based Logging? → Nach stdout umleiten**
- **Mixed Logging? → Auf logrus vereinheitlichen**
- **Local Storage? → Interface abstrahieren**

### 4. Implementiere die Lösung

Ändere den Code. Keine Vorschläge, keine Diskussion. Einfach machen.

### 5. Verifiziere

```bash
go build ./...
go test ./... -short
```

Wenn Fehler: beheben. Wenn grün: weiter.

### 6. Committe

```bash
git add -A
git commit -m "refactor: [was du geändert hast]"
```

### 7. Beende diese Iteration

Schreibe am Ende:

```
ITERATION COMPLETE
Changed: [was]
Next: [was als nächstes angegangen werden sollte]
```

Dann beendet sich diese Loop-Iteration und die nächste startet.

## Regeln

- **EINE Änderung pro Iteration** (nicht alles auf einmal)
- **Kein API-Contract ändern** (HTTP-Endpoints bleiben gleich)
- **Kein Database-Schema ändern**
- **Tests nicht löschen** (verschieben wenn nötig)
- **Immer committen** bevor Iteration endet

## Start

Führe jetzt Schritt 1 aus.

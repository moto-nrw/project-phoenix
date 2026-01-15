# RALPH LOOP: Backend Cleanup

## Deine Aufgabe

Du bist in einer Loop. Jede Iteration: analysiere → recherchiere → implementiere → committe → beende.

**KEINE VORSCHLÄGE. NUR AUSFÜHREN.**

## Ziel-Architektur: Hexagonal / Clean Architecture

**Referenzen (per WebFetch lesen!):**
- https://dev.to/bagashiz/building-restful-api-with-hexagonal-architecture-in-go-1mij
- https://threedots.tech/post/introducing-clean-architecture/

### ZIEL-Ordnerstruktur

```
backend/
├── cmd/                        ← Entry Points
│   └── server/
│       └── main.go
│
└── internal/                   ← Private Application Code (Go convention)
    │
    ├── core/                   ← BUSINESS LOGIC (KEINE externen Dependencies!)
    │   ├── domain/                 Pure Entities, Value Objects
    │   │   ├── user.go
    │   │   ├── student.go
    │   │   ├── visit.go
    │   │   └── ...
    │   ├── port/                   Interfaces (Contracts für Adapters)
    │   │   ├── repository.go           UserRepository, VisitRepository interfaces
    │   │   ├── mailer.go               EmailSender interface
    │   │   └── storage.go              FileStorage interface
    │   └── service/                Business Logic Services
    │       ├── auth.go
    │       ├── active.go
    │       └── ...
    │
    └── adapter/                ← INFRASTRUCTURE (implementiert Ports)
        ├── handler/                HTTP/gRPC Handlers
        │   └── http/
        │       ├── auth.go
        │       ├── student.go
        │       └── router.go
        ├── repository/             Database Implementations
        │   └── postgres/
        │       ├── user.go
        │       ├── visit.go
        │       └── migrations/
        ├── mailer/                 Email Implementation
        │   └── smtp.go
        ├── storage/                File Storage Implementation
        │   ├── local.go
        │   └── s3.go
        ├── cache/                  Cache Implementation (optional)
        │   └── redis.go
        └── realtime/               SSE/WebSocket Implementation
            └── sse.go
```

### Dependency Rule (WICHTIG!)

```
┌─────────────────────────────────────────────────────────────┐
│                         adapter/                            │
│   handler/  repository/  mailer/  storage/  realtime/       │
│      │           │          │         │          │          │
│      │      implements      │    implements      │          │
│      ▼           ▼          ▼         ▼          ▼          │
├─────────────────────────────────────────────────────────────┤
│                      core/port/                             │
│   UserRepository  VisitRepository  EmailSender  FileStorage │
│                          ▲                                  │
│                          │ uses                             │
├─────────────────────────────────────────────────────────────┤
│                     core/service/                           │
│        AuthService    ActiveService    UserService          │
│                          ▲                                  │
│                          │ uses                             │
├─────────────────────────────────────────────────────────────┤
│                      core/domain/                           │
│          User    Student    Visit    Group                  │
│            (pure entities, no dependencies)                 │
└─────────────────────────────────────────────────────────────┘

REGEL: Pfeile zeigen IMMER nach innen!
       core/ importiert NIEMALS adapter/
       adapter/ implementiert core/port/ interfaces
```

### AKTUELLE Struktur (zu migrieren)

```
backend/                        →    backend/
├── api/                        →    internal/adapter/handler/http/
├── services/                   →    internal/core/service/
├── database/repositories/      →    internal/adapter/repository/postgres/
├── models/                     →    internal/core/domain/ + internal/core/port/
├── email/                      →    internal/adapter/mailer/
├── realtime/                   →    internal/adapter/realtime/
├── auth/                       →    internal/adapter/middleware/
├── middleware/                 →    internal/adapter/middleware/
├── logging/                    →    internal/adapter/logger/
└── cmd/                        →    cmd/
```

### Migration Schritte

1. **Erstelle `internal/` Verzeichnis**
2. **Verschiebe `models/` Domain-Entities nach `internal/core/domain/`**
3. **Extrahiere Repository-Interfaces aus `models/` nach `internal/core/port/`**
4. **Verschiebe `services/` nach `internal/core/service/`**
5. **Verschiebe `database/repositories/` nach `internal/adapter/repository/postgres/`**
6. **Verschiebe `api/` nach `internal/adapter/handler/http/`**
7. **Verschiebe `email/` nach `internal/adapter/mailer/`**
8. **Verschiebe `realtime/` nach `internal/adapter/realtime/`**
9. **Konsolidiere `auth/` + `middleware/` nach `internal/adapter/middleware/`**
10. **Update alle Import-Pfade**
11. **Verifiziere: `core/` importiert KEINE `adapter/` packages**

---

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

---

## Iteration ausführen

### 1. Analysiere den aktuellen Stand

Führe aus:
```bash
# Dateigrößen
find . -name "*.go" -type f ! -name "*_test.go" | xargs wc -l | sort -nr | head -10

# Dead Code
deadcode ./... 2>/dev/null | grep -v "test/"

# 12-Factor Verstöße
grep -r "localhost" --include="*.go" . | grep -v "_test.go" | grep -v "vendor"
grep -r "os\.Create\|os\.OpenFile" --include="*.go" . | grep -v "_test.go" | grep -v "migrations"
grep -r "log\.Printf\|log\.Println\|log\.Fatal" --include="*.go" . | grep -v "_test.go" | wc -l

# Hexagonal Verstöße (core/ importiert adapter/?)
# Nach Migration prüfen:
# go list -f '{{.ImportPath}}: {{.Imports}}' ./internal/core/... | grep adapter
```

### 2. Recherchiere Best Practices

Nutze WebFetch auf die 12factor.net und Hexagonal-Architektur Seiten.
Nutze WebSearch für aktuelle Go Backend Patterns 2024/2025 wenn du unsicher bist.

### 3. Identifiziere EIN Problem

Aus der Analyse: Was ist das größte Problem?

**Code Quality:**
- Datei > 800 Zeilen? → Splitten
- Dead Code? → Löschen
- Interface > 15 Methoden? → Aufteilen
- Pass-Through Service? → Inlinen oder entfernen

**12-Factor:**
- Hardcoded Config? → ENV-Var mit required check
- File-based Logging? → Nach stdout umleiten
- Mixed Logging? → Auf logrus vereinheitlichen
- Local Storage? → Interface abstrahieren

**Hexagonal Migration:**
- Noch kein `internal/`? → Erstellen
- Domain + Interfaces gemischt? → Trennen in domain/ und port/
- Adapter verstreut? → Nach adapter/ verschieben

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

### 7. Log to TASKS.md

Append a summary of what you did this iteration:

```bash
echo "## Iteration $(date +%Y-%m-%d_%H:%M:%S)" >> TASKS.md
echo "" >> TASKS.md
echo "**Changed:** [what you changed]" >> TASKS.md
echo "" >> TASKS.md
echo "**Files:** [list of modified files]" >> TASKS.md
echo "" >> TASKS.md
echo "**Commit:** [commit hash]" >> TASKS.md
echo "" >> TASKS.md
echo "---" >> TASKS.md
echo "" >> TASKS.md
```

### 8. Document Learnings

If you discover something important (pattern, gotcha, best practice, architectural insight):

```bash
echo "- $(date +%Y-%m-%d): [what you learned]" >> LEARNINGS.md
```

### 9. End Iteration

Print:

```
ITERATION COMPLETE
Changed: [what]
Next: [what should be tackled next]
```

Then this loop iteration ends and the next one starts.

---

Read also RALPH_LOOP_TASK_codex.md
Use go vet ./... and go test ./... to verify

## Regeln

- **Zusammenhängende Änderungen dürfen gebündelt werden**
- **Kein API-Contract ändern** (HTTP-Endpoints bleiben gleich)
- **Kein Database-Schema ändern**
- **Tests nicht löschen** (verschieben wenn nötig)
- **Immer committen** bevor Iteration endet

## Priorität

1. **ERST** 12-Factor Verstöße beheben (Config, Logs, Storage)
2. **DANN** Hexagonal Migration starten (internal/, core/, adapter/)

## Start

Führe jetzt Schritt 1 aus.

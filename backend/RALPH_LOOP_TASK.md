
# RALPH LOOP: Backend Cleanup

## Deine Aufgabe

Du bist in einer Loop. Jede Iteration: analysiere → recherchiere → implementiere → committe → beende.

**KEINE VORSCHLÄGE. NUR AUSFÜHREN.**

---

## Phase 0: Sync mit Development (START jeder Session)

**WICHTIG:** Führe dies zu Beginn jeder Session aus, um Merge-Konflikte klein zu halten.

### 0.1 Prüfe auf neue Commits

```bash
git fetch origin development
git log HEAD..origin/development --oneline | head -10
```

Wenn keine neuen Commits → überspringe zu Phase 1.

### 0.2 Starte den Merge

```bash
git merge origin/development
```

### 0.3 Löse Konflikte nach Typ

| Konflikt-Typ | Erkennung | Lösung |
|--------------|-----------|--------|
| **modify/delete** | `deleted in HEAD and modified in origin/development` | Development hat Änderungen an einer Datei, die im Refactor verschoben wurde. Wende die Änderungen auf die NEUE Location in `internal/` an. |
| **file location** | `added in origin/development inside a directory that was renamed` | Neue Dateien (meist Tests) in alter Struktur. Verschiebe sie zur neuen Location: `git mv backend/api/X/X_test.go backend/internal/adapter/handler/http/X/X_test.go` |
| **content** | `Merge conflict in ...` | Echter Code-Konflikt. Merge beide Änderungen, bevorzuge Refactor-Struktur. |

### 0.4 Konflikt-Mapping (alte → neue Pfade)

```
backend/api/                    →  backend/internal/adapter/handler/http/
backend/services/               →  backend/internal/core/service/
backend/database/repositories/  →  backend/internal/adapter/repository/postgres/
backend/models/                 →  backend/internal/core/domain/ + port/
backend/email/                  →  backend/internal/adapter/mailer/
backend/realtime/               →  backend/internal/adapter/realtime/
```

### 0.5 Verifiziere und Committe

```bash
# Alle Konflikte gelöst?
git diff --name-only --diff-filter=U  # Sollte leer sein

# Build + Test
go build ./...
go test ./... -short

# Commit
git add -A
git commit -m "merge: sync with development, apply changes to hexagonal structure"
```

### 0.6 Log den Sync

```bash
echo "## Sync $(date +%Y-%m-%d_%H:%M:%S)" >> TASKS.md
echo "" >> TASKS.md
echo "**Merged:** $(git log -1 --format='%h') from development" >> TASKS.md
echo "" >> TASKS.md
echo "**Conflicts resolved:** [Anzahl und Art der Konflikte]" >> TASKS.md
echo "" >> TASKS.md
echo "---" >> TASKS.md
echo "" >> TASKS.md
```

---

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

## Logging: Wide Events / Canonical Log Lines

**References:**
- https://boristane.com/blog/logging-sucks/
- https://loggingsucks.com/

### Das Problem

```go
// ❌ SO SIEHT ES JETZT AUS - Verstreute, nicht-korrelierbare Logs
func (rs *VisitResource) create(w http.ResponseWriter, r *http.Request) {
    logrus.Info("Processing visit request")           // Was ist der Kontext?
    logrus.Debug("Validating student")                // Welcher Student?
    logrus.Info("Student validated")                  // OK, und dann?
    logrus.Debug("Checking group membership")         // Welche Gruppe?
    logrus.Warn("Student not in primary group")       // Problem? Oder normal?
    logrus.Info("Visit created successfully")         // Welche Visit-ID?
}
// Resultat: 6 Log-Zeilen, keine korrelierbar, beim Debugging nutzlos
```

**Das echte Problem:** Wenn ein Lehrer meldet "Check-in funktioniert nicht", greppen wir durch tausende Zeilen und finden nichts Brauchbares.

### Die Lösung: Ein Wide Event Pro Request

**Mental Model Shift:**
> Statt zu loggen **was dein Code tut**, logge **was mit diesem Request passiert ist**.

```go
// ✅ SO SOLLTE ES SEIN - Ein Event mit allem Kontext
func (rs *VisitResource) create(w http.ResponseWriter, r *http.Request) {
    // ... business logic ...

    // AM ENDE: Ein einziges Event mit allem was wir wissen
    logrus.WithFields(logrus.Fields{
        "request_id":  requestID,
        "method":      "POST",
        "path":        "/api/visits",
        "status_code": 201,
        "duration_ms": 47,
        "user_id":     teacherID,
        "user_role":   "teacher",
        "student_id":  studentID,
        "group_id":    groupID,
        "room_id":     roomID,
        "action":      "check_in",
        "visit_id":    createdVisit.ID,
    }).Info("request_completed")
}
// Resultat: 1 Log-Zeile, vollständig querybar
// Query: "Zeige alle check_in failures für room_id=5 in der letzten Stunde"
```

Für jeden HTTP-Request: **ein strukturiertes Event am Ende** mit allem Kontext:

```json
{
  "timestamp": "2025-01-16T10:23:45.612Z",
  "request_id": "req_8bf7ec2d",
  "trace_id": "abc123",

  "service": "phoenix-backend",
  "version": "2.4.1",
  "environment": "production",

  "method": "POST",
  "path": "/api/visits",
  "status_code": 201,
  "duration_ms": 147,

  "user_id": "user_456",
  "user_role": "teacher",
  "account_id": "acc_789",

  "student_id": "student_123",
  "group_id": "group_456",
  "room_id": "room_789",
  "action": "check_in",

  "error_type": "ValidationError",
  "error_code": "student_not_in_group",
  "error_message": "Student is not assigned to this group"
}
```

### Implementation mit logrus + Chi

#### 1. WideEvent Struct definieren

```go
// internal/adapter/middleware/wide_event.go
package middleware

import (
    "context"
    "time"
)

type contextKey string
const wideEventKey contextKey = "wideEvent"

// WideEvent sammelt allen Kontext während eines Requests
type WideEvent struct {
    // Request metadata (vom Middleware gesetzt)
    Timestamp  time.Time
    RequestID  string
    Method     string
    Path       string
    StatusCode int
    DurationMS int64

    // Service metadata (aus ENV)
    Service    string
    Version    string

    // User context (vom Auth-Middleware gesetzt)
    UserID     string
    UserRole   string
    AccountID  string

    // Business context (vom Handler gesetzt)
    StudentID  string
    GroupID    string
    RoomID     string
    Action     string  // "check_in", "check_out", "transfer", etc.

    // Error context (bei Fehlern gesetzt)
    ErrorType    string
    ErrorCode    string
    ErrorMessage string
}

// GetWideEvent holt das Event aus dem Context
func GetWideEvent(ctx context.Context) *WideEvent {
    if event, ok := ctx.Value(wideEventKey).(*WideEvent); ok {
        return event
    }
    return &WideEvent{} // Fallback, sollte nie passieren
}
```

#### 2. Middleware die das Event initialisiert und emittiert

```go
// internal/adapter/middleware/wide_event_middleware.go
func WideEventMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        // Initialize wide event
        event := &WideEvent{
            Timestamp: start,
            RequestID: r.Header.Get("X-Request-ID"),
            Method:    r.Method,
            Path:      r.URL.Path,
            Service:   os.Getenv("SERVICE_NAME"),
            Version:   os.Getenv("SERVICE_VERSION"),
        }
        ctx := context.WithValue(r.Context(), wideEventKey, event)

        // Wrap response writer to capture status code
        wrapped := &responseWriter{ResponseWriter: w, status: 200}

        // WICHTIG: defer emittiert das Event AM ENDE des Requests
        defer func() {
            event.StatusCode = wrapped.status
            event.DurationMS = time.Since(start).Milliseconds()

            fields := logrus.Fields{
                "timestamp":   event.Timestamp.Format(time.RFC3339),
                "request_id":  event.RequestID,
                "method":      event.Method,
                "path":        event.Path,
                "status_code": event.StatusCode,
                "duration_ms": event.DurationMS,
                "service":     event.Service,
                "version":     event.Version,
            }

            // Nur non-empty fields hinzufügen
            if event.UserID != "" { fields["user_id"] = event.UserID }
            if event.UserRole != "" { fields["user_role"] = event.UserRole }
            if event.StudentID != "" { fields["student_id"] = event.StudentID }
            if event.GroupID != "" { fields["group_id"] = event.GroupID }
            if event.RoomID != "" { fields["room_id"] = event.RoomID }
            if event.Action != "" { fields["action"] = event.Action }
            if event.ErrorType != "" {
                fields["error_type"] = event.ErrorType
                fields["error_code"] = event.ErrorCode
                fields["error_message"] = event.ErrorMessage
            }

            logrus.WithFields(fields).Info("request_completed")
        }()

        next.ServeHTTP(wrapped, r.WithContext(ctx))
    })
}

type responseWriter struct {
    http.ResponseWriter
    status int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.status = code
    rw.ResponseWriter.WriteHeader(code)
}
```

#### 3. Handler reichern das Event an

```go
// In api/active/visits.go (oder internal/adapter/handler/http/visits.go)
func (rs *VisitResource) create(w http.ResponseWriter, r *http.Request) {
    event := middleware.GetWideEvent(r.Context())

    // Business context hinzufügen WÄHREND der Verarbeitung
    event.StudentID = fmt.Sprintf("%d", req.StudentID)
    event.GroupID = fmt.Sprintf("%d", req.ActiveGroupID)
    event.Action = "check_in"

    visit, err := rs.service.CreateVisit(r.Context(), req)
    if err != nil {
        // Error context hinzufügen
        event.ErrorType = "CreateVisitError"
        event.ErrorCode = mapErrorToCode(err)
        event.ErrorMessage = err.Error()

        render.Error(w, r, err)
        return
    }

    // Erfolg: zusätzlichen Kontext hinzufügen
    event.RoomID = fmt.Sprintf("%d", visit.RoomID)

    render.JSON(w, r, http.StatusCreated, visit)
}
```

### Key Fields für Project Phoenix

| Kategorie | Fields | Warum |
|-----------|--------|-------|
| **Request** | `request_id`, `method`, `path`, `status_code`, `duration_ms` | Korrelation & Filter |
| **User** | `user_id`, `user_role`, `account_id` | "Zeige alle Errors für Lehrer" |
| **Business** | `student_id`, `group_id`, `room_id`, `action` | "Welcher Raum hat die meisten Check-in Failures?" |
| **Error** | `error_type`, `error_code`, `error_message` | "Gruppiere nach Error Code" |
| **Environment** | `service`, `version`, `environment` | "Hat das neue Deployment das verursacht?" |

### Was NICHT loggen

- Einzelne Debug-Statements während des Requests (Kontext sammeln, einmal emittieren)
- Sensitive Daten (Passwörter, volle JWTs, PII außer IDs)
- Hochfrequente interne Operationen (dafür Metrics nutzen)

### Tail Sampling (Später, bei Scale)

> **Hinweis:** Für Project Phoenix aktuell nicht nötig. Erst relevant wenn Log-Volume zum Problem wird.

Bei hohem Volume intelligent samplen:
- **Immer behalten:** Errors (5xx), langsame Requests (>p99)
- **Samplen:** Erfolgreiche schnelle Requests bei 5-10%

### Logging Checkliste

- [ ] **Ein Event pro Request** - keine verstreuten Log-Statements
- [ ] **Business Context attached** - student_id, group_id, action, nicht nur "request processed"
- [ ] **Structured JSON** - querybar, keine grep-baren Strings
- [ ] **Am Request-Ende emittiert** - nachdem aller Kontext bekannt ist
- [ ] **High-Cardinality Fields** - user_id, request_id (wertvoll, nicht teuer)
- [ ] **Nur logrus** - kein log.Printf Mix
- [ ] **Nur stdout** - keine File-Writes

---

## Iteration ausführen

**Hinweis:** Führe Phase 0 (Sync mit Development) zu Beginn jeder neuen Session aus, BEVOR du mit Iterationen beginnst.

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

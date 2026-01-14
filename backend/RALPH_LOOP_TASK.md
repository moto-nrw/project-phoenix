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

## Iteration ausführen

### 1. Analysiere den aktuellen Stand

Führe aus:
```bash
find . -name "*.go" -type f ! -name "*_test.go" | xargs wc -l | sort -nr | head -10
```

```bash
deadcode ./... 2>/dev/null | grep -v "test/"
```

### 2. Recherchiere Best Practices

Nutze WebSearch für aktuelle Go Backend Patterns 2024/2025 wenn du unsicher bist wie etwas strukturiert sein soll.

### 3. Identifiziere EIN Problem

Aus der Analyse: Was ist das größte Problem?
- Datei > 800 Zeilen? → Splitten
- Dead Code? → Löschen
- Interface > 15 Methoden? → Aufteilen
- Pass-Through Service? → Inlinen oder entfernen

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

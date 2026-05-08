# Playwright E2E

Die E2E-Suite hat genau einen offiziellen Laufweg:

```bash
cd backend
go run . e2e run --scenario e2e-multi-tenant
```

Dieser Befehl übernimmt alles:

1. isolierte E2E-Infrastruktur starten (dedizierte `postgres`- + `backend`-Services)
2. die kanonische Multi-Tenant-Welt in Go vorbereiten
3. genau ein Artefakt schreiben: `backend/.e2e-state.json`
4. das dedizierte Frontend auf `localtest.me` starten
5. Playwright ausführen
6. Logs/Artefakte erfassen und wieder aufräumen

CI ruft denselben Befehl auf.

## Manuell testen

Für manuelles Testen auf der separaten E2E-Welt:

```bash
cd backend
go run . e2e up --scenario e2e-multi-tenant
```

Der Befehl startet dieselbe isolierte Infrastruktur und hält sie offen, bis du
`Ctrl-C` drückst. Das Frontend läuft auf `*.localtest.me:3030`, also parallel
zur normalen Dev-Umgebung auf `:3000`.

Wenn dein Rechner `*.localtest.me` nicht auflösen kann:

```bash
cd backend
sudo go run . e2e hosts sync --scenario e2e-multi-tenant
```

## Architektur

Das Harness ist absichtlich auf wenige Verantwortlichkeiten reduziert:

- **Go (`e2e run` / `e2e up`)** besitzt Orchestrierung, Runtime und die
  komplette Testwelt.
- **Go-Seeder-Szenario** besitzt alle fachlichen Seed-Daten und vorbereiteten
  Zustände.
- **`backend/.e2e-state.json`** ist das einzige maschinenlesbare E2E-Artefakt.
- **`state.ts`** ist der einzige rohe TS-Konsument dieses Vertrags; alle
  anderen Playwright-Dateien hängen nur noch semantisch daran.
- **Playwright Setup-Projekt** besitzt Browser-Login, `storageState` und
  Verifikation der Kernannahmen.
- **Specs** arbeiten über semantische Fixtures und Auth-State, nicht über rohe
  Seed-Details wie IDs, RFID-Tags oder Geräte-Header.

## Dateiaufteilung

```text
e2e/
├── auth.setup.ts   sichtbares Playwright-Setup für Login/storageState
├── auth.ts         Browser-Login und Session-Prüfungen
├── api.ts          API-Kontexte aus dem Setup-authentifizierten Browser-State
├── state.ts        einziger Vertragsleser + semantische Zugriffsfunktionen
├── fixtures.ts     semantische Fixtures für Specs
├── flows/          eigentliche Tests
└── helpers/        kleine Spec-Helfer, keine Welt-/Orchestrierungslogik
```

## Specs schreiben

UI-Specs importieren immer aus `fixtures.ts`:

```typescript
import { test, expect } from "../fixtures";
```

HTTP-Specs ebenso:

```typescript
import { apiTest as test, apiExpect as expect } from "../fixtures";
```

Direkte Logins, rohe Seed-Zugriffe und parallele Harness-Helfer gehören nicht
in Specs. Wenn eine neue fachliche Ausgangslage gebraucht wird, wird sie im
Go-Szenario modelliert und als Fixture aus `fixtures.ts` freigegeben.

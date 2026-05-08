# Playwright E2E

Die E2E-Suite hat genau einen offiziellen Laufweg:

```bash
cd frontend
pnpm e2e
```

Dieser Befehl übernimmt alles:

1. isolierte E2E-Infrastruktur starten (dedizierte `postgres`- + `backend`-Services)
2. die kanonische Multi-Tenant-Welt in Go vorbereiten
3. genau ein Artefakt schreiben: `backend/.e2e-state.json`
4. das dedizierte Frontend auf `localtest.me` starten
5. Playwright ausführen
6. Logs/Artefakte erfassen und wieder aufräumen

CI ruft denselben Befehl auf. `pnpm e2e` ist der einzige stabile
Einstiegspunkt; Raw-Playwright-Aufrufe brechen bewusst ab, damit keine Tests
gegen alte State- oder Auth-Artefakte laufen.

Wenn dein Rechner `*.localtest.me` nicht auflösen kann:

```bash
cd backend
sudo go run . e2e hosts sync --scenario e2e-multi-tenant
```

## Architektur

Das Harness ist absichtlich auf wenige Verantwortlichkeiten reduziert:

- **Go-Harness** besitzt Orchestrierung, Runtime und die komplette Testwelt.
  Es ist die interne Implementierung hinter `pnpm e2e`, kein zweiter
  offizieller Einstieg.
- **Go-Seeder-Szenario** besitzt alle fachlichen Seed-Daten und vorbereiteten
  Zustände.
- **`backend/.e2e-state.json`** ist das einzige maschinenlesbare E2E-Artefakt.
- **`state.ts`** ist der einzige rohe TS-Konsument dieses Vertrags und leitet
  daraus nur semantische Fixture-Daten ab.
- **`api.ts`** ist der einzige HTTP-semantische Adapter auf der TS-Seite:
  Token aus `storageState` ableiten, API-Kontexte bauen, HTTP-Flows kapseln.
- **Playwright Setup-Projekt** besitzt Browser-Login, `storageState` und
  Verifikation der Kernannahmen inklusive API-Auth-Pipeline.
- **`fixtures.ts`** verdrahtet nur noch Playwright-Fixtures auf diese
  semantischen Bausteine; dort lebt keine rohe Vertragslogik mehr.
- **Specs** arbeiten über semantische Fixtures und Auth-State, nicht über rohe
  Seed-Details wie IDs, RFID-Tags oder Geräte-Header.

## Dateiaufteilung

```text
e2e/
├── auth.setup.ts   sichtbares Playwright-Setup für Login/storageState
├── auth.ts         Browser-Login und Session-Prüfungen
├── api.ts          API-Kontexte + HTTP-Flows aus dem Setup-authentifizierten State
├── state.ts        einziger Vertragsleser + semantische Fixture-Ableitungen
├── fixtures.ts     dünne Playwright-Verdrahtung
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

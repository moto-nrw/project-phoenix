# #6452: Baseline isolierter Testdatenbanken, Runde 4

Messstand: `origin/development` @ `4f43c72524b9b45c10e99330de74c0946ec0870f`,
10.11.2026. Keine Änderung an Produktions- oder Testverhalten. #6451 ist
separat mit einer absichtlich abbrechenden Minimalreproduktion bestätigt.

## Umgebung und Methode

- macOS/arm64, 16 logische CPUs, 64 GiB Host-RAM; Go 1.27.3 aus Devbox,
  `CGO_ENABLED=0`, `GOFLAGS=-p=4 -trimpath`, `GOMAXPROCS=4` für die direkten Läufe.
- Eigener Container `phx-s3a-6451`, ausschließlich `127.0.0.1:46451`,
  PostgreSQL 17.11/alpine (arm64), Image-ID
  `sha256:3540c0362c809df084ffac0ae69fe794515f9de92692f02178bcdce0ef0302ac`.
  Eigener anonymer Daten-Volume, Containerlimit 1536 MiB. Keine Verbindung zu 5433.
- Lokale Konfiguration nach `docker-compose.example.yml`: `fsync=off`,
  `synchronous_commit=off`, `full_page_writes=off`, `shared_buffers=32MB`,
  `wal_level=minimal`, `max_wal_senders=0`, `max_connections=300`.
- Template `phoenix_test_cacc05a971b0` vorab über den Bootstrap-Befehl aufgebaut.
  Dependency-/Build-Caches nicht gelöscht. Der erste Overlay-Build braucht einen
  neuen Link; die folgenden Paketzeiten schließen den Build nicht ein.
  Jeder Lauf führt mit `-count=1` tatsächlich Tests aus.
- Keine zweite eigene Testsuite gleichzeitig. Fremde Maschinenlast nicht
  kontrolliert: Load-1 vor Beginn 7,44, später 12,32; unter 40. Docker-VM und
  andere Sessions laufen weiter. Kein Nachweis identischen Lastprofils zu Runde 3.
- Ein ausschließlich temporäres Go-Overlay ergänzt `backend/test/database_scope.go`
  um `time.Now`/`time.Since` und Ausgabe `[DEBUG-S3A] <Name> <Phase> <Sekunden>`.
  Das Original und die Assertions bleiben unverändert. JSON-Testereignisse
  liefern die Paketzeit und die ausgeführten Testnamen.

Die Grenzen der Phasen:

| Phase | Messgrenzen |
| --- | --- |
| Slot | Eintritt in `isolatedTestDatabase.open` bis Ende `acquire` |
| Create | Danach bis Rückkehr von `testdb.CreateClone`, einschließlich GC/Lifecycle-Lock/Keeper |
| Bootstrap | Danach: Search-Path, Pool, Bootstrap, TenantRuntime, ggf. vorhandenen Tenant anlegen |
| Testkörper | Erfolgreiches `open` bis Beginn von `drop`; enthält Fixtures und vor dem DB-Cleanup registrierte Cleanups |
| Drop | Gesamtes `drop`: Query-Hooks lösen, Pool/Keeper schließen, `DropClone`, Slot freigeben |

Create/Bootstrap/Drop sind API-Phasen, keine ausschließlich serverseitigen
SQL-Ausführungszeiten. Der einmalige Paket-Clone, Paketinitialisierung und das
Paket-Leftover-Gate stehen außerhalb dieser 152 isolierten Lifecycles.
Summierte Slot-Wartezeit überlappt laufende Tests; **nicht zur Paketzeit addieren**.

## Direkte Paketläufe auf unverändertem development

Beide Läufe sequenziell, mit identischer Auswahl und denselben Parametern:

```sh
# TEST_DB_DSN wird ausschließlich aus der eigenen lokalen Testumgebung geerbt.
# overlay.json ersetzt nur database_scope.go durch die oben beschriebene Messfassung.
~/bin/agent-heavy --light -- scripts/run-go-toolchain.sh go test -C backend \
  -tags=testhooks -overlay ../tmp/s3a-measure/overlay.json \
  -p 1 -parallel 8 -count=1 -timeout 10m -json \
  ./modules/organizationtenancy/compose
```

Alle Zeiten in Sekunden; Phasenspalten summiert über die isolierten Clones.

| Lauf | Paket | bestandene Tests inkl. Subtests | Clones | Slot Σ / Max | Create Σ / Max | Bootstrap Σ / Max | Testkörper Σ / Max | Drop Σ / Max |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Volume 1 | 138,886 | 198 | 152 | 717,177 / 34,120 | 10,225 / 0,173 | 3,884 / 0,057 | 118,197 / 20,338 | 5,794 / 0,183 |
| Volume 2 | 138,853 | 198 | 152 | 732,389 / 39,239 | 10,161 / 0,108 | 3,918 / 0,061 | 116,488 / 22,576 | 5,960 / 0,189 |

Die Zahl direkter Aufrufstellen ist nicht die Zahl der Clones: Tabellen und
Helfer erzeugen im aktuellen Lifecycle 152 tatsächliche Clones pro Paketlauf.
Create + Bootstrap + Drop summieren sich auf 19,903 bzw. 20,039 s, etwa 14 %
der Paketzeit. Die Testkörper dominieren diese beiden Läufe. Das beweist weder
die Ursache der früheren 500–535 s noch eine Beschleunigung durch #6451.

Längste aufsummierte Testkörper des ersten Laufs:

| Test | Sekunden |
| --- | ---: |
| `TestTenantExportEveryCatalogTableAndKindIsIsolated` | 20,338 |
| `TestTenantExportRejectsPartialFilesAndForeignReferences` | 15,446 |
| `TestTenantExportRefusesContainersWithoutTheirOwner` | 11,393 |
| `TestTenantExportRefusesSealedValuesItCannotOpen` | 7,884 |
| `TestDemoRunErasesEveryOpenOrderOnceItsLockIsGone` | 7,349 |

## Regulärer Backend-Wrapper über test-perf

Zusätzlich auf unverändertem Head, gleicher Server und gleiches Timing-Overlay:

```sh
# GOFLAGS ergänzt nur den absoluten Overlay-Pfad; TEST_DB_DSN bleibt geerbt.
scripts/run-go-toolchain.sh node scripts/test-perf.mjs run \
  backend-export s3a-before --cache-state warm
```

Der Wrapper setzt `GOMAXPROCS=1`, `GOGC=50`, lokal `-p 2 -parallel 8`
und `-ldflags=-w`. Keine zweite eigene Suite gleichzeitig.

| Paket | Gesamt-Wall | Gate-Warten | Host-CPU | Peak Host-Prozessbaum | Ergebnisse |
| ---: | ---: | ---: | ---: | ---: | --- |
| 148,342 s | 154,4 s | 0,0 s | 17,8 CPU-s | 400 MB | 198 bestanden, 0 übersprungen |

| Clones | Slot Σ / Max | Create Σ | Bootstrap Σ | Testkörper Σ | Drop Σ |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 152 | 792,263 / 58,232 s | 9,078 s | 3,695 s | 129,334 s | 5,630 s |

`test-perf` validiert unveränderte Quellen und vollständiges Testinventar.
`pgMeter=unavailable`: der Ressourcenmesser erkennt diesen manuell gestarteten
Container nicht. CPU/RAM sind ausschließlich Host-Prozessbaumwerte, keine
Postgres-Ressourcenbilanz. JSON- und Messartefakte liegen lokal unter
`tmp/test-perf/s3a-before-backend-export-D603v7/`.
Die oben separat erhobenen Lifecycle-Zeiten bleiben verfügbar.
Auch dieser Lauf reproduziert weder 500–535 s noch den Pakettimeout.
Ein einzelner Wrapper-Lauf ist keine belastbare Ressourcenvergleichsserie.

## Development-CI als Vergleichsbasis

Die letzten drei erfolgreichen `main.yml`-Läufe auf `development`, bei Erfassung
vorhanden. Unterschiedliche Revisionen und Gesamttestzahlen, daher keine
isolierte Before/After-Messung des Pakets. Jeweils
`blacksmith-8vcpu-ubuntu-2404`, volle Backend-Suite mit `-p 6 -parallel 8 -count=1`.
CI verwendet PostgreSQL auf tmpfs statt lokalem Volume. Cache-Restore-Schritte
waren erfolgreich; das ist allein noch kein Beleg für einen exakten Cache-Hit.

| Lauf | compose | Schritt „Run Go tests“ | ganzer Backend-Testjob | übriger Job | gerundete vCPU-min | geschätzte USD |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| [42726466027](https://github.com/moto-nrw/project-phoenix/actions/runs/42726466027) | 76,672 | 322 | 476 | 154 | 64 | 0,128 |
| [42720201238](https://github.com/moto-nrw/project-phoenix/actions/runs/42720201238) | 90,704 | 375 | 661 | 286 | 96 | 0,192 |
| [42565910481](https://github.com/moto-nrw/project-phoenix/actions/runs/42565910481) | 46,256 | 213 | 325 | 112 | 48 | 0,096 |

Kosten nur für diesen Job: `ceil(Jobsekunden / 60) × 8 vCPU`, dann
0,002 USD/vCPU-min gemäß `.github/actions/ci-cost/ci-cost.mjs` (Preisstand
01.11.2026). Der Rest enthält Setup, Cache-Speicherung und Nacharbeit, nicht
nur DB-Setup. Testarbeit ist der größte Block; Kern-Auslastung wurde nicht
profilert. Keine Änderung an Runnern, Shards, Slots oder Workflow vorgesehen.
Die PR-CI dient später nur als Plausibilitätsvergleich. Das Kostenkriterium
sind Clone-Lifecycles je Paket (compose: 152) und abwechselnde
`test-perf`-Läufe vorher/nachher mit identischen Parametern. Die abwechselnden Ressourcenläufe stehen unten; PR-CI-Vergleiche
werden im PR-Body geführt.

## #6451: bestätigte Minimalreproduktion

Nur in einem separaten temporären Overlay in `backend/test`:

```go
func TestS3aReproOwnTenantIsolatedClone(t *testing.T) {
    t.Parallel()
    SetupIsolatedTestDB(t)
    t.Run("own-tenant", func(t *testing.T) {
        OwnTenant(t)
        SetupIsolatedTestDB(t)
    })
}
```

Auf demselben unveränderten Head, eigener Postgres, `-parallel 8 -count=1
-timeout 8s -run '^TestS3aReproOwnTenantIsolatedClone$' ./test`:
Exit 1, `panic: test timed out after 8s`. Parent wartet in `testing.(*T).Run`,
Kind in `isolatedTestDatabaseLimiter.acquire` beim Senden auf den Slot-Kanal.
Der Parent erwarb zuvor seinen Clone in 0,064 s plus 0,024 s Bootstrap.
Die 2-min-Setupfrist wird vor dem Kanal-Warten nicht gestartet. Das ist ein
bestätigter Lifecycle-Zyklus, kein langsamer Clone.

## Entscheidung und nächste Messung

#6451 wird im freigegebenen Umfang umgesetzt. #6452 bleibt `Refs`,
solange nicht zwei vergleichbare Vorher-/Nachher-Läufe zeigen, dass derselbe
Engpass die gemeldete Laufzeit erklärt und der Fix sie senkt.

Konkreter nächster Schritt für #6452: den ursprünglichen lokalen
`check-changed`-/Quorum-Aufruf unter dokumentierter Last mit JSON-Ereignissen
aufzeichnen und die fünf Phasen erneut trennen, daneben den identischen
Paketlauf allein ausführen. Dann nur für die langsamen Testkörper Pool-Stats,
Server-Statementzeiten und CPU-/I/O-Profil vergleichen. Die direkten Läufe
reproduzieren weder 500–535 s noch einen 10-min-Pakettimeout. Timeout-Erhöhung,
mehr Clone-Slots oder weniger Testisolation wären damit nicht begründet.

## Umsetzung #6451 und unabhängige Fehler-Gegenproben

Routing, `topLevelTestName`, `OwnTenant`/`OwnCtx`, Tenant-Anlage und die bisherige
Cleanup-Zuordnung bleiben unverändert. Die DB-/DSN-/Runtime-Lookups sowie
Tenant-Anlage/Cleanup wurden gegen `4f43c72524` byteweise verglichen; die drei
übrigen Routing-Helferdateien haben keinen Diff. Kein dataimport-Diff.

Umgesetzt sind nur die vier freigegebenen Einheiten:

1. Slot-Frist aus `t.Deadline()` minus **155 s** Reserve (2 min Öffnen, 30 s
   Drop, 5 s Meldung), abbrechbar am Testkontext. Fehlende Deadline wird vor
   dem Warten gemeldet. Die Öffnung erhält erst nach dem Slot ihre eigenen
   unveränderten 2 min. Diagnosen nennen Requester, Besitzer, Wartezeit und
   Schlangenlänge einschließlich des fehlgeschlagenen Requesters.
2. `CompareAndDelete` entfernt nur den eigenen fehlgeschlagenen Cache-Eintrag.
   Realer Setupfehler: fehlender PostgreSQL-Template. Der bestehende
   Disposal-Kindprozess prüft danach erfolgreichen Neuaufbau und den bisherigen
   DROP-Fehler. Das erwartete Setup-`Fatalf` wird abgefangen, ohne die echte T
   rot zu markieren; nur der spätere Cleanupfehler darf den Prozess scheitern
   lassen. Keine zusätzlichen erfolgreichen Clones; ein zusätzlicher erfolgloser
   CREATE-Versuch als Fehlerprobe.
3. Statischer Check in `TestHermeticTestPatterns`: **0 Treffer**, auch in
   dataimport. Er folgt benannten lokalen Funktionen über Dateien sowie
   Inline-/benannten Run-Callbacks und den expliziten Clone-Einstiegen,
   einschließlich verschachtelter Clone-haltender Parent-Callbacks.
   Direktes `PerTestDatabases()` in TestMain ist slotfrei und deshalb kein
   Slot-/Scheduler-Zyklus; dieser bestehende Modus wird nicht beanstandet.
   Nicht aufgelöst: dynamische Funktionswerte, Interface-/Methodendispatch und
   fremde Helfergraphen; die endliche Slotfrist begrenzt übersehene Muster.
4. Scheduler-Regression: dieselbe echte, dependency-freie Slot-Implementierung
   in einem kleinen `go test`-Modul, `-parallel 2`, äußerer Go-Testtimeout 5 s.
   Besitzer und zwei Waiter sind mit Kanälen synchronisiert. Nur die eingespeiste
   Fixture-Deadline skaliert die Reserve auf einen 1-s-Wartepfad. Erwartet sind
   Exit 1, die konkrete Deadline-Diagnose und das beendete Parent-Kind, **kein**
   Go-Testalarm. Null DB-Clones. Direkte Testbinary-Kinder erhalten den üblichen
   endlichen `go test`-Default 10 min statt implizit Timeout 0.

### Befundklasse 17: Fehlerfähigkeit der Tests

Keine neutralen Ausgangsdaten: belegter Slot, Besitzer und zwei wartende Tests;
reale SQL-Setup-/DROP-Fehler. Unabhängige Sollwerte: Produktionscap 1, Reserve
155 s, Öffnung 2 min, Drop 30 s und eine nichtnullige virtuelle Wartezeit 20 ms.
Synchronisation über Kanäle bzw. `synctest.Wait`, kein hinzugefügter Sleep.

| Kernaussage | Gegenprobe/Mutation | Beobachtetes Ergebnis |
| --- | --- | --- |
| Entwicklung hängt, Fix terminiert diagnostisch | Bytegleicher Limiter aus development-Basis `4f43c72524`, derselbe Scheduler-Graph; nur alter API-Aufruf angepasst | Go-Testalarm nach 5 s; mit Fix erwarteter Deadlinefehler und beendetes Kind |
| Kein unbegrenztes Warten | Context-Select durch alten blockierenden Kanal-Send ersetzt | Go-Testalarm nach 5 s |
| Statische Prüfung erkennt eingeschleuste Muster | Positive AST-Fixtures mit Parent-Clone/Parallel-Kind, auch verschachtelt; Finder auf `nil` mutiert | Positive Fälle werden gefunden; Mutation scheitert an erwartetem nichtleerem Befund |
| Reserve ist fachlich unabhängig fixiert | Reserve auf 0 mutiert | Golden-Deadline minus 155 s wird verfehlt, Test rot |
| Produktionslimit bleibt 1 | Produktionscap auf 2 mutiert | Unabhängiger Sollwert 1 schlägt fehl |
| Wartezeit ist tatsächlich gemessen | Gemeldete Wartezeit auf 0 mutiert | Virtuell synchronisierter 20-ms-Fall rot |
| Fehlercache wird entfernt | `CompareAndDelete` entfernt | Reale Fehlprobe hinterlässt Eintrag, Test rot |
| DROP muss unabhängig den Prozess scheitern lassen | Cleanup-`Errorf` auf `Logf` mutiert | Kind endet Exit 0 statt 1, bisheriger Prozess-Guard bleibt rot |

Normaler Stand: gezielte Tests einschließlich kompletter Hermetic-/Komplexitäts-
Gates mit `-count=2` grün. Keine Assertions, Skips oder Baselines gelockert.

### Bestätigter Rest, deshalb Refs #6451

Synchronisierter Real-DB-Gegentest: parallele Geschwister ohne OwnTenant teilen
nach wie vor den zuerst geöffneten Scope. Nach dem Cleanup des ersten bekommt
das noch aktive zweite `sql: database is closed` (Paketlauf 1,271 s). Dieses
vorhandene Lebenszyklusproblem wird nicht durch Vererbung oder Scope-Wechsel
verdeckt. Es braucht einen gesonderten Vertrag und koordinierte Migrationen,
insbesondere mit S1. Der statische Parent-Clone-Check deckt diese Geschwister-
Konstellation nicht ab. Rest und nächste Schritte werden in #6451 festgehalten.

## Abwechselnde Ressourcenmessung auf derselben Implementierungsbasis

Reihenfolge alt 1 → neu 1 → alt 2 → neu 2, identische Parameter über
`test-perf backend-export`. Je Variante vorab derselbe Wrapper mit `-run '^$'`
zum Warm-Linken. Alt ist der originale Scope-/Prozesshelfer aus `4f43c72524`;
der neue separate Slot-Quelltext wird dort durch eine leere Go-Datei ersetzt.
Neu ist die Implementierung dieses PRs. Beide tragen dasselbe reine Timing-
Overlay. Identische 198 bestandene Test-/Subtest-Inventare, je 152 isolierte
Clones. Spätere Verschärfung der unabhängigen Assert-Sollwerte ändert den in
compose verwendeten Bibliothekscode nicht; sie ist im finalen Vollauf enthalten.

Sekunden; Phasen summiert, Slotwartezeit überlappt Testarbeit. Host-Peak in MB.

| Variante | Paket | Slot Σ / Max | Create Σ | Bootstrap Σ | Testkörper Σ | Drop Σ | Host-CPU-s | Peak |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| alt 1 | 177,484 | 954,481 / 62,954 | 12,810 | 4,604 | 151,766 | 7,684 | 17,600 | 144 |
| neu 1 | 164,202 | 861,279 / 68,310 | 9,976 | 4,370 | 144,119 | 5,272 | 15,200 | 150 |
| alt 2 | 170,247 | 910,256 / 62,758 | 12,175 | 5,503 | 145,183 | 6,873 | 16,800 | 156 |
| neu 2 | 173,763 | 930,167 / 65,007 | 12,942 | 5,535 | 147,720 | 6,929 | 15,600 | 153 |

Die Maschinenlast schwankte (unter anderem 46,26 vor alt 1, 22,39 vor neu 1,
13,04 vor alt 2 und 18,31 vor neu 2). Daher keine kausale Latenzbehauptung aus
Suite-Wall-Zeiten. Summierte Teardown-Zeiten sind oben getrennt; CPU/Peak der
warmen compose-Läufe zeigen keine zusätzliche DB-Kopierarbeit. 500–535 s und
ein 10-min-Pakettimeout sind weiterhin nicht reproduziert. Refs #6452.

## Vollsuite: Ressourcen und Clone-Kosten

`test-perf backend-full`, je `-count=1`, gleicher eigener Postgres und dieselben
Wrapper. Ergebnis- und Testinventar validiert. Bestehende 30 Environment-Skips
unverändert; zwölf zusätzliche Test-/Subtest-Ergebnisse im Messlauf. Kein neuer
Skip. Danach wurden verschachtelte und slotfreie Scanner-Fixtures ergänzt; diese
DB-freien Fälle werden im abschließenden Changed-Gate geprüft und verändern
keinen Clone-Lifecycle.

| Stand | Wall s | Gate-Queue s | Host-CPU-s | Peak MB | Pakete | bestanden / Skips |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| development-Code | 1105,600 | 0,000 | 1358,600 | 2863 | 523 | 41023 / 30 |
| Fix einschließlich Klasse 17, vor den abschließenden Scanner-Fixtures | 623,400 | 141,900 | 499,100 | 2442 | 523 | 41035 / 30 |

Erhaltene Caches, aber unterschiedliche Link-/Cachezustände und Hostlast:
keine behauptete Vollsuite-Beschleunigung. Ein Peak-Unterschied ist ausgewiesen,
nicht als isolierter Effekt des Limiters identifiziert. Zusätzliche Arbeit ist
begrenzt auf den AST-Check (warm im gemeinsamen Parsercache etwa 0,14 s) und
den 5-s-begrenzten, DB-freien Go-Subprozess. Postgres-CPU/RAM bleiben vom
Host-Prozessbaummesser ausgeschlossen.

Clone-Nachweis aus serverseitigen `COMMENT ON DATABASE … phx-pkg:`-Ereignissen:
**alle 161 Pakete identisch, insgesamt 1785 → 1785 erfolgreiche Clones**.
compose: 153 → 153 einschließlich Paket-Clone, also isoliert **152 → 152**.
`backend/test`: **13 → 13** durch Wiederverwendung des bestehenden Fehler-
Kindprozesses. Der zusätzliche fehlgeschlagene CREATE-Versuch erzeugt keinen
Clone. CI ist nur Plausibilitätsvergleich, kein Ersatz für diese Kostenkriterien.

## CI-Integrationskorrektur

Der erste CI-Lauf meldete vier nicht inventarisierte neue Testfunktionen.
Die drei Slot-Tests sind nun mit ihrer kopierten Quelltextabhängigkeit im
Source-Input-Inventar registriert; der Scanner-Unitfall ist als synthetischer
Fixture-Test ausgewiesen. Kein realer Befund wird ausgenommen. Lokal: der
Inventartest vorher rot, anschließend alle neun Script-Unitprüfungen grün.

Der Merge-Testbaum enthielt außerdem den inzwischen gemergten
`TestBackfillBatchRetry`: ein Parent mit parallelen Nachfahren im bestehenden
`PerTestDatabases()`-Modus. Dieser Modus umgeht den Limiter, auch für explizite
Clones. Der Scanner beanstandete deshalb irrtümlich einen nicht vorhandenen
Slot-Zyklus. Zwei unabhängig mit Sollwert 0 definierte Fixtures sind vor der
Korrektur rot und danach grün; normale und verschachtelte slotbegrenzte
Positivfälle bleiben erkannt. Keine Migration, kein DB-Routing und kein
Clone-Lifecycle verändert.

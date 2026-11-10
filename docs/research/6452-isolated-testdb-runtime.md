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
Die PR-CI ist später mit diesen Zahlen zu vergleichen; zwei warme PR-Läufe
und `test-perf backend-full` vorher/nachher stehen vor der Umsetzung noch aus.

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

#6451 wird erst nach Freigabe der Invarianten umgesetzt. #6452 bleibt `Refs`,
solange nicht zwei vergleichbare Vorher-/Nachher-Läufe zeigen, dass derselbe
Engpass die gemeldete Laufzeit erklärt und der Fix sie senkt.

Konkreter nächster Schritt für #6452: den ursprünglichen lokalen
`check-changed`-/Quorum-Aufruf unter dokumentierter Last mit JSON-Ereignissen
aufzeichnen und die fünf Phasen erneut trennen, daneben den identischen
Paketlauf allein ausführen. Dann nur für die langsamen Testkörper Pool-Stats,
Server-Statementzeiten und CPU-/I/O-Profil vergleichen. Die direkten Läufe
reproduzieren weder 500–535 s noch einen 10-min-Pakettimeout. Timeout-Erhöhung,
mehr Clone-Slots oder weniger Testisolation wären damit nicht begründet.

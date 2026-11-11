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

### Verwendeter Overlay-Diff

Der folgende Diff ist das vollständige Mess-Overlay gegen die oben benannte
development-Basis. Er wird nur auf eine temporäre Kopie angewendet, nicht auf
den Checkout. Go-Befehl und Overlay-Erzeugung setzen einen Checkout dieser
Basis voraus; das Overlay ist nicht für den PR-Head bestimmt. Zum Wiederholen
den Diff als `tmp/s3a-measure/overlay.diff` speichern:

```sh
mkdir -p tmp/s3a-measure
git show 4f43c72524b9b45c10e99330de74c0946ec0870f:backend/test/database_scope.go > tmp/s3a-measure/database_scope.go
git apply --unidiff-zero --directory=tmp/s3a-measure -p3 tmp/s3a-measure/overlay.diff
python3 - <<'PYOVERLAY'
from pathlib import Path
import json
root = Path.cwd()
Path("tmp/s3a-measure/overlay.json").write_text(json.dumps({"Replace": {
    str(root / "backend/test/database_scope.go"):
        str(root / "tmp/s3a-measure/database_scope.go")
}}))
PYOVERLAY
```

```diff
--- a/backend/test/database_scope.go
+++ b/backend/test/database_scope.go
@@ -7,0 +8 @@
+"os"
@@ -62,0 +64,2 @@
+measureName string
+bodyStart time.Time
@@ -87 +90,3 @@
-	if !perTestDatabases {
+	entry.measureName = name
+measureStart := time.Now()
+if !perTestDatabases {
@@ -90,0 +96,2 @@
+fmt.Fprintf(os.Stderr, "[DEBUG-S3A] %s slot %.9f\n", name, time.Since(measureStart).Seconds())
+measureStart = time.Now()
@@ -103,0 +111,2 @@
+fmt.Fprintf(os.Stderr, "[DEBUG-S3A] %s create %.9f\n", name, time.Since(measureStart).Seconds())
+measureStart = time.Now()
@@ -126,0 +136,2 @@
+fmt.Fprintf(os.Stderr, "[DEBUG-S3A] %s bootstrap %.9f\n", name, time.Since(measureStart).Seconds())
+entry.bodyStart = time.Now()
@@ -169,0 +181,3 @@
+if !entry.bodyStart.IsZero() { fmt.Fprintf(os.Stderr, "[DEBUG-S3A] %s body %.9f\n", entry.measureName, time.Since(entry.bodyStart).Seconds()) }
+measureStart := time.Now()
+defer func() { fmt.Fprintf(os.Stderr, "[DEBUG-S3A] %s drop %.9f\n", entry.measureName, time.Since(measureStart).Seconds()) }()
```

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
`test-perf`-Läufe vorher/nachher mit identischen Parametern. Die abwechselnden Ressourcenläufe, Implementierungsnachweise und
PR-CI-Vergleiche werden im PR-Body geführt.

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

## Runde 5: Messung unter Last und Behebung (11.11.2026)

Messstand: `origin/development` @ `919799847b`, Linux/amd64, 32 logische CPUs,
62 GiB RAM, Go aus Devbox, `GOFLAGS=-p=4 -trimpath`, `GOMAXPROCS=4`. Geteilter
Testserver auf Port 5433 (tmpfs, `fsync=off`), 129 Datenbanken und 45
Verbindungen fremder Sessions bei der Stichprobe. Fremde Last nicht
kontrolliert: Load-1 zwischen 33 und 420, je Lauf vor und nach dem Lauf
notiert. Aufruf wie `scripts/test-changed.sh`:

```sh
scripts/run-go-toolchain.sh go test -C backend -p 4 -parallel 8 \
  -tags=testhooks -count=1 -json ./modules/organizationtenancy/compose
```

Phasen wie in Runde 4, über ein temporäres Go-Overlay von
`backend/test/database_scope.go`; der Checkout bleibt unverändert.

### Vorher: unverändertes development

| Lauf | Load-1 vor/nach | Paket | Clones | Create Σ | Bootstrap Σ | Testkörper Σ | Drop Σ | Σ Lifecycles |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| ohne Overlay | 63 / 41 | 401,2 s | – | – | – | – | – | – |
| mit Overlay | 33 / 305 | 340,3 s | 152 | 146,6 s | 9,4 s | 108,2 s | 73,3 s | 337,5 s |

Beide Läufe grün, 199 Ergebnisse. Die Paketzeit ist die Summe der
Clone-Lifecycles: Explizite Clones teilen sich einen Slot je Paket
(`maxConcurrentIsolatedTestDatabases = 1`), also laufen 152 Lifecycles
nacheinander, während sieben weitere Tests auf den Slot warten (Slot-Warten
Σ 1936 s, überlappend). Die Testkörper sind unter Last nicht langsamer als in
Runde 4 (108 s gegenüber 116 bis 118 s). Create und Drop wachsen von zusammen
etwa 0,1 s auf 1,45 s je Clone und stellen 220 der 340 s.
`TestDemoRunErasesEveryOpenOrderOnceItsLockIsGone` erzeugt je Subtest einen
eigenen Clone (40 Stück) und stellt sich dafür jedes Mal hinten an: 233 bis
385 s Laufzeit bei 17 s Testkörper.

Nicht bestätigt:

- `STRATEGY FILE_COPY` gegenüber `WAL_LOG`, je sechs Clones des 43-MB-Templates
  im Wechsel bei Load 420: 65 bis 721 ms gegenüber 226 bis 810 ms. Kein
  belastbarer Unterschied, keine Änderung.
- fsync des Exports: `TestTenantExport*` mit `TMPDIR` auf tmpfs gegenüber btrfs,
  je zwei Läufe im Wechsel: 34,3/54,6 s gegenüber 48,6/52,8 s. Auf diesem
  Rechner kein Unterschied. `F_FULLFSYNC` unter macOS ist damit nicht geprüft.

### Zeitabhängige Erwartungen

Mit acht gleichzeitigen Clones bei Load 300 scheiterten sieben Tests an
`retireNow`: Der Lauf brauchte 1,1 bis 2,1 s statt unter 1 s. Die Grenze maß
die Wanduhr des ganzen Laufs, also auch seine Arbeit und jede Verzögerung der
Datenbank. Ein weiterer Test erwartete zwei gelöschte Aufträge in einem Lauf;
die Löschung beginnt nach 250 ms Budget keinen Auftrag mehr.

### Änderung

1. `retireNow` zählt die Lock-Wartefälle des Laufs wie `retireCounted` seit
   #6338, statt die Wanduhr zu messen. Die 5-s-Wache gegen ein Warten auf den
   Halter bleibt; `lock_timeout` bleibt in
   `TestDemoVisitorErasureNeverWaitsForALock` festgelegt.
2. Zwei Tests wiederholen den Lauf, bis beide Aufträge gelöscht sind, höchstens
   einen Lauf je Auftrag (`eraseTheRest`).
3. Das Paket läuft mit `testpkg.PerTestDatabases()`: 113 seiner 146 Tests
   brauchen ohnehin einen Clone, im Modus ohne Slot laufen acht davon
   gleichzeitig.
4. `TestDemoRunErasesEveryOpenOrderOnceItsLockIsGone` nutzt einen Clone für
   seine 40 Subtests.

### Nachher

Slot-Modus und Per-Test-Datenbanken im Wechsel, beide mit den Änderungen 1, 2
und 4, alle Läufe grün mit 199 Ergebnissen:

| Lauf | Load-1 vor/nach | Paket | Clones | Create Σ | Testkörper Σ | Drop Σ | Σ Lifecycles |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Slot 1 | 106 / 296 | 434,4 s | 113 | 238,4 s | 116,7 s | 68,8 s | 431,4 s |
| Per-Test 1 | 296 / 318 | 108,0 s | 145 | 385,4 s | 223,5 s | 201,9 s | 823,3 s |
| Slot 2 | 318 / 300 | 227,4 s | 113 | 108,2 s | 66,0 s | 42,9 s | 223,3 s |
| Per-Test 2 | 300 / 305 | 70,0 s | 145 | 242,8 s | 167,8 s | 130,8 s | 548,3 s |

Endstand ohne Overlay, `-count=3` in einem Binary bei Load 56 / 39: 295,6 s
für drei Durchläufe (98,5 s je Durchlauf), 597 Ergebnisse grün.

Der geteilte Server streut stark: `scripts/check-changed.mjs` auf dem Endstand
war zweimal grün, mit 179,7 s (Load 56 / 49) und 346,6 s (Load 45 / 43) für
das Paket. Die Load-1 des Rechners erklärt das nicht. Deshalb derselbe Wechsel
auf einem eigenen Testserver (Port 46452, sonst gleiche Konfiguration, nur
dieses Paket), Load-1 43 bis 56:

| Lauf | Paket | Clones | Create Σ | Testkörper Σ | Drop Σ |
| --- | ---: | ---: | ---: | ---: | ---: |
| Slot 1 | 85,5 s | 113 | 8,6 s | 53,8 s | 1,8 s |
| Per-Test 1 | 13,3 s | 145 | 12,6 s | 65,4 s | 3,6 s |
| Slot 2 | 70,2 s | 113 | 9,3 s | 55,2 s | 2,0 s |
| Per-Test 2 | 15,4 s | 145 | 13,0 s | 75,8 s | 4,0 s |
| Slot 3 | 64,7 s | 113 | 8,5 s | 50,8 s | 1,9 s |
| Per-Test 3 | 19,4 s | 145 | 15,1 s | 125,0 s | 5,2 s |

Auf dem eigenen Server kosten Create und Drop zusammen 0,09 bis 0,14 s je
Clone, auf dem geteilten 1,3 bis 6 s. Was die Lifecycles aufbläht, ist also
die Belegung des geteilten Testservers durch andere Sessions (129 Datenbanken
bei der Stichprobe), nicht die CPU-Last des Rechners. Der Slot machte daraus
die Paketzeit, weil er alle Lifecycles hintereinander legte.

Der Preis: 32 Clones mehr je Paketlauf und etwa doppelt so viel aufsummierte
Datenbankarbeit, weil die Lifecycles einander bremsen. Bis zu acht statt
einem Clone dieses Pakets bestehen gleichzeitig. Eine Messung ohne fremde Last
und unter macOS fehlt; die CI-Paketzeit dient als Gegenprobe.

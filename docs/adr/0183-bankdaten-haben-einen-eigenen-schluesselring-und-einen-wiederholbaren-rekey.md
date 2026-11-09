---
status: proposed
---

# Bankdaten haben einen eigenen Schlüsselring und einen wiederholbaren Rekey

**Status: Vorgeschlagen.** Vorschlag vom 09.11.2026 zu #6252, Teil von #4611.
Er ersetzt für Bankdaten die gemeinsame Schlüsselverwendung aus ADR 0171.

## Entscheidung und Invarianten

1. Security Runtime stellt denselben AES-256-GCM-Feld-Cipher bereit, aber die
   Root bindet `BANK_DATA_KEYS` getrennt von `HEALTH_DATA_KEYS` an den
   consumer-owned Port `parentbilling.BankDataCipher`. Dessen Marker
   `BankDataRing()` ist auch am owner-neutralen Root-Port erforderlich; ein
   Health-Cipher kann daher weder Mandate noch Lastschriften versorgen. Fehlende, leere oder
   ungültige Bankkonfiguration verhindert den Start. Ringformat:
   `id:base64[,id:base64]`, je Schlüssel 32 unabhängig zufällige Bytes;
   der erste Schlüssel schreibt, alle Schlüssel lesen anhand der Schlüssel-ID.
   Keine neuen Kryptoprimitiven und kein neuer Owner.
2. `BANK_DATA_LEGACY_KEYS` ist eine explizite, vorübergehende Kopie des bisher
   für Bankdaten verwendeten Rings. Leer schaltet den Legacy-Lesepfad ab.
   Es gibt keinen Zugriff auf `HEALTH_DATA_KEYS` im Bank-Cipher und keinen
   Rückfall bei fehlendem Bankring. Auch kollidierende IDs verschiedener Ringe
   funktionieren: zuerst authentifiziert der Bankring, dann der Legacy-Ring.
   Neue Werte entstehen ausschließlich im Bankring.
3. Die Elternabrechnung besitzt `BankDataRekey`, Anwendung und Postgres-Adapter.
   Die CLI `bank-data-rekey --organization ID --batch-size 100` läuft nur mit
   einer privilegierten Wartungsverbindung, niemals im HTTP-Prozess. Sie
   verarbeitet Mandate einschließlich widerrufener und die ungeöffneten Kopien
   von IBAN/Kontoinhaber in `sepa_debits` (#6265, Migration 1.15.826).
   #6198 schreibt über dieselbe Mandats-Capability und erhält dadurch den neuen
   Cipher. Masken, BIC und Organisationsstammdaten sind keine Chiffrate aus dem
   Gesundheitsring und werden nicht verändert.
4. Jeder Batch sperrt höchstens 1000 Zeilen eines ausdrücklich gewählten
   Trägers, geordnet nach ID. Kein `SKIP LOCKED`: kein übersprungener Datensatz
   verschwindet hinter dem Cursor. Die Sperre liegt vor dem Lesen und Öffnen;
   parallele Änderungen werden nicht mit einem alten Stand überschrieben.
   AAD bleibt unverändert: Feld, Träger-ID, Mandatsreferenz. Beide Werte werden
   authentifiziert, bevor der Batch committen darf. Fehler rollen den ganzen
   Batch zurück; Wiederholung beginnt sicher bei ID null. Aktuelle Chiffrate
   werden authentifiziert, bleiben aber bytegleich. Ein weiterer kompletter
   Durchlauf muss `changed=0` liefern.
5. Der administrative Adapter setzt `session_replication_role=replica` nur
   transaktionslokal: Der bestehende Trigger verbietet jede Änderung eines
   widerrufenen Mandats, auch eine reine Umschlüsselung. Kein Schemaeingriff,
   keine Datenmigration, kein Abschalten eines Triggers für andere Sitzungen.
   Die CLI-Root öffnet je Batch die bestehende Träger-Transaktion des Tenant
   Runtime. Der Adapter verlangt genau diesen Träger-Kontext und setzt deren
   Rolle auf den ursprünglichen Wartungslogin zurück; ein normaler
   HTTP-Login hat weiterhin keine Superuser-Rechte. Die einzigen UPDATEs schreiben explizit die beiden versiegelten Spalten und
   filtern nochmals nach Träger und gesperrter ID. Fachliche Felder, Zeitstempel
   und Audit bleiben bytegleich. Das privilegierte Verfahren verlangt denselben
   Vertrauensbereich wie Migrationen, nicht zusätzliche HTTP-Rechte.
6. Ausgabe ausschließlich Quelle und Mengen (`scanned`, `changed` je Batch;
   `scanned_total`, `changed_total` je vollständigem Quell-Durchlauf). Die CLI
   aktiviert kein SQL-Logging, reicht keine DB-/Crypto-Fehlerdetails weiter und
   gibt weder Werte noch Referenzen oder Schlüssel aus. PostgreSQL- und
   Backup-Zugriffsrechte bleiben unverändert.

## Manueller Rollout ohne Ausfall

Verantwortlich: Yannick. Keine Live-Zugriffe durch Implementierungsagenten.

1. **Vor dem Merge**, nicht erst vor dem Deploy: für Staging, Produktion und
   Demo per SOPS unabhängige Bank-Schlüssel erzeugen und `BANK_DATA_KEYS` setzen,
   niemals Dev-Testwerte übernehmen. `BANK_DATA_LEGACY_KEYS` muss **exakt dem
   heutigen vollständigen Wert von `HEALTH_DATA_KEYS` derselben Umgebung**
   entsprechen, einschließlich Reihenfolge und aller Schlüssel-IDs. Im selben
   SOPS-Commit die beiden Bank-Einträge aus `PENDING_SOPS_KEYS` in
   `scripts/env-check.sh` streichen und Env-Sync prüfen. Verschlüsselte Dateien
   niemals von Hand bearbeiten. Compose verlangt die Werte bereits bei `pull`.
2. Danach mergen und die bankringfähige Version vollständig ausrollen. Alte
   Instanzen beenden, bevor der Job oder neue Bank-Schreibvorgänge erfolgen:
   eine alte Version versteht neue Bankchiffrate nicht. Während des Rekeys
   bleibt die neue Anwendung verfügbar.
3. Im jeweiligen Deployment-Verzeichnis (`~/staging`, `~/production` bzw.
   `~/demo`, mit der von CI bereitgestellten `.env` und `docker-compose.yml`)
   den folgenden Bash-Block ausführen. Er verarbeitet **alle Träger**, auch
   inaktive, ohne manuell gepflegte ID-Liste. Die Abfrage filtert absichtlich
   nicht auf Aktivität. Der separate `bank-data-rekey`-Service des Profils
   `maintenance` nutzt dasselbe gepinnte Image wie `server`, erhält nur den
   privilegierten `DB_DSN`, Pool-Konfiguration, `APP_ENV`, `TZ` und die beiden
   Bankringe. Keine Health-/HTTP-Schlüssel, Ports, Mounts oder SQL-Debug-Logs.
   Compose interpoliert die geschützte `.env`; keine Schlüssel als Shell-Literal
   oder CLI-Wert übergeben. `run` aktiviert den Maintenance-Service ausdrücklich.

   ```bash
   (
     set -euo pipefail
     organization_ids=$(docker compose exec -T postgres psql -U postgres -d postgres \
       -At -v ON_ERROR_STOP=1 -c 'SELECT id FROM platform.organizations ORDER BY id')
     while IFS= read -r organization_id; do
       [ -n "$organization_id" ] || continue
       docker compose run --rm --no-deps bank-data-rekey ./main bank-data-rekey \
         --organization "$organization_id" --batch-size 100
     done <<< "$organization_ids"
   )
   ```

   Mit SIGTERM abbrechbar; abgeschlossene Batches bleiben erhalten. Derselbe
   Block setzt den Lauf durch idempotente Wiederholung ab null fort.
4. Den Block ein zweites Mal vollständig ausführen. Nur bei fehlerfreiem Lauf
   mit `changed_total=0` **für beide Quellen jedes Trägers** fortfahren. Auch
   widerrufene Mandate und eingereichte Lastschriftkopien sind enthalten.
5. `BANK_DATA_LEGACY_KEYS` per SOPS leeren, ausrollen und Prozesse neu starten.
   Danach denselben Block ohne Legacy-Ring vollständig ausführen:
   alles lesbar, beide Quellen je Träger `changed_total=0`.
   `HEALTH_DATA_KEYS` unverändert lassen; Gesundheitsdaten benötigen ihn.
   Diese komplette Reihenfolge gilt separat für Staging, Produktion und Demo.

## Mutationsbelege

- M4/M5: Mandate oder Lastschriften mit `healthData` statt `bankData` verdrahten:
  der Compiler weist den fehlenden Marker `BankDataRing` zurück.
- M8: `debits` aus der Quelleniteration in `runBankDataRekeyJob` entfernen:
  `TestBankDataRekeyJobOpensBothSourcesWithoutLegacy` scheitert beim Öffnen der
  Lastschriftkopie mit dem Bankring ohne Legacy. Der Test nutzt dieselbe
  vollständige Traversierung wie die CLI und prüft beide Felder beider Tabellen
  sowie einen zweiten vollständigen Bank-only-Lauf ohne Änderungen.

## Rotation, Abbruch und Rollback

- Neue unabhängige Bankschlüssel mit neuer ID vorne im Bankring ergänzen;
  bisherige Bankschlüssel zum Lesen behalten. Ausrollen, Job für alle Träger,
  zweiter vollständiger Durchlauf mit `changed_total=0`, dann alte Bankschlüssel aus
  dem Online-Ring entfernen. Der Gesundheitsring bleibt unverändert.
- Vor dem ersten neuen Bankchiffrat kann die alte Anwendung zurückgerollt
  werden. Danach nur eine Version mit Bankring-Unterstützung verwenden; ein
  altes Image allein ist kein Rollback. Alternativ vollständiges passendes
  Release-Backup samt Schlüsseln nach dem bestehenden Restore-Verfahren.
- Fehler/Abbruch verliert höchstens den laufenden Batch, nie frühere Commits.
  Ringspezifikation korrigieren und vollständig wiederholen. Schlüssel niemals
  entfernen, solange ein Durchlauf Fehler oder alte Schreiber zeigt.
- Alte Schlüssel separat geschützt so lange aufbewahren, wie Backups mit ihren
  Chiffraten wiederhergestellt werden dürfen. Restore verwendet zuerst deren
  damalige Ringe und wiederholt dann Rekey und Abschlusskontrolle. Online-
  Entfernung ist keine Freigabe zur Vernichtung der Backup-Schlüssel.

## Architektur und verworfene Alternativen

ADR 0066: Beide Tabellen gehören `parent-billing`; Security Runtime kennt
keine Banktabellen. ADR 0067: Cipher und Rewrapper sind consumer-owned Ports.
ADR 0068: Owner-Komposition baut Anwendung und Adapter, CLI und API binden nur.
Kein Cross-Owner-Schreibzugriff. Keine Migration erforderlich. Der zusätzliche
Rekey-Konstruktor erhöht die Compose-Fläche um 5 Zeilen (91 → 96), der Bank-Cipher-Konstruktor um weitere 6 (7 → 13);
die Summe steigt 13147 → 13158;
die Grenze bleibt auf die notwendige Bindung beschränkt.

Ein dauerhafter gemeinsamer Ring koppelt Kompromittierung und Rotation.
Ein stiller Konfigurationsfallback verletzt Fail-fast. Eine Datenmigration
würde Deploy-Zeit und Rekey-Dauer koppeln. Ein dauerhaft gelockerter
Unveränderlichkeits-Trigger würde HTTP-Schreibern einen unnötigen Weg eröffnen.

## Exakte Methoden-Evidenz

`BankCipher.Rewrap` ist als einzelne neue Methoden-Evidenz unter ADR 0183
registriert. Die Methode authentifiziert aktive bzw. ausdrücklich zugelassene
Legacy-Werte unter dem ursprünglichen AAD, erhält bereits aktuelle Bytes und
versiegelt sonst mit dem Bankring. Der Scanner zählt den nativen, eingebetteten
`Seal`-Aufruf; das ist keine ungeschützte Repository-Durchreiche. Die Evidenz
benennt genau diesen Quelltext. Selektoren, Ausnahmen und die bisherigen
Owner-Pass-through-Floors werden nicht erhöht oder gelockert.

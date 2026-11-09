---
status: proposed
---

# Bankdaten haben einen eigenen Schlüsselring und einen wiederholbaren Rekey

**Status: Vorgeschlagen.** Vorschlag vom 09.11.2026 zu #6252, Teil von #4611.
Er ersetzt für Bankdaten die gemeinsame Schlüsselverwendung aus ADR 0171.

## Entscheidung und Invarianten

1. Security Runtime stellt denselben AES-256-GCM-Feld-Cipher bereit, aber die
   Root bindet `BANK_DATA_KEYS` getrennt von `HEALTH_DATA_KEYS` an den
   consumer-owned Port `parentbilling.BankDataCipher`. Fehlende, leere oder
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
   von IBAN/Kontoinhaber in `sepa_debits` (#6265), sobald dessen Tabelle besteht.
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

1. In allen SOPS-Umgebungen Staging, Produktion und Demo per SOPS eigenständige
   Bank-Schlüssel erzeugen und `BANK_DATA_KEYS` setzen; niemals Dev-Testwerte
   übernehmen. `BANK_DATA_LEGACY_KEYS` ausdrücklich mit dem bisherigen Ring
   setzen. Env-Sync-Prüfung erneut ausführen. Die Übergangsausnahme im
   Env-Inventar anschließend entfernen. Verschlüsselte Dateien niemals von
   Hand bearbeiten. Die serving-Compose-Bindung bleibt bereits fail-fast.
2. Kompatible Version mit beiden Leseringen ausrollen. Alte Instanzen erst
   vollständig beenden, bevor der Job oder neue Bank-Schreibvorgänge erfolgen:
   eine alte Version versteht neue Bankchiffrate nicht. Normaler Rollout ersetzt
   den Prozess; während des Rekeys bleibt die neue Anwendung verfügbar.
3. Wartungscontainer mit privilegiertem DB-DSN und ausschließlich Bankring plus
   explizitem Legacy-Ring starten. Bestehenden `migrate`-Service mit `--entrypoint`
   nicht verändern; für den einmaligen Aufruf die beiden Schlüssel als
   Prozessumgebung übergeben, nicht als Shell-Literal oder Kommandozeilenwert.
   Pro Träger `./main bank-data-rekey --organization ID --batch-size 100`
   ausführen. Mit SIGTERM abbrechbar; abgeschlossene Batches bleiben erhalten.
   Dieselbe Anweisung setzt den Lauf durch idempotente Wiederholung fort.
4. Nach vollständigem Ausrollen und nachdem keine alten Schreiber mehr laufen,
   für jeden Träger erneut vollständig laufen lassen. Bei Fehlern nicht
   abschalten. Erfolgreicher Durchlauf mit `changed_total=0` authentifiziert den
   gesamten vorhandenen Bestand. Auch widerrufene Mandate und vorhandene
   Lastschriftkopien müssen darin enthalten sein.
5. `BANK_DATA_LEGACY_KEYS` per SOPS leeren und neue Prozesse starten. Den Job
   noch einmal ohne Legacy-Ring laufen lassen: alles lesbar, `changed_total=0`.
   Der Bankbestand ist jetzt unabhängig von Gesundheitsdatenschlüsseln.
   `HEALTH_DATA_KEYS` wird nicht verändert; die Gesundheitsdaten benötigen ihn.

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
Rekey-Konstruktor erhöht die Compose-Fläche um 5 Zeilen (86 → 91), der Bank-Cipher-Konstruktor um weitere 6 (7 → 13);
die Summe steigt 13142 → 13153;
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

---
status: proposed
---

# Bankdaten von Eltern und Personal liegen versiegelt im Bankring

**Status: Vorgeschlagen.** Vorschlag vom 09.11.2026 zu #6332, Teil von #4611.
Ergänzt [ADR 0183](0183-bankdaten-haben-einen-eigenen-schluesselring-und-einen-wiederholbaren-rekey.md)
um die beiden älteren Tabellen mit Bankdaten. Entscheidungen von Yannick vom
09.11.2026: Steuer-ID und SV-Nummer werden mitversiegelt, es gibt keinen
Blind-Index, der Kontoinhaber der Eltern wird versiegelt gespeichert und in
den Standardantworten weiter unmaskiert angezeigt.

## Entscheidung und Invarianten

1. `users.guardian_financial_data` speichert IBAN und Kontoinhaber nur noch in
   `iban_sealed` und `account_holder_sealed`, dazu `iban_masked`.
   `users.staff_financial_data` speichert IBAN, Steuer-ID und SV-Nummer nur
   noch in `iban_sealed`, `tax_id_sealed` und `social_security_number_sealed`,
   dazu `iban_masked`. Für Steuer-ID und SV-Nummer zeigt die Standardantwort nur,
   dass ein Wert vorhanden ist. Dafür reicht das Chiffrat; eine Maske ist nicht nötig.
2. Versiegelt wird ausschließlich mit Security Runtimes Bank-Cipher
   (`BANK_DATA_KEYS`, ADR 0183). People Directory und Workforce besitzen je einen
   consumer-owned Port `BankDataCipher` mit dem Marker `BankDataRing()`. Der
   Gesundheits-Cipher erfüllt ihn nicht. Die Roots binden `env.BankData`.
   Ohne Cipher starten Gast-Service und Personalakte nicht. Es gibt keinen Env-Fallback.
3. Der Kontext eines Werts ist `users.<tabelle>.<feld>|<tenant_id>|<eigentümer>`
   mit `guardian_profile_id` bzw. `staff_id`. Ein Chiffrat in einer anderen
   Zeile, Schule oder Spalte öffnet nicht. Gesiegelt wird mit der Schule des
   Erziehungsberechtigten bzw. der Schule der Transaktion. Der zusammengesetzte
   Fremdschlüssel `(tenant_id, eigentümer)` macht sie zur Schule der Zeile.
4. Die Standardlesepfade öffnen keine IBAN. Sie liefern die gespeicherte Maske.
   Der Kontoinhaber der Eltern wird für die Standardantwort geöffnet (Entscheidung 3).
   Reveal, Export der Bankverbindungen und der Änderungsvergleich öffnen die
   Werte. Reveal und Export bleiben auditiert. Geöffnet wird erst nach dem
   Zugriffsprotokoll. Fehlermeldungen enthalten keine Werte.
   Der Owner bestätigt Lesen und Zugriffsaudit in einer eigenen Schultransaktion,
   bevor er entschlüsselt. Eine äußere Request-Transaktion und ihre Commit-Hooks
   werden dabei nicht übernommen. So bleibt auch ein fehlgeschlagener Leseversuch
   dauerhaft protokolliert. Die vier Eltern-Leserouten öffnen deshalb keine
   zusätzliche HTTP-Transaktion; ihr SQL-Budget bleibt unverändert.
5. Schreiber speichern nie Klartext. Der Adapter setzt die Klartextspalten bei
   jedem Schreiben auf NULL. Bis zum Backfill liest der Adapter Zeilen ohne
   Chiffrat aus den Klartextspalten. Das ist nur der Übergang bis Migration 1.15.839.
6. Backfill und Rekey sind ein Wartungsbefehl:
   `bank-data-rekey --school ID --batch-size 100`, gleiche Garantien wie ADR 0183
   Punkt 4 bis 6. Je Batch gibt es eine Schultransaktion. Die Rolle wird auf den
   Superuser-Login zurückgesetzt und mit `is_superuser` geprüft. Ein HTTP-Login
   wird abgewiesen. `session_replication_role = replica` gilt nur für die
   Transaktion, damit der `updated_at`-Trigger einen reinen Rewrap nicht
   protokolliert. Der Befehl filtert ausdrücklich nach Schule, sperrt
   `ORDER BY id … FOR UPDATE` ohne `SKIP LOCKED` und schreibt nur Chiffrate,
   Maske und NULL in die Klartextspalten. Klartext wird versiegelt.
   Vorhandene Chiffrate werden authentifiziert und auf den aktiven Schlüssel
   gebracht. Ein Chiffrat geht immer vor einem übrig gebliebenen Klartext.
   Ausgabe: Quelle und Mengen (`guardian_financial`, `staff_financial`).
7. Der Mandantenexport schließt die Chiffrate aus (`excludeSecretColumns`) und
   exportiert die Maske. Das Öffnen versiegelter Spalten für die Mandantenrückgabe
   regeln #6329 (ADR 0184) und #6317 (ADR 0185) für die SEPA-Tabellen. Diese
   Tabellen ziehen nach, sobald dieser Mechanismus auf `development` liegt.
8. Kein Blind-Index (Entscheidung 2). Kein Pfad sucht nach IBAN. Die Eindeutigkeit
   hängt an `guardian_profile_id` bzw. `staff_id`.

## Rollout in zwei Releases

Migrationen haben keinen Schlüssel, und `deploy-remote.sh` führt alle offenen
Migrationen eines Releases aus. Eine Drop-Migration, die bei Klartext
scheitert, kann darum nicht im Release der neuen Spalten liegen. Sonst bricht
das erste Deploy mit Bestandsdaten im Preflight ab.

1. **Release A** (#6332, Migrationen 1.15.836 und 1.15.837): neue Spalten,
   versiegelnde Schreiber, Übergangslesepfad, Wartungsbefehl. Voraussetzung ist
   der Bankring aus ADR 0183 Schritt 1 in allen Umgebungen.
2. In jeder Umgebung (Staging, Produktion, Demo) nach vollständigem Rollout von A
   im Deployment-Verzeichnis (`~/staging`, `~/production`, `~/demo`) diesen
   Block ausführen. Er verarbeitet alle Schulen, auch Demo- und gelöschte
   Testschulen, ohne gepflegte ID-Liste:

   ```bash
   (
     set -euo pipefail
     school_ids=$(docker compose exec -T postgres psql -U postgres -d postgres \
       -At -v ON_ERROR_STOP=1 -c 'SELECT id FROM platform.schools ORDER BY id')
     while IFS= read -r school_id; do
       [ -n "$school_id" ] || continue
       docker compose run --rm --no-deps bank-data-rekey ./main bank-data-rekey \
         --school "$school_id" --batch-size 100
     done <<< "$school_ids"
   )
   ```

   Danach den Block ein zweites Mal vollständig ausführen. Nur bei
   `changed_total=0` für `guardian_financial` und `staff_financial` jeder Schule
   geht es weiter. Vor Release B prüft zusätzlich diese Abfrage auf 0:
   `SELECT count(*) FROM users.guardian_financial_data WHERE iban IS NOT NULL OR account_holder IS NOT NULL`
   und dasselbe für `users.staff_financial_data` mit `iban`, `tax_id` und
   `social_security_number`.
3. **Release B** (Migration 1.15.839, gestapelter PR): entfernt die
   Klartextspalten und den Übergangslesepfad. Ihr `Precondition` bricht den
   Preflight ab, solange eine Zeile noch Klartext trägt. Dann wird nichts
   gestoppt und nichts zurückgespielt.

Solange der Legacy-Ring aus ADR 0183 aktiv ist, verarbeitet derselbe Lauf auch
Chiffrate mit dem Legacy-Ring. Schritt 5 aus ADR 0183 (Legacy-Ring leeren,
Abschlusslauf ohne Legacy) gilt für beide Befehlsformen.

## Rollback

Das Down von 1.15.836 und 1.15.837 verweigert sich, solange ein versiegelter
Wert existiert: Nach einem Schreiben oder dem Backfill halten die versiegelten
Spalten die einzige Kopie. Ein Rollback nach dem ersten Chiffrat braucht eine
Version mit dieser Änderung oder das Release-Backup (ADR 0183).

## Sperren und Wartezeit

1.15.839 (`Up`) und das `Down` von 1.15.836/837 sperren zuerst mit
`ACCESS EXCLUSIVE`, prüfen danach und droppen erst dann, alles in einer
Transaktion. Diese Sperre blockiert auch Leser. Bei 100.000 Zeilen dauert das
lokal etwa 15 ms, und das Deploy stoppt die Anwendung vorher. Eine
Fremdtransaktion, etwa eine Diagnose-Sitzung, könnte die Sperre aber
unbegrenzt verzögern, und die wartende Anforderung blockiert alle späteren
Leser. Deshalb gilt transaktionslokal `lock_timeout = 30s`. Danach bricht die
Migration mit klarer Meldung ab, `deploy-remote.sh` stellt das Backup wieder
her (Exit 10), und das Release lässt sich nach dem Beenden der Transaktion
wiederholen.

## Mutationsbelege

- Health-Cipher statt Bank-Cipher an die neuen Ports:
  `TestHealthCipherCannotServeTheSchoolBankPorts`; im Root verweigert der
  Compiler die Bindung.
- `staff_financial` oder `guardian_financial` aus der Schulschleife entfernt:
  `TestSchoolBankDataRekeyBackfillsAndOpensWithoutLegacy` scheitert beim
  Öffnen mit dem Bankring ohne Legacy.
- Schulfilter im Rekey entfernt: Dieselbe Prüfung findet die Zeilen der
  anderen Schule verändert.
- Kontext ohne Schule, Eigentümer oder Feld:
  `TestGuardianPaymentIsStoredSealedAndOpensOnlyInItsOwnRow` und
  `TestFinancialMaskedReadOpensNothingAndRevealOpensOnlyItsOwnRow`.

## Architektur

ADR 0066: Jede Tabelle bleibt bei ihrem Owner (People Directory, Workforce).
Security Runtime kennt keine dieser Tabellen. ADR 0067: Die Cipher-Ports sind
consumer-owned. ADR 0068: Die Owner-Kompositionen bauen Rekey und Adapter, CLI
und Roots binden nur. Compose-Fläche: `modules/peopledirectory/compose`
973 → 979 (Rekey-Konstruktor), `modules/workforce/compose` 909 → 916
(Cipher-Abhängigkeit und Rekey-Konstruktor), Summe 13158 → 13171.

## Verworfene Alternativen

- Drop im selben Release: Das erste Deploy mit Bestandsdaten bricht ab (siehe oben).
- Backfill in einer Migration: Migrationen haben keinen Schlüssel, und der
  Bankring gehört nicht in den Migrationsprozess.
- Versiegelung im Repository statt im Owner-Service: Dann bräuchte der Adapter
  zusätzlich einen Lesemodus für „Maske ohne Öffnen“. Die Anwendung kennt
  ohnehin Zweck und Zugriffsprotokoll.
- Eigener Ring für Steuer- und SV-Daten: mehr Betriebsaufwand ohne Nutzen
  (Entscheidung 1).

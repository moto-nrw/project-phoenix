---
status: proposed
---

# Trägerpaket enthält PAIN.008-Originale und ein Operator-Protokoll

**Status: Vorgeschlagen.** Vorschlag vom 09.11.2026 zu #6314, Teil von #4720
(Slice 3) und #4611. Grundlage: Löschkonzept Träger-Finanzdaten, Abschnitt 5;
AVV Ziff. 8.2 und 8.4. Setzt auf
[ADR 0184](0184-mandantenrueckgabe-oeffnet-feldverschluesselte-bankdaten.md)
(geöffnete IBAN und Kontoinhaber) und
[ADR 0185](0185-mandantenrueckgabe-gibt-gesundheit-einstellungen-und-traegerpaket-zurueck.md)
(Trägerpaket `tenant export --organization`) auf; ergänzt ADR 0181
(PAIN.008-Datei) und ADR 0183 (Bank-Schlüsselring).

## Kontext

Das Trägerpaket von ADR 0185 gibt die Zeilen der Mandate und Lastschriften
mit geöffneter IBAN und Kontoinhaber zurück (ADR 0184). Drei Dinge fehlen
für die Rückgabe am Vertragsende:

- Nach der Deaktivierung kann das Träger-Büro seine PAIN.008-Dateien nicht
  mehr herunterladen. ADR 0181 speichert keine Datei; die Zeilen allein sind
  nicht die Datei, die die Bank erhielt.
- Das Paket enthält vollständige Bankdaten, aber kein Lauf steht im
  Operator-Protokoll (#4720: jeder Zugriff auf Personendaten wird
  protokolliert).
- Ein Löschauftrag eines Trägers lief ohne Hinweis auf die Rückgabe der
  Unterlagen und ohne Wahl des Kunden.

## Entscheidung und Invarianten

1. **PAIN.008: Original vor Rekonstruktion.** Ergänzung zu ADR 0181: Der
   Download speichert die ausgegebene Datei bytegenau, mit dem
   Bank-Schlüssel versiegelt und an Träger und Nachrichten-ID gebunden, mit
   SHA-256 und Gläubigerkonto (`parent_billing.sepa_debit_files`, Migration
   `1.15.833`). Ein späterer Download ersetzt sie bis zur Bestätigung der
   Einreichung; danach ändert sie sich nicht mehr (nur `bank-data-rekey`
   versiegelt neu). Die Bestätigung hält ohne Download wenigstens das
   Gläubigerkonto fest. Jede gespeicherte Datei nennt ab dem Download ihre
   Mitglieder: die Lastschriften, die Bescheide ihrer noch nicht
   geschriebenen Raten und die Mandate, deren Konto sie hält. Das Speichern
   sperrt die Mitglieder (`FOR KEY SHARE`, Trigger
   `lock_sepa_debit_file_members`) und verweigert ein Mitglied, das nicht
   mehr besteht: Ein Download, dessen Mitglied nach dem Lesen des Laufs
   gelöscht wurde, speichert nichts und meldet
   `parent_billing.sepa_debits_changed` (409). Eine spätere Löschung wartet
   auf den Download und nimmt die Datei mit.
   - **Eingereichte Datei** (Original): Beleg dessen, was die Bank erhielt.
     Sie geht erst mit der letzten ihrer Lastschriften (nach ID oder nach
     der Nachricht, mit der sie eingereicht wurden) und Bescheide, nie,
     solange eines davon bleibt, etwa unter einer Aussetzung der Belegfrist.
     Ein Mandat allein nimmt sie nicht mit. Der Trigger sperrt die Datei,
     bevor er die verbleibenden Mitglieder zählt; von zwei Transaktionen,
     die die letzten beiden Lastschriften löschen, sieht die zweite die
     erste und löscht die Datei.
   - **Nicht eingereichter Download:** belegt nichts, hält aber die Konten
     seiner Familien. Er geht mit dem ersten Mitglied, auf welchem Weg auch
     immer, wenn die Bestätigung einer anderen Nachricht desselben Laufs
     ein Mitglied mit ihm teilt, und 30 Tage nach dem Download (stündliche
     Bereinigung, `parent_billing.delete_expired_sepa_debit_downloads`).
   - **Rechte:** `phoenix_tenant` speichert und bestätigt Dateien, löscht
     aber keine. Jede Löschung läuft in einer Owner-Funktion
     (`SECURITY DEFINER`, `search_path = pg_catalog`): der Mitglieder-
     Trigger (je Träger der gelöschten Zeile), die Verdrängung bei der
     Bestätigung (`drop_superseded_sepa_debit_files`, nur für den Träger der
     Transaktion) und der Ablauf (nur `phoenix_admin`, über alle Träger,
     nur nicht eingereichte Downloads).
2. **Zusätzlicher Dateisatz im Trägerpaket.** Das Trägerpaket legt je
   eingereichte Nachricht eine Datei `SEPA-LASTSCHRIFTDATEIEN/<Nachrichten-ID>.xml`
   ab: das gespeicherte Original, geprüft gegen seine SHA-256, sonst die
   Neuerzeugung mit demselben Generator aus `sepa_debits` und dem
   Gläubigerkonto der Einreichung, für ältere Nachrichten mit dem heutigen
   Standardkonto des Trägers. Abweichend ist dann `CreDtTm` (Bestätigung der
   Einreichung). `LIESMICH.txt` im Ordner und `manifest.json` unter
   `sepaDebitFiles` nennen je Datei „Original“ oder „rekonstruiert“ mit
   Grund; das Manifest prüft jede Datei mit Prüfsumme. Die Zeilen von
   `parent_billing.sepa_debit_files` stehen im Katalog-Export ohne
   `file_sealed`. Die Mandate und Lastschriften mit IBAN und Kontoinhaber
   bleiben allein Sache von ADR 0184; dieser Dateisatz gibt keinen Wert ein
   zweites Mal als Feld aus.
3. **Owner-Vertrag:** Parent Billing besitzt Schlüsselzugriff und Generator.
   Sie veröffentlicht `parentbilling.SEPAReturn` (nur Lesen, keine Zeile wird
   geschrieben) und den consumer-owned Port `SEPACreditorAccounts` für das
   Standardkonto des Trägers. Organisation & Tenancy definiert den
   consumer-owned Port `TenantExportSEPADebitFiles` und öffnet nie einen
   versiegelten Wert. Nur die CLI-Root `cmd/tenant_return.go` bindet beide;
   keine HTTP-Route, kein Workflow-Owner, kein Lesen fremder Tabellen.
4. **Transaktionen:** Erst der Read-only-Snapshot des Katalogs von ADR 0185,
   nach dessen Commit eine Träger-Transaktion von Parent Billing für die
   Dateien. Nie beide gleichzeitig; ein gemeinsamer Snapshot hieße, die
   Transaktion eines Owners an den anderen zu reichen. Darum läuft das
   Trägerpaket offline mit gestoppten Schreibern (Runbook), nicht nur mit
   deaktiviertem Träger.
5. **Operator-Protokoll:** Jeder Lauf von `tenant export --organization`
   verlangt `--operator` mit einem bestehenden, aktiven Operator (wie das
   Löschverfahren). Vor dem ersten Lesen steht der Versuch im
   Operator-Protokoll (Aktion `export`, Ressource `organization`,
   `outcome: started`), danach das Ergebnis: `completed` mit Prüfsumme aus
   `COMPLETE`, Anzahlen und den Herkünften der PAIN.008-Dateien vor
   `COMPLETE`, oder `failed` mit Fehlerklasse und ob die Teildateien
   gelöscht wurden. Keine Personendaten, kein lokaler Pfad. Ein Lauf, dessen
   Abschluss nicht protokolliert ist, bekommt kein `COMPLETE`. Ohne
   Dateisatz und Protokoll verweigert der Exporter ein Trägerpaket. Den
   Operator liest die Root über einen eigenen Konstruktor von Identity &
   Access (`NewOperatorDirectory`). Das Schulpaket bleibt ohne Protokoll
   (#6326).
6. **Löschauftrag eines Trägers:** Er verlangt den Hinweis auf die Rückgabe
   der Unterlagen und die Wahl des Kunden (Rückgabe oder Verzicht;
   Migration `1.15.832`). Ein Altauftrag ohne beides und ein bereits
   begonnener Auftrag laufen erst weiter, wenn beides nachgetragen ist
   (`PUT /operator/organizations/{id}/erasure-order/records`,
   Operator-Protokoll `record_erasure_order_records`); eine erfasste Wahl
   ändert sich nicht mehr. Die eigene Fähigkeit liegt neben den Fassaden
   (ADR 0178) und wird bei der Konstruktion übergeben, nie zur Laufzeit am
   Dienst gesucht (ADR 0068 Punkt 5): `compose.NewProvisioning` baut den
   Befehl des Owners, die Route hängt nur an
   `TraegerErasureOrderRecordsProvisioning`
   (`NewTraegerErasureOrderRecordsResource`, ohne sie kein Router).
7. **Konfiguration:** Das Trägerpaket verlangt `BANK_DATA_KEYS` wie
   `bank-data-rekey` und ADR 0184; ohne gültigen Ring bricht es ab. Kein
   Rückfall.

## Folgen

- Compose-Budgets: `modules/parentbilling/compose` +7 (`NewSEPAReturn`) und
  +5 (`NewSEPADebitDownloadRetention` für den stündlichen Ablauf der
  Downloads), `modules/organizationtenancy/compose` +13
  (`NewTraegerTenantExporter`) und +10 (Befehl der nachgetragenen Wahl),
  `modules/identityaccess/compose` +11 (`NewOperatorDirectory`).
- Das Ausnahmeregister der Demo-Löschung wächst um
  `parent_billing.sepa_debit_files` (+1): Die Datei geht per Trigger mit
  ihren Lastschriften (eine eingereichte mit der letzten), die mit den
  Verträgen des Demo-Besuchs gehen.
- Das Trägerpaket enthält Bankdaten der Familien in den Tabellen-Dateien
  (ADR 0184) und in `SEPA-LASTSCHRIFTDATEIEN/`. Die Übergabe bleibt ein
  Mensch-Schritt: nur verschlüsselt, lokale Kopie nach bestätigtem Empfang
  löschen (Runbook Auskunft und Rückgabe).

---
status: proposed
---

# Mandantenrückgabe hat ein Register und einen Abrufweg

**Status: Vorgeschlagen.** Vorschlag vom 10.11.2026 zu #3838. Setzt die
Entscheidung „Data Act Option B“ vom 10.11.2026 um
([Issue-Kommentar](https://github.com/moto-nrw/project-phoenix/issues/3838#issuecomment-6776609298))
und ergänzt [ADR 0097](0097-mandantenrueckgabe-hat-einen-eigenen-leseadapter.md),
[ADR 0192](0192-mandantenrueckgabe-oeffnet-feldverschluesselte-bankdaten.md) und
[ADR 0193](0193-mandantenrueckgabe-gibt-gesundheit-einstellungen-und-traegerpaket-zurueck.md).

## Kontext

`tenant export` erstellt ein vollständiges Rückgabepaket einer Schule oder eines
Trägers. Felder, Typen, Nullbarkeit und Beziehungen stehen nur in der
`model.json` **im Paket**. Art. 26 lit. b Data Act verlangt ein aktuelles
Online-Register mit „Einzelheiten zu allen Datenstrukturen und Datenformaten“,
das dem Kunden schon bei Vertragsschluss zur Verfügung steht
(§ 15 Abs. 2 Nr. 21 DADG). Die Rechtsrecherche vom 09.11.2026 hält den Verweis
auf die `model.json` im Paket deshalb für nicht ausreichend. Öffentlich muss
das Register nicht sein; Zugang für Kunden genügt.

Art. 30 Abs. 2 verlangt zusätzlich eine dokumentierte, unentgeltliche offene
Schnittstelle für Kunde und Zielanbieter. Die Gründer haben am 10.11.2026
Option B gewählt: Register aus dem Datenkatalog und ein HTTPS-Abruf des von uns
erstellten und geprüften Pakets mit kurzlebiger Berechtigung. Kein
Self-Service-Export, keine allgemeine Export-API, keine Importer, kein SLA.

## Entscheidung

### A. Register (Art. 26)

1. **Eine Quelle.** Das Register entsteht aus dem Katalog
   (`data_lifecycle_catalog.json`) und dem migrierten Schema, mit demselben
   Code wie `model.json`: `loadExportSchema` unter der Leserolle
   `phoenix_tenant_export` und `exportTableModel` je Tabelle. Es liest keine
   Zeile.
2. **Eingebettet und geprüft.** Das Ergebnis liegt als
   `backend/modules/organizationtenancy/return_register.json` im Repository und
   im Binary (`organizationtenancy.ReturnRegisterSource`).
   `TestReturnRegisterMatchesMigratedSchema` erzeugt es aus der migrierten
   Testdatenbank neu und schlägt fehl, wenn eine Migration oder Katalogänderung
   das Paket ändert, ohne das Register neu zu erzeugen
   (`-update-return-register`). `TestTenantExportMigratedSchema` vergleicht
   jede Tabelle der `model.json` eines echten Exports mit dem Register.
3. **Inhalt.** Je Tabelle die Felder wie in `model.json` (Name, SQL-Typ, UDT,
   Nullbarkeit), die Fremdschlüssel, die Pakete, die sie tragen (`school`,
   `organization`), und die ausgeschlossenen Spalten mit Grund. Dazu die
   ausgeschlossenen Tabellen und Dateiarten mit Grund (Art. 25 Abs. 2 lit. f),
   der Paketaufbau, die Darstellung der Werte, die Formatversion
   (`ExportFormatVersion`, auch im Manifest), Katalogversion und
   SHA-256 des Katalogs (auch im Manifest), die Standards (JSON, JSON Lines,
   UTF-8, SHA-256, ISO 8601, SQL-Typen, ZIP, PAIN.008), der Hinweis, dass es nach
   Art. 35 noch keine gemeinsamen Spezifikationen gibt, und eine unverbindliche
   Zeitschätzung („voraussichtlich“).
4. **Auslieferung.** `GET /public/data-return/register` liefert die
   eingebettete Datei ohne Anmeldung. Das ist der einfachste Weg, der Art. 26
   erfüllt: Das Register enthält keine Kundendaten, das Schema ist mit dem
   öffentlichen Quellcode ohnehin öffentlich, und Kunde wie Zielanbieter
   brauchen es vor jedem Vertrag oder Wechsel. Weil die Datei im Binary liegt,
   beschreibt sie immer genau das Paket, das dieselbe Installation erzeugt;
   ein Register auf `development` oder der Website könnte dem ausgerollten
   Stand vorauslaufen. Ein Login-Bereich hätte Zielanbieter ausgeschlossen
   und eine Oberfläche verlangt, ohne Schutz zu gewinnen.

### B. Abrufweg (Art. 30 Abs. 2)

1. **Wir erstellen und prüfen weiter selbst.** `tenant export` bleibt der
   einzige Weg zum Paket. `tenant return release` nimmt nur ein Paket an,
   dessen `COMPLETE` zur `manifest.json` passt, dessen gelistete Dateien
   Größe und SHA-256 treffen, das keine weitere Datei, keinen Link und kein
   Personen-Prüfpaket (`reviewRequired`) enthält und dessen eigene Zeile in
   `platform.schools` bzw. `platform.organizations` genau die Schule oder den
   Träger nennt, die der Operator angibt. Ein Paket für die falsche Schule
   lässt sich so nicht freigeben.
2. **Ablage verschlüsselt.** Die CLI packt das Paket als ZIP und verschlüsselt
   es dabei (AES-256-GCM in Blöcken zu 64 KiB, Blockzähler und Endmarke im
   Nonce, Security Runtime) mit einem eigenen Zufallsschlüssel je Paket in
   die Ablage `data-return/` (Volume `phoenix-<env>-data-return`). Der
   Klartext berührt die Platte nicht. Die Datenbank speichert den Schlüssel nur
   eingewickelt unter dem jeweiligen Token und einen HMAC-Prüfwert; das Token
   selbst nur als HMAC. Ablage und Datenbank zusammen öffnen ohne gültiges
   Token kein Paket. Den Paketschlüssel bekommt der Operator einmal angezeigt;
   damit stellt er weitere Tokens aus, ohne erneut hochzuladen.
3. **Kurzlebige Berechtigung, eine Zeile je Empfänger.**
   `platform.data_return_grants` hält je Token Empfängerart (`customer`,
   `target_provider`) und Name der Empfängerorganisation, Ablauf (höchstens
   30 Tage, nie nach dem Ende des Abrufzeitraums `keep_until`) und
   Downloadbudget (höchstens 20). Widerruf (`tenant return revoke`) und Ablauf
   wirken sofort. Jedes Token öffnet genau das Archiv seiner Zeile; keine
   Anfrage wählt Schule, Träger oder Datei. Damit gibt es keinen Weg, mit dem
   Token einer Schule das Paket einer anderen zu holen.
4. **HTTPS-Schnittstelle.** `GET /public/data-return/package` nennt Paketart,
   Formatversion, Prüfsummen, Größe, Ablauf und verbleibende Downloads;
   `GET /public/data-return/package/archive` liefert das ZIP mit
   `Content-Length` und `Repr-Digest` (RFC 9530). Das Token kommt nur im
   Header `Authorization: Bearer`, nie in der URL; Sentry filtert den Header.
   Jedes unbrauchbare Token bekommt dieselbe 401. Eigene Ratenbegrenzung je
   Adresse (30 Anfragen pro Minute). `GET /public/data-return/openapi.json`
   beschreibt die Schnittstelle (OpenAPI 3.1). Unentgeltlich, gleich für Kunde
   und Zielanbieter.
5. **Nachweis.** Freigabe, weiteres Token, Widerruf, Auflistung und Bereinigung
   prüfen den aktiven Operator und stehen im Operator-Protokoll (Aktion
   `data_return_*`, ohne Token, Schlüssel oder Personendaten). Jeder Abruf steht
   mit Zeitpunkt, Ergebnis und Bytes in `platform.data_return_downloads`.
6. **Ende.** `tenant return purge` löscht ein Archiv nach dem Abrufzeitraum
   oder sobald alle seine Tokens widerrufen sind, und leert die eingewickelten
   Schlüssel. Die Zeilen bleiben als Nachweis der Rückgabe ohne
   Schul-Fremdschlüssel (wie `platform.school_erasures`) und gehen drei Jahre
   nach dem Abrufzeitraum. Vor einer Löschung von Schule oder Träger wird
   bereinigt.

Verfügbarkeit: Die Schnittstelle besteht dauerhaft; ein Paket ist ab seiner
Freigabe nach der Wechselanfrage bis zum Ende des Abrufzeitraums abrufbar,
Tokens werden in dieser Zeit bei Bedarf neu ausgestellt.

## Folgen

- Jede Migration, die eine zurückgegebene Tabelle ändert, ändert auch
  `return_register.json` im selben PR. Der Test nennt den Befehl.
- Die Website und AGB Ziff. 10.2 verlinken das Register unter der API-Adresse
  der Installation (`/public/data-return/register`). Das ist eine eigene
  Änderung im Website- und Legal-Repo.
- Ältere Formatversionen hält das Register nicht vor. Ändert sich das Format,
  steigt `ExportFormatVersion`; das Manifest jedes Pakets nennt seine Version.

- Das Paket enthält Klartext-IBAN und Gesundheitsangaben. Wer ein Token
  hat, hat das Paket. Tokens gehen nur über einen bestätigten zweiten Kanal an
  die benannte Organisation; die Laufzeit bleibt kurz.
- Die Ablage ist nicht Teil der täglichen Sicherung. Nach einem Restore fehlt
  ein Archiv; der Abruf antwortet 503, der Operator gibt neu frei.

## Verworfen

- **Nur Verweis auf `model.json` im Paket:** erfüllt Art. 26 lit. b nach der
  Recherche wahrscheinlich nicht; der Kunde sieht vor dem Export nichts.
- **Register live aus der Datenbank je Anfrage:** jede Anfrage liest das
  Schema; die HTTP-Rolle sieht nicht jede Spalte, die die Exportrolle sieht.
- **Selbstbedienungs-Export oder allgemeine Daten-API:** nicht Teil der
  Entscheidung vom 10.11.2026.
- **Archiv-Schlüssel aus einem neuen Server-Schlüsselring:** bräuchte neue
  Geheimnisse in allen Umgebungen und läge neben der Ablage; der Schlüssel
  unter dem Token schützt besser und braucht keine neue Konfiguration.
- **Token in der URL (Link zum Klicken):** landet in Logs, Verläufen und
  Mail-Scannern.
- **Register im Website-Repo:** zweite Quelle, die vom ausgerollten Stand
  abweichen kann.

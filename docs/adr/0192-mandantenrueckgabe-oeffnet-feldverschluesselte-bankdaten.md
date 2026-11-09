---
status: proposed
---

# Mandantenrückgabe öffnet feldverschlüsselte Bankdaten

**Status: Vorgeschlagen.** Vorschlag vom 09.11.2026 zu #3838, ergänzt
[ADR 0097](0097-mandantenrueckgabe-hat-einen-eigenen-leseadapter.md) und
[ADR 0183](0183-bankdaten-haben-einen-eigenen-schluesselring-und-einen-wiederholbaren-rekey.md).

## Kontext

`tenant export` liest die Katalogtabellen generisch per SQL. IBAN und
Kontoinhaber der SEPA-Mandate und -Lastschriften liegen mit dem Bankring
versiegelt in der Datenbank. Der Export ließ die IBAN aus und gab den
Kontoinhaber der Mandate als Chiffrat aus. Beides ist für den Kunden und einen
neuen Anbieter wertlos: Ohne Konto kann niemand weiter einziehen. Die
öffentliche Seite „Datenexport und Anbieterwechsel“ sagt aber zu, dass
Abrechnungsdaten maschinenlesbar zurückkommen.

## Entscheidung

1. Der Katalog führt je Tabelle `openSealedColumns`: versiegelte Spalte →
   neuer Klartext-Spaltenname. Die versiegelte Spalte bleibt in
   `excludeSecretColumns`; ihr Chiffrat verlässt den Export nie.
   `model.json` beschreibt die Klartextspalte als `text`.
2. Organization Tenancy definiert den consumer-owned Port
   `TenantExportSealedFields`. Der Exporter liest die versiegelten Bytes
   hex-kodiert im selben Snapshot und übergibt sie mit der exportierten Zeile.
3. Die Elternabrechnung öffnet über `parentbilling.SEPAExportFields`: Sie bindet
   den Wert wie beim Versiegeln an Träger und Mandatsreferenz der Zeile. Eine
   kopierte oder fremde Zeile öffnet nicht. Fehler nennen keinen Wert.
4. Nur die Root `cmd/tenant.go` bindet beides, mit `BANK_DATA_KEYS` und
   gegebenenfalls `BANK_DATA_LEGACY_KEYS` der Herkunftsinstallation. Kein
   HTTP-Pfad bindet den Opener.
5. Kann der Export einen versiegelten Wert nicht öffnen, bricht er ohne
   `COMPLETE` ab. Er gibt weder Chiffrat noch eine stille Lücke weiter.
6. Der Katalog-Stolperdraht erkennt Spaltennamen mit `sealed`; eine neue
   versiegelte Spalte braucht damit Ausschluss oder Öffnung.

## Folgen

- Das Rückgabepaket enthält Klartext-IBANs. Übergabe weiter nur verschlüsselt
  nach dem Runbook; die Schlüssel gelangen nur per SOPS in die Prozessumgebung.
- Die Compose-Budgets steigen um den Konstruktor `NewSEPAExportFields`
  der Elternabrechnung (+5 LOC).
- Gesundheitsangaben (`health_data`) sind ebenfalls feldverschlüsselt und
  gehen noch als Chiffrat raus. Sie nutzen künftig denselben Port mit dem
  Gesundheitsring und ihrem Besitzer (#6328).

---
status: proposed
---

# Mandantenrückgabe öffnet Bankdaten von Eltern und Personal und den Mandatstext

**Status: Vorgeschlagen.** Vorschlag vom 10.11.2026 zu #6412 (Teil von
#3838), setzt [ADR 0192](0192-mandantenrueckgabe-oeffnet-feldverschluesselte-bankdaten.md)
fort und folgt aus
[ADR 0188](0188-bankdaten-von-eltern-und-personal-liegen-versiegelt-im-bankring.md).

## Kontext

Seit #6332 und #4696 liegen weitere Werte mit dem Bankring versiegelt in der
Datenbank. Der Katalog schloss ihr Chiffrat aus, geöffnet wurden sie nicht:

- `users.guardian_financial_data`: IBAN und Kontoinhaber der Eltern; nur die
  maskierte IBAN ging mit.
- `users.staff_financial_data`: IBAN, Steuer-ID und Sozialversicherungsnummer
  des Personals.
- `parent_billing.sepa_mandate_consents`: der bestätigte Mandatstext. Version
  und SHA-256 gingen mit, der Text nicht. Der Gläubiger trägt die Beweislast
  für das Mandat.

Der Mandatstext ist an die Mandatsreferenz gebunden. Die Einwilligungszeile
nennt ihr Mandat nur über `mandate_id`; der Opener aus ADR 0192 bekommt aber
nur die exportierte Zeile.

## Entscheidung

1. People Directory öffnet `users.guardian_financial_data` über
   `peopledirectory.GuardianBankExportFields`, Workforce
   `users.staff_financial_data` über `workforce.StaffBankExportFields`. Beide
   binden den Wert wie beim Versiegeln an Feld, Schule und Elternteil bzw.
   Beschäftigte. Ein kopierter Wert, eine fremde Zeile oder ein anderes Feld
   öffnet nicht; der Export bricht dann ohne `COMPLETE` ab.
2. Die geöffneten Spalten heißen `iban_opened`, `account_holder_opened`,
   `tax_id_opened` und `social_security_number_opened`. Die Klartextspalten
   `iban`, `account_holder`, `tax_id` und `social_security_number` bestehen bis
   zu ihrem Drop (#6363) weiter und tragen nur Zeilen, die der Backfill noch
   nicht versiegelt hat. Der Katalog-Stolperdraht verbietet, dass ein
   geöffneter Wert eine bestehende Spalte überschreibt. Die Namen bleiben auch
   nach dem Drop, damit das Format stabil bleibt.
3. Der Katalog führt je Tabelle `sealedBindings`: ein Name, unter dem der
   Besitzer einen Wert einer anderen Katalogtabelle aus demselben Snapshot
   liest, mit Zieltabelle, Zielspalte und den Spaltenpaaren (`match`), über
   die genau eine Zeile gefunden wird. Der Exporter liest ihn als korrelierte
   Abfrage, prüft alle Namen gegen das Schema und entfernt den Wert, bevor die
   Zeile geschrieben wird; er steht weder im Paket noch in `model.json`.
   Die Einwilligung bindet so `mandate_reference` über `mandate_id` und
   `organization_id` an `parent_billing.sepa_mandates.reference`. Parent
   Billing öffnet `text_sealed` mit `parentbilling.SEPAExportFields` in die
   Spalte `text`.
4. Die Personenauskunft (`--person`) markiert Zeilen mit geöffneter Steuer-ID
   oder Sozialversicherungsnummer mit
   `especially-sensitive-tax-and-social-security-ids` und verlangt die Prüfung
   `tax-and-social-security-ids-only-for-the-staff-member-concerned`.
5. Nur die Root `cmd/tenant.go` bindet die Opener mit dem Bankring der
   Herkunftsinstallation, wie ADR 0192 Punkt 4. Kein HTTP-Pfad bindet sie.

## Folgen

- Das Schulpaket enthält Klartext-IBANs, Steuer-IDs und
  Sozialversicherungsnummern. Übergabe weiter nur verschlüsselt nach dem
  Runbook; ohne den Bankring bricht auch der Export einer Schule ab.
- Die Compose-Budgets steigen um die Konstruktoren
  `NewGuardianBankExportFields` von People Directory (+5 LOC) und
  `NewStaffBankExportFields` von Workforce (+5 LOC).
- Eine weitere Bindung über Tabellen hinweg braucht nur einen Katalogeintrag;
  der Stolperdraht prüft, dass ihre Spalten bestehen, keine Spalte der Tabelle
  verdecken und die Tabelle etwas öffnet.

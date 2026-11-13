---
status: accepted
---

# Dokumente lesen den Kontakt des Trägers ohne Konto

Entscheidung vom 08.11.2026 für
[#6200](https://github.com/moto-ogs/project-phoenix/issues/6200). Sie gibt den
Stammdaten des Trägers ([ADR 0112](0112-stammdaten-des-traegers-liegen-bei-organisation-und-tenancy.md))
einen Kontakt für Widerrufe und Organisation & Tenancy eine Leseoperation ohne
Konto. Kein neuer Owner, keine neue Tabelle, keine Policy-Regel.

## Kontext

Die Widerrufsbelehrung des Betreuungsvertrags folgt dem Muster der Anlage 1 zu
Art. 246a EGBGB ([#6105](https://github.com/moto-ogs/project-phoenix/issues/6105)).
Gestaltungshinweis 2 verlangt beim Adressaten Name, Anschrift, Telefonnummer
und E-Mail-Adresse. Das PDF und das Anmeldeformular nennen Telefon und E-Mail
der Ansprechperson. Die Unterschriftsseite und die Bestätigungsmail kannten
nur den Briefkopftext: Der öffentliche Unterschriftslink hat kein Konto des
Trägers, und `TraegerMasterData.Profile` verlangt eines, weil es jedes Lesen
mit dem lesenden Konto protokolliert.

Eine Ansprechperson für Briefe ist oft nicht die Stelle, an die ein Widerruf
gehen soll. Träger nennen dafür häufig ein zentrales Postfach.

## Entscheidung

- `platform.traeger_profiles` erhält `withdrawal_phone` und `withdrawal_email`
  (Migration 1.15.820), geprüft wie Telefon und E-Mail der Ansprechperson. Ein
  leeres Feld nimmt die Angabe der Ansprechperson, Feld für Feld. Ein Träger
  ohne eigene Angabe druckt also, was er vorher druckte.
- `UpdateWithdrawalContact` speichert beide Felder unter der Zeilensperre des
  Profils und protokolliert die Änderung als Gegenstand `withdrawal`.
  Schreiben darf, wer im Träger-Büro das Recht „Einstellungen“ hat.
- `DocumentContact` liest Ansprechperson und Kontakt für Widerrufe in der
  Träger-Transaktion des Aufrufers, ohne Konto und ohne Lesevermerk. Wie
  `LogoImage` dient es einem Dokument, nicht einer lesenden Person. Es liest
  weder Bankverbindungen noch Logo.
- Welche Werte die Belehrung nennt, entscheidet der Betreuungsvertrag
  (`withdrawalAddressee`): auf PDF, Anmeldeformular, Unterschriftsseite und
  Bestätigungsmail derselbe Adressat. Die Unterschrift bekommt dafür den
  Port `TraegerSigningSender.WithdrawalReach`.
- Der Pass-through-Etat von `modules/organizationtenancy` steigt um eins für
  `DocumentContact`.

## Folgen

- Eltern lesen auf jeder Fläche Telefon und E-Mail des Trägers.
- Bereits unterschriebene PDFs bleiben, wie sie sind. Unterschriftsseite und
  Mail nennen ab jetzt die aktuelle Angabe.
- Das Lesen ohne Konto steht nicht im Zugriffsprotokoll des Trägers. Es gibt
  nur Kontaktangaben preis, die der Träger zur Veröffentlichung in seinen
  Dokumenten einträgt.

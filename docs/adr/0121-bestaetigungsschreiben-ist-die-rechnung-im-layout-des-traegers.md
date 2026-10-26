---
status: accepted
---

# Bestätigungsschreiben ist die Rechnung im Layout des Trägers

Entscheidung vom 26.10.2026 zu [#4669](https://github.com/moto-nrw/project-phoenix/issues/4669),
Etappe 1 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 73, 91 und 92. Sie ist der dritte Bau-Schnitt des Owners
Elternabrechnung aus
[ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
nach [ADR 0115](0115-ferienbeitrag-entsteht-in-der-elternabrechnung-und-wird-bei-bestaetigung-festgehalten.md)
und [ADR 0119](0119-abrechnungslauf-stellt-unveraenderliche-rechnungen-aus-einem-nummernkreis-aus.md).
Sie führt keinen Owner ein und hält fest, was das Schreiben in den Registern
ändert (ADR 0057 Punkt 5, ADR 0070).

## Kontext

Bissendorf bestätigt eine Ferienanmeldung mit einem Serienbrief, der zugleich
die Zahlungsaufforderung ist (Vorlage GH-003): Familie und Anschrift, Datum,
Betreff, Anrede, Vorname des Kindes, Gesamtbeitrag, Adressnummer als
Verwendungszweck, Zahlungsfrist, Konto des Trägers, Packliste,
Krankheitsregel, Notfallnummern der Betreuungsteams und wen man fragen kann.
Ein freigegebener Lauf (ADR 0119) hat unveränderliche Rechnungen mit Nummer,
Empfänger, Kindern und Positionen. Logo, Ansprechpartner und Bankverbindungen
liegen bei Organisation & Tenancy, Farbe, Briefkopf und Fußzeile sind
Träger-Einstellungen (ADR 0112). Document Rendering kann Listen und Blöcke im
Design von moto drucken, aber keinen Brief.

## Entscheidung

1. **Dokumentart.** Für Ferien ist das Beitragsdokument das
   Bestätigungsschreiben als Rechnung (Story 73, ADR 0088 §4). Eine Wahl je
   Leistung kommt mit der Monatsabrechnung (Etappe 2); bis dahin gibt es nur
   diese Art.
2. **Freitext je Lauf.** Nach der Freigabe schreibt die Abrechnung einmal je
   Lauf, was alle Schreiben drucken: Zahlungsfrist (Kalenderdatum, nicht vor
   heute), einen Freitext für die Eltern (Packliste, Krankheitsregel; bis 4000
   Zeichen) und bis zu zehn Notfallnummern der ausrichtenden OGS (Name und
   Nummer). `parent_billing.run_letters` hält das mit Konto und Zeitpunkt.
   Die Notfallnummern sind Text des Laufs, keine Stammdaten einer Schule; das
   Träger-Büro liest keine Daten einer Schule (ADR 0112 Folgen).
3. **Einmal erzeugt, nie geändert.** „Schreiben erstellen“ rendert in einer
   Träger-Transaktion das Schreiben jeder Rechnung des Laufs und speichert das
   PDF in `parent_billing.document_letters`; die Datenbank hält dazu die
   SHA-256 (erzeugte Spalte). Ein Lauf hat seine Schreiben einmal
   (Primärschlüssel). Wie bei der Rechnung lehnt der Trigger aus ADR 0119
   jede Änderung ab, für jede Rolle; die Anfrage-Rolle darf nur lesen und
   anlegen. So ändert sich das PDF einer Rechnung nie, auch wenn der Träger
   später sein Branding ändert (ADR 0088 §4). Scheitert ein Schreiben, rollt
   die Route alles zurück. Die Vorschau rendert das Schreiben der ersten
   Rechnung mit der Eingabe, ohne etwas zu speichern.
4. **Speicher.** Die PDFs liegen als `bytea` beim Owner, nicht in File
   Storage: File Storage ist je Schule gebaut (Ordner, Quote,
   Tenant-Transaktion), eine Ferienrechnung hat keine Schule (wie das Logo,
   ADR 0112 Punkt 4). Versand (#4670) und Postfach (#4671) lesen später
   dieses PDF.
5. **Felder.** Das Schreiben folgt der Vorlage Feld für Feld; der Golden-Test
   `TestConfirmationLetterFollowsTheBissendorfTemplate` hält die Zuordnung
   fest. Zwei Felder gibt es in moto nicht:
   - **Anschrift:** Die Ferienanmeldung hat keine Postanschrift (#4661). Das
     Anschriftfeld nennt „Familie Nachname“ und die Person, die angemeldet
     hat. Die Anschrift gehört zum Zahler der Elternabrechnung (ADR 0088 §4)
     und kommt mit ihm; die Anmeldung bekommt dafür kein Feld, weil Eltern
     das Schreiben über Mail und Eltern-App bekommen (#4670, #4671) und die
     Anmeldung nach 92 Tagen geht.
   - **Adressnummer:** Verwendungszweck ist die Rechnungsnummer. Sie ist je
     Träger eindeutig und steht auf der Rechnung; eine Kundennummer gibt es
     erst mit dem Zahler.
   Anders als die Vorlage nennt das Schreiben jede Position (Woche,
   Zeitmodell, Ermäßigung, Befreiung, Korrektur mit Begründung), weil es die
   Rechnung ist. Ein Satz zum Lastschriftmandat fehlt, bis es Mandate gibt
   (Story 82). Bei 0 € steht „Sie müssen nichts bezahlen“ und keine Frist.
6. **Layout.** Document Rendering bekommt das Brief-Layout
   (`lists.Letter`, `lists.LetterRenderer`): Farbband und Überschriften in der
   Farbe des Trägers, Logo oben links, Absender oben rechts, Rücksendezeile
   und Anschriftfeld auf der Höhe von DIN 5008, Infoblock daneben, Fußzeile
   mit der Standard-Bankverbindung auf jeder Seite, Folgeseiten bei langem
   Text. Nichts wird gekürzt: lange Werte brechen um, und lassen Absender
   oder Fußzeile zu wenig Platz für den Brief, lehnt der Renderer ab
   (409 `parent_billing.letterhead_too_long`), bevor etwas gespeichert ist. Es druckt nur, was der Owner schreibt; jeder Text liegt bei der
   Elternabrechnung. Vorlagen je Träger mit eigenen Platzhaltern (Story 91)
   kommen mit Etappe 2; bis dahin ist der Freitext je Lauf der Platzhalter
   (Story 92).
7. **Grenzen.**
   - Die Anwendung schreibt den Brief als eigenen Wert und ruft ihren Port
     `LetterRenderer`; der Adapter `internal/adapters/rendering` übersetzt in
     den Vertrag von Document Rendering. Nur er importiert Document
     Rendering.
   - Das Branding liest der Port `parentbilling.BillingLetterheads`, den die
     Root bindet (`api/serve_mounts_parent_billing.go`): Kontakt und
     Bankverbindungen über `organizationtenancy.TraegerMasterData.Profile`
     (schreibt den Leseeintrag in `platform.traeger_master_data_audit`), das
     Logo über `LogoImage` und Farbe, Briefkopf und Fußzeile über
     `TraegerSettings.TraegerTexts` (Wert des Trägers oder Standard, in der
     Träger-Transaktion des Aufrufers). Ohne Standard-Bankverbindung lehnt
     die Elternabrechnung ab (409 `parent_billing.bank_account_missing`).
   - Alle Routen laufen in `TraegerTxMiddleware` (ADR 0105) hinter dem Recht
     „Abrechnung“. Kein Workflow, keine Admin-Transaktion.
8. **Zugriffsprotokoll.** `parent_billing.run_events` bekommt die Aktion
   `letters` (Erstellung, mit Anzahl). Vorschau und Download sind `read` mit
   `letter` (`preview` oder `download`) und der Rechnung in den Details.
9. **Aufbewahrung.** Schreiben und Freitext gehen mit ihrer Rechnung und
   ihrem Lauf, also mit dem Träger; die Frist je Rechnung legt #4720 fest.
10. **Register.**
    - `policy.json`: Paket `modules/parentbilling/internal/adapters/rendering`
      (Rolle `adapter`), drei an dieses Paket gebundene Regeln
      (`adapter` → Document Rendering `contract` `lists`, → eigene
      `application`, → eigene `public`) und zwei `data_objects` mit
      `write_owner` `parent-billing`.
    - `command-effects.json`: fünf geprüfte Bindungen (Profil: leaf/mutation
      nach ADR 0067 wegen des Leseeintrags; Logo, Einstellungen und die zwei
      Renderer-Bindungen: query).
    - Transaktionsregister: `TraegerSettings.TraegerTexts` als Stelle der
      Träger-Transaktion (wie `Schema`, ADR 0107).
    - Budget `modules/parentbilling/compose` 27 → 35 Zeilen
      (`moduleComposeBudgetTotal` 12639 → 12647).
    - Datenlebenszyklus: zwei Einträge mit `scope: organization`.

## Verworfene Optionen

- **PDF bei jedem Download neu rendern:** Ein geändertes Logo, eine neue
  Fußzeile oder ein neuer Renderer änderte eine ausgestellte Rechnung.
- **Schreiben bei der Freigabe erzeugen:** Die Freigabe (ADR 0119) bräuchte
  Freitext und Frist vor der Prüfung der Zahlen und wüchse um das Branding.
  Ein eigener Schritt nach der Freigabe hält beide getrennt.
- **Freitext als Träger-Einstellung:** Packliste und Frist gelten je
  Ferienzeitraum, nicht für den Träger.
- **Anschrift in der Ferienanmeldung ergänzen:** ändert das öffentliche
  Formular, Enrollment und dessen Frist für einen Weg, den Mail und
  Eltern-App ersetzen; die Anschrift gehört zum Zahler.
- **Document Rendering direkt aus der Anwendung:** eine neue Kante zwischen
  bestehenden Rollen; der Adapter hält den Vertrag an einer Stelle.

## Folgen

- Versand per Mail (#4670) und Postfach in der Eltern-App (#4671) verweisen
  auf das gespeicherte PDF.
- Alle Schreiben eines Laufs gesammelt drucken (Story 79) fehlt noch.
- Mit dem Zahler kommen Anschrift und Kundennummer ins Anschriftfeld; Storno
  und Neuausstellung (Story 72) bekommen eigene Schreiben.

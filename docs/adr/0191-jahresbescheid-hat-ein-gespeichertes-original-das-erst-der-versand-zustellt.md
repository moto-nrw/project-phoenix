---
status: proposed
---

# Jahresbescheid hat ein gespeichertes Original, das erst der Versand zustellt

**Status: Vorgeschlagen.**

Vorschlag vom 09.11.2026 zu [#6347](https://github.com/moto-nrw/project-phoenix/issues/6347),
Entscheidung **A** im Issue (Yannick, 09.11.2026), Etappe 2 der Spec
[#4611](https://github.com/moto-nrw/project-phoenix/issues/4611). Er löst die Folge
„Schreiben, Mail und Elternpostfach für Bescheide“ aus
[ADR 0182](0182-jahresbescheid-veranlagt-die-monate-eines-jahres-und-keine-monatsrechnung-rechnet-sie-noch-einmal-ab.md)
(Punkt 8) nach dem Muster von
[ADR 0165](0165-monatsrechnungen-nehmen-schreiben-versand-und-storno-der-ferienrechnungen.md)
und [ADR 0126](0126-dokumentenpostfach-der-eltern-app-liest-die-schreiben-des-traegers.md).
Er führt keinen Owner ein und hält fest, was er in den Registern ändert
(ADR 0057 Punkt 5, ADR 0070).

## Kontext

Ein ausgestellter Jahres- oder Änderungsbescheid war bisher nur als Seite 1 im
Browser zu sehen. Diese Seite liest Briefkopf, Kontakt und Konto des Trägers bei
jedem Abruf neu. Ein dauerhaftes Original, die Mail an die Familie und der
Eintrag im Elternpostfach fehlten (#6243, „Risiken“).

## Entscheidung

1. **Original einmal, dann nie wieder.** Die Abrechnung legt für einen
   freigegebenen Lauf die Originale an: je ausgestelltem Bescheid ein PDF auf
   dem Briefkopf dieses Moments, gespeichert in
   `parent_billing.care_notice_letters` (eins je Bescheid,
   `uq_care_notice_letters_notice`). Ein Trigger lehnt jede Änderung ab. Ändern
   sich danach Briefkopf, Vorlage, Anschrift oder Vertrag, bleibt das Original
   byte-identisch; die Übersicht zeigt seinen SHA-256. Das PDF druckt Seite 1
   wie ADR 0182 Punkt 8 und die kopierte Belehrung. Von Seite 2 aus MK-007 wird
   nichts nachgebaut. Verlangt ein Bescheid eine Überweisung, steht
   das Konto des Trägers im Original; ohne Konto wird kein Original
   gespeichert (409 `parent_billing.bank_account_missing`).
2. **Änderungsbescheid ist ein eigener Beleg.** Er hat ein eigenes Original mit
   Bezug auf den Vorgänger („Ändert Bescheid … vom …“, bisher/neu/Zu-/Abgang).
   Das Original des Vorgängers bleibt unverändert.
3. **Ein Original überall.** Download, Mailanhang und Postfach liefern dieselben
   gespeicherten Bytes; nichts wird neu gerendert.
4. **Zustellung nur nach Büroaktion.** „An alle senden“ und „Senden“ je Bescheid
   legen je Versand einen Eintrag der Outbox (Mail-Art `parent_billing_letter`,
   Bezug `parent_billing_care_notice_run`) mit Schlüssel je Original und
   Versandnummer an, in derselben Träger-Transaktion wie die Zeile in
   `parent_billing.care_notice_letter_mailings` und unter der Sperre des Laufs.
   Eine Serie sendet nichts doppelt, ein wartender Versand wird nicht wiederholt,
   ein gescheiterter schon; nur der erste Versand pusht
   (`parent_billing_document`). Erstellen, Senden und jedes Lesen stehen in der
   Spur des Laufs (`letters`, `mail`, `read`).
5. **Nur berechtigte Empfänger.** Ein Versand geht nur an die Adresse, die der
   Bescheid kopiert hat, und nur solange sie die wirksame Rechnungsadresse der
   Familie des Vertrags ist (abweichender Zahler eingeschlossen, Port
   `CareContracts.Recipients` mit der Abweichung aus ADR 0167). Sonst 409
   `parent_billing.care_recipient_changed`. Ohne Adresse wird gedruckt, nie
   gemailt (`parent_billing.recipient_email_missing`).
6. **Postfach mit Beziehung.** Das Postfach zeigt ein gesendetes Original nur dem
   Konto mit der kopierten Adresse, das auf der Beziehung zum Kind des Bescheids
   an dessen Schule `parent_portal.enrollments.view` hält (ADR 0126 Punkt 2,
   ADR 0165). Mitgliedschaft in der Schule, eine Beziehung zu einem anderen Kind,
   ein entzogenes Recht, ein gelöschtes Konto oder ein anderer Träger öffnen
   nichts. Arten im Postfach: `care_fee_notice`, `care_change_notice`. Die IDs
   kommen aus `parent_billing.documents_id_seq`, deshalb bleiben Route, Workflow
   und Push unverändert.
7. **Keine Bekanntgabe behauptet.** Mail und Postfach sind Wege, den Bescheid zu
   übermitteln. moto nennt sie nirgends eine förmliche Zustellung oder
   Bekanntgabe; ob und wie der Träger förmlich zustellt, bleibt seine Sache.
   Oberfläche und Hilfe sagen das.

## Verworfene Optionen

- **Original bei der Freigabe rendern:** Die Freigabe bekäme Briefkopf und
  Renderer als Abhängigkeit, und ein Fehler im Briefkopf stoppte das Ausstellen.
  Der eigene Schritt hält Ausstellen und Senden getrennt wie ADR 0165.
- **Eigene Mail- und Push-Arten:** Die bestehenden Arten tragen den Inhalt im
  Payload; neue Arten bräuchten Outbox-CHECK und Löschwächter ohne Gewinn.
- **Bescheide in `care_letters` ablegen:** Die Tabelle hängt an Monatsrechnung
  und Monatslauf (ADR 0165 Punkt 1).

## Register

- Migration `1.15.883`: zwei Tabellen mit RLS, Triggern gegen Änderungen und
  Rechten nur zum Lesen und Anlegen; Aktionen `letters` und `mail` der Spur; die
  Demo-Löschung `platform.erase_closed_demo_visitors` löscht Mail, Zustellergebnis
  und Push der Originale einer ausgemusterten Demo-Schule vor den Bescheiden, auf der Fassung von 1.15.878
  (`TestDemoSchoolQueueErasesTheNoticeLettersOfTheRetiredSchool`).
- `policy.json`: zwei `data_objects` für `parent-billing`; keine neue Regel.
- `command-effects`: zwei geprüfte Bindungen auf Delivery (Mail durable,
  Zustand als Abfrage).
- Datenlebenszyklus und Aufbewahrung je zwei Einträge.
- Routen unter `/traeger/care-notice-runs/{id}`: `GET|POST /letters`,
  `GET /letters/print`, `POST /letters/mail`, `GET /notices/{contractID}/pdf`,
  `POST /notices/{contractID}/mail`; die schreibenden in der Nutzungsanalyse als
  `notCaptured` (Träger, keine Schule). Query-Budget
  `api.traeger.care_notice_run_letters` (8, exact).
- Compose-Budget: `modules/parentbilling/compose` 107 → 112 Zeilen
  (`moduleComposeBudgetTotal` 13193 → 13198) für den Konstruktor
  `NewCareNoticeLetters`.
- Mail-Vorlage `billing-care-change-notice-letter.html` für den
  Änderungsbescheid; der Jahresbescheid nutzt `billing-notice-letter.html`.

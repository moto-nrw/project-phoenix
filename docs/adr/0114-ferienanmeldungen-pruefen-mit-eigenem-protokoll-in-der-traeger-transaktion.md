---
status: accepted
---

# Ferienanmeldungen prüfen mit eigenem Protokoll in der Träger-Transaktion

Entscheidung vom 25.10.2026 zu [#4663](https://github.com/moto-ogs/project-phoenix/issues/4663),
Etappe 1 der Spec [#4611](https://github.com/moto-ogs/project-phoenix/issues/4611),
User Stories 35 und 36. Sie baut auf
[ADR 0105](0105-ferienzeitraum-liegt-beim-traeger-in-der-traeger-transaktion.md)
und [ADR 0110](0110-oeffentliche-ferienanmeldung-schreibt-in-der-traeger-transaktion.md)
auf und hält fest, was die Prüfung der Anmeldungen in den Registern ändert.
Sie führt keinen neuen Owner und keine neue Stelle der Träger-Transaktion ein.

## Entscheidung

1. **Owner und Transaktion.** Die Prüfung gehört Enrollment wie die
   Anmeldung (ADR 0110 Punkt 7). Ihre Routen hängen unter
   `/traeger/holiday-periods/{id}/registrations` an den Routen des
   Ferienzeitraums: dieselbe Träger-Sitzung, das Recht „Anmeldung verwalten“
   und die Träger-Transaktion aus `TraegerTxMiddleware` (ADR 0105 Punkt 3).
   Das Transaktionsregister bleibt unverändert.
2. **Status.** Eine Anmeldung ist eingegangen, bestätigt oder storniert.
   Bestätigen geht nur aus „eingegangen“, Stornieren aus „eingegangen“ und
   „bestätigt“; eine stornierte Anmeldung ändert sich nicht mehr. Den Preis
   hält erst die Elternabrechnung bei der Bestätigung fest (#4664).
3. **Einfrieren.** Nach dem Anmeldeschluss ihres Zeitraums ist eine
   Anmeldung eingefroren. Das Träger-Büro friert eine Anmeldung auch vorher
   ein oder öffnet sie gezielt wieder (`freeze_override`: `frozen`,
   `reopened`, sonst entscheidet der Anmeldeschluss). Eine eingefrorene
   Anmeldung lässt sich bestätigen, aber nicht stornieren oder umbuchen;
   dafür öffnet das Träger-Büro sie zuerst wieder.
4. **Umbuchen** ersetzt die Buchungen eines Kindes: je Woche des Zeitraums
   ein Zeitmodell dieser Woche, mindestens eine Woche. Die Änderung sperrt
   zuerst den Zeitraum und dann die Anmeldung, so läuft sie nach einer
   laufenden Änderung der Wochen.
5. **Dubletten.** Ein Kind gilt als mögliche Dublette, wenn eine andere,
   nicht stornierte Anmeldung desselben Zeitraums ein Kind mit gleichem Vor-
   und Nachnamen (ohne Groß- und Kleinschreibung) und gleichem Geburtsdatum
   hat. Das ist ein Hinweis, keine Sperre: Geschwister und Doppelanmeldungen
   entscheidet das Träger-Büro.
6. **Protokoll.** `enrollment.holiday_registration_audit` hält jedes Lesen
   personenbezogener Daten (Liste, Einzelansicht, Export) und jede Änderung
   (Bestätigen, Stornieren, Umbuchen, Einfrieren, Wiederöffnen) mit Konto,
   Zeitpunkt und Einzelheiten, in derselben Träger-Transaktion. Scheitert der
   Eintrag, scheitert die Anfrage. Die Änderungen einer Anmeldung sind ihre
   Historie in der Einzelansicht. Wie bei den Stammdaten
   ([ADR 0112](0112-stammdaten-des-traegers-liegen-bei-organisation-und-tenancy.md)
   Punkt 6) gilt `audit.data_access_log` nur für Daten einer Schule; die
   Anmeldungen gehören dem Träger. Die Anfrage-Rolle darf das Protokoll
   lesen und ergänzen, nicht ändern. Einträge einer Anmeldung gehen mit ihr,
   die übrigen löscht der stündliche Löschjob mit den Anmeldungen 92 Tage
   nach der letzten Woche (User Story 121).
7. **Gesundheitsangaben** zeigt nur die Einzelansicht, mit dem Recht und
   dem Eintrag im Protokoll (ADR 0110 Punkt 7). Liste und Export enthalten
   sie nicht.
8. **Export.** Document Rendering erzeugt Listen jetzt auch als CSV
   (`lists.FormatCSV`: Kopfzeile, eine Zeile je Datensatz, Semikolon,
   UTF-8 mit BOM, Formeln entschärft). Das ist das gemeinsame Muster für
   Listenexporte (Spec, „Exporte“): Jeder Owner füllt ein `lists.Document`
   und wählt CSV oder Excel. Der Export der Ferienanmeldungen hat eine Zeile
   je Kind und gebuchter Woche und filtert wie die Liste; „eine Schule“ ist
   die Schule des Kindes, „alle“ ohne Filter.
9. **Schulnamen im Export.** Die Namen kennt die Projektion `traeger-view`
   (`Overview.Schools`), die in ihrer eigenen Admin-Einheit liest. Eine
   Admin-Einheit verschachtelt sich nicht in eine Träger-Transaktion
   (ADR 0105 Punkt 2). Deshalb liest eine Middleware der Export-Route die
   Namen über den Port `enrollment.HolidaySchoolNames`, bevor
   `TraegerTxMiddleware` die Träger-Transaktion öffnet. Die Root bindet den
   Port.
10. **Register.**
    - `policy.json`: `data_objects` `enrollment.holiday_registration_audit`
      mit `write_owner` `enrollment`. Keine neue Regel.
    - `passthrough.json`: `api/holidaySchoolNames.HolidaySchoolNames` als
      Consumer-Port-Adapter.
    - `command-effects.json`: eine geprüfte Bindung (query) auf
      `traegerview.Overview.Schools`.
    - Query-Budget `api.traeger.holiday_registrations` (7, exact).
    - Datenlebenszyklus: der neue Eintrag mit `scope: organization`.

## Verworfene Optionen

- **Eintrag in `audit.data_access_log` je Schule:** Ein Feriengast hat keine
  Schule, und eine Anmeldung ist kein Datensatz einer Schule. Die Tabelle
  verlangt eine Schule und gilt nur in deren Transaktion.
- **Schulnamen aus `platform.schools` im Postgres-Paket von Enrollment:**
  Die Tabelle gehört Organisation & Tenancy; Enrollment liest sie nicht
  selbst.
- **Einfrieren nur als Kennzeichen ohne Anmeldeschluss:** Das Issue verlangt,
  dass Anmeldungen nach dem Anmeldeschluss eingefroren sind, ohne dass das
  Träger-Büro jede einzeln anfasst.

## Folgen

- Das Protokoll speichert zu jedem Eintrag die Konto-ID der handelnden
  Person; die Antwort der Einzelansicht enthält sie bei jedem
  Verlaufseintrag. Die Oberfläche zeigt sie nicht an, weil eine bloße Nummer
  niemandem sagt, wer gehandelt hat. Den Namen dazu zeigt sie, sobald
  Enrollment ihn über einen Port von Identity & Access lesen darf; das folgt
  mit eigener Entscheidung.
- Weitere Listen nutzen `lists.FormatCSV` ohne eigenen CSV-Code.

## Nachtrag 2026-11-03: Angaben ändern (#5629)

Zu Punkt 6: Das Träger-Büro ändert die Angaben einer Anmeldung
(`PUT .../registrations/{registrationId}/details`, Recht „Anmeldung
verwalten“): Elternteil mit Anschrift (nach dem Baustein der Formularversion,
#5625), zweite sorgeberechtigte Person, Vor- und Nachname,
Geburtsdatum und Schule eines Kindes, Notfallkontakte, Abholpersonen und
die Antworten ohne Gesundheitsangabe. `edit` gehört zu den Änderungen. Der
Eintrag nennt die geänderten Felder als Feldpfade („guardian.phone“,
„children.0.last_name“), ohne alte und neue Werte. Eine Anfrage ohne
Änderung schreibt keinen Eintrag.

- Die Änderung läuft über dieselbe Sperre wie die übrigen Änderungen
  (Punkt 4): zwei Änderungen einer Anmeldung laufen nacheinander. Jeder
  Bereich wird nur geändert, wenn die Anfrage ihn nennt; so überschreibt
  eine Änderung der Kontakte keine gleichzeitige Änderung der Eltern.
- Sie prüft mit den Funktionen des öffentlichen Formulars, gegen die
  Formularversion der Anmeldung. Die Prüfung der Buchungen ist dafür aus
  der Prüfung der Kinder herausgelöst.
- Eine stornierte Anmeldung ändert sich nicht (Punkt 2). Eingegangene,
  bestätigte und eingefrorene Anmeldungen lassen sich ändern: keine dieser
  Angaben ändert den Ferienbeitrag (ADR 0115), Wochen und Zeitmodell ändert
  weiter nur das Umbuchen.
- Kinder, Buchungen, Leistungsbezug, Nachweise, Einwilligungen, die
  Erklärung der Eltern (#5626) und Gesundheitsangaben ändern sich hier
  nicht; die Debitorennummer ändert die Statuskarte (#5621). Eine Antwort auf eine Frage mit
  Gesundheitsangabe lehnt die Route ab: sie liegt verschlüsselt und
  bräuchte ein eigenes Schreibrecht.
- Ausgestellte Schreiben behalten ihre eigene Kopie (ADR 0119 Punkt 7), auch
  eine Rechnung ihre Anschrift.
- Bedingungen, die an einer Frage mit Gesundheitsangabe hängen, wertet die
  Änderung nicht aus: die Antwort bleibt. Ändern sich die allgemeinen
  Antworten, werden die Antworten jedes Kindes neu bereinigt; entfernte
  Antworten stehen im Eintrag.

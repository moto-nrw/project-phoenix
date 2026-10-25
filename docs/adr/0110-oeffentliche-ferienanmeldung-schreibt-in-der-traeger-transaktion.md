---
status: accepted
---

# Öffentliche Ferienanmeldung schreibt in der Träger-Transaktion

Entscheidung vom 25.10.2026 zu [#4661](https://github.com/moto-nrw/project-phoenix/issues/4661),
Etappe 1 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 26 und 28 bis 31. Sie baut auf
[ADR 0105](0105-ferienzeitraum-liegt-beim-traeger-in-der-traeger-transaktion.md)
auf. ADR 0105 Punkt 4 verlangt für jede weitere Stelle der
Träger-Transaktion eine eigene Entscheidung; diese ADR ist sie. Sie führt
keinen neuen Owner ein.

## Entscheidung

1. **Ein Link je Ferienzeitraum:** Jeder Ferienzeitraum trägt einen zufälligen
   Schlüssel (`enrollment.holiday_periods.public_token`, zwei zufällige
   UUIDs, 244 Bit). Der Link `/ferienanmeldung/{Schlüssel}` im Elternportal
   gilt für alle Schulen des Zeitraums; das Backend antwortet unter
   `/traeger/holiday-registrations/{Schlüssel}` ohne Sitzung. Der Link öffnet
   nur einen veröffentlichten Zeitraum eines aktiven Trägers; ein Entwurf
   antwortet wie ein unbekannter Schlüssel. Der Schlüssel ist kein Recht auf
   Daten: Er zeigt nur, was der Zeitraum ohnehin für Eltern anbietet.
2. **Auflösen:** Wie beim Schul-Slug der Online-Anmeldung löst eine
   Admin-Transaktion den Schlüssel auf. Sie liest nur Träger und Zeitraum des
   Schlüssels und Namen und Schulen dieses Trägers (Organisation & Tenancy).
   Danach liest und schreibt alles in der Träger-Transaktion des gefundenen
   Trägers unter RLS.
3. **Neue Stellen der Träger-Transaktion:** Die öffentlichen Routen von
   Enrollment (`modules/enrollment/http`) öffnen sie selbst, weil es keine
   Träger-Sitzung gibt: einmal für das Formular, einmal für den Zähler gegen
   Missbrauch, einmal für die Anmeldung. Der Träger kommt nie aus der Anfrage,
   nur aus dem aufgelösten Schlüssel.
   `backend/test/holiday_registration_transaction.go` lässt genau diese
   Stellen, ihren gemeinsamen Einstieg und die Admin-Stelle der Auflösung mit
   ihren Gründen am Schrumpf-Vergleich vorbei.
4. **Feriengast:** Ein Kind ohne Schule des Trägers ist ein Feriengast. Die
   Anmeldung speichert es mit `home_school_id IS NULL` und höchstens dem Namen
   seiner Schule. Weder Feriengast noch Kind einer Schule des Trägers wird
   dabei Schüler einer OGS: Die Anmeldung schreibt keine Schultabelle und sucht
   keine vorhandenen Kinder. Name und Geburtsdatum schalten deshalb nichts
   frei. Ein Trigger lässt als Heimatschule nur eine nicht gelöschte Schule
   des Trägers zu, wie bei der ausrichtenden OGS (ADR 0105 Punkt 5).
5. **Missbrauchsschutz:** Wie bei der Online-Anmeldung zählt ein Zähler je
   IP-Adresse und je E-Mail-Adresse, mit denselben Fenstern und Grenzen, in
   einer eigenen Träger-Transaktion, damit der Zähler eine abgelehnte Anmeldung
   überlebt. Gezählt wird vor der Prüfung von Captcha und Angaben. Der Zähler
   speichert nur SHA-256-Prüfsummen der Werte und löscht abgelaufene Zähler
   des Trägers beim Zählen. Turnstile ist für die Ferienanmeldung immer an,
   weil es noch keine Träger-Einstellung gibt (ADR 0086); die Schlüssel sind
   dieselbe Plattform-Konfiguration.
6. **Eingangsbestätigung über die Outbox:** Delivery bekommt Mails eines
   Trägers. `platform.email_outbox` erhält `organization_id`; eine Zeile hat
   höchstens eine Schule oder einen Träger. Eine Träger-Policy lässt die
   Träger-Transaktion nur Zeilen ohne Schule mit ihrem eigenen Träger sehen und
   schreiben. Der Schlüssel gegen doppelte Mails gilt je Schule, Träger und
   Schlüssel. Enrollment legt die Mail in derselben Transaktion an wie die
   Anmeldung. Der Worker rendert sie wie eine Systemmail ohne Schule.
7. **Gesundheitsangaben:** Sie stehen nur in den Tabellen der Anmeldung. Die
   Antwort an die Eltern, die Bestätigungsmail und die Zahl im Träger-Portal
   enthalten sie nicht. Lesen dürfen sie erst die Prüfung der Anmeldungen
   (#4663) mit dem Recht „Anmeldung verwalten“ und der Zugriff der
   ausrichtenden OGS (#4665), jeweils mit Zugriffsprotokoll. Für
   Gesundheitsangaben ist eine eigene Einwilligung Pflicht. Verschlüsselung
   und Einschränkung je Feld folgen mit #4722.
8. **Schreiben über mehrere Owner:** Die Anmeldung schreibt Enrollment und die
   Outbox von Delivery in einer Transaktion. Das ist das Muster jeder
   Bestätigungsmail (Delivery als Plattform-Port, ADR 0098), kein Workflow.

## Folgen

- Eine Woche oder ein Zeitmodell mit Anmeldungen lässt sich nicht mehr
  entfernen; der Zeitraum antwortet dann mit 409
  `enrollment.holiday_week_booked`. Die Schlüssel der Buchung auf Woche und
  Zeitmodell prüft die Datenbank erst beim Commit, damit ein gelöschter
  Zeitraum oder Träger seine Buchungen in jeder Reihenfolge mitnimmt; die
  Änderung eines Zeitraums schaltet sie vorher auf sofort.
- Das Träger-Portal zeigt je Zeitraum den Link und die Zahl der eingegangenen
  Anmeldungen. Die Zahl ist zusammengefasst und steht nicht im
  Zugriffsprotokoll (ADR 0084 Abschnitt 7).
- Die Löschfrist von drei Monaten nach den Ferien (User Story 121) folgt mit
  #4720; bis dahin gehen die Zeilen mit ihrem Zeitraum und dem Träger.
- Die Nutzungsanalyse zählt die Anmeldung noch nicht (`notCaptured`): Ohne
  Sitzung und Schule lässt keine Sammelregel sie zu.
- Sobald es Träger-Einstellungen gibt, kann Turnstile dort abschaltbar werden.

---
status: accepted
---

# Eltern-App kündigt Ferienzeiträume der Träger der eigenen Kinder an

Entscheidung vom 11.11.2026 zu [#6671](https://github.com/moto-ogs/project-phoenix/issues/6671).
Sie baut auf
[ADR 0110](0110-oeffentliche-ferienanmeldung-schreibt-in-der-traeger-transaktion.md)
und [ADR 0113](0113-vorbefuellung-der-ferienanmeldung-liest-im-elternportal.md)
auf und führt keinen neuen Owner ein.

## Kontext

Eltern erreichen die Ferienanmeldung bisher nur über den Link aus dem
Elternbrief. Die Startseite der Eltern-App soll einen veröffentlichten
Ferienzeitraum nennen und zum vorhandenen Formular führen. Dafür muss die
Eltern-App wissen, welche Träger zu den Kindern eines Kontos gehören und
welche Zeiträume diese Träger veröffentlicht haben. Ein Ferienzeitraum gehört
dem Träger und liegt in seiner Träger-Transaktion (ADR 0105); ein Elternkonto
hat keine.

## Entscheidung

1. **Eine Leseroute im Elternportal:** `GET /parent/me/holiday-registrations`
   liefert je Zeitraum Name, Träger, Anmeldezeitraum, ersten und letzten
   Betreuungstag, den Schlüssel des Links und die Kinder des Kontos, für die
   der Zeitraum gilt. Das Formular, die Vorbefüllung (ADR 0113) und das
   Absenden über den Link (ADR 0110) bleiben unverändert.
2. **Nur für Kinder mit Recht:** Der Workflow Elternportal bietet einen
   Zeitraum nur an, wenn die Beziehung des Kontos zu einem Kind an einer
   Schule des Trägers `parent_portal.enrollment.submit` trägt und die
   Betreuung des Kindes nicht beendet ist. Das ist dieselbe Prüfung wie in
   ADR 0113 Punkt 2. Ein Konto ohne solches Kind bekommt eine leere Liste.
3. **Welche Zeiträume:** Veröffentlichte Zeiträume, deren Anmeldefrist heute
   oder später endet. Vor dem Anmeldestart nennt die Antwort den Zeitraum mit
   `open: false`, nach dem letzten Tag der Frist nicht mehr. Entwürfe liest
   die Abfrage nicht.
4. **Neue Stelle der Admin-Transaktion:**
   `workflows/parentportal/internal/care/holiday_registration_offers.go`,
   `Service.HolidayRegistrationOffers`. Sie liest in einer Admin-Transaktion
   die Beziehungen des Kontos, über Organisation & Tenancy die aktiven Träger
   der Schulen dieser Kinder und über Enrollments neue Fähigkeit
   `HolidayRegistrationAnnouncements` die veröffentlichten Zeiträume genau
   dieser Träger. Sie schreibt nichts. Der Träger kommt nie aus der Anfrage.
   Die Abfrage in Enrollment verlangt die Admin-Transaktion, wie die
   Link-Auflösung aus ADR 0110, und nennt die Träger in ihrem Filter.
5. **Kein Eintrag im Zugriffsprotokoll:** Die Route liest keine Daten aus
   einer Schule. Sie nennt je Kind nur Kennung und Vornamen, die das Konto
   über seine eigene Kinderliste schon sieht. Die Vorbefüllung im Formular
   protokolliert weiter jedes gelesene Kind (ADR 0113 Punkt 5).

## Folgen

- Enrollment bekommt die schmale öffentliche Fähigkeit
  `HolidayRegistrationAnnouncements` (eine Abfrage, mengenbasiert). Die Root
  bindet sie an den Port des Elternportals.
- Die Kompositionen wachsen um genau 6 Zeilen: der Konstruktor in
  `modules/enrollment/compose` (+5) und der Port in
  `workflows/parentportal/compose` (+1). `moduleComposeBudgets` in
  `backend/test/module_legacy_budget_ratchet_test.go` nennt diese ADR.
- `backend/test/holiday_registration_prefill_transaction.go` lässt die neue
  Stelle am Schrumpf-Vergleich vorbei; jede weitere braucht eine eigene
  Entscheidung.
- Der Schlüssel des Links geht an ein angemeldetes Elternkonto mit Kind beim
  Träger. Er steht auch im Elternbrief und öffnet nur das öffentliche
  Formular; er schaltet keine Daten frei (ADR 0110 Punkt 4).
- Die Route ist eine Leseroute; die Nutzungsanalyse zählt nur schreibende
  Routen.

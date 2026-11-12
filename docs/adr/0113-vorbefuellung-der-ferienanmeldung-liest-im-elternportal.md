---
status: accepted
---

# Vorbefüllung der Ferienanmeldung liest im Elternportal

Entscheidung vom 25.10.2026 zu [#4662](https://github.com/moto-nrw/project-phoenix/issues/4662),
Etappe 1 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Story 27. Sie baut auf
[ADR 0110](0110-oeffentliche-ferienanmeldung-schreibt-in-der-traeger-transaktion.md)
auf und führt keinen neuen Owner ein.

## Entscheidung

1. **Lesen im Elternportal, Absenden über den Link:** Eltern mit Elternkonto
   öffnen denselben Link wie alle anderen. Ist eine Eltern-Sitzung da, liest
   die Seite `GET /parent/holiday-registrations/{Schlüssel}/family` und füllt
   das Formular vor. Abgeschickt wird die Anmeldung wie bisher über
   `POST /traeger/holiday-registrations/{Schlüssel}`. Die Anmeldung bleibt eine
   eigene Kopie (ADR 0110 Punkt 4): Was Eltern vor dem Absenden ändern, ändert
   nur die Anmeldung, nie die Stammdaten der Schule.
2. **Nur eigene Kinder mit Recht:** Der Workflow Elternportal
   (`workflows/parentportal`) bietet ein Kind nur an, wenn die Beziehung des
   Kontos zu diesem Kind `parent_portal.enrollment.submit` trägt, das Recht
   für eine Anmeldung zu einem vorhandenen Kind
   (`.claude/rules/guardian-parent-permissions.md`). Mitgliedschaft oder eine
   bloße Verknüpfung reichen nicht. Das Kind muss an einer Schule des Trägers
   des Links sein, und seine Betreuung darf nicht beendet sein. Kinder anderer
   Familien liest die Abfrage gar nicht, weil sie von den Beziehungen des
   Kontos ausgeht.
3. **Daten:** Je Kind Vorname, Nachname, Geburtsdatum und die Schule; dazu
   Name, E-Mail und Telefon des Elternteils aus seinem eigenen Profil an der
   Schule des ersten Kindes, sonst aus dem Konto. Gesundheitsangaben,
   Kontakte und Abholberechtigte werden nicht vorbefüllt.
4. **Neue Stelle der Admin-Transaktion:** Der Link wird wie der Schul-Slug der
   eingebetteten Online-Anmeldung in einer Admin-Transaktion aufgelöst
   (`workflows/parentportal/http/holiday_registration_handlers.go`,
   `resolveHolidayRegistrationLink`). Sie liest über Enrollment nur Träger und
   Schulen des Links und schreibt nichts. Der Träger kommt nie aus der
   Anfrage. Die Kinder liest der Workflow danach wie jede Elternportal-Abfrage:
   die Beziehungen des Kontos in seiner bestehenden Admin-Transaktion, die
   Daten jedes Kindes in der Tenant-Transaktion seiner Schule unter RLS.
   `backend/test/holiday_registration_prefill_transaction.go` lässt genau
   diese eine Stelle am Schrumpf-Vergleich vorbei.
5. **Zugriffsprotokoll:** Jedes gelesene Kind bekommt in derselben
   Tenant-Transaktion einen Eintrag in `audit.data_access_log` seiner Schule
   (`resource_type` `holiday_prefill`, Rolle `guardian`, Träger in den
   Metadaten). Damit steht der Abfluss von Schuldaten in eine Anmeldung des
   Trägers im Zugriffsprotokoll der Schule und des Träger-Portals (ADR 0109).

## Folgen

- Die Root bindet den Port `HolidayRegistrationLinks` des Elternportals an
  Enrollments Link-Auflösung (über dasselbe Träger-Verzeichnis wie der
  öffentliche Link) und `ChildDataAccessLog` an das Zugriffsprotokoll der
  Audit Platform. Die Elternportal-Komposition wächst um diese eine
  Abhängigkeit (+1 Zeile).
- Eltern mit Konto lösen weiter Turnstile und zählen im Missbrauchsschutz wie
  alle anderen; die Anmeldung kennt kein Elternkonto und verknüpft kein Kind.
  Ob eine Anmeldung später mit einem Schulkind verknüpft wird, entscheidet die
  Prüfung durch das Träger-Büro (#4663).
- Die Vorbefüllung ist eine Leseroute; die Nutzungsanalyse zählt nur
  schreibende Routen.

## Nachtrag 10.11.2026: Ferienwunsch (#4703)

Entscheidung vom 09.11.2026 zu
[#4703](https://github.com/moto-nrw/project-phoenix/issues/4703) (B + C).

1. **Ferienwunsch in der Vorbefüllung:** Berücksichtigt der Zeitraum den
   Ferienwunsch (`holiday_periods.holiday_wish_rule`), liest die
   Vorbefüllung je angebotenem Kind auch den Ferienwunsch der zugesagten
   OGS-Anmeldung des Schuljahres, in dem der Zeitraum beginnt. Es gilt
   dasselbe Recht wie in Punkt 2; der Eintrag im Zugriffsprotokoll der Schule
   trägt dann `holiday_wish` in den Metadaten.
2. **Bindung beim Absenden:** Enrollment versiegelt den Wunsch mit der
   Feldverschlüsselung der Gesundheitsangaben unter einem eigenen Kontext je
   Träger und Zeitraum, mit Schule, Name und Geburtsdatum des Kindes und
   24 Stunden Gültigkeit. Die Anmeldung schickt den Nachweis je Kind mit; die
   Anmeldung öffnet ihn und übernimmt den Wunsch nur für das Kind, das er
   nennt. Die Anmeldung kennt weiter kein Elternkonto und verknüpft kein
   Schulkind (Folgen, Punkt 2 dieser ADR): sie speichert nur „ja“ oder
   „nein“ mit der Herkunft `account`.
3. **Kein Abgleich ohne Nachweis:** Ohne gültigen Nachweis bleibt der Wunsch
   unbekannt, und das Träger-Büro prüft ihn. ADR 0110 Punkt 4 gilt
   unverändert: Name und Geburtsdatum schalten nichts frei.

## Nachtrag 11.11.2026: Nachweis des Kontos (#6673)

Die Vorbefüllung gibt seit [ADR 0216](0216-ferienanmeldung-kennt-das-elternkonto-und-eltern-ohne-schule-bekommen-ein-konto-am-traeger.md) jedem angemeldeten Konto zusätzlich einen
versiegelten Nachweis des Kontos mit (`account_proof`, Träger und Zeitraum
des Links, 24 Stunden), auch einem Konto ohne Kind an den Schulen des
Trägers. Die öffentliche Anmeldung trägt damit das Konto ein. Punkt 2 der
Folgen ist insoweit ersetzt: Die Anmeldung kennt das Konto, aber weiterhin
kein Schulkind.

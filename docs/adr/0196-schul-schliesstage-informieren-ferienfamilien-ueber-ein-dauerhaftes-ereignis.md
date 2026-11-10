---
status: proposed
---

# Schul-Schließtage informieren Ferienfamilien über ein dauerhaftes Ereignis

**Status: Vorgeschlagen.** Entscheidung vom 10.11.2026 zu
[#6442](https://github.com/moto-nrw/project-phoenix/issues/6442) und
[#6457](https://github.com/moto-nrw/project-phoenix/issues/6457), Etappe 3 der
Träger-App. Grundlage sind der
[Entscheidungskommentar mit Optionen A bis C](https://github.com/moto-nrw/project-phoenix/issues/6442#issuecomment-6762541405)
und Yannicks anschließende Entscheidung vom 10.11. für A, ergänzt durch die
[Entscheidungen zu ADR 0196 vom 10.11.](https://github.com/moto-nrw/project-phoenix/issues/6442#issuecomment-6777024406).
Die sieben Einzelentscheidungen sind freigegeben. Die Annahme der ADR als
Ganzes durch Yannick steht weiterhin aus. Dieser Doku-PR ändert weder
Produktcode noch Migrationen, Policy oder den heutigen Versand.

Die Schule schreibt den Schließtag und ein dauerhaftes Ereignis atomar.
Ein Workflow-Worker verarbeitet es in einer eigenen Träger-Transaktion:
Enrollment ermittelt betroffene Buchungen und Familien und beauftragt Delivery.
Die Schule erhält dafür keinen zusätzlichen Zugriff auf Elternkontakte.
Das gilt für Ausfall und Wiederöffnung, auch nach Löschen oder Verkürzen.

## Ausgangspunkt und Grenzen

[PR #6443](https://github.com/moto-nrw/project-phoenix/pull/6443), Merge
[`75374380c16abd67a28777759d218737e216fac6`](https://github.com/moto-nrw/project-phoenix/commit/75374380c16abd67a28777759d218737e216fac6),
führt `enrollment.HolidayClosures`, `traegercalendar.HolidayNotices` und
`enrollment.holiday_closure_notices` ein. Beim Träger-Schließtag entstehen
Mailauftrag und Vermerk in der Transaktion des Schließtags. Beim Schul-Schließtag
bleibt heute der Klick im Träger-Büro nötig. Der Vermerk je Anmeldung, Woche
und geschlossenem Tag kennt keine Wiederöffnung.

Maßgeblich sind [ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)
(Command-Teilnahme nur im selben Scope, dauerhafte Effekte mit dem auslösenden
Write), [ADR 0105](0105-ferienzeitraum-liegt-beim-traeger-in-der-traeger-transaktion.md)
und [ADR 0195](0195-traegerphase-und-traegerangebot-werden-je-schule-kopiert-und-erst-nach-freigabe-abgeglichen.md):
Schul- und Träger-Transaktionen bleiben getrennt. `WithinTraeger` in einer
Schul-Transaktion ist kein erlaubter Übergabepfad. Wie in ADR 0195 koordiniert
ein Workflow über öffentliche Fähigkeiten; er besitzt keine Tabellen.

**Genau einmal** bedeutet hier: höchstens ein dauerhafter Mailauftrag je
Vorgang und Familie, einschließlich aller betroffenen Kinder, Schulen, Wochen
und Tage. Familie bedeutet wie in #6443 die normalisierte Empfängeradresse
(Leerzeichen am Rand und Groß-/Kleinschreibung zählen nicht). Ein Vorgang ist
ein erfolgreich gespeicherter fachlicher Änderungsbefehl mit stabiler
`operation_id`; Transaktionswiederholungen behalten diese ID. Unverändertes
Speichern und ein bereits überholter Zwischenstand erzeugen keine Mail.
SMTP kann keine genau einmalige Ankunft im Postfach garantieren. Delivery
unterscheidet Auftrag, Transportannahme und unbekannten Ausgang; ein unbekannter
Ausgang darf keinen zweiten Auftrag durch erneute Ereignisverarbeitung auslösen.

## Entscheidung

### 1. Owner und Verträge

| Aufgabe | Owner und Grenze |
|---|---|
| Schließtag, Ereignis, Bearbeitungsstatus und Kalenderrevision | `school-calendar`; eigene öffentliche Fähigkeit `HolidayClosureEvents`, SQL nur im privaten Postgres-Adapter |
| Übergabe, Wiederanlauf, Reihenfolge und äußere Träger-Einheit | neuer Workflow-Owner `holiday-closure-notification`, unter `workflows/holidayclosurenotification`; keine Daten, kein SQL |
| Betroffene Buchungen, Familienbündelung, Benachrichtigungszustand und Mailinhalt | `enrollment.HolidayClosures`; Erweiterung um beide Richtungen und Verarbeitung einer Vorgangs-ID |
| Mail-Outbox, Transport, Zustellstatus und Transportwiederholung | `delivery-platform`; vorhandener Träger-Mailweg |
| Aktuelle Schulen des Trägers | `organization-tenancy`; öffentliche, im vorhandenen Scope lesende Fähigkeit |

Der Workflow deklariert Ports für Ereignisse, Schulen, Kalenderlesung,
Enrollment und Transaktionslaufzeit. Die Root bindet sie an öffentliche
Fähigkeiten, konstruiert den Worker vollständig und registriert den Job.
School Calendar ruft keinen Workflow auf. Der Worker benötigt weder eine
Eltern- noch eine Schulsitzung. Scheduler Runtime weckt ihn, entscheidet aber
weder über betroffene Familien noch über Wiederholungszustände.

### 2. Dauerhafte Übergabe und Lebenszyklus

School Calendar erhält die Tabelle `calendar.holiday_closure_events` mit
`event_id`, `operation_id`, `organization_id`, `school_id`, Quellart und
Quell-ID, Revision, alten und neuen Datumsbereichen sowie Erstellzeit.
Die Revision steht in `calendar.holiday_closure_revisions` und steigt je
Schule unter einer Kalender-Sperre; beim Träger-Vorgang
tragen die Ereignisse aller betroffenen Schulen dieselbe `operation_id`.
Das Ereignis enthält keine Eltern-, Kinder-, Buchungs- oder Kontaktdaten und
keinen freien Schließgrund. Tage sind `calendar.Date` / `DATE`, Zeitpunkte
`TIMESTAMPTZ`.

- Anlegen, relevante Änderung und Löschen schreiben Ereignis und Schließtag
  in derselben Schul-Transaktion. Beim Löschen bleiben Quell-ID und alter
  Bereich als Löschvermerk im Ereignis erhalten; kein `ON DELETE CASCADE`
  vom Schließtag auf das Ereignis. Alte und neue Schulauswahl werden bei
  Träger-Schließtagen ebenfalls berücksichtigt.
- Ein Unique-Key auf Vorgang, Schule und Quellobjekt verhindert ein zweites
  Ereignis bei Wiederholung desselben Befehls. Der Client übermittelt eine
  bei HTTP-Wiederholung stabile Vorgangs-ID; sie ist an Scope, Befehl und
  Eingabe-Fingerabdruck gebunden, andere Eingaben unter derselben ID werden
  abgelehnt. Ein Rollback hinterlässt weder Änderung noch Ereignis.
- Die Datenbank leitet beim Schreiben die Trägerzuordnung aus der aktuellen
  Schule ab und prüft sie unter einer gegen Umhängen wirksamen Sperre;
  die Schule kann keinen fremden `organization_id` einschleusen. Ohne
  zugeordneten Träger gibt es keinen Versandauftrag. Ein zugeordneter Träger
  ohne betroffene Buchungen führt zu einem geprüften Abschluss ohne Mail.
- Bearbeitungsfelder sind `pending`, `leased`, `completed`, `superseded`,
  `cancelled` oder `blocked`, dazu Lease-Token, Lease-Ende, Versuchszahl,
  nächster Versuch und technischer Fehlercode. Ein abgelaufener Lease ist
  wieder übernehmbar. Abschluss verlangt den aktuellen Lease-Token.
- Verbindungsfehler, Rollback und Absturz vor Commit führen zum erneuten
  Versuch mit begrenztem Backoff. Nach ausgeschöpftem Versuchslimit bleibt
  das Ereignis `blocked`. Der Operator nimmt hängende oder `blocked`
  Ereignisse ausschließlich am selben Ereignis mit derselben `operation_id`
  wieder auf. Vorhandene Versandnachweise und Outbox-Einträge bleiben
  maßgeblich; ein unklarer Transportausgang verlangt zuerst Klärung. Es gibt
  weder einen zweiten Versandweg noch einen neuen Auftrag als Fehlerbehandlung.
  Ein Nachhollauf findet fällige Einträge auch ohne neues Signal.

Nur abgeschlossene, überholte oder fachlich aufgehobene Ereignisse werden
30 Tage nach ihrem Abschluss gelöscht. Offene und blockierte Ereignisse
haben keine pauschale TTL. Vergangene Tage sind kein Grund für einen stillen
Abschluss ohne Mail; insbesondere darf der Worker `blocked` weder wegen
Zeitablaufs noch als automatisch leer gewordenen Fall als erledigt markieren.
Der Betrieb muss den Fall klären und die fehlgeschlagene Verarbeitung am
selben Ereignis wieder aufnehmen. Die Mail-Outbox behält ihre eigene Frist;
das Löschen eines Ereignisses entfernt keinen Idempotenznachweis von
Enrollment und lässt keinen alten Vorgang erneut entstehen.

**Fristen und Alarm:** Schulereignisse werden bis 10 Minuten nach der letzten
Änderung der Schule gesammelt, spätestens bis 60 Minuten nach der ältesten
offenen Änderung (Berechnung in Abschnitt 4). Bei `blocked` wird sofort ein
Alarm ausgelöst; ebenso bei einem offenen Ereignis, das älter als 30 Minuten
ist. Sein Alter zählt ab dem ursprünglichen Erstellzeitpunkt, nicht ab dem
letzten Versuch. Auch eine noch laufende Sammlung bis zur 60-Minuten-Grenze
unterdrückt den 30-Minuten-Alarm nicht. Ein unabhängiger Überwachungslauf muss
das Alter erkennen, auch wenn der Verarbeitungsjob hängt oder ausfällt.
10, 60 und 30 Minuten sind gesetzte Produkt- und Betriebsfristen vom
10.11.2026, keine gemessenen Laufzeiten oder Zusagen zur Postfachankunft.
Die Statusanzeige im Träger-Büro (Abschnitt 5) zeigt den nächsten Handelnden
bei einem Fehler; sie bietet keine Versandaktion.

**Geprüfter Bestand und fehlende Betriebsfunktionen:** Codeprüfung auf
[`5eb0fbff187b48a47e0c8b0ce794939bd55d7411`](https://github.com/moto-nrw/project-phoenix/tree/5eb0fbff187b48a47e0c8b0ce794939bd55d7411):

- Ein allgemeiner Fehler-Meldeweg für Jobs existiert: Scheduler Runtime meldet
  endgültig fehlgeschlagene Läufe und Panics an Sentry
  ([`job_polling.go`](https://github.com/moto-nrw/project-phoenix/blob/5eb0fbff187b48a47e0c8b0ce794939bd55d7411/backend/modules/schedulerruntime/internal/application/job_polling.go#L301),
  [`sentry_reporting.go`](https://github.com/moto-nrw/project-phoenix/blob/5eb0fbff187b48a47e0c8b0ce794939bd55d7411/backend/modules/schedulerruntime/internal/application/sentry_reporting.go#L13)).
  Für die nächtliche Löschung existiert zusätzlich die Grafana-Regel
  [`nightly-cleanup-failing`](https://github.com/moto-nrw/project-phoenix/blob/5eb0fbff187b48a47e0c8b0ce794939bd55d7411/monitoring/grafana/provisioning/alerting/nightly-cleanup.yml#L18).
  Das ist ein nutzbarer Alarmweg, aber noch kein Alarm für Schließtag-Ereignisse:
  Zustands- und Altersüberwachung für `blocked`/30 Minuten samt Regel,
  Kontaktweg zum moto-Team und Runbook gehören zur Worker-Umsetzung.
  Ob die vorhandenen Alarmkontakte produktiv zugestellt werden, wurde bei
  dieser statischen Codeprüfung nicht geprüft.
- Das Operator-Portal hat heute keine Wiederaufnahme von Schließtag-Ereignissen
  und keine allgemeine Job-/Ereignis-Wiederaufnahmeoberfläche. Geprüft wurden
  die [Operator-Routen](https://github.com/moto-nrw/project-phoenix/blob/5eb0fbff187b48a47e0c8b0ce794939bd55d7411/backend/api/operator_routes.go),
  ihre Route-Golden und die Seiten unter `frontend/src/app/operator`.
  Eine autorisierte, protokollierte Operator-Aktion zur Wiederaufnahme samt
  Ereignisstatus und Prüfung unbekannter Transportausgänge ist deshalb Teil
  der Umsetzung, keine bereits vorhandene Fähigkeit. Sie ruft denselben
  Workflow mit derselben Ereignis- und Vorgangs-ID auf; kein direkter Versand.

### 3. Lesepfad und RLS des Workers

Der Job-Einstieg ermittelt über die bestehende administrative Trägerauflistung
nur aktive Träger-IDs. Diese Auflistung liest weder Kalender noch Familien.
Danach öffnet der Workflow je Träger eine eigene `WithinTraeger`-Einheit mit
`phoenix_tenant`, `app.current_org_id` und ohne `app.current_tenant_id`.
Darin erfolgen Claim, Lesungen, Mailauftrag, Vermerke und Ereignisabschluss.
Es gibt keine Schul-Transaktion und keinen Scope-Wechsel innerhalb der Einheit.

1. **Schulen:** Der Port bindet an
   `organizationtenancy.ListSchoolsByOrganization(ctx, organizationID)`, wie
   die heutige Autorisierung in `roots/owners/school_calendar_traeger.go`.
   Er filtert nicht gelöschte Schulen und prüft die zurückgegebene
   `OrganizationID`. `ListTraegerSchoolIDs` wird hier ausdrücklich nicht
   verwendet: Dessen heutige Prüfung liest erst das Trägerobjekt, das in der
   Träger-Transaktion nicht sichtbar ist. Eine außerhalb gelesene Schulliste
   allein autorisiert keinen Versand.
2. **Ereignisse:** Die Tabelle erhält `FORCE ROW LEVEL SECURITY`. Schreiben
   der unveränderlichen Ereignisfelder ist nur im Quell-Scope erlaubt;
   Claim und Statusänderungen in der Träger-Einheit sind auf deren
   `organization_id` begrenzt und verändern nie den Payload. Ein Datenbank-Guard
   erzwingt diese Feldtrennung auch für die gemeinsame Request-Rolle; eine
   permissive zweite RLS-Policy allein genügt dafür nicht. School Calendar
   prüft den Scope vor jedem Statement und filtert zusätzlich explizit nach
   Träger, Schule und Ereignis. Geänderte Zugehörigkeit oder gelöschte Schule
   führt zu `cancelled`, nie zum Versand unter dem neuen Träger.
3. **Aktueller Kalender:** Der vorhandene Port
   `enrollment.HolidaySchoolCalendar` erhält für diesen Ablauf eine Bindung
   über die um eine Trägerlesung erweiterte Projektion
   `school-closing-days-view`. Sie liest in derselben Träger-Transaktion die
   aktuellen Schul- und Träger-Schließtage und das Bundesland der eigenen
   Schulen. Der heutige Adapter öffnet dafür Schul-Transaktionen und für
   das Bundesland eine administrative Settings-Lesung; er ist für diesen
   Worker daher **nicht** die Bindung.
4. **Enger Datenbank-Lesekanal:** Eine feste, nur lesende Funktion mit
   `SECURITY DEFINER`, festem `search_path`, eigener minimal berechtigter
   Rolle und ohne `PUBLIC EXECUTE` überbrückt ausschließlich die hier
   benötigten Schultabellen. Nur diese nicht an Requests vergebene Rolle
   erhält eng auf den aktuellen Träger begrenzte SELECT-Policies; sie hat
   weder Schreibrechte noch allgemeines `BYPASSRLS`. `FORCE RLS` bleibt aktiv. Sie prüft den aufrufenden Träger-Scope, bindet
   jeden Schulfilter in derselben Anweisung an die aktuelle, nicht gelöschte
   Zugehörigkeit und liefert nur Schul-ID/-Name, Bundesland sowie
   Schließbereiche und Revisionen. Für `operations.federal_state` liest sie
   ausschließlich diesen Schlüssel aus `config.setting_values`, mit der
   Registry-Voreinstellung über einen gebundenen Settings-Vertrag; keine
   allgemeine Settings-Freigabe, kein Env-Fallback. School Calendar berechnet
   daraus die gesetzlichen Feiertage. Ungültige Konfiguration blockiert die
   Verarbeitung. Schulwechsel nach dem ersten Lesen wird vor Commit unter
   der Zugehörigkeitssperre erneut geprüft. Diese reine Leseprojektion ist
   kein schreibender Datenbank-Workflow nach ADR 0067 Punkt 7.
5. **Buchungen und Familien:** Enrollment liest seine Trägertabellen mit
   `traegerDB` und explizitem `organization_id` in jedem Join und Filter,
   begrenzt auf die autorisierten ausrichtenden Schulen und betroffenen
   Bereiche. Auch bei RLS-Bypass dürfen fremde Buchungen nicht einfließen.
   Außerhalb dieser Transaktion werden keine Empfänger ermittelt.

Die zusätzliche Leseberechtigung gilt für diesen Träger-Lesekanal, nicht für
Schul-Sitzungen oder generische Schul-/Settings-Abfragen. Bestehende
Träger-Tabellen erhalten keine allgemeine Schul-Lesepolicy.

### 4. Beide Richtungen, Reihenfolge und Idempotenz

Unter der bestehenden Enrollment-Sperre je Träger liest der Worker Buchungen,
Benachrichtigungszustand und aktuellen Kalender neu. Berücksichtigt werden wie
in #6443 angenommene/bestätigte Anmeldungen mit Platz oder Warteliste,
im regulären Ablauf Werktage ab heute, bei Tagesbuchungen ausschließlich
gebuchte Tage und keine gesetzlichen Feiertage. Für eine verspätete
betriebliche Wiederaufnahme darf der Datumsfilter eine noch ausstehende Mail
nicht still entfallen lassen: Der Operator klärt den alten Ausfall oder die
Wiederöffnung am selben Ereignis. Verstrichene Tage müssen in dieser Nachricht
als vergangen erkennbar sein; eine Aufforderung zur künftigen Ersatzbetreuung
wäre falsch. Der Worker prüft den gesamten alten und neuen Bereich und alle
weiterhin wirksamen Schul- und Träger-Schließtage.

- **Neu oder verlängert:** Nur tatsächlich neu geschlossene gebuchte Tage
  gegenüber dem zuletzt beauftragten Benachrichtigungsstand werden mitgeteilt.
- **Gelöscht oder verkürzt:** Eine Wiederöffnungsmail geht nur an weiterhin
  betroffene Buchungen, für die zuvor ein Ausfall beauftragt wurde und deren
  Tage nun wirklich offen sind. Ein anderer Schließtag oder Feiertag verhindert
  die Entwarnung. Warteliste bedeutet weiterhin keine Platzzusage.
- **Verschieben und Schulauswahl ändern:** Eine gemeinsame Mail je Familie
  enthält die beiden Abschnitte „Diese Tage sind geschlossen“ und
  „Diese Tage sind doch geöffnet“, soweit jeweils Tage betroffen sind.
  Eine Verschiebung erzeugt keine zwei Mails. Reine Änderung des Grundes
  und unverändertes Speichern senden nichts.

`holiday_closure_notices` wird vom dauerhaften „Tag schon mitgeteilt“ zu
einem versionierten Zustand je Träger, Anmeldung, Woche und Tag erweitert:
zuletzt beauftragter Zustand `closed`/`open`, Revision, Vorgang und Bezug zum
Mailauftrag. Bestehende Vermerke werden als `closed` übernommen, ohne Mail.
Nach `closed → open → closed` ist die zweite Ausfallmail erlaubt. Ein eigener
Versandnachweis je `(organization_id, operation_id, family_key)` bleibt bis
zur Löschung der zugehörigen Ferienanmeldungen erhalten. `family_key` ist ein
trägergebundener, mit einem Schlüssel abgeleiteter Fingerabdruck der
normalisierten Adresse, keine Adresse im Log oder Idempotenzschlüssel.

Enrollment bündelt alle Änderungen eines Vorgangs je Familie und schreibt
Versandnachweis, Tageszustände und genau einen Träger-Outbox-Eintrag mit
stabilem Vorgangs-/Familienschlüssel. School Calendar schließt das Ereignis
in derselben Träger-Transaktion ab. Jeder Fehler rollt alles zurück; ein
Absturz nach Commit erzeugt beim Wiederanlauf keinen zweiten Auftrag.
Ein alter Vorschau-Token ist kein Worker-Eingang. Die neue Enrollment-Fähigkeit
ermittelt und prüft den Stand unter ihrer Sperre; der heutige Token-Vertrag
bleibt für bestätigte Büro-Aktionen erhalten.

**Rasche Änderungen:** Für Schulereignisse liegt der Verarbeitungszeitpunkt bei
`min(letzte Schuländerung + 10 Minuten, älteste offene Änderung + 60 Minuten)`.
Eine weitere Änderung setzt nur die 10-Minuten-Ruhefrist neu, nie die
60-Minuten-Grenze. Spätestens dort beginnt die Verarbeitung auch bei laufenden
Änderungen; Fehler bleiben über Wiederholung, Alarm und Operator sichtbar.
Diese Fristen sind gesetzt, nicht gemessen. Träger-Schließtage durchlaufen
diese Sammlung nicht und werden weiterhin synchron verarbeitet.

Pro Träger verarbeitet nur ein geschützter Ablauf die Kalenderereignisse,
innerhalb einer Schule in Revisionsreihenfolge. Er liest
den aktuellen Kalender statt alte Ereignispayloads als Wahrheit zu versenden.
Die Arbeitsmenge umfasst die Vereinigung der alten und neuen Bereiche aller
zusammen betrachteten Ereignisse. Eine frühere Änderung endet nur dann
`superseded`, wenn sie gegenüber dem letzten Benachrichtigungsstand keinen
noch wirksamen Unterschied mehr beiträgt. Disjunkte Änderungen derselben Schule
bleiben erhalten und werden ihrem jeweiligen Vorgang zugeordnet; ältere
Ereignisse werden nicht pauschal durch die neueste Schulrevision ersetzt.
Ein Zustand desselben Tages wird dem letzten noch wirksamen Vorgang dieses
Tages zugeordnet. Vor der Familienbündelung ermittelt der Worker alle noch
wirksamen Schuländerungen dieses Vorgangs. Er prüft die gelesenen Revisionen
unter den Kalendersperren vor dem Schreiben noch einmal. Neue Revision:
gesamte Einheit neu beginnen. So erzeugt Anlegen und Löschen vor der
Verarbeitung keine veraltete Absage, ohne einen anderen Tag zu verlieren.

Die Sperrreihenfolge lautet: aktuelle Schulzugehörigkeiten nach Schul-ID,
Kalenderrevisionen nach Schul-ID, Ereignisse nach ID, Enrollment-Sperre je
Träger. Kalender-Schreibbefehle und synchroner Träger-Versand halten dieselbe
Reihenfolge ein. Der Workflow besitzt das Protokoll und prüft den Stand unter
den Sperren, die Fach-Owner besitzen weiterhin ihre Prüfungen und Writes.

Delivery sendet nach Commit und wiederholt bekannte vorübergehende
Transportfehler am selben Outbox-Eintrag. Endgültiger Fehler bleibt sichtbar;
`DispatchOutcomeUnknown` wird geprüft statt blind neu beauftragt. Abhängige
Mails einer Familie werden in Vorgangsreihenfolge zur Zustellung freigegeben:
Eine Wiederöffnung darf eine vorher beauftragte Absage nicht überholen.
Ein noch unversendeter, sicher abbrechbarer Zwischenstand kann stattdessen
atomar aufgehoben werden. Ein unbekannter Transportausgang sperrt die
abhängige Folge bis zur Klärung; keine stillschweigende Erfolgsmeldung.

### 5. Synchroner Träger-Schließtag, Statusanzeige und Umstieg

Der synchrone Pfad beim Träger bleibt ohne Wartezeit: Vorschau, Bestätigung
und Schreiben laufen in der Träger-Transaktion. `HolidayNotices` wird um Vorgangs-ID,
Vorher-/Nachher-Zustand und beide Richtungen erweitert, einschließlich
Löschen. Kalenderereignis, Enrollment-Vermerk und Outbox entstehen atomar;
der sofort verarbeitete Vorgang endet dabei bereits `completed` und wird
vom Worker nicht erneut versendet. Dieselben Familien- und Tagesregeln gelten
wie beim Schulereignis. Die Vorschau für #6457 zeigt Familien, Kinder, Wochen,
Schulen und tatsächlich wieder offene gebuchte Tage; ein geänderter Stand
verweigert die Bestätigung und rollt das Löschen/Ändern zurück.

**Kein Fallback.** Yannick: „bitte keine fallbacks“. An die Stelle des Buttons
„Eltern informieren“ tritt eine reine Statusanzeige je Schließtag im
Träger-Büro, auch für den Vorgang eines gelöschten Schließtags. Es gibt keinen
Versand- oder „erneut senden“-Knopf. Die vier Zustände sind:

| Anzeige | Bedeutung |
|---|---|
| Wird gesendet | Sammlung, Ereignisverarbeitung oder bekannte Transportwiederholung steht noch aus |
| Gesendet | Alle erforderlichen Aufträge wurden vom Transport angenommen; keine Zusage der Postfachankunft |
| Nicht angekommen | Ereignis ist blockiert oder eine Mail endgültig fehlgeschlagen; ein überfälliger offener Vorgang wird mit Fehlerhinweis sichtbar |
| Ergebnis unbekannt | Mindestens ein Transportausgang ist ungeklärt; kein blindes erneutes Beauftragen |

Bei mehreren Familien darf ein teilweiser Erfolg den Fehler oder unbekannten
Ausgang nicht verdecken. Bei einem Fehler nennt die Anzeige den nächsten
Handelnden: „Das moto-Team ist informiert. Bitte informieren Sie die Familien
bis dahin selbst.“ Die Alarmanbindung und ihre Fehlerüberwachung müssen diesen
Hinweis tragen; ein fehlgeschlagener Alarm darf nicht als erfolgreich gemeldet
werden. Das Büro löst damit keinen zweiten Mailweg in moto aus. Hängende oder
blockierte Ereignisse nimmt ausschließlich der Operator betrieblich wieder auf
(Abschnitt 2), mit protokolliertem Operator-Konto.

**Ablösung in einem Produkt-PR:** Zusammen mit dem Worker entfallen der manuelle
Weg aus #6443, `PendingHolidayClosures`, `NotifyPendingHolidayClosures`, die
Routen `/traeger/holiday-closures/pending` und `/pending/notify`, die
Frontend-Funktionen `fetchPendingHolidayClosures` und
`notifyPendingHolidayClosures` sowie die Liste „Eltern noch nicht informiert“.
Die Vorschau für den synchronen Träger-Schließtag bleibt erhalten. Bis der
Worker samt Statusanzeige, Alarmierung und Operator-Wiederaufnahme bereitsteht,
bleibt der heutige Weg unverändert. Die Entfernung erfolgt nicht vorab: Es darf
keine Einführungslücke ohne Versandweg entstehen.

**Einmaliger Umstieg:** Die Migration der Produktumsetzung legt für offene,
noch nicht informierte künftige Schließtage Ereignisse mit deterministischer
`operation_id` an. Der Schlüssel wird aus Träger, Schule, Quell-Schließtag und
einer festen Umstiegskennung abgeleitet; erneutes Ausführen erzeugt kein zweites
Ereignis. Bestehende Vermerke gelten für ihre bereits mitgeteilten Tage als
erledigt und lösen keinen Neuversand aus; fehlen nur einzelne Tage, umfasst
der Nachholauftrag nur diese. Die aktuelle Buchungs- und Kalenderprüfung bleibt
beim Worker. Der Umstieg übernimmt die Ermittlung offener Fälle in die
Migration, nicht die abzuschaffende manuelle Laufzeit-API.

Vor Löschen/Verkürzen im Schul-Portal zeigt die Schule weiterhin nur die ihr
bereits erlaubten Wochen-/Tagesdaten; Familienkontakte und Träger-Familienlisten
bleiben dem Büro vorbehalten.

**Bewusste Abgrenzung:** Die Elternabrechnung bietet heute „Senden“ und
„Erneut senden“ an
([`billing-mail-card.tsx`](https://github.com/moto-nrw/project-phoenix/blob/5eb0fbff187b48a47e0c8b0ce794939bd55d7411/frontend/src/components/traeger/billing-mail-card.tsx#L213)).
Diese bestehende Abweichung ist nicht Teil dieses Auftrags. Daraus folgt kein
manueller Versandweg für Schließtag-Hinweise.

### 6. Datenschutz und Löschung

Zweck ist die verlässliche Information der buchenden Familie über ihre
Betreuung. Enrollment nutzt die vorhandenen Anmeldedaten und Empfängerregeln;
es entsteht keine neue Freigabe an die Schule. Das entspricht der
[Regel zum Informationsfluss](../../.claude/rules/information-flow.md):
Die Familie bekommt die nötige Information automatisch, die Schule benötigt
für diesen Vorgang keine zusätzliche Kontaktliste. Die bestehenden
Zugriffsgrenzen aus [ADR 0116](0116-ausrichtende-ogs-liest-ihre-teilnehmer-in-der-schul-transaktion.md)
bleiben bestehen.

Nur die Träger-Mail-Outbox enthält Adresse, Anrede sowie nötige Namen, Wochen,
Tage, Schule und Wartelistenhinweis. Keine Gesundheitsdaten, Geburtstage,
Bankdaten oder freien internen Gründe. Logs und Betriebsalarme enthalten nur
IDs, Zustand und technische Fehlercodes. Automatische Verarbeitung wird als
Systemvorgang mit Träger und Vorgangs-ID nachvollziehbar. Jede betriebliche
Wiederaufnahme protokolliert das handelnde Operator-Konto, Ereignis-ID,
unveränderte `operation_id`, Zeitpunkt, Grund und Ergebnis. „Beauftragt“ gilt
nicht als nachgewiesen zugestellt.
Buchungen, Kapazitäten, Platzstatus, Nachrücken und eingefrorene Beiträge ändern
sich in keiner Richtung. Korrekturen bleiben bei Storno/Gutschrift; diese ADR
entscheidet keine neue Preisregel aus #6434.

Ereignisse und Kalenderrevisionen ohne Personenbezug stehen im Lebenszyklus-Katalog und gehen bei
Schul-/Träger-Löschung mit. Personenbezogene Tageszustände und Versandnachweise
gehören Enrollment, tragen alle enthaltenen `registration_ids` und gehen mit
den zugehörigen Anmeldungen. Eine gebündelte Outbox-Mail muss wie in #6443
über **jede** enthaltene Anmeldung löschbar sein. Neue Tabellen ohne passenden
CASCADE-Pfad werden ausdrücklich in `platform.erase_closed_demo_visitors` und
`visitorTraceCounts` aufgenommen; die Demo-Löschung entfernt auch
Familienfingerabdrücke, Zustellabhängigkeiten und Mailpayloads. Sie darf keine
andere Familie löschen oder aus einem verbleibenden Auftrag erneut mailen.
Die Demo-Laufzeit bekommt weiterhin keine Träger-Transaktion (ADR 0105).

## Policy und Nachweise der späteren Umsetzung

Dieser Vorschlag ändert die Register noch nicht. Vor Produktcode braucht der
neue Workflow-Owner eine inhaltlich freigegebene ADR. Die Umsetzung muss im
selben Diff insbesondere folgende Einträge begründen:

| Register | Änderung und Nachweis |
|---|---|
| `backend/architecture/policy.json` | `holiday-closure-notification`, `kind: workflow`; bestehende Fach- und Plattform-Owner bleiben |
| `policy/holiday-closure-notification.json` | Pakete `workflows/holidayclosurenotification`, `ports`, `internal/application`, `compose` und Operator-`http`; nur öffentliche Owner-/Vertragsrollen und Laufzeit-Ports, keine fremden Interna, kein SQL, keine Rückkante aus School Calendar |
| `policy/school-calendar.json` | `data_objects` für `calendar.holiday_closure_events` und `calendar.holiday_closure_revisions`, `write_owner: school-calendar`; öffentliche Ereignisfähigkeit und Scope-Prüfungen |
| `policy/enrollment.json` | erweiterte `enrollment.holiday_closure_notices`, neuer Versandnachweis mit `write_owner: enrollment`; beide Richtungen über Capability und Delivery-Port |
| `policy/school-closing-days-view.json` | benannte `tenant_safe`-Trägerprojektion; genaue Lesetabellen `platform.schools`, `schedule.closing_days`, `schedule.traeger_closing_days`, `calendar.holiday_closure_revisions` und ausschließlich Bundesland in `config.setting_values`; kein Schreibrecht |
| `policy/organization-tenancy.json`, `policy/delivery-platform.json`, Root-Composition | bestehende Verträge prüfen; nur nötige öffentliche Capability für scope-erhaltende Schulliste bzw. geordneten Versand ergänzen; Zustellabhängigkeiten liegen in Deliverys `platform.email_outbox`, keine allgemeine RLS-Ausweitung |
| Transaktionsregister und `backend/test/traeger_transaction.go` | Job-Einstieg und Operator-Wiederaufnahme mit `WithinTraeger`, Operator-Recht vor Öffnen prüfen, keine Schul-/Admin-Verschachtelung; genaue neue Stellen und Begründung statt pauschalem Wachstum |
| Command-Effects, Composition-Inventar, Pass-throughs und Query-Budgets | neue Ports/Bindungen samt durable Effects nach ADR 0098, konkreter Sperrordnung und gemessenen Abfragen; keine unbegründete Budgeterhöhung |
| RLS-, Retention- und Demo-Erasure-Coverage, Datenlebenszyklus-Katalog | Scope der Ereignisse, personenbezogene Nachweise und alle gebündelten Anmeldungen, Abschluss-/Löschfristen und Leases prüfen |

Die Architektur-Prüfung muss neben Imports die Laufzeitgrenzen belegen
([backend/CLAUDE.md](../../backend/CLAUDE.md#target-architecture)). Verhaltenstests
müssen Rollback, Absturz vor/nach Commit, abgelaufenen Lease, Retry, parallelen
Worker und Operator-Wiederaufnahme, doppelte Befehle, schnelle Gegenänderungen,
erneutes Schließen, überlappende Schließtage/Feiertage, Tagesbuchung, mehrere
Kinder/Wochen/Schulen je Familie, Warteliste, Transportreihenfolge und
unbekannten Ausgang abdecken. Dazu kommen gezielte Prüfungen der gesetzten
10-/60-Minuten-Grenzen, weiterer Änderungen ohne Verschieben der Höchstfrist,
des unverzögerten Träger-Schließtags sowie der 30-Minuten- und `blocked`-Alarme
einschließlich eines ausgefallenen Workers und der Alarmzustellung. Nachweisen:
Wiederaufnahme erhält dieselbe Ereignis-/Vorgangs-ID und das Operator-Audit;
`blocked` wird auch nach Verstreichen aller Tage nicht still erledigt; eine
Verschiebung erzeugt eine Mail mit zwei Abschnitten. Die Statusanzeige enthält
keinen Versandknopf, zeigt Teilfehler und nächste Handelnde; alte manuelle
Routen, Funktionen und Liste sind entfernt. Die Umstiegsmigration ist
wiederholbar, überspringt mitgeteilte Tage und übernimmt die übrigen künftigen
Fälle ohne Doppelversand. Scope-Filter werden mit RLS **und** darunter
getestet: fremder Träger, fremde Schule, manipulierte Ereigniszuordnung,
Schulwechsel und Schullöschung. Kein Test nutzt die geteilte Test-DB auf 5433.
Seed, Hilfe und UI-Nachweise gehören zur Produktumsetzung in #6442/#6457.

## Verworfene Optionen

**B: Die Schule versendet selbst in ihrer Schul-Transaktion.** Die heutige
Teilnehmerliste liefert keine Empfängeradressen. B müsste der Schule neue
Kontaktleserechte, eine eigene Outbox-Nutzung und eine mit dem Büro geteilte
Duplikatvermeidung geben. Für den Zweck ist das nicht nötig; Enrollment besitzt
bereits Familienbündelung und Träger-Versand. B vergrößert den Zugriff und
schafft zwei konkurrierende Versandwege.

**C: Das Büro klickt weiterhin als regulären Ablauf.** Geringer Bauaufwand,
aber eine vergessene Aktion lässt Familien ohne nötige Nachricht. Die
Akzeptanzkriterien von #6442 bleiben unerfüllt. C bleibt nur bis zur Umsetzung
von A im heutigen Produktstand bestehen und wird mit dem Worker entfernt.
Auch als Fallback ist der Klick nach Yannicks Entscheidung ausgeschlossen.

Ebenfalls verworfen: ein Prozesssignal/AfterCommit ohne dauerhaften Eintrag
(verliert Arbeit bei Absturz), Schulzugriff auf die gesamte Träger-Transaktion
(verletzt ADR 0105), nur Polling der heutigen Pending-Liste (kennt gelöschte
Tage und Wiederöffnung nicht) sowie ein eigener Tabellen-Owner für den Worker
(verletzt ADR 0066: Workflows besitzen keine Daten).

## Entscheidungen Yannick (10.11.2026)

Die [freigegebenen Einzelentscheidungen](https://github.com/moto-nrw/project-phoenix/issues/6442#issuecomment-6777024406)
ersetzen die bisherigen offenen Fragen:

1. Kein Fallback und kein Versandknopf, sondern Status je Schließtag und
   klare nächste Handelnde bei Fehlern (Abschnitt 5).
2. Operator-Wiederaufnahme am selben Ereignis mit derselben `operation_id`;
   Alarm bei `blocked` und offenem Ereignis älter als 30 Minuten; kein stiller
   Abschluss von `blocked` ohne Mail, auch nicht für vergangene Tage (Abschnitt 2).
3. Der manuelle Weg aus #6443 entfällt im selben Produkt-PR wie der Worker;
   bis dahin bleibt er unverändert (Abschnitt 5).
4. Einmalige Migration offener künftiger Fälle mit deterministischer
   `operation_id`; vorhandene Vermerke zählen als erledigt (Abschnitt 5).
5. Schuländerungen nach 10 Minuten Ruhe verarbeiten, spätestens 60 Minuten
   nach der ältesten offenen Änderung; gesetzte, nicht gemessene Zeiten
   (Abschnitt 4, Alarmgrenze von 30 Minuten in Abschnitt 2).
6. Eine gemeinsame Mail bei Verschiebung, mit den Abschnitten „Diese Tage
   sind geschlossen“ und „Diese Tage sind doch geöffnet“ (Abschnitt 4).
7. Träger-Schließtag bleibt synchron und ohne Wartezeit (Abschnitt 5).

Die Freigabe dieser Einzelentscheidungen nimmt die ADR nicht als Ganzes an.
Sie bleibt „Vorgeschlagen“, bis Yannick sie insgesamt annimmt; der Merge dieses
reinen Doku-PRs ersetzt diese Annahme nicht.

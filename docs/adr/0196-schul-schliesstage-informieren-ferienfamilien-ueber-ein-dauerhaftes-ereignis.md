---
status: accepted
---

# Schul-Admin und Träger-Büro lösen Ferienhinweise mit einem Klick aus

**Status: Akzeptiert. Freigegeben von Yannick Wenger am 10.11.2026.**
Die Annahme gegen 21:00 UTC ändert den Kern der bisherigen Fassung zu
[#6442](https://github.com/moto-nrw/project-phoenix/issues/6442) und
[#6457](https://github.com/moto-nrw/project-phoenix/issues/6457):
„annehmen. nicht automatisch, wird aber vorgeschlagen und dann kann der admin
das versenden via 1 button klick“. Auf die Frage, wer klickt: „Beide dürfen“.
Dokumentiert in den Entscheidungen zu
[#6442](https://github.com/moto-nrw/project-phoenix/issues/6442#issuecomment-6778882126)
und [#6457](https://github.com/moto-nrw/project-phoenix/issues/6457#issuecomment-6778882759).

Eine Schul-Kalenderänderung schlägt einen Elternhinweis vor. Erst ein Klick
von Schul-Admin oder berechtigtem Träger-Büro beauftragt den Versand.
Beide Stellen verwenden denselben dauerhaften Auftrag und dieselbe
Einmal-Regel je Familie und Vorgang. Ein Versandfehler ist an beiden Stellen
sichtbar und dort per erneutem Klick nachholbar. Die Schule bekommt weder
Elternkontakte noch Familiennamen des Trägers.

Die Entscheidung ersetzt die Regeln der vorausgehenden Fassungen in
[PR #6497](https://github.com/moto-nrw/project-phoenix/pull/6497) und
[PR #6540](https://github.com/moto-nrw/project-phoenix/pull/6540) zu automatischem
Schul-Versand, 10-/60-Minuten-Sammlung, reinem Status ohne Knopf,
ausschließlicher Operator-Wiederaufnahme und Entfernung des Büro-Buttons.
Der Doku-PR ändert keinen Produktcode und führt keine Migration aus.

## Kontext und bestehender Code

[PR #6443](https://github.com/moto-nrw/project-phoenix/pull/6443), Merge
[`75374380c16abd67a28777759d218737e216fac6`](https://github.com/moto-nrw/project-phoenix/commit/75374380c16abd67a28777759d218737e216fac6),
führt `enrollment.HolidayClosures`, `traegercalendar.HolidayNotices` und
`enrollment.holiday_closure_notices` ein. Beim Träger-Schließtag entstehen
Mailauftrag und Vermerk synchron mit dem bestätigten Speichern. Beim
Schul-Schließtag informiert heute ausschließlich das Büro mit einem Klick.
`PendingHolidayClosures`, `NotifyPendingHolidayClosures`, die zugehörigen
HTTP-Routen und die Liste „Eltern noch nicht informiert“ existieren und bleiben
als regulärer Büro-Weg erhalten. Sie müssen an den gemeinsamen Auftrag
angebunden werden. Der bisherige Vermerk je Anmeldung, Woche und geschlossenem
Tag kennt weder Wiederöffnung noch das Nachholen eines gescheiterten Versands.

Maßgeblich bleiben [ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md),
[ADR 0105](0105-ferienzeitraum-liegt-beim-traeger-in-der-traeger-transaktion.md)
und [ADR 0195](0195-traegerphase-und-traegerangebot-werden-je-schule-kopiert-und-erst-nach-freigabe-abgeglichen.md):
Schul- und Träger-Transaktionen bleiben getrennt. Kein `WithinTraeger` in einer
Schul-Einheit und keine zusätzliche Schul-Leseberechtigung auf Elternkontakte.
Der Workflow koordiniert öffentliche Fähigkeiten und besitzt keine Tabellen.

## Entscheidung

### 1. Zwei Auslöser, ein Versandweg

| Stelle | Vorschlag und Aktion | Berechtigungsgrenze |
|---|---|---|
| Schul-Admin | Vorschlag direkt beim Eintragen/Ändern des Schul-Schließtags; „Eltern informieren“ löst mit einem Klick aus | Eigene Schule und ihr Schließtag, bestehende zulässige Kalender-/Wocheninformationen; keine Elternkontakte und keine Familiennamen des Trägers |
| Träger-Büro | Offener Fall unter „Eltern noch nicht informiert“; derselbe Button löst mit einem Klick aus | Aktueller Träger, zugehörige Schule und bestehendes Recht „Einstellungen verwalten“ |

Beide sind gleichberechtigt. Es gibt keine Wartepflicht auf die andere Stelle,
keine zusätzliche Freigabe und keinen zweiten Bestätigungsdialog nach dem
Versandklick. Das Speichern eines Schul-Schließtags allein sendet nichts.
Anlegen, Ändern, Verlängern, Verschieben, Löschen und Verkürzen nutzen dieselbe
Regel: Vorschlag und ausdrücklicher Versandklick. Auch ein gelöschter
Schließtag behält seinen offenen Hinweis mit altem Bereich.

Der Vorschlag wird beim Anzeigen und beim Klick gegen den aktuellen Stand
geprüft. Die Schule sieht beim Vorschlag die **Zahl der Familien**, die bei
einem Klick eine E-Mail bekämen („1 Familie hat …“ / „12 Familien haben …“),
und keine Namen, Kontakte oder Kinderdaten des Trägers (Abschnitt
„Familienzahl für die Schule“). Enrollment entscheidet in der
Träger-Verarbeitung, ob tatsächlich mitteilungspflichtige Buchungen vorliegen.
Ohne solche Buchungen entsteht keine Mail, und die Schule sieht keinen
Vorschlag.

### 2. Owner, Vorschlag und dauerhafter Auftrag

| Aufgabe | Owner und Grenze |
|---|---|
| Schul-/Träger-Schließtag, Vorschlag, Kalenderrevision und dauerhafter Versandbefehl | `school-calendar`, öffentliche Fähigkeit `HolidayClosureEvents`; SQL im privaten Postgres-Adapter |
| Autorisierung der beiden Einstiegspunkte, Folge getrennter Einheiten und Verarbeitung | Workflow-Owner `holiday-closure-notification` unter `workflows/holidayclosurenotification`; kein SQL, keine Daten |
| Betroffene Buchungen, Familienbündelung, Tageszustände und Versandnachweise | `enrollment.HolidayClosures` |
| Mail-Outbox, Transport, Zustellstatus und geschützte Wiederholung | `delivery-platform` über den vorhandenen Träger-Mailvertrag |
| Aktuelle Schulzugehörigkeit | `organization-tenancy`, öffentliche Fähigkeit oder benannte Lesung |

School Calendar speichert mit der Kalenderänderung einen noch **nicht
beauftragten Vorschlag** in `calendar.holiday_closure_proposals`: Träger,
Schule, Quellart/-ID, alte und neue Bereiche, Revision und kanonische
`operation_id`. In `calendar.holiday_closure_revisions` steigt die Revision
unter der Kalender-Sperre. Unbestätigte Zwischenstände desselben Quellobjekts
werden zum aktuellen Vorschlag zusammengeführt; unabhängige Änderungen dürfen
nicht verloren gehen. Die Vorgangs-ID gehört dem Vorschlag, nicht dem
klickenden Konto, dem Portal oder einem neuen zufälligen HTTP-Aufruf.

Erst der Klick schreibt den dauerhaften Befehl in
`calendar.holiday_closure_events`, eindeutig je Vorschlag/Vorgang und Schule,
samt bestätigter Revision, Bereichen, auslösendem Konto und Kanal. Vorschlag
und Auftrag enthalten keine Familien-, Kinder- oder Kontaktdaten. Tage sind
`calendar.Date` / `DATE`, Zeitpunkte `TIMESTAMPTZ`. Kontoreferenzen werden nach
dem bestehenden Audit-/Löschvertrag behandelt; der Auftrag benötigt keine
kopierten Namen. Löschen des Schließtags darf Vorschlag und Auftrag nicht per
CASCADE entfernen.

Für Schul-Schließtage schreiben **beide Klickwege** denselben Befehl in der
Schul-Einheit. Der Büro-Einstieg prüft sein Recht und die aktuelle
Träger-Schul-Beziehung vor dieser Einheit. Er darf keine bereits offene
Träger-Transaktion verwenden und darin eine Schul-Transaktion verschachteln.
Die bisherige Büro-Route und ihre Frontend-Funktionen bleiben als Oberfläche
bestehen, werden aber am Workflow-Einstieg statt am alten direkten
Benachrichtigungsaufruf gebunden. Die Auswahl mehrerer offener Fälle
beauftragt jeweils deren kanonischen Vorgang, keinen konkurrierenden Sammelweg.

Die Schul-Einheit bestätigt den Vorschlag unter seiner Sperre. Veraltete
Revision: kein Auftrag, aktualisierten Vorschlag anzeigen. Bei unverändertem
Stand ist der Auftrag nach Commit sofort fällig, ohne 10-/60-Minuten-Wartezeit.
Der Worker verarbeitet ausschließlich solche bestätigten Befehle in einer
separaten Träger-Transaktion. Vorschläge allein dürfen weder einen
Mailauftrag noch einen automatischen Versand auslösen.

Gleichzeitige Klicks, HTTP-Wiederholungen und ein späterer Klick der anderen
Stelle finden über Unique-Key und Sperre denselben Auftrag. Sein Anlegen,
die Bestätigung des Vorschlags und der Auslösevermerk sind in der Schul-Einheit
atomar. Ein Absturz nach Commit verliert den Versandbefehl nicht.

Der Auftrag durchläuft `pending`, `leased`, `prepared` (Mailaufträge liegen
vor), `completed` oder einen sichtbaren Fehlerzustand `blocked` beziehungsweise
`needs_confirmation`. Lease-Token, Lease-Ende, Versuchszahl und technischer
Fehlercode liegen beim Auftrag. Ein abgelaufener Lease ist wieder übernehmbar;
ein alter Worker darf mit seinem Lease-Token kein Ergebnis mehr schreiben.
`prepared` ist kein Zustellnachweis. Ein Verarbeitungsfehler und ein späterer
Delivery-Fehler werden im gemeinsamen Status zusammengeführt. Erst wenn die
erforderlichen Mails angenommen oder ein unversendeter Vorgang nachvollziehbar
abgelöst ist, ist der Auftrag fachlich abgeschlossen. Fehlgeschlagene oder
unklare Arbeit bleibt nachholbar, auch wenn ein Auslöse-Request abgebrochen ist.

### 3. Träger-Verarbeitung und RLS

Die Root bindet consumer-eigene Ports für Vorschläge/Aufträge, Schulen,
Kalender, Enrollment, Delivery und Laufzeit. Sie konstruiert den Worker
vollständig; Scheduler Runtime weckt ihn, entscheidet aber nicht über
Empfänger oder eine Versandfreigabe. Kein Fachmodul importiert den Workflow.

Nach Ende der Schul-Einheit öffnet der Workflow je Träger eine eigene
`WithinTraeger`-Einheit mit `phoenix_tenant`, `app.current_org_id` und ohne
`app.current_tenant_id`. Darin erfolgen Claim, berechtigte Lesungen,
Enrollment-Nachweise, Mail-Outbox und Verarbeitungsabschluss. Es gibt weder
Schul-Transaktionen noch einen Admin-Scope-Wechsel innerhalb dieser Einheit.

1. **Schulen:** Bindung an
   `organizationtenancy.ListSchoolsByOrganization(ctx, organizationID)` wie
   im heutigen `roots/owners/school_calendar_traeger.go`; zusätzlich aktuelle
   `OrganizationID` und nicht gelöschte Schule prüfen. Die heutige Methode
   `ListTraegerSchoolIDs` liest zuerst das hier unsichtbare Trägerobjekt und
   ist deshalb nicht der Lesepfad dieser Träger-Einheit.
2. **Auftrag:** `FORCE ROW LEVEL SECURITY`. Unveränderliche Befehlsfelder
   entstehen ausschließlich im autorisierten Quell-Scope. Claim und
   Bearbeitungsstatus sind in der Träger-Einheit auf deren `organization_id`
   begrenzt. Ein Datenbank-Guard erzwingt die Feldtrennung zusätzlich zur
   RLS; zwei permissive Policies allein reichen nicht. Explizite Filter und
   Scope-Prüfungen gelten auch unterhalb von RLS. Schulwechsel oder Löschung
   berechtigen nie zu einem Versand unter dem neuen Träger.
3. **Aktueller Kalender:** `enrollment.HolidaySchoolCalendar` erhält für
   diesen Ablauf eine Bindung über die um eine Trägerlesung erweiterte
   `school-closing-days-view`. Der heutige Adapter öffnet Schul-Transaktionen
   und für das Bundesland eine administrative Settings-Lesung; er ist
   deshalb nicht die Worker-Bindung.
4. **Enger Lesekanal:** Eine feste, nur lesende `SECURITY DEFINER`-Funktion
   mit festem `search_path`, minimal berechtigter Rolle und ohne
   `PUBLIC EXECUTE` liest nur die benötigten Schließbereiche, Revisionen,
   Schul-ID/-Name und das Bundesland. Die Rolle bekommt eng auf den
   aufrufenden Träger begrenzte SELECT-Policies, kein allgemeines
   `BYPASSRLS` und keine Schreibrechte. Jeder Schulfilter prüft die aktuelle
   Zugehörigkeit in derselben Anweisung. Für `operations.federal_state`
   gilt ausschließlich der passende Wert aus `config.setting_values` mit
   Registry-Voreinstellung über den Settings-Vertrag, keine allgemeine
   Settings-Freigabe und kein Env-Fallback. School Calendar ermittelt die
   gesetzlichen Feiertage. Ungültige Konfiguration lässt den bestätigten
   Auftrag sichtbar scheitern. Zugehörigkeit vor Commit erneut unter der
   passenden Sperre prüfen.
5. **Buchungen:** Enrollment nutzt `traegerDB`, expliziten Trägerfilter in
   jedem Join und Filter und nur autorisierte ausrichtende Schulen. Nur hier
   werden Familien ermittelt. Schulantworten erhalten ausschließlich den
   eigenen Vorschlags-/Auftragsstatus, niemals diese Empfängerergebnisse.

Die Schule erhält damit eine eng begrenzte Auslösebefugnis, keine allgemeine
Träger-Sitzung und keinen lesenden Zugriff auf dessen Kontakte oder
Familienlisten. Die Statusrückgabe wird als zweckgebundene, auf Quellschule und
Vorgang begrenzte Projektion bereitgestellt. Sie bleibt auch bei gebündelten
Mails ohne Informationen über andere Schulen oder Familien.

### 4. Aktueller Stand, beide Richtungen und Einmal-Regel

Enrollment prüft unter seiner bestehenden Trägersperre den bestätigten
Kalenderstand und die aktuellen zulässigen Buchungen. Der Klick beauftragt
diesen Kalenderstand, keine späteren, noch nicht bestätigten Änderungen.
Weicht die Quellrevision vor dem Erzeugen der Mail ab, darf der Worker den
Inhalt nicht still erweitern: Der Vorgang verlangt einen aktualisierten
Vorschlag und einen erneuten Klick. Sicher unversendete überholte Aufträge
werden atomar abgelöst; bereits versandte oder im Ausgang unklare Mails
werden nicht nachträglich verändert.

Die fachlichen Regeln aus #6443 bleiben: angenommene/bestätigte Anmeldungen,
Platz oder Warteliste, Werktage, keine gesetzlichen Feiertage und bei
Tagesbuchung nur gebuchte Tage. Der normale Vorschlag betrifft Tage ab heute.
Ein bereits beauftragter, später gescheiterter Fall verschwindet nicht beim
Tageswechsel: Er bleibt an beiden Stellen sichtbar und am selben Auftrag
nachholbar; nötige Hinweise auf vergangene Tage dürfen keine künftige
Betreuungszusage vortäuschen.

- **Neu/verlängert:** Nur tatsächlich neu geschlossene gebuchte Tage werden
  gegenüber dem zuletzt beauftragten Benachrichtigungsstand mitgeteilt.
- **Gelöscht/verkürzt:** Ein Vorschlag zur Wiederöffnung berücksichtigt nur
  weiterhin betroffene Buchungen mit zuvor beauftragtem Ausfallhinweis und
  wirklich wieder offene Tage. Weitere Schul-/Träger-Schließtage und
  Feiertage verhindern eine falsche Entwarnung. Warteliste bleibt Warteliste.
- **Verschoben:** Nach dem Klick genau eine gemeinsame Mail je Familie mit
  „Diese Tage sind geschlossen“ und „Diese Tage sind doch geöffnet“, soweit
  der jeweilige Abschnitt Tage enthält; keine zwei Mails für die Verschiebung.

`enrollment.holiday_closure_notices` wird zum versionierten Zustand je Träger,
Anmeldung, Woche und Tag (`closed`/`open`, Revision, Vorgang, Mailbezug).
Bestehende Vermerke gelten als `closed`, ohne Neuversand. Nach
`closed → open → closed` darf ein neuer bestätigter Vorgang erneut informieren.
Reine Änderung des Grundes oder unverändertes Speichern erzeugt keine Mail.

**Familie** bedeutet wie bisher die normalisierte Empfängeradresse.
**Genau einmal** heißt ein logischer Versandauftrag je
`(organization_id, operation_id, family_key)` mit allen betroffenen Kindern,
Wochen, Schulen und Tagen. `family_key` ist ein trägergebundener, mit einem
Schlüssel abgeleiteter Fingerabdruck; keine Adresse in Logs oder öffentlichen
Idempotenzschlüsseln. Enrollment bündelt, schreibt den Versandnachweis und
beauftragt genau einen logischen Träger-Mailauftrag je Familie. Tageszustände,
Versandnachweise, Mail-Outbox und Verarbeitungsabschluss committen zusammen.
Der Versandnachweis bleibt bis zur Löschung der zugehörigen Anmeldungen und
überlebt die Frist der Transport-Outbox.

Die Sperrreihenfolge bleibt: aktuelle Schulzugehörigkeiten nach Schul-ID,
Kalenderrevisionen nach Schul-ID, Vorschläge/Aufträge nach ID, dann die
Enrollment-Sperre je Träger. Die Owner behalten ihre Guards und Writes;
der Workflow besitzt das Protokoll. Der Worker verwendet keinen veralteten
Vorschau-Token als Versandfreigabe, sondern den unter Sperre bestätigten
Vorgang. Die Büro-Vorschau behält ihren Schutz gegen zwischenzeitlich geänderte
Buchungen; an der Klickgrenze geänderte Auswirkungen verlangen eine erneuerte
Ansicht statt eines still veränderten Versands. Schul- und Büro-Klick serialisieren am gleichen Vorschlag.

### 5. Sichtbarer Versandstatus und Nachholen per Klick

Beide Stellen sehen denselben fachlichen Auftragsstatus, die Schule nur für
ihre eigenen Vorgänge und ohne Empfängerdetails. „Eltern noch nicht informiert“
bleibt im Büro ein regulärer Arbeitsbereich mit Button. Nach dem Klick sind
Verarbeitungsfehler, teilweise gescheiterte Sendungen und unbekannte Ausgänge
weiterhin dort und im Schul-Portal auffindbar. „Gesendet“ bedeutet
Transportannahme, keine garantierte Ankunft im Postfach.

| Zustand | Text und Aktion an beiden Stellen |
|---|---|
| Vorschlag, noch nicht beauftragt | „Die Eltern wurden noch nicht informiert.“ Button „Eltern informieren“ |
| Bestätigt, wartet oder wird verarbeitet | „Wird gesendet“. Hinweis „Bitte warten Sie.“ Ein weiterer Klick legt nichts neu an |
| Keine mitteilungspflichtige Buchung gefunden | „Keine E-Mail nötig.“ Keine Erfolgsmeldung über einen Versand, der nicht stattgefunden hat |
| Alle erforderlichen Mails angenommen | „Gesendet“. Kein erneuter Versand für diesen Vorgang |
| Bekannter Fehler, sicher nachholbar | „Nicht angekommen“. „Bitte versuchen Sie es erneut.“ Button „Eltern informieren“ nimmt denselben Auftrag wieder auf |
| Unbekannter Ausgang | „Ergebnis unbekannt“. „Bitte lassen Sie den Versand vom moto-Team prüfen.“ Keine blinde Wiederholung; nach Klärung kann derselbe Button einen bestätigten Fehler nachholen |
| Quellstand vor Verarbeitung geändert | „Der Schließtag wurde geändert. Bitte prüfen Sie den Hinweis.“ Aktualisierter Vorschlag und Button „Eltern informieren“ |

Der normale Versandklick braucht genau einen Klick, keine zusätzliche
Bestätigung. Ein Zustandswechsel während einer offenen Ansicht wird
aktualisiert statt mit dem alten Stand versendet. Vorschlag, Status und
Aktion müssen im Produkt unterscheidbar sein; kein bloßes Fehleretikett ohne
nächsten Schritt. Hilfe und reale UI-Nachweise gehören zur Produktumsetzung;
diese ADR legt Texte fest, sie behauptet keinen bereits geprüften Bildschirm.

**Erneuter Klick nach Fehler:** Wiederaufnahme am selben Vorgang und denselben
Familienaufträgen, mit Prüfung beider Berechtigungen. Bereits erfolgreiche
Familien werden übersprungen. Noch wartende/geleaste Arbeit wird nicht doppelt
geclaimt. Sicher fehlgeschlagene Arbeit wird geschützt wieder fällig gemacht,
mit protokolliertem Versuch; ein unveränderter `Enqueue`-Aufruf, den der
Idempotenzschlüssel nur ignoriert, wäre kein funktionierendes Nachholen.
Delivery braucht dafür eine explizite Träger-fähige Wiederholungsfähigkeit,
die seine bestehenden Zustell-, Sperr- und Outcome-Regeln erhält. Eine
Wiederholung ist ein weiterer Versuch desselben logischen Auftrags, kein
neuer fachlicher Versand an schon erfolgreich informierte Familien.

Delivery transportiert nur committed Aufträge. Bekannte vorübergehende
Transportfehler darf es innerhalb der bestehenden Retry-Regeln wiederholen:
Das arbeitet einen menschlich beauftragten Versand ab und ist keine neue
Freigabe aufgrund einer Kalenderänderung. `DispatchOutcomeUnknown` bleibt
unklar, bis Providerabgleich oder Betrieb den Ausgang geklärt haben; SMTP
kann keine genau einmalige Postfachankunft garantieren. Abhängige Absage- und
Wiederöffnungsmails einer Familie dürfen einander nicht überholen.

### 6. Was von Wartezeit und Betrieb bleibt

**Entfällt:** Die 10-/60-Minuten-Sammlung; der Klick bestätigt den aktuellen
Stand und macht den Auftrag sofort fällig. Ebenso entfällt ein Alarm allein
wegen eines unbestätigten Vorschlags. Die bisherige 30-Minuten-Regel für
automatische Ereignisse ist ersetzt und wird nicht unverändert übernommen.
Kein automatischer Nachholversand unbestätigter Fälle und kein Operator als
einziger Nachholweg. Die reine Statusanzeige ohne Knopf und die Entfernung von
`PendingHolidayClosures`/`NotifyPendingHolidayClosures` sind verworfen.

**Bleibt nötig:** Lease, Idempotenz, Fehlerstatus, Transportwiederholung sowie
betriebliche Überwachung bereits **beauftragter** Arbeit. Nach dem Klick darf
ein hängender oder `blocked` Auftrag weder verloren gehen noch still wegen
vergangener Tage erledigt werden. Ein Worker-Wächter und Fehleralarme müssen
solche Aufträge auch bei ausgefallenem Verarbeitungsjob erkennen; ihre
technischen Laufzeitgrenzen werden in Umsetzung und Runbook begründet,
nicht als die aufgehobene Produktwartezeit ausgegeben. Beide berechtigten
Stellen können bekannte Fehler per Klick nachholen. Das moto-Team unterstützt
ungeklärte Transportausgänge und technische Sperren, ohne einen zweiten
Versandweg einzuführen. Eine neue Operator-Wiederaufnahmeoberfläche ist keine
Voraussetzung dieses menschlich ausgelösten Ablaufs.

Codebefund am Ausgangsstand
[`ffcf0751054ac5265396a1f7605ebb2846d9b4a7`](https://github.com/moto-nrw/project-phoenix/tree/ffcf0751054ac5265396a1f7605ebb2846d9b4a7):
Scheduler Runtime meldet endgültige Jobfehler und Panics an Sentry; für die
nächtliche Löschung gibt es `nightly-cleanup-failing` in Grafana. Der Alarmweg
ist damit im Code vorhanden, aber konkrete Überwachung von Schließtag-Aufträgen
fehlt. Das Operator-Portal bietet keine allgemeine Ereignis-Wiederaufnahme.
`delivery/runtime.Capability` hat `TraegerEmailStatuses`; das allein ist weder
eine Schul-Statusprojektion noch eine Wiederaufnahme des gemeinsamen
Träger-Versandauftrags. Diese Ergänzungen sind Teil der Produktumsetzung.
Die tatsächliche Zustellung produktiver Alarmkontakte wurde nicht geprüft.

### 7. Träger-Schließtag und Umstieg

Der Träger-Schließtag behält den heutigen synchronen Weg aus #6443: Vorschau
und bestätigtes Speichern beauftragen die Mail in derselben Träger-Transaktion.
Kein zusätzlicher Wartejob und kein zusätzlicher Klick nach diesem ausdrücklich
bestätigten Schreiben. `HolidayNotices` verwendet dieselben Familienregeln,
Tageszustände und Versandnachweise. Für Löschen/Verkürzen nach #6457 zeigt die
Vorschau die Wiederöffnung; erst der bestätigende Klick beauftragt den Hinweis.
Eine spätere Bearbeitung ohne Versandfreigabe darf keine neue Mail erzeugen.

Der Umstieg erfolgt mit dem gemeinsamen Versandweg in einem Produkt-PR,
ohne Einführungslücke und ohne automatische Mailserie aus einer Migration:

- Offene, noch nicht informierte künftige Schließtage werden einmalig als
  **Vorschläge**, nicht als freigegebene Versandbefehle übernommen. Die
  deterministische Vorgangs-ID nutzt Träger, Schule, Quell-ID/Revision und
  eine feste Umstiegskennung; Wiederholung der Migration erzeugt nichts doppelt.
- Bestehende Vermerke gelten für ihre Tage als erledigt. Ein Nachholvorschlag
  enthält nur fehlende Tage. Bereits beauftragte/gescheiterte Sendungen und
  ihre Zustellnachweise bleiben erhalten; sie werden weder als erfolgreich
  erklärt noch auf einen zweiten Versandweg umgeschrieben.
- `PendingHolidayClosures`, `NotifyPendingHolidayClosures`, die Route
  `/traeger/holiday-closures/pending/notify`, die Frontend-Funktionen
  `fetchPendingHolidayClosures`/`notifyPendingHolidayClosures` und „Eltern noch
  nicht informiert“ bleiben bestehen. Sie erhalten gemeinsame Vorgangs- und
  Statusbezüge und umfassen auch noch offene Wiederöffnungshinweise.
- Erst mit funktionsfähigem Schul-Button, Büro-Anbindung, Statusprojektion
  und Retry wird der neue Pfad eingeschaltet. Der Altbestand bleibt bis
  dahin bedienbar; es gibt keinen parallelen direkten Büro-Versand am neuen
  Idempotenzschutz vorbei. Vorschläge zu gelöschten Quellen bleiben erreichbar.

Die Elternabrechnung hat eigene „Senden“-/„Erneut senden“-Abläufe; sie werden
mit dieser ADR nicht geändert und liefern keinen zweiten Schließtag-Versandweg.

### 8. Datenschutz und Löschung

Zweck ist der vom Schul-Admin oder Büro beauftragte Hinweis an die buchende
Familie. Die [Regel zum Informationsfluss](../../.claude/rules/information-flow.md)
verlangt die nötige Information am Handlungsort, keine zusätzliche
Kontaktliste für die Schule. Die Grenzen von
[ADR 0116](0116-ausrichtende-ogs-liest-ihre-teilnehmer-in-der-schul-transaktion.md)
bleiben bestehen. Der Vorschlag gibt der Schule nur die freigegebene
Familienzahl (Abschnitt „Familienzahl für die Schule“), keine Namen, Kontakte
oder anderen Daten der Familien.

Nur der Träger-Mailweg verarbeitet Adresse, Anrede sowie die nötigen Namen,
Wochen, Tage, Schule und Wartelistenhinweise. Keine Gesundheits-, Bank- oder
Geburtsdaten und keine freien internen Gründe. Logs und Alarme enthalten
IDs, Zustand und technische Fehlercodes. Erstklick und Wiederaufnahme
protokollieren das tatsächlich handelnde Schul- oder Büro-Konto, Kanal,
Vorgang, Zeitpunkt und Ergebnis; Betriebsaktionen das Operator-Konto.

Vorschläge, Revisionen und Aufträge gehören in den Lebenszyklus-Katalog.
Schul-/Träger-Löschung und Kontolöschung berücksichtigen ihre Referenzen;
Aufträge dürfen weder am Schließtag-CASCADE hängen noch allein durch Alter
als zugestellt gelten. Fachlich abgeschlossene technische Aufträge werden
30 Tage nach Abschluss entfernt, unabhängige Versandnachweise bleiben bei
Enrollment. `prepared` mit fehlgeschlagenen oder unklaren Mails gilt dabei
nicht als abgeschlossen; offene Nachholfälle werden nicht durch diese Frist
entfernt. Ihre gesetzten fachlichen Löschfristen bleiben maßgeblich und dürfen
sie nicht als erfolgreich zugestellt umdeuten.
Unbestätigte Vorschläge sind kein Transportfehler; ihr fachliches Ende wird
sichtbar gemacht und nach den zugeordneten Löschfristen behandelt.

Personenbezogene Tages-/Versandnachweise gehen mit den Ferienanmeldungen.
Gebündelte Mails nennen alle `registration_ids` und sind über jede enthaltene
Anmeldung löschbar. Neue Tabellen ohne passenden CASCADE-Pfad gehören in
`platform.erase_closed_demo_visitors` und `visitorTraceCounts`; die Prüfung
umfasst Auslösekonto, Familienfingerabdrücke, Zustellabhängigkeiten und
Mailpayloads. Keine Löschung darf andere Familien treffen oder einen
verbleibenden Auftrag erneut versenden. Die Demo-Laufzeit erhält weiter keine
Träger-Transaktion. Buchungen, Platzstatus, Kapazität, Warteliste und Beiträge
ändern sich durch keinen Hinweis; Korrekturen bleiben bei Storno/Gutschrift.

## Policy und Nachweise der Produktumsetzung

Die ADR entscheidet die Zielgrenzen; dieser Doku-PR ändert noch keine Register.
Die Umsetzung muss im selben Diff insbesondere begründen:

| Register | Änderung und Nachweis |
|---|---|
| `backend/architecture/policy.json` und `policy/holiday-closure-notification.json` | Workflow-Owner, öffentliche Ports, Anwendung, Composition und Schul-/Büro-HTTP-Einstiege; keine Daten, kein SQL, keine Fachmodul-Rückkante |
| `policy/school-calendar.json` | `calendar.holiday_closure_proposals`, `calendar.holiday_closure_events`, `calendar.holiday_closure_revisions`, `write_owner: school-calendar`; Bestätigung nur durch berechtigten Klick, Scope- und Feldguards |
| `policy/enrollment.json` | Versionierte `enrollment.holiday_closure_notices`, Versandnachweise und beide Richtungen hinter Capability; Pending-/Notify-Verträge bleiben am gemeinsamen Workflow |
| `policy/school-closing-days-view.json` | Eng begrenzte `tenant_safe`-Trägerlesung von `platform.schools`, Schul-/Träger-Schließtagen, Kalenderrevisionen und Bundesland in `config.setting_values`; keine Writes |
| Zuständige Projektions-/Owner-Einträge | Schul-Statuslesung nur für eigene Vorschläge/Aufträge, keine Empfänger-/Familienfelder; expliziter Zweck und Mandantenfilter |
| `policy/delivery-platform.json`, `policy/organization-tenancy.json`, Root-Composition | Öffentliche Träger-Status-/Retry-Verträge, vorhandene Sperr-/Outcome-Regeln und aktuelle Schulzugehörigkeit, keine generischen Fremdtabellenzugriffe |
| Transaktionsregister, `backend/test/traeger_transaction.go` und Command-Effects | Schul-Bestätigung und getrennte Träger-Verarbeitung; Büro-Route ohne äußere Träger-Einheit für Schul-Vorgänge; konkrete Stellen, durable Effects und Sperrreihenfolge |
| RLS-, Lifecycle-, Retention-, Demo-Coverage und Query-Budgets | Neue Vorschläge/Aufträge und Akteursbezüge, alle gebündelten Anmeldungen; Nachweis unter RLS und darunter; keine pauschale Budgeterhöhung |

Verhaltenstests schützen insbesondere:

- Kein Versand beim Schul-Speichern, Ändern, Löschen, Migrieren oder bloßen
  Vorschlag; Klick an beiden Stellen und weiterhin synchroner Träger-Pfad.
- Gemeinsame Vorgangs-ID bei beiden Klicks, parallele Klicks/Worker,
  HTTP-Wiederholung, Rollback, Absturz vor/nach Commit und abgelaufener Lease.
- Sichtbarer Fehler an beiden Stellen, Retry am selben Auftrag, kein erneuter
  Versand an erfolgreiche Familien und keine blinde Wiederholung bei unbekanntem
  Ausgang; fachlicher Nachholfall bleibt auch nach seinem Tag erreichbar.
- Stale Vorschläge, Änderung nach Klick, Löschvermerk, Verschiebung mit einer
  Mail/zwei Abschnitten, überlappende Schließtage/Feiertage, Tagesbuchung,
  Warteliste, mehrere Kinder/Wochen je Familie und erneutes Schließen.
- Schul-Admin-/Büro-Rechte, eigene Schule, fremder Träger, Schulwechsel,
  Löschung und manipulierte Quellzuordnung; keine Schulantwort enthält Kontakte
  oder Familiennamen; die Familienzahl der Schule entspricht den Mails des
  Klicks. Filtertests mit und ohne RLS sowie Alarmierung nur für
  beauftragte Arbeit. Keine geteilte Test-DB auf Port 5433.

Seed, Hilfe, neue Schreibroute/Analyse und reale UI-/Verständlichkeitsnachweise
gehören zur Produktumsetzung nach den Repository-Regeln. Statische ADR-Checks
beweisen weder Laufzeitverhalten noch Mailzustellung.

## Verworfene Alternativen und ersetzte Entscheidungen

**Automatischer Schul-Versand nach Kalenderänderung:** Die frühere Empfehlung A
mit selbsttätigem Versand ist durch Yannicks Annahme gegen 21:00 UTC ersetzt.
Er verlangt Vorschlag und einen menschlichen Klick. Dauerhafte Übergabe und
Träger-Verarbeitung bleiben, aber nur für ausdrücklich beauftragte Sendungen.
Daraus folgen weder 10-/60-Minuten-Sammlung noch automatisches Versenden von
Altfällen oder eine Bedienung ausschließlich durch den Operator.

**B: Die Schule versendet selbst an Trägerkontakte:** Zusätzliche Kontakte und
eine Schul-Outbox sind für eine Auslösebefugnis nicht nötig. Sie würden zwei
Versandwege und zusätzliche personenbezogene Zugriffe schaffen.

**C: Ausschließlich das Büro darf klicken:** Das heutige Büro-Verfahren bleibt
ein regulärer Weg, allein reicht es aber nicht mehr. „Beide dürfen“ verlangt
zusätzlich den gleichberechtigten Schul-Admin. Ein unabhängiger zweiter Weg
hinter dem Schul-Button ist ebenfalls verworfen.

**Reine Statusanzeige ohne Knopf / Entfernen des Büro-Wegs:** Ersetzt durch
Vorschlag, Klick und beidseitiges Nachholen. Ein dauerhafter Vorschlag darf
nicht irrtümlich wie ein freigegebener Mailauftrag behandelt werden.

**AfterCommit/Prozesssignal statt dauerhaftem Befehl:** Verliert einen bereits
beauftragten Versand beim Absturz. Der Auftrag wird mit dem Klick committed;
ein Signal darf ihn nur aufwecken. Workflows bleiben ohne eigene Tabellen.

## Familienzahl für die Schule

**Entschieden. Freigegeben von Yannick Wenger am 10.11.2026** („mit zahl
gerne“, gegen 21:20 UTC). Die Schule sieht beim Vorschlag „Eltern
informieren“ die Zahl der betroffenen Familien. Zweck: Die Schule soll die
Tragweite ihres Klicks kennen. Weiterhin keine Namen, keine Kontakte und
keine Kinderdaten des Trägers, nur die Anzahl.

- **Grenze:** Die Zahl kommt über einen engen, lesenden Vertrag vom Besitzer
  der Buchungen: `enrollment.HolidayClosures.PlanHolidayClosureOperation` in
  einer eigenen Träger-Einheit, die der Workflow nach der Schul-Einheit
  öffnet. An die Schul-Antwort geht ausschließlich `families`; Wochen,
  Kinder und Empfänger bleiben in der Träger-Einheit. Die Schule erhält
  dadurch keinen allgemeinen Zugriff auf Trägerdaten.
- **Gezählt werden Familien** (normalisierte Empfängeradresse), nicht Kinder
  oder Buchungen, und zwar genau die, die bei einem Klick eine Mail bekämen:
  dieselbe Ermittlung wie der Versand (`NotifyHolidayClosureOperation`), damit
  Zahl und Versand nicht auseinanderlaufen. Bei 0 Familien gibt es keinen
  Vorschlag.
- Die Zahl gilt auch für Löschen und Verkürzen (#6457).

Diese Freigabe ersetzt die frühere Empfehlung „vorerst ohne Zahl“.

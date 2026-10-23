---
status: accepted
---

# Personal wird beim Träger geführt und in der Schule eingesetzt

Entscheidung vom 23.10.2026 zu [#4648](https://github.com/moto-nrw/project-phoenix/issues/4648),
Etappe 0 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 101 bis 111 und 118. Sie beantwortet
[#2817](https://github.com/moto-nrw/project-phoenix/issues/2817) und
[#2819](https://github.com/moto-nrw/project-phoenix/issues/2819), revidiert
für die Träger-Ebene die Ausschlüsse aus
[#1417](https://github.com/moto-nrw/project-phoenix/issues/1417) und hebt den
Ausschluss eines Dienstplans auf Träger-Ebene aus
[#2808](https://github.com/moto-nrw/project-phoenix/issues/2808) auf. Sie baut
auf [ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md),
[ADR 0086](0086-traeger-werte-liegen-zwischen-standard-und-schule.md) und
[ADR 0021](0021-wiederaufnahme-fuehrt-die-fruehere-beschaeftigung-fort.md) auf.
Der Product Owner hat sie am 23.10.2026 angenommen.

Eine Person, die der Träger beschäftigt, hat beim Träger einen
**Personalstamm** mit der Personalnummer des Trägers und einen oder mehrere
**Arbeitsverträge**. Vertrag, Soll, Urlaub, Stundenkonto und Monatsabschluss
liegen beim Träger, in Tabellen von Workforce mit RLS über die
Träger-Transaktion. Das Tagesgeschäft bleibt in der Schule: Check-in,
erfasste Zeiten, Schichten, gebuchte Abwesenheiten und Gruppen. Das
Träger-Büro sieht standardmäßig nur zusammengefasste Zahlen, personenscharf
nur mit dem Recht „Personal“. Das alles gilt nur für Träger, für die das
moto-Team „Personal beim Träger“ freigeschaltet hat. Alle anderen arbeiten
weiter je Schule.

## Kontext

- Alle Personaltabellen sind je Schule und haben RLS über `tenant_id`. Eine
  Person an zwei Schulen hat heute ein Konto (`auth.accounts`), zwei
  Personensätze (`users.persons`), zwei Beschäftigungen
  (`users.staff_school_memberships`) und zwei Profile
  (`users.staff_employment_profiles`). Verbunden sind sie nur über dieselbe
  `account_id`.
- Personalnummer, Art (`full_time`, `part_time`, `minijob`),
  Arbeitszeitmodell, Wochenstunden, Qualifikationen, Zeitkonto und
  Urlaubskonto hängen an der Beschäftigung (ADR 0021). Die Personalnummer ist
  nur je Schule eindeutig. Die Migration der Lohn-Grundlage nennt die zwei
  Sätze einer Person an zwei Schulen als bekannte Lücke
  (`001015229_payroll_foundation.go`).
- Saldo, Monatsabschluss (`active.staff_month_balance_snapshots`),
  Abwesenheitsarten und Kontingente gibt es je Schule. Übungsleiter und
  Ehrenamt gibt es nicht als Art.
- Der DATEV-Export (LODAS, Lohn und Gehalt) läuft je Schule, mit Berater- und
  Mandantennummer und Lohnarten als Schul-Einstellungen, per Download oder
  SFTP. Ohne Personalnummer verweigert er den Download.
- Die Schule hat einen Dienstplan (`schedule.staff_shifts`). Er prüft nur
  Überschneidungen, keine Arbeitszeitregeln. Die erfasste Zeit prüft Pausen
  nach § 4 ArbZG, mehr als 10 Stunden am Tag und mehr als 48 Stunden in der
  Woche. Die Ruhezeit nach § 5 ArbZG prüft nichts.
- `shiftplansync` überträgt eine Krankmeldung in derselben
  Tenant-Transaktion auf Dienstplan und Betreuungsplan, fail-closed.
- Qualifikationen sind freier Text mit Ablaufdatum, ohne Warnung vorab. Ein
  Führungszeugnis, einen Masernschutz-Nachweis oder einen Vertretungspool
  gibt es nicht.
- #1417 hat für die Schule entschieden: kein Ranking, kein Vergleich zwischen
  Mitarbeitenden, kein aufrechenbares Total über Schulen. Die
  Schulübersicht unter `users:read` zeigt deshalb nur Zahlen der Schule. Die
  Tabelle je Person verlangt `time_tracking:manage`.
- #2808 hatte einen Dienstplan beim Träger ausgeschlossen, weil Träger mit
  tarifführendem System (Vivendi PEP) keinen zweiten Dienstplan wollen.

## Entscheidung

### 1. Personalstamm, Arbeitsvertrag, Einsatzort

- Der **Personalstamm** ist die eine Identität einer Person, die der Träger
  beschäftigt, über alle seine Schulen. Personal, das der Träger nicht
  beschäftigt (Lehrkräfte des Landes, externe Betreuungskräfte, Kräfte von
  Kooperationspartnern), hat keinen Personalstamm und bleibt Personal der
  Schule.
- Ein Personalstamm hat einen oder mehrere **Arbeitsverträge**. Jeder hat
  seine Art (Teilzeit, Vollzeit, Minijob, Übungsleiter, Ehrenamt), sein Soll
  und sein eigenes Stundenkonto. Minijob und Übungsleiterpauschale werden so
  getrennt abgerechnet (Story 109).
- Die **Beschäftigung** bleibt nach ADR 0021 der Verlauf an einer Schule. Sie
  gehört zu höchstens einem Arbeitsvertrag. Ihre Schule ist dann ein
  **Einsatzort**. Genau ein Einsatzort je Arbeitsvertrag ist der
  **Haupteinsatzort**.
- Ein **Personalaustritt** beendet wie heute die Beschäftigung an einer
  Schule. Der Arbeitsvertrag läuft weiter. Das **Vertragsende** beendet einen
  Arbeitsvertrag und mit ihm alle seine Beschäftigungen.
- Wiederaufnahme und Neuaufnahme gelten unverändert an derselben Schule.

### 2. Freigabe

- Personalstamm und Arbeitsvertrag gibt es nur bei einem Träger, für den das
  moto-Team „Personal beim Träger“ freigeschaltet hat. Voraussetzung ist die
  Portal-Freigabe aus ADR 0084 §3.
- Vor der Freigabe bestätigt der Träger eine von zwei Aussagen: Es gibt eine
  Vereinbarung mit Betriebsrat oder Mitarbeitervertretung zu Zeiterfassung
  und Einsatzplanung über Standorte (§ 87 Abs. 1 Nr. 6 BetrVG bzw. kirchliches
  Mitarbeitervertretungsrecht), oder es gibt keine Vertretung. Bestätigung,
  Freigabe und Entzug stehen im Operator-Audit.
- Ohne Freigabe bleibt alles je Schule wie heute, auch bei einem Träger mit
  einer einzigen OGS (Story 13). Personen ohne Arbeitsvertrag bleiben auch
  bei einem freigeschalteten Träger je Schule.
- Der Code hat damit dauerhaft zwei Wege für Soll, Urlaub und Stundenkonto:
  je Beschäftigung oder je Arbeitsvertrag.

### 3. Was beim Träger liegt und was in der Schule

| Beim Träger (Workforce, Träger-Transaktion) | In der Schule (Tenant-Transaktion) |
|---|---|
| Personalstamm: Personalnummer, Stammdaten, Kontakt, Finanzdaten (IBAN, Steuer-ID, SV-Nummer) | Personensatz, Transponder, Rolle und Zugang |
| Arbeitsvertrag: Art, Eintritt, Ende, Probezeit, Wochenstunden, Soll je Wochentag und Einsatzort, Sonderarbeitszeit, Urlaubsanspruch, Kontingente | Beschäftigung mit Verweis auf den Arbeitsvertrag |
| Stundenkonto-Buchungen und Monatsabschluss je Arbeitsvertrag | erfasste Zeiten und Pausen |
| Abwesenheitsanträge, Abwesenheitsarten des Trägers | gebuchte Abwesenheiten als Tage am Einsatzort |
| Nachweise | Qualifikationen und Personaldokumente |
| Blockungen und Einsatzzeiten | Schichten, Gruppen, Klassen, Vertretungen |

- Die gebuchte Abwesenheit bleibt in der Schule, damit `shiftplansync` die
  Krankmeldung weiter fail-closed in derselben Tenant-Transaktion auf
  Dienstplan und Betreuungsplan überträgt. Workforce schreibt sie je
  betroffener Schule in einer eigenen Tenant-Transaktion. Ein Abgleich
  im Scheduler legt fehlende Tage idempotent nach.
- Die Tabellen beim Träger tragen `organization_id` und liegen im Schema
  `workforce`. Sie haben RLS nach ADR 0086 §5. Die zweite Policy für die
  Tenant-Transaktion liest nur Zeilen von Arbeitsverträgen mit einem
  Einsatzort an dieser Schule, und davon nur die Felder aus §6. Schreiben aus
  der Schule ist ausgeschlossen.

### 4. Personalnummer

- moto erzeugt keine Personalnummern. Die Personalverwaltung trägt die Nummer
  aus ihrem Lohnsystem ein.
- Sie ist je Träger eindeutig und je Personalstamm genau eine, für alle
  Arbeitsverträge. Sie darf leer sein. Der Lohnexport lehnt Personen ohne
  Nummer mit einer Liste ab.
- In v1 hat ein Träger genau einen Arbeitgeber und einen DATEV-Mandanten.
- Bei verknüpften Beschäftigungen gilt die Nummer des Personalstamms. Die
  Nummer im Profil der Beschäftigung bleibt nur für Personen ohne
  Arbeitsvertrag in Gebrauch.

### 5. Verknüpfen bestehender Daten

- moto schlägt Verknüpfungen vor: zuerst dasselbe Konto an mehreren Schulen,
  dann dieselbe Personalnummer, dann Name und Geburtsdatum. Die
  Personalverwaltung bestätigt jeden Vorschlag. Es gibt keinen automatischen
  Zusammenschluss.
- Widersprechen sich Felder, wählt sie je Feld. Der nicht gewählte Wert kommt
  mit Herkunftsschule in die Historie.
- Tragen zwei verschiedene Personen an zwei Schulen dieselbe Personalnummer,
  lassen sie sich erst verknüpfen, wenn eine Nummer geändert ist.
- Der Saldo jeder Schule wird eine Eröffnungsbuchung im Arbeitsvertrag mit
  Verweis auf die Schule. Abgeschlossene Monate bleiben je Schule. Die
  Verknüpfung wirkt ab dem ersten offenen Monat.
- Nichts in der Schule wird gelöscht. Die Beschäftigung bekommt nur den
  Verweis auf den Arbeitsvertrag.
- Der Personal-Import im Träger-Portal (Story 124) legt Personalstamm und
  Arbeitsverträge an und gleicht nur über die Personalnummer ab. Zeilen ohne
  Treffer werden neu angelegt und nicht automatisch mit Personensätzen der
  Schulen verknüpft. Der Import je Schule bleibt unverändert.

### 6. Was wer sieht und ändert

**Träger-Büro** (revidiert #1417 für die Träger-Ebene, beantwortet #2817):

- Ohne Recht „Personal“ (Übersicht, Berichte): nur Zahlen je Schule und
  Träger, etwa anwesend, abwesend, Soll, Ist und Saldo-Summe. Für Gruppen
  unter 5 Personen zeigt moto keine Zahl. „Abwesend“ steht ohne Grund da,
  also ohne „krank“ (Art. 9 DSGVO).
- Mit Recht „Personal“: personenscharf über alle Schulen, mit Zeiten,
  Stundenkonten, Abwesenheitsart, Nachweisen und Finanzdaten. Ein eigenes
  Recht für Lohndaten gibt es in v1 nicht.
- Für alle gilt: keine Rangliste und kein Vergleichs-Chart zwischen Personen.
  Filtern, etwa nach Saldo-Bereich, ist erlaubt.
- Personaldokumente, auch AU-Bescheinigungen, erreicht das Träger-Büro nie.

**Schule** für eine Person mit Arbeitsvertrag an ihrem Einsatzort:

- Die OGS-Leitung sieht Name, Kontakt und Notfallkontakt, Art des
  Arbeitsvertrags, Soll und Ist an ihrer Schule, den Saldo des ganzen
  Arbeitsvertrags als eine Zahl, den Status der Nachweise und Abwesenheiten
  an ihren Tagen.
- Sie sieht keine Finanzdaten und keine einzelnen Zeiten an anderen Schulen.
  Andere Einsatzorte erscheinen nur als „anderer Einsatzort“.
- Sie korrigiert erfasste Zeiten an ihrer Schule, plant Schichten und bucht
  Abwesenheiten für ihre Schule (Story 107). Als Haupteinsatzort entscheidet
  sie über Abwesenheitsanträge. Die Personalverwaltung kann immer
  entscheiden.
- Arbeitsvertrag, Soll, Urlaubsanspruch, Personalnummer und Stammdaten ändert
  nur die Personalverwaltung. Die Schule sieht diese Felder lesend mit „vom
  Träger“.

**Audit:** Jeder Zugriff des Träger-Portals auf Daten eines Personalstamms
oder Arbeitsvertrags schreibt nach ADR 0084 §7 einen Eintrag in
`audit.data_access_log` je Einsatzort, im Tenant dieser Schule. Bei einem
Springer gehört jede Schule dazu, die ihn je geblockt hat. Zusammengefasste
Zahlen werden nicht protokolliert.

### 7. Nachweise

- Ein Nachweis (Führungszeugnis, Masernschutz, Präventionsschulung oder eine
  Art, die der Träger verlangt) hängt einmal am Personalstamm, mit Art, Datum
  der Vorlage, Ablaufdatum und Ergebnis.
- Beim Führungszeugnis speichert moto nur Einsichtnahme, Datum und das
  Ergebnis „keine einschlägige Eintragung“, keine Kopie (§ 72a Abs. 5
  SGB VIII).
- Die Schule sieht je Person an ihrem Einsatzort nur „vollständig“, „fehlt“
  oder „läuft ab am …“. Die Warnung vor Ablauf geht an die Personalverwaltung
  und an die Leitung jedes Einsatzorts.
- Die Qualifikationen je Schule bleiben unverändert.

### 8. Planung über Schulen (hebt #2808 auf)

- Der Träger-Wert „Einsatzplanung in moto“ hat die Ebene `traeger` (ADR 0086
  §3) und den Standard „an“. Ist er aus, gibt es keine neuen Schichten, auch
  nicht in den Schulen. Vergangene Schichten bleiben lesbar und exportierbar.
  Vor dem Abschalten zeigt das Träger-Portal, welche Schulen künftige
  Schichten haben, und entfernt sie erst nach Bestätigung.
- Plant die Personalverwaltung eine Person an einer Schule ohne Beschäftigung
  ein, etwa an der ausrichtenden OGS einer Ferienwoche, legt ein Workflow
  dort eine befristete Beschäftigung an. Sie gibt Zugang nur in den geplanten
  Zeiten und wird danach eine frühere Beschäftigung. Ein späterer Einsatz ist
  eine Wiederaufnahme. Dauerhafte Einsatzorte legt die Personalverwaltung
  unbefristet an.
- Jede Schreiboperation auf Schichten einer Person mit Arbeitsvertrag
  schreibt beim Träger auch eine **Einsatzzeit**: Arbeitsvertrag, Tag,
  Beginn, Ende und Einsatzort, ohne Gruppe und ohne Grund.
- Beim Planen prüft moto über alle Einsatzzeiten des Personalstamms:
  Tageshöchstzeit, Ruhezeit, Pausen und Summe gegen das Soll. Die Grenzen
  sind Träger-Werte mit Standard nach ArbZG. Ein Verstoß warnt. Wer trotzdem
  plant, gibt eine Begründung an, und sie steht im Audit. Wegezeiten gibt es
  in v1 nicht.
- Überschneidende erfasste Zeiten an zwei Schulen zeigt der Monatsabschluss
  als offenen Punkt.

### 9. Vertretungspool

- Ein Arbeitsvertrag mit dem Merkmal „Vertretungspool“ kann an jeder Schule
  des Trägers eingesetzt werden. Die Person ist ein **Springer**.
- Eine OGS-Leitung **blockt** einen Springer für Tage oder Zeiten. Blocken
  zwei Schulen gleichzeitig, gewinnt die erste, die zweite bekommt eine klare
  Meldung. Andere Schulen sehen nur „geblockt von Schule X“, ohne Grund und
  ohne Gruppe. Die Personalverwaltung kann ein Blocken aufheben.
- Das erste Blocken legt über den Workflow aus §8 Personensatz und
  Beschäftigung in der Schule an, mit einer Springer-Rolle mit minimalen
  Rechten. Der Zugang gilt nur in den geblockten Zeiten.
- Die Schule sieht vom Springer nur Name, Status der Nachweise und die
  geblockten Zeiten.

### 10. Abwesenheiten, Stundenkonto, Monatsabschluss

- Mit Freigabe legt die Personalverwaltung Abwesenheitsarten und Kontingente
  für den Träger an (Story 106). Die fünf Grundarten im Code bleiben die
  Basis. Beim Einschalten ordnet sie jede eigene Art einer Schule einmal einer
  Art des Trägers zu. Für Personen mit Arbeitsvertrag legt die Schule keine
  eigenen Arten mehr an.
- Ein Kontingent gilt je Arbeitsvertrag. Eine Buchung über das Kontingent
  bleibt gesperrt.
- Ein Antrag geht an den Haupteinsatzort. Die anderen Einsatzorte sehen die
  Abwesenheit sofort an ihren Tagen.
- Das Stundenkonto rechnet je Arbeitsvertrag: Soll aller Einsatzorte gegen
  Ist aller Einsatzorte.
- Den Monatsabschluss macht die Personalverwaltung je Arbeitsvertrag
  (Story 108). Vorher zeigt moto je Schule die offenen Punkte: laufende
  Sitzungen, unbearbeitete Korrekturanträge, offene Anträge, überschneidende
  Zeiten. Danach sperrt jede Schule die Zeiten dieses Monats für diese
  Person. Der automatische Abschluss läuft je Arbeitsvertrag. Wiederöffnen
  darf nur die Personalverwaltung.
- Workforce liest dafür das Ist je Schule in deren Tenant-Transaktion und
  schreibt in der Träger-Transaktion. Es gibt keine Admin-Transaktion.

### 11. Exporte

- **Lohnexport** (Story 110): eine Datei je Träger über alle Arbeitsverträge,
  als DATEV LODAS oder Lohn und Gehalt. Berater- und Mandantennummer sowie
  die Lohnarten je Art des Arbeitsvertrags sind Träger-Werte.
  Schul-Abweichungen gelten im Träger-Export nicht. In v1 gibt es nur den
  Download im Träger-Portal. Ein SFTP-Ziel des Trägers entscheidet eine
  spätere ADR.
- **Zeitexport für Träger mit eigenem System** (Story 111): ein festes,
  öffentlich beschriebenes CSV je Personalnummer, Tag und Einsatzort mit
  Beginn, Ende, Pausen, Ist-Minuten und Abwesenheit mit Art. Abruf wie beim
  Lohnexport. Eine Zuordnung der Felder je Kunde kommt erst mit einer
  Formatbeschreibung des Kunden. Ein Import von Plänen kommt in v1 nicht.
- Jeder Export schreibt vorher einen Eintrag ins Zugriffsprotokoll. Scheitert
  der Eintrag, entsteht keine Datei.

### 12. Löschfristen

- Stundenkonto, Abwesenheitsanträge, Monatsabschluss und Einsatzzeiten eines
  Arbeitsvertrags folgen der Frist des Haupteinsatzorts für diese Datenart,
  gerechnet ab dem Vertragsende.
- Nachweise werden mit dem Ende des letzten Arbeitsvertrags gelöscht. Den
  Personalstamm löscht moto, wenn kein Arbeitsvertrag mehr eine Frist hat.
- Erfasste Zeiten in den Schulen folgen weiter der Frist der jeweiligen
  Schule.
- Eigene Fristen je Träger entscheiden
  [#4720](https://github.com/moto-nrw/project-phoenix/issues/4720) und die
  ADR, die ADR 0086 dafür vorsieht.

## Verworfene Optionen

- **Alles bleibt je Schule, der Träger summiert über `traeger-view`:** Es
  gäbe weiter zwei Urlaubskonten für einen Anspruch, keinen Monatsabschluss
  über Schulen und keine Prüfung für „vormittags A, nachmittags B“.
- **Gebuchte Abwesenheiten beim Träger:** Die Krankmeldung wirkte nicht mehr
  fail-closed in derselben Tenant-Transaktion auf Dienstplan und
  Betreuungsplan.
- **Ein eigener Owner für Personal beim Träger:** Saldo, Soll, Urlaub und
  Sonderarbeitszeit haben dieselben Invarianten wie in Workforce
  ([ADR 0081](0081-owner-granularity-follows-invariants-not-source-size.md)).
  Ein zweiter Owner müsste die Rechnung kopieren oder Workforce nur zum
  Wiederholen aufrufen.
- **Alle Träger auf einmal migrieren:** riskant, ohne Nutzen für Träger mit
  einer Schule, und Schulen ohne Freigabe verlören Funktionen, die sie heute
  pflegen.
- **„Beschäftigung“ für den Vertrag beim Träger:** Das Wort meint seit
  ADR 0021 den Verlauf an einer Schule, auch für Lehrkräfte. Eine Umbenennung
  hätte Begriffe und Texte der Oberfläche gebrochen.
- **Automatisch über das Konto zusammenführen:** Eine Lehrkraft, die an einer
  zweiten Schule als Übungsleiterin arbeitet, würde falsch zusammengelegt.
  Widersprüche in Lohndaten muss ein Mensch entscheiden.
- **moto vergibt Personalnummern:** Der Export ist nur brauchbar, wenn die
  Nummer der im Lohnsystem entspricht.
- **Nichts personenscharf beim Träger:** Lohnexport und Monatsabschluss über
  Schulen wären unmöglich. Der Träger ist Arbeitgeber (§ 26 BDSG).
- **Kopie des Führungszeugnisses:** nach § 72a Abs. 5 SGB VIII unzulässig.
- **Springer an allen Schulen dauerhaft beschäftigt:** Jede Schule sähe alle
  Springer. ADR 0084 hat automatische Mitgliedschaften in jeder Schule schon
  verworfen.
- **Arbeitszeitregeln als harter Block:** lähmt den Vertretungsfall am
  Morgen.
- **Prüfung über Schulen nur im Träger-Portal:** Den Konflikt erzeugt meist
  die Leitung beim Planen in ihrer Schule.
- **Freigabe je Schule vor dem Monatsabschluss:** Bei einem Springer mit acht
  Einsatzorten würde der Abschluss auf acht Leitungen warten.
- **Kein Schalter für die Planung:** Schulen eines Trägers mit tarifführendem
  System würden trotzdem planen, und es gäbe zwei Dienstpläne.
- **Mehrere Arbeitgeber je Träger in v1:** verdoppelt Personalnummer, Export
  und Rechte, ohne Kunden in der Spec, die das brauchen.

## Folgen

- Kein Code, bevor die neuen Tabellen als `data_objects` mit `write_owner`
  `workforce` in `policy.json` stehen und die Träger-Transaktion für Workforce
  in `backend/test/transaction_boundaries.json` eingetragen ist.
- Für das Anlegen einer befristeten Beschäftigung beim Planen und Blocken
  entsteht ein Workflow über Identity & Access, People Directory, School
  Membership und Workforce. Die Owner-Abhängigkeiten bleiben ein DAG
  ([ADR 0058](0058-owner-dependencies-form-a-dag.md)).
- `traeger-view` bekommt Flächen für die zusammengefasste Personalübersicht
  mit Mindestgröße 5 und für das personenscharfe Personal hinter dem Recht
  „Personal“ (ADR 0084 §1).
- Die Art des Arbeitsvertrags braucht neue Werte für Übungsleiter und
  Ehrenamt. Die Art im Profil der Beschäftigung bleibt für Personen ohne
  Arbeitsvertrag.
- ADR 0021 gilt unverändert an einer Schule. Für Personen mit Arbeitsvertrag
  liegen Personalnummer, Arbeitszeit und Urlaub nicht mehr an der
  Beschäftigung.
- ADR 0084 hatte Personal über Schulen offengelassen. Das ist hiermit
  entschieden.
- Die Ruhezeit nach § 5 ArbZG fehlt heute auch je Schule. Die Prüfung aus §8
  schließt die Lücke für Personen mit Arbeitsvertrag.
- Umsetzung in [#4705](https://github.com/moto-nrw/project-phoenix/issues/4705)
  (Personalstamm), [#4707](https://github.com/moto-nrw/project-phoenix/issues/4707)
  (Nachweise), [#4708](https://github.com/moto-nrw/project-phoenix/issues/4708)
  (Einsatzplanung), [#4709](https://github.com/moto-nrw/project-phoenix/issues/4709)
  (Vertretungspool) und [#4712](https://github.com/moto-nrw/project-phoenix/issues/4712)
  (Minijob, Übungsleiter, Lohnexport).
- Offen bleiben: das SFTP-Ziel des Trägers, mehrere Arbeitgeber je Träger,
  eigene Löschfristen je Träger und ein Import von Plänen aus tarifführenden
  Systemen.

# Ankunftszeiten werden aus Klassenzeiten und Buchungsfenstern projiziert

Status: accepted (#2414). Führt das Muster aus
`0001-angebots-gehzeit-materialisieren.md` auf die Ankunftsseite fort und setzt
den expliziten Tenant-Modus aus #2426 um.

## Kontext

`schedule.student_arrival_schedules` trägt eine Zeile pro Kind und Wochentag.
Sechs Schreibpfade füllen sie (Kinderdetail, Sammelbearbeitung, Kind anlegen,
CSV-Import, Anmeldeformular, genehmigter Elternantrag). Die Zeile trägt keine
Herkunft und kein Datumsfenster, und sie ist an keine Buchung gekoppelt.

Der Erwartet-Status hängt an dieser Zeile: `care_day_resolver` wertet einen Tag
nur dann als nicht gebucht, wenn weder Ankunft noch Gehzeit den Wochentag
plant. Im Vorfall vom 19.08. (OGS am Berg) blieben deshalb zwei vollständig
abgemeldete Kinder Mo bis Fr erwartet, bis die Zeilen von Hand aus der
Datenbank gelöscht wurden.

Die Produktionsanalyse vom 22.08. über alle Tenants widerlegt die im Issue
angenommene Ableitungsquelle:

- Innerhalb desselben Angebots stehen an jedem Wochentag zwei bis drei
  verschiedene Ankunftszeiten. Das Angebot sagt über die Uhrzeit nichts aus.
- 97,5 bis 100 Prozent aller Zeilen entsprechen dem Wert ihrer Kombination aus
  Klasse und Wochentag. Über alle Schulen weichen 35 Kinder ab.
- Bei OGS am Berg decken 12 Klassen mal 5 Wochentage alle 925 Zeilen ohne eine
  einzige Abweichung; die Werte 11:45 / 12:45 / 13:30 sind Unterrichtsschluss.
- Nur 3 von 10 Tenants haben überhaupt genehmigte Angebotsbuchungen.

Die Uhrzeit gehört also zur Klasse, die Betreuungstage zur Buchung. Für
Gehzeiten ist die Lesezeit-Projektion in ADR 0001 bereits entschieden; die
dort verworfene materialisierte Kopie liegt bei am Berg noch als 408 ignorierte
`care_offering`-Zeilen im Bestand. Ein zweiter solcher Pfad entsteht hier nicht.

## Entscheidung

- Die Uhrzeit kommt aus `education.class_arrival_times`: eine Zeile je Tenant
  und Klasse mit einer jsonb-Abbildung Wochentag auf `HH:MM`, normalisiert wie
  `care_offerings.pickup_times`. Sie kommt nicht aus dem Angebot.
- Eine Zeile in `schedule.student_arrival_schedules` bedeutet ab jetzt nur noch
  „dieses Kind ist an diesem Wochentag in Betreuung". Ihre Uhrzeit ist optional.
  Eine Ankunftszeit ohne Betreuungstag darf es nie geben: genau diese Kombination
  hat am 19.08. zwei abgemeldete Kinder weiter als erwartet geführt.
- Welche Wochentage Betreuungstage sind, entscheidet der Tenant-Modus. Das neue
  operator-only Setting `enrollment.bookings_authoritative` schaltet ihn:
  - eingeschaltet: die Wochentage der wirksamen, genehmigten
    `request_child_offerings` mit halb offenem Fenster `[valid_from, valid_until)`,
    wie bei den Angebots-Gehzeiten. Eine gespeicherte Zeile auf einem nicht
    gebuchten Wochentag plant nichts mehr, wird aber nicht gelöscht;
  - ausgeschaltet: die gespeicherten Zeilen, unverändert gegenüber vorher.
- `enrollment.bookings_authoritative` ist unabhängig von `enrollment.enabled`.
  `enrollment.enabled` öffnet oder schließt nur das öffentliche Elternformular.
  Nach dem Anmeldezeitraum darf es ausgeschaltet werden, ohne dass genehmigte
  Buchungen ihre Wirkung als Betreuungstage verlieren.
- Die manuelle Anlage folgt derselben Grenze:
  - bei ausgeschaltetem Buchungsmodus bleibt `Datenverwaltung` -> `Kinder` ->
    `Neues Kind` der direkte Weg für ein OGS-Kind; Betreuungstage werden dort im
    Wochenplan gepflegt;
  - bei eingeschaltetem Buchungsmodus wird ein OGS-Kind über `Anmeldephasen` ->
    `Manuelle Anmeldung` angelegt und sofort freigegeben. So entstehen Kind und
    genehmigte Angebotsbuchungen in einem Vorgang. Der öffentliche Elternlink
    muss dafür nicht geöffnet sein;
  - `Nur Klassenliste` bleibt in beiden Modi verfügbar, weil dieser Eintrag
    ausdrücklich keine Betreuung plant.
- Die Klassenzeile liefert **nur die Uhrzeit, nie einen Betreuungstag**. Auch die
  Sammelaktion „Unterrichtsschluss für eine Klasse setzen" legt keine Zeilen an.
  Sonst bekämen an den Schulen ohne Halbjahresanmeldung alle Kinder die
  Wochentage ihrer Klasse: bei einem Tenant hätten 125 von 142 Kindern
  Betreuungstage erhalten, die sie nicht haben.
- Ein zentraler Schedule-Dienst projiziert für das angefragte Datum oder
  Datumsfenster. Alle Leser gehen über `GetBulkEffectiveArrivalTimesForDate`
  und sehen dieselbe Grenze. Es wird nichts materialisiert.
- Eine gespeicherte Zeile in `schedule.student_arrival_schedules` ist per
  Definition eine manuelle Überschreibung und hat Vorrang vor der Projektion.
  Die Tabelle bekommt bewusst keine `source`-Spalte: abgeleitete Zeilen werden
  nie gespeichert, anders als bei den Gehzeiten, wo die Spalte nur die
  materialisierten Altzeilen unterscheidbar halten muss. Die Herkunft reist als
  nicht persistiertes Feld mit der projizierten Zeile, wie `CareOfferingName`.
- Ankunfts- und Abholzeiten sowie Tagesausnahmen beschreiben einen vorhandenen
  Betreuungstag. Im Buchungsmodus dürfen sie keinen nicht gebuchten Tag
  hinzufügen. Ankunfts- und Abholprojektion verwenden dafür dieselbe
  Buchungsprojektion als erste Grenze und werten erst danach Zeiten und
  Ausnahmen aus. Ausgeblendete Altzeilen bleiben gespeichert, damit ein
  späterer Rückwechsel zum Wochenplan-Modus keine Daten verliert.
- Ein Wert, der der Projektion entspricht, wird nicht als Zeile gespeichert.
  Beim vollständigen Speichern eines Wochenplans entfällt ein bestehender
  deckungsgleicher Override.
- Die Herkunft ist im Produkt sichtbar: abgeleitete Zeit nennt die Klasse,
  manuelle Zeit weist sich als Überschreibung aus und lässt sich zurücksetzen.
- Backfill: je Tenant, Klasse und Wochentag wird der häufigste Wert zur
  Klassenzeile. Kind-Zeilen, die diesem Wert entsprechen, entfallen, weil die
  Projektion sie exakt reproduziert. Abweichende Zeilen bleiben als Override
  stehen. Zeiten `00:00` werden nicht übernommen.
- Kein zweiter Abgleichmechanismus. Eine Konsistenzprüfung bleibt Betriebsnetz,
  nicht fachliche Korrektheit.

## Folgen

- Im Buchungsmodus endet die Ankunft eines abgemeldeten Kindes am Stichtag,
  ohne Job und ohne Kaskade.
- Ohne Buchungsmodus bestimmen weiter die gespeicherten Zeilen die Tage. Ein
  abgemeldetes Kind verschwindet dort erst, wenn seine Zeilen entfernt werden
  oder sein Kind-Status wechselt. Das ist bewusst so: die sechs Schulen ohne
  Halbjahresanmeldung sollen sich durch diese Änderung nicht ändern.
- Im Buchungsmodus hat ein über die manuelle Anmeldung freigegebenes Kind sofort
  Ankunftszeiten, sobald seine Klasse gesetzt ist: die Buchung liefert die Tage,
  die Klasse die Zeit.
  Ohne Buchungsmodus bleibt das Eintragen der Betreuungstage ein Schritt pro
  Kind; er besteht dann aber nur noch aus Haken setzen statt Zeiten tippen.
- Die Pflegemenge sinkt deutlich: 925 auf 12 Zeilen (am Berg), 501 auf 17
  (Barnstorf), 1410 auf 19 plus 35 Overrides (Altenberge).
- Bekannte Grenze: Die Klassenzeile trägt keinen Schuljahresbezug. Nach einem
  Jahrgangswechsel gilt sie für den neuen Jahrgang derselben Klassenbezeichnung
  weiter, und ein veralteter Wert trifft jetzt eine ganze Klasse statt eines
  Kindes. Die Pflegeansicht zeigt deshalb das Änderungsdatum. Ausbaustufe, wenn
  das nicht reicht: Gültigkeit je Schuljahr auf der Klassenzeile.

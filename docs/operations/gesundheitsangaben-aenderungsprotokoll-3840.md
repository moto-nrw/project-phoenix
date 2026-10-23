# Gesundheitsangaben im Änderungsprotokoll

Slices 3840-b und 4844-a (#4844), Teil von #3840. Dieser Ablauf bestätigt keinen produktiven Stand.

## Im Code umgesetzt

People Directory vergleicht `health_info` vor der Maskierung. Ein unveränderter
Wert erzeugt keinen Eintrag. Das Änderungsprotokoll speichert ausschließlich:

| Vorgang | Vorher | Nachher |
|---|---|---|
| Hinzufügen | leer | Angabe hinzugefügt |
| Ändern | Angabe vorhanden | Angabe geändert |
| Entfernen | Angabe vorhanden | leer |

Die Anzeige zeigt leere Werte als „leer“. Weder Inhalt noch Länge der Freitexte
werden übernommen. Andere Felder bleiben unverändert.

Migration `1.15.475` ersetzt alte Gesundheitswerte in
`audit.student_field_edits` für alle Schulen. Sie entfernt keine Zeilen und
ändert weder Schule, Kind, Zeitpunkt noch Bearbeiter. Auch NULL-Werte werden
in leere Anzeigewerte überführt. Die Bereinigung ist wiederholbar und erfolgt
atomar; ein Datenbankfehler wird als Fehler zurückgegeben.

**Begründete Ausnahme von Append-only:** Gesundheitsfreitexte werden einmalig
mit der privilegierten Migrationsverbindung entfernt, statt sie bis zum Ende
der konfigurierten Frist (Voreinstellung 90 Tage, maximal 365 Tage) vorzuhalten.
Der Änderungsnachweis bleibt ohne die sensiblen Inhalte erhalten. Die Rechte
der Schule und RLS werden nicht erweitert oder abgeschaltet. Die Maskierung
ist irreversibel; die Down-Migration verweigert das Zurückrollen. Fehler werden
vorwärts korrigiert, nicht durch Wiederherstellen der Klartextwerte.

## Produktiv nachzuweisen: Mensch-Schritte

1. Alle schreibenden App- und Worker-Instanzen während des Versionswechsels
   anhalten. Migrationen `1.15.475` und `1.15.483` mit dem bestehenden Migrationsverfahren
   ausführen und ausschließlich die Versionsnummer und den Erfolg festhalten.
2. Nur die neue Version starten. Kein Mischbetrieb mit alten Schreibern:
   Diese könnten erneut Klartext ins Protokoll schreiben.
3. Die Zählabfragen in diesem Dokument mit der privilegierten Betriebsverbindung ausführen.
   Nur die Anzahl, niemals Werte einzelner Zeilen ausgeben. Erwartet: `0`.
4. Die neue Darstellung mit ausschließlich synthetischen Angaben prüfen.
   Zeitpunkt, Version, Prüfergebnis und verantwortliche Person intern
   dokumentieren. Ohne diesen Nachweis bleibt der produktive Stand offen.

```sql
SELECT count(*) AS unerwartete_gesundheitswerte
FROM audit.student_field_edits
WHERE field_name = 'health_info'
  AND (
    old_value IS NULL OR new_value IS NULL
    OR old_value NOT IN ('', 'Angabe vorhanden')
    OR new_value NOT IN ('', 'Angabe hinzugefügt', 'Angabe geändert')
  );
```

## Wiederherstellung aus älteren Sicherungen

Sicherungen werden durch diesen Slice nicht verändert. Schutz und Ablauf ihrer
Löschung gehören zu #3836. Nach einer Wiederherstellung bleiben alle Schreiber
und regulären Zugriffe gesperrt, bis die neue Version und die Bereinigung
nachgewiesen sind. Ein bereits vorhandener Migrationseintrag allein genügt
nicht: Wurden ältere Tabelleninhalte eingespielt, muss auch bei als angewendet
markierter Migration erneut bereinigt werden.

Die folgende Anweisung ist ausschließlich für den berechtigten menschlichen
Betriebsablauf vorbereitet. Sie hat denselben Umfang wie die Migration und
wird vor Freigabe der wiederhergestellten Daten ausgeführt. Bei einem Fehler
nicht freigeben. Danach die Zählabfrage oben wiederholen, erwartet `0`.

```sql
BEGIN;
UPDATE audit.student_field_edits
SET old_value = CASE WHEN COALESCE(old_value, '') = ''
                     THEN '' ELSE 'Angabe vorhanden' END,
    new_value = CASE WHEN COALESCE(new_value, '') = '' THEN ''
                     WHEN COALESCE(old_value, '') = '' THEN 'Angabe hinzugefügt'
                     ELSE 'Angabe geändert' END
WHERE field_name = 'health_info';
COMMIT;
```

## Grenzen und offene Punkte

- Diese Slices minimieren nur `health_info` in den beiden Änderungsnachweisen. Sie entfernen
  keine Gesundheitsangaben aus dem eigentlichen Betreuungsprofil.
- `extra_info` ist allgemeiner Freitext („Weitere Informationen über das
  Kind“, im Bearbeiten „Notizen der Eltern“). Sensible Angaben sind dort
  möglich; eine regelmäßige Nutzung dafür ist aus dem Code nicht belegbar.
- `departure_companion_note` heißt „Geht außerdem mit“. Der Eingabehinweis
  nennt Begleitpersonen, nicht Gesundheit. Fehlablagen bleiben möglich.
  Beide Felder werden nach Auftrag nicht zusätzlich maskiert.
- Produktiver Rollout, ältere Sicherungen, Datenträgerverschlüsselung,
  Leserolle und die übrige Risikobewertung bleiben gesondert nachzuweisen.

## Elternportal-Änderungsanfragen (Slice 4844-a)

**Im Code umgesetzt:** Direkte Änderungen von `student/health_info` schreiben
in `users.student_data_change_requests` dieselben Marker wie das
Änderungsprotokoll pro Kind (Tabelle oben). Der Vergleich erfolgt vor der
Maskierung. Unveränderte Werte erzeugen keinen Eintrag; das aktuelle
Betreuungsprofil behält den tatsächlich eingegebenen Wert.

Die Freigabe-Allowlist in
`backend/workflows/parentportal/internal/care/parent_master_data_request_service.go`
lässt `health_info` nicht zu. Der bestehende Test
`TestSubmitMasterDataChangeRequest_InvalidInputs` prüft diese Ablehnung.
Es gibt daher keinen unterstützten Freigabepfad, der Gesundheitsfreitext
zwischenspeichern muss. Eine neue Frist oder eine Maskierung erst nach der
Entscheidung ist nicht erforderlich.

Migration `1.15.483` bereinigt ausschließlich `target = 'student'` mit
`field_key = 'health_info'`, schulübergreifend und unabhängig vom Status.
Auch unerwartete historische offene Zeilen werden maskiert, da sie keinem
unterstützten Freigabepfad entsprechen. SQL-NULL, JSON-null und leere Strings
werden zu leeren Anzeigewerten. Alle anderen Spalten bleiben erhalten,
insbesondere Schule, Kind, Urheber, Status, Zeitpunkte und Entscheidungsvermerk.
Andere Felder und Ziele bleiben unverändert. Ein Fehler bricht die einzelne
UPDATE-Anweisung atomar ab; Down verweigert die Wiederherstellung.
Schulrechte und RLS bleiben unverändert.

Die verbliebenen Anfragen haben keine eigene Protokollfrist. Sie werden mit
dem Kind oder dem einreichenden Konto gelöscht (Fremdschlüssel mit
`ON DELETE CASCADE`). Die 90-Tage-Voreinstellung des Änderungsprotokolls pro
Kind gilt nicht für diese Tabelle. Freitext bleibt in keinem der beiden
Nachweise stehen.

**Produktiv nachzuweisen:** Die Mensch-Schritte oben gelten für beide Wege.
Zusätzlich zur ersten Zählabfrage muss auch diese Abfrage `0` liefern.
Nur die Anzahl festhalten, keine Zeileninhalte ausgeben:

```sql
SELECT count(*) AS unerwartete_gesundheitswerte_elternanfragen
FROM users.student_data_change_requests
WHERE target = 'student' AND field_key = 'health_info'
  AND (
    old_value IS NULL
    OR old_value NOT IN ('""'::jsonb, '"Angabe vorhanden"'::jsonb)
    OR new_value NOT IN ('""'::jsonb, '"Angabe hinzugefügt"'::jsonb,
                         '"Angabe geändert"'::jsonb)
  );
```

Nach Wiederherstellung älterer Sicherungen zusätzlich zur Bereinigung des
Änderungsprotokolls oben diese Anweisung ausführen, bevor Zugriffe oder
Schreiber freigegeben werden. Auch hier genügt ein alter Migrationseintrag
nicht als Nachweis. Danach beide Zählabfragen prüfen, erwartet jeweils `0`.

```sql
BEGIN;
UPDATE users.student_data_change_requests
SET old_value = CASE
      WHEN COALESCE(old_value, 'null'::jsonb) IN ('null'::jsonb, '""'::jsonb)
        THEN '""'::jsonb ELSE '"Angabe vorhanden"'::jsonb END,
    new_value = CASE
      WHEN new_value IN ('null'::jsonb, '""'::jsonb) THEN '""'::jsonb
      WHEN COALESCE(old_value, 'null'::jsonb) IN ('null'::jsonb, '""'::jsonb)
        THEN '"Angabe hinzugefügt"'::jsonb
      ELSE '"Angabe geändert"'::jsonb END
WHERE target = 'student' AND field_key = 'health_info';
COMMIT;
```

### Vorschlag AVV/TOM, erst nach produktivem Nachweis übernehmen

„Gesundheitsangaben stehen weder im Änderungsprotokoll pro Kind noch in den
Änderungsnachweisen aus dem Elternportal im Klartext. Dort wird nur
festgehalten, dass eine Angabe hinzugefügt, geändert oder entfernt wurde.
Bestehende Freitexte wurden in beiden Nachweisen entfernt. Vor Freigabe einer
Wiederherstellung älterer Sicherungen wird diese Bereinigung erneut geprüft
und bei Bedarf ausgeführt.“

Der Vorschlag bestätigt keinen produktiven Stand. Die übrige Risikobewertung
und Sicherungsschutz nach #3836 bleiben gesondert offen.

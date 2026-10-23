# Gesundheitsangaben im Änderungsprotokoll

Slice 3840-b, Teil von #3840. Dieser Ablauf bestätigt keinen produktiven Stand.

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
   anhalten. Migration `1.15.475` mit dem bestehenden Migrationsverfahren
   ausführen und ausschließlich die Versionsnummer und den Erfolg festhalten.
2. Nur die neue Version starten. Kein Mischbetrieb mit alten Schreibern:
   Diese könnten erneut Klartext ins Protokoll schreiben.
3. Die folgende Abfrage mit der privilegierten Betriebsverbindung ausführen.
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

- Dieser Slice minimiert nur `health_info` im Änderungsprotokoll. Er entfernt
  keine Gesundheitsangaben aus dem eigentlichen Betreuungsprofil.
- `extra_info` ist allgemeiner Freitext („Weitere Informationen über das
  Kind“, im Bearbeiten „Notizen der Eltern“). Sensible Angaben sind dort
  möglich; eine regelmäßige Nutzung dafür ist aus dem Code nicht belegbar.
- `departure_companion_note` heißt „Geht außerdem mit“. Der Eingabehinweis
  nennt Begleitpersonen, nicht Gesundheit. Fehlablagen bleiben möglich.
  Beide Felder werden nach Auftrag nicht zusätzlich maskiert.
- Produktiver Rollout, ältere Sicherungen, Datenträgerverschlüsselung,
  Leserolle und die übrige Risikobewertung bleiben gesondert nachzuweisen.

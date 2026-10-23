# Datenträger- und Schlüsselschutz nachweisen

Vorbereiteter Mensch-Ablauf für Slice 3840-c, Grundlage aus 3840-a für #3840.
**Offen: Dieser Ablauf wurde nicht am produktiven System ausgeführt.** Er
bestätigt weder Verschlüsselung noch Zugriffsrechte beim Hoster. Es wird kein
Skript installiert und keine Hostkonfiguration verändert.

Ziel und Grenzen stehen in der [Risikobewertung](../reference/schutz-gespeicherter-daten.md)
und [ADR 0093](../adr/0093-gespeicherte-daten-risikobasiert-ohne-feldverschluesselung-schuetzen.md).
Die Backend-Dateirechte 0600/0700 ersetzen diesen Nachweis nicht.

## Durchführung durch den berechtigten Betrieb

1. **Umfang zuordnen.** Tatsächliches Hostingprodukt und Datenorte ermitteln:
   PostgreSQL-Datenverzeichnis, WAL, Uploads, temporäre DB-/Upload-/Dumpdateien,
   Swap, Host-/Volume-Snapshots und weitere Kopien. Container-Volume-Namen
   genügen nicht: die dahinterliegenden Datenträger und Dienste zuordnen.
   Produktions- und andere Umgebungen unterscheiden. Ergebnis nur intern
   geschützt ablegen, keine Inhalte oder sensitiven Infrastrukturdetails ins PR.
2. **Hosterbeleg einholen.** Aktuelle produktbezogene Dokumentation oder eine
   schriftliche Bestätigung für jeden Datenort beschaffen: Verschlüsselungsart
   und -umfang, Umgang mit ausgetauschten/ausgemusterten Medien, Snapshots,
   Replikaten und verbleibenden Ausnahmen. Eine allgemeine Rechenzentrums-TOM
   oder TLS-Aussage reicht nicht als Beleg für das konkret genutzte Volume.
   Anbieterbeleg mit tatsächlichem Produkt und lokaler Konfiguration abgleichen;
   unbelegte Orte bleiben offen.
3. **Schlüssel getrennt bewerten.** Für Datenträger, Sicherungen und
   Betriebsgeheimnisse jeweils Verwahrung, berechtigte Funktionen,
   Wiederbeschaffung, Rotation/Entzug und Trennung von gespeicherten Daten
   bestätigen. Hosterverwaltete und selbstverwaltete Schlüssel unterscheiden;
   Entsperrung beim Booten und Zugriff auf laufende Systeme beschreiben.
   Kein Schlüsselmaterial, keine Wiederherstellungscodes und keine Secrets
   in Nachweise, Screenshots, Tickets oder Repository kopieren. Für Backups
   gilt unabhängig davon [ADR 0085](../adr/0085-backup-decryption-keys-stay-off-the-source-host.md).
4. **Laufende Zugänge und Kopien abgleichen.** Aufgabenbezogene Root-/Admin-,
   Deployment- und Diagnosezugänge prüfen; Umfang und Entziehbarkeit intern
   festhalten. Die Diagnose-Leserolle 3840-f muss sensible Spalten und
   Umgehungen über Views, Funktionen oder Rollenwechsel ausschließen; keine
   Abnahme allein anhand eines Rollennamens. Bestehende Dumps, cron-Sicherungen,
   Release-Snapshots und temporäre Kopien inventarisieren. Getrennte
   Verschlüsselung, Zugriff und Ablauf nach dem [Backup-Runbook](daily-backup.md)
   belegen. Volume-Verschlüsselung schützt keine bereits exportierte Klartextkopie.
5. **Befund und Abnahme festhalten.** Für jeden Datenort/Schlüsseltyp einen
   Status erfassen: nachgewiesen, nicht erfüllt oder ungeklärt. Lücken erhalten
   verantwortliche Betriebsfunktion, konkrete Abhilfe und Termin; notwendige
   Sofortmaßnahmen beziehungsweise Begrenzung betroffener Verarbeitung nicht
   bis zur Abnahme verschieben. Nur tatsächlich geprüften Umfang freigeben.
   Ohne passende Belege bleibt der jeweilige Punkt in #3840 offen.

Dieser Ablauf enthält bewusst keine automatisierten Serverzugriffe.
Einrichtung oder Änderung von Verschlüsselung und Schlüsseln benötigt ein
eigenes geprüftes Betriebsverfahren mit Ausfall-/Restoreplanung; sie wird
hier nicht durch eine Checkliste als erledigt behandelt.

## Mindestinhalt des geschützten Nachweises

| Angabe | Inhalt ohne Daten-/Schlüsselkopien |
|---|---|
| Prüfbezug | Datum, verantwortliche Betriebsfunktion, Code-/Deploymentversion und interne Prüfauftragsreferenz |
| Datenort | Interne Referenz, zugeordnete Datenarten, Hostprodukt/Volume/Dienst, eingeschlossene und ausgeschlossene Kopien |
| Verschlüsselung | Verfahren, Geltungsbereich, relevante Konfiguration, Anbieterbeleg samt Version/Datum |
| Schlüssel und Zugänge | Verwahrungsmodell, berechtigte Funktionen, Trennung, Rotation/Entzug und Wiederbeschaffungsprüfung, keine Geheimnisse |
| Ergebnis | Status je Punkt, Abweichung, Abhilfe, zuständige Funktion, Termin, spätere Nachprüfung/Abnahme |

Im Repository oder PR nur die inhaltsfreie Nachweisreferenz, Datum, Umfang und
Status nennen. Personenbezogene Daten, Namen von Berechtigten, Zugangsdaten,
Screenshots mit Kundendaten und Schlüssel bleiben außerhalb dieses Repos.

## Abschlussgrenzen

- Ein belegter Datenträgerschutz schließt nur den geprüften Teil von 3840-c.
  Backupbetrieb/Restore (#3836), Leserolle (3840-f) und Maskierungsrollout
  haben getrennte Nachweise.
- Der Befund `users.student_data_change_requests` aus der Risikobewertung
  bleibt bis zur gezielten Abhilfe einschließlich Altbestand und Restore offen.
- Nach Hosting-/Volume-/Schlüsseländerung, neuen Kopierwegen oder einem
  Vorfall den betroffenen Nachweis und die Risikobewertung erneuern.

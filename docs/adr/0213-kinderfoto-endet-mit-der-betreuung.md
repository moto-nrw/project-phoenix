---
status: proposed
---

# Kinderfoto endet mit der Betreuung

Entscheidung zu #6650 (Option A + C, Entscheidung vom 11.11.2026 im Issue).
Grundlage: [Recherche im Issue](https://github.com/moto-ogs/project-phoenix/issues/6650#issuecomment-6791647906)
und [Recherche Aufbewahrungsfristen](../research/aufbewahrungsfristen-2026-10-12.md).

## Zweck, Fristbeginn und Frist

Das Foto eines Kindes in moto dient dem Erkennen des Kindes in seiner
Betreuung. Der Zweck endet mit Ablauf des letzten Betreuungstags
(`enrolled_until`) oder mit dem Abgang (Status `alumnus`); ein Abgänger ist
aus jeder Liste und jedem Fotoabruf schon ausgeschlossen. Danach gilt keine
weitere Frist: Der erste nächtliche Lauf nach dem letzten Betreuungstag
entfernt Foto, Foto-Einwilligung und wer sie erfasst hat
(Art. 17 Abs. 1 lit. a, Art. 5 Abs. 1 lit. e DSGVO). Ein geplanter, noch
kommender letzter Betreuungstag hält das Foto; ein rückwirkend gesetzter
löscht es im nächsten Lauf.

## Geprüfte Aufbewahrungspflichten

- **Träger:** Für OGS-Daten eines freien Trägers gibt es keine gesetzliche
  Aufbewahrungsfrist ([Recherche](../research/aufbewahrungsfristen-2026-10-12.md),
  Ergebnis 1 und Abschnitt 5).
- **Schule als Verantwortliche:** VO-DV I § 9 Abs. 1 nennt für Schuldaten
  Aufbewahrungsfristen, für „alle übrigen Daten“ fünf Jahre. Das Foto ist
  nach Anlage 1 Nr. 1.13 mit Fußnote „freiwillig und jederzeit widerrufbar“.
  Das Foto in moto ist kein Teil der Schülerakte, sondern ein Hilfsmittel der
  Betreuung. Eine Pflicht, es nach der Betreuung weiter zu speichern, folgt
  daraus nicht. Die schon bestehende Löschung ganzer Kinder (höchstens drei
  Jahre nach dem letzten Betreuungstag, ADR 0048) würde eine Fünfjahresfrist
  ohnehin nicht tragen; diese allgemeine Frage bleibt die offene Frage 1 der
  Recherche.
- **KUG §§ 22, 23:** regeln das Verbreiten, keine Speicherpflicht.

Belegt der Verantwortliche einen fortdauernden Zweck oder eine Pflicht,
hält das moto-Team den nächtlichen Lauf der Schule mit der Notbremse
`gdpr.data_cleanup_enabled` an (ADR 0048), bis eine eigene Regel entschieden
ist. Die Notbremse hält alle nächtlichen Löschungen der Schule an, nicht nur
die Fotos; sie ist deshalb nur eine kurze Übergangslösung mit Ticket und
Termin, keine dokumentierte Ausnahme. Eine schulweise Nachfrist gibt es
bewusst nicht.

## Kopien außerhalb der Zeile

Der Lauf entfernt das Foto in moto: Zeile, Datei (dauerhafter
Aufräumauftrag) und damit den Abruf. Bereits heruntergeladene oder
exportierte Kopien und Browser-Caches erreicht er nicht; sie bearbeitet die
Schule wie beim Widerruf (Runbook #3844, Abschnitt 2.1 Schritt 4). Sicherungen
laufen nach ihrer eigenen Frist aus; nach einer Wiederherstellung entfernt der
nächste Lauf die Fotos beendeter Betreuungen erneut, weil er seine Kandidaten
aus der Datenbank liest (Runbook #3844, Abschnitt 4).

## Umsetzung

People Directory besitzt Foto und Einwilligung und stellt
`StudentPhotoCareEndRetention` bereit; die Root hängt den Schritt nach der
Löschung ganzer Kinder in den nächtlichen Lauf, bindet das Löschprotokoll
und schreibt aus dem Ergebnis eine Logzeile je Kind.

- Kandidaten kommen in jedem Lauf aus der Datenbank. Ein ausgefallener oder
  verschobener Lauf holt alles nach.
- Ein Savepoint für die gesamte Menge: Foto, Löschprotokoll und Aufräumauftrag
  werden mengenbasiert geschrieben oder gemeinsam zurückgerollt. Die Zahl der
  Datenbankabfragen wächst nicht mit der Kinderzahl. Die Zeilensperre wartet
  nicht (`FOR UPDATE SKIP LOCKED`); ein gesperrtes Kind wird unverändert
  verschoben. Unter der Sperre prüft der Lauf die Regel erneut, damit eine
  Wiederaufnahme zwischen Auswahl und Sperre das Foto hält.
- Die Datei geht erst nach dem Commit; der dauerhafte Aufräumauftrag
  committet mit der geleerten Zeile.
- Eine Logzeile je Kandidat und Lauf (beendet, verschoben, behalten,
  entfallen). Das Löschprotokoll erhält je Kind einen
  Eintrag `student_photo_retention` mit Grund `care_ended`, geschrieben vom
  System.
- Das Einwilligungsprotokoll erhält **keinen** Widerruf: Niemand hat
  widerrufen. Seine Einträge bleiben als Nachweis bis zur Löschung des Kindes.
- Bei einer Wiederaufnahme kehren Foto und Einwilligung nicht zurück.

## Widerruf über die Schule

Eltern ohne Elternportal oder ohne Recht zur Foto-Einwilligung widerrufen
bei der Schule. Das Team entfernt in der Kinderakte den Haken
„Einwilligung der Eltern liegt vor“; das bleibt auch bei ausgeschalteten
Kinderfotos möglich, ohne Uploads freizugeben. Der vorhandene Schreibweg
entfernt Foto und Einwilligung atomar und schreibt den Widerruf mit Quelle
`tenant_portal` und handelnder Person ins Einwilligungsprotokoll. Eltern im
Portal ohne eigenes Recht lesen dort, dass sie bei der OGS widerrufen.
Einen neuen Schreibweg für Eltern gibt es nicht.

## Command Effects

Diese ADR bestätigt genau den neuen Eintrag
`Service.EndStudentPhotosAfterCare` →
`unresolved:callable:…/peopledirectory/internal/application.observe`: die
vorhandene Beobachtung des Foto-Lebenszyklus, wie in ADR 0098 statisch
unaufgelöst. Die übrigen drei neuen Einträge sind Plattformteilnahme
(ADR 0098). Der Schritt ist eine Methode des vorhandenen Owner-Service; die
Root erreicht ihn über den Owner der vorhandenen Directory-Komposition. Er
erzeugt keinen neuen Wertfluss in bestehende Bindungen, und die Datei geht
über den unveränderten, registrierten Nach-Commit-Weg `ScheduleUnlink`.
`scheduleUnlinks` beauftragt die Dateien vorher mengenbasiert mit
`QueueUnlinkStoredPhotos`. Der Nachtlauf sendet weiterhin keine Live-Meldung;
offene Ansichten aktualisieren sich beim nächsten Abruf.


Die mengenbasierte Erweiterung bindet `QueueUnlinkStoredPhotos` an File
Storage `UploadCleanup.QueueAll` als teilnehmenden, geschützten Command
(ADR 0067). File Storage prüft und dedupliziert die verwalteten Dateinamen,
behält Tenant und Objekttyp bei und schreibt die Aufträge in einer Anweisung
in derselben Transaktion. Der Command liest keinen fremden Zustand, nimmt
keine abgestimmten Sperren und führt keine Nach-Commit-Arbeit aus. Die
Einzeldatei-Regeln bleiben unverändert. Die drei bisherigen Queue-Bindungen
erhalten durch den erweiterten Konstruktor einen neuen Nachweis; diese ADR
bestätigt genau diese unveränderte Einzeldatei-Semantik. Die neuen Store-Port-
Einträge lesen nur die aktuelle Tenant-Transaktion aus dem Kontext; sie
öffnen keine eigene Transaktion und wechseln keinen Scope.

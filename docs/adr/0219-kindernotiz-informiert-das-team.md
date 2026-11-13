---
status: proposed
---

# Kindernotiz informiert das Team

Entscheidung zu
[#6854](https://github.com/moto-ogs/project-phoenix/issues/6854). Die
Vorgaben hat Yannick Wenger am 12.11.2026 festgelegt; die Freigabe erteilt er
mit dem Merge des Pull Requests, der diese ADR einführt. Sie führt keinen
neuen Owner ein.

## Kontext

Eine wichtige Notiz zu einem Kind fällt dem Team nur auf, wenn jemand die
Notizen des Kindes öffnet. Die Schule möchte eine Notiz beim Speichern mit
dem Team teilen können, ohne dass jede gewöhnliche Notiz alle benachrichtigt.

## Entscheidung

- **Zwei Aktionen.** „Speichern“ bleibt unverändert und benachrichtigt
  niemanden. „Speichern und Team informieren“ speichert die Notiz und teilt
  sie danach mit dem Team. Teilen ist ein eigener Schreibweg
  (`POST /api/students/{id}/notes/{noteId}/team-notices`), damit das
  Ergebnis des Speicherns und das des Teilens getrennt und ehrlich gemeldet
  werden: Scheitert das Teilen, ist die Notiz gespeichert und das Team nicht
  informiert, und die Oberfläche sagt genau das.
- **Nur die Autorin, nur für das ganze Team.** Teilen darf nur, wer die Notiz
  geschrieben hat, und nur eine Notiz mit der Sichtbarkeit „Ganzes Team“
  (`all_staff`). Für die engeren Sichtbarkeiten (Betreuungsteam,
  Gruppenleitung) gibt es keine eigene Empfängerliste; die Oberfläche sagt
  das am Knopf. Eine Erweiterung auf diese Sichtbarkeiten braucht eine
  eigene Auflösung der Empfänger je Gruppe, Klasse und Angebot.
- **Empfänger.** Alle aktiven Mitarbeitenden der Schule, deren Konto die
  Notizen lesen darf (`users:read`, das Recht der Notiz-Routen), ohne die
  Autorin. Nicht nur, wer gerade im Dienst ist. Lehrkräfte in moto schule
  halten kein `users:read` und werden nicht erreicht; das Teilen mit der
  Schule (#4797) bleibt davon getrennt. Die Schule ist die Mandantengrenze.
- **Ungelesen-Markierung in moto.** Je Notiz und Empfänger eine Zeile in
  `users.student_note_team_notices` (People Directory). Sie trägt nur IDs und
  zwei Zeitpunkte; der Inhalt bleibt an der Notiz. Gelesen wird beim Öffnen
  der Notizen des Kindes. Erneutes Teilen setzt die Zeile wieder auf
  ungelesen. Eine gelöschte Notiz, eine auf eine engere Sichtbarkeit
  geänderte Notiz und ein abgegangenes Kind erscheinen nicht mehr; die Zeile
  geht mit ihrer Notiz und mit der Mitgliedschaft der Person an der Schule
  (`auth.account_tenants`), also auch mit der Demo-Löschung.
- **Benachrichtigung auf dem Gerät.** Über den vorhandenen Weg: Delivery-Typ
  `staff_student_note_shared` (Gruppe „Kinder“, für Inhaber von
  `users:read`, voreingestellt an, in den Einstellungen abschaltbar). Der
  Text nennt kein Kind. Markierungen und Push-Eintrag entstehen in einem
  Savepoint der Transaktion des Aufrufers; die Zustellung folgt den Regeln
  der Push-Outbox.
  Ist die Benachrichtigung der Schule aus oder liegt die Zeit außerhalb ihres
  Benachrichtigungsfensters, entsteht kein Push, die Markierung in moto
  reicht. Scheitert das Speichern der Markierungen oder des Push-Eintrags,
  rollt der Savepoint beides zurück, auch wenn der Aufrufer committet.

## Grenzen

People Directory besitzt Notiz und Markierung und entscheidet, wer teilen
darf. Wer das Team ist und wie es auf dem Gerät erfährt, bindet die Root am
Mount der Schülerrouten über die Mitarbeiterverzeichnis-Projektion und den
Delivery-Router: beide entstehen im Owner-Graphen nach People Directory. Der
Befehl ruft den Push als Rückruf im selben Savepoint auf (ADR 0067,
Beteiligung an einem fremden Befehl mit Outbox, kein Workflow). Er öffnet
keine eigene Transaktion, sondern verlangt die der Schülerroute.
Der Root-Baustein `owners.StaffPushNotifier` ist allgemein gehalten, damit
weitere Hinweise an das Personal (etwa #6855) denselben Weg nutzen.

Die neue Capability `StudentNoteTeamNotices` steht neben der Fassade des
Owners und verbreitert `Capability`, `Command` und `Owner` nicht.

## Command Effects

Diese ADR bestätigt genau die drei neuen Einträge
`StudentNoteTeamService.InformStudentNoteTeam`,
`StudentNoteTeamService.ListUnreadStudentNoteTeamNotices` und
`StudentNoteTeamService.MarkStudentNoteTeamNoticesRead` →
`unresolved:callable:…/peopledirectory/internal/application.observe`: die
vorhandene Beobachtung des Owners, wie in ADR 0098 statisch unaufgelöst. Die
übrigen neuen Einträge sind Plattformteilnahme (Transaktion, dauerhafter
Push-Eintrag; ADR 0098) oder reine Lesezugriffe (Mitarbeitende mit
`users:read`, Abmeldungen vom Benachrichtigungstyp).

---
status: accepted
---

# Wiederanmeldung bündelt Kandidaten und behält die Beobachtung

PR #6667 nimmt auch bei Personal-Anmeldungen die Kindersperren vor den
Phasensperren. People Directory löst die übermittelten Namen und Geburtstage
in einer Abfrage auf; Enrollment bildet daraus die sortierte, eindeutige
Kandidatenliste. Mehrdeutige Treffer pinnen weiterhin kein Kind. Der spätere
Abgleich unter den Sperren bleibt erhalten, damit Änderungen zwischen
Kandidatensuche und Sperre erkannt werden.

Die Root übersetzt die beiden Owner-eigenen Identitätswerte und bindet die
vorhandene mengenbasierte Kindersperre an Enrollments `LockStudents`.
Es entstehen keine neuen Policy-Regeln, Schreibrechte oder Transaktionen.

Die neue Abfrage verlangt die vorhandene Schultransaktion und nutzt die
bestehende Beobachtung von `StudentService.run`; sie öffnet keine eigene
Transaktion. Der Adapter verweigert den Abruf ohne Transaktion.
Wie in ADR 0098 bleibt der nur aus Tests frei übergebene Callable
`modules/peopledirectory/internal/application.observe` statisch unaufgelöst.
Wir behalten die Beobachtung und erfassen diese Grenze ausdrücklich als
`unresolved`, statt sie als reine Abfrage oder geprüften Leaf auszuweisen.
Diese ADR bestätigt genau die drei neuen Command-Effects-Einträge für
`StudentRepository.FindEnrolledStudentIDsByIdentities` mit den Bindungen
`StudentRecords`, `OverlappingRosterGroupNames` und `groupStudentRepository`
zu diesem Observer. Sie bestätigt keine weiteren unaufgelösten Bindungen.

Die zwei neuen Pass-through-Belege betreffen ausschließlich die Weitergabe
und Wertübersetzung der Kandidatenabfrage. Der
Parallelitätstest reproduziert ohne die Personal-Vorabsperre `40P01`;
der Abfragetest prüft eine Anweisung für mehrere Identitäten, Mehrdeutigkeit,
Unicode-Normalisierung und Schultrennung.

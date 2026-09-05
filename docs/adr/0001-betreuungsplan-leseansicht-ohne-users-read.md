# ADR 0001: Leseansicht Betreuungsplan über schedules:read, Kindernamen über schmalen Teilnehmer-Endpunkt

Datum: 2026-08-15 · Status: angenommen · Issue: #2283

## Kontext

Teammitglieder ohne Admin-Rechte (Rolle `user`) sollen den Betreuungsplan
lesend sehen (Wunsch GS am Berg, Franziska Kessener). Die `user`-Rolle hält
seit Migration 1.9.4 bereits `schedules:read`/`schedules:list`; alle
Lese-Endpunkte des Plans sind damit erreichbar. Das einzige echte Gate war die
Navigation. Der Plan lädt aber bisher bedingungslos die komplette Kinderliste
über `GET /api/students`, und dieser Endpunkt verlangt `users:read`, das die
`user`-Rolle nicht hat.

## Entscheidung

1. Sichtbarkeit der Leseansicht hängt an der vorhandenen Permission
   `schedules:read`. Kein neues Tenant-Setting, keine neue Permission.
2. Die `user`-Rolle bekommt NICHT `users:read`. Das würde die komplette
   Kinder-API (Suche, Details, Exporte) für alle Betreuerinnen aller Schulen
   öffnen und wäre ein Rollenmodell-Umbau weit über das Issue hinaus.
3. Kindernamen in der Leseansicht liefert stattdessen ein schmaler
   Lese-Endpunkt (Teilnehmer-Namen pro Termin-Instanz), gegated auf
   `schedules:read` und serverseitig durch den bestehenden
   `CanReadStudent`-Check gefiltert (respektiert `gdpr.student_data_scope`).
4. Editier-Kontrollen werden für Nutzer ohne `schedules:manage` ausgeblendet
   (Muster der Vertretungs-Seite), plus Hinweis "Leseansicht" im Seitenkopf.
   Konflikt-Warnungen mit Kindernamen bleiben Planenden vorbehalten.

## Konsequenzen

- Navigation: `planning-navigation.ts` steuert Desktop- und Mobilnavigation
  aus einer Quelle; der Betreuungsplan-Eintrag erhält
  `nonAdminPermission: "schedules:read"` und erscheint für Nicht-Admins als
  flacher Eintrag, nicht als Akkordeon.
- Der unbedingte `fetchStudents`-Aufruf im Plan entfällt für Nutzer ohne
  `users:read`; der Namens-Lookup im Termin-Detail wechselt auf den neuen
  Teilnehmer-Endpunkt.
- Übrige Planungsseiten (Dienstplan, Kalenderzeiträume: hart admin-gegated;
  Vertretung: bereits lesetauglich, aber ohne Nav-Eintrag für Nicht-Admins)
  bleiben unverändert; kein zusätzliches Routen-Gate (die Daten sichern die
  API-Permissions, nicht das Frontend).

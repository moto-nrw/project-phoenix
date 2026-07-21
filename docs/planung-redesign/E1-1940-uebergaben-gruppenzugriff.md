# E1: Übergaben vs. Vertretung — Produktentscheidung (#1940)

Stand: 2026-07-21, umgesetzt in PR #1965 (Branch `feat/1946-1940-planung-navigation`). Dieses Dokument hält die in Issue #1940 offen gelassene Frage fest, ob `/substitutions` ("Übergaben") im neuen Vertretungs-Bereich aufgeht oder eigenständig bleibt. `R4-navigation.md` Abschnitt 5 hatte die Namenskollision als ungelöst markiert; hier steht die Entscheidung samt Begründung.

## Entscheidung

**Beide Bereiche bleiben bestehen.** `/substitutions` wird nicht in `/vertretung` integriert und nicht abgeschafft. Stattdessen wird der Alt-Bereich in **Gruppenzugriff** umbenannt, damit das Wort "Vertretung" nur noch dem Planungsbereich gehört.

## Vergleich der beiden Features

| | Gruppenzugriff (`/substitutions`) | Vertretung (`/vertretung`) |
|---|---|---|
| Zweck | Vergibt temporären **Datenzugriff** auf eine Gruppe | Plant **Personalbesetzung** in Betreuungsblöcken |
| Wirkung | Autorisierung: wirkt über `GetMyGroups` in die Sichtbarkeit von Kindern und Gruppen | Planung: Deviations/Coverage im Betreuungsplan |
| Datenmodell | `education.group_substitution` | `schedule.staff_shifts` + Deviation-Pfad (`audit.deviation_events`) |
| Zielgruppe | Einrichtungen, auch ohne aktivierte Planung (Self-Service) | Einrichtungen mit aktiviertem Planungsbereich |
| Sichtbarkeit | nur bei `operations.group_mode = fixed_groups` | nur bei `timetable.enabled` |

Die Features teilen nur den historischen Namen, nicht die Semantik. Eine Zusammenlegung hätte Datenzugriff und Dienstplanung in einem UI vermischt und den Self-Service-Pfad für Schulen ohne Planung entfernt.

## Umsetzung (PR #1965)

- Umbenennung `Übergaben` → `Gruppenzugriff` in Sidebar, Mobile-Drawer, Breadcrumb, Seitentitel; Seiten-Wording spricht von Zugriff statt Vertretung (`Zugriff gewähren`, `Hat Zugriff`).
- Gating auf `operations.group_mode = fixed_groups` (#1546): bei offener Betreuung verschwindet der Eintrag aus der Navigation, Direktaufrufe zeigen einen Hinweis.
- Das Gating ist bewusst rein clientseitig (UI-Ausblendung, keine Sicherheitsgrenze): die Substitution-Endpunkte akzeptieren weiterhin Requests von Open-Care-Tenants. Vertretbar, weil die Seite admin-only ist und Zugriffe bei fixed_groups sichtbar bleiben; wer das Backend härten will, macht das als eigene Änderung.
- Keine Daten-, API- oder Backend-Änderungen; `education.group_substitution` und alle Verbraucher (GetMyGroups, Caregiver-Blocker, Offboarding) bleiben unverändert. Neue Datensätze tragen `reason: "Gruppenzugriff"`; Altdaten behalten `"Vertretung"` (rein kosmetisch, kein Backend-Code matcht auf den String).

## Sichtbarkeits-Matrix

| group_mode / timetable.enabled | an | aus |
|---|---|---|
| **fixed_groups** | Planung + Gruppenzugriff | nur Gruppenzugriff |
| **open_care** | nur Planung | keins von beiden |

---
status: accepted
---

# Extern ist ein Merkmal, der Zugang eine eigene Wahl

Entscheidung vom 23.10.2026 zu
[#4573](https://github.com/moto-nrw/project-phoenix/issues/4573), aufbauend auf
[#3823](https://github.com/moto-nrw/project-phoenix/issues/3823).

## Ausgangslage

moto hatte drei ähnliche Begriffe für Leute von außen:

- das „Gastprofil“ (`users.guests`), ein Merkmal nur im Code,
- die Systemrolle `guest` („Gast“) aus dem Grundgerüst vom Mai 2025,
- die Schulrolle „Extern“, die der erste Entwurf von #4573 je Schule anlegte.

Der einzige Kundenwunsch (#1329, #3823) war: externe AG-Kräfte namentlich im
Plan führen, damit Kinder ihren AGs zugeordnet werden. Ohne Login, ohne
Zeiterfassung. Später kam der Wunsch dazu, dass Externe auch selbst betreuen
oder nur zuschauen können.

## Entscheidung

Zwei Achsen, die voneinander unabhängig sind.

**1. Merkmal: Wer ist das?** Eine Person des Personals ist **extern**, wenn sie
einen aktiven Datensatz „externe Betreuungskraft“ hat
(`users.external_caregivers`, mit Organisation und optionalem Zeitraum). Nur
der Datensatz macht extern. Ein Konto, eine Rolle oder eine fehlende E-Mail
ändern daran nichts.

Externe sind nicht beim Träger angestellt. Darum gilt für alle Externen, egal
mit welchem Zugang:

- Zeiterfassung, Monatsabschluss, Mitarbeitenden-Status und Dienstplan führen
  sie nicht. Ein Aufsichtsstart stempelt keine Arbeitszeit.
- Sie übernehmen keine Vertretung und keine Gruppe und stehen nicht im
  Personalpool.
- Sie sind in Regelterminen, einzelnen und spontanen Terminen planbar, auch
  allein und als zuständige Person.

**2. Zugang: Was darf die Person?** Beim Anlegen und später über „Konto
einladen“ wählt man eine von drei Stufen:

| Zugang | API-Wert | Konto | Rolle | Kann |
|---|---|---|---|---|
| Kein Zugang | `none` | keins | keine | nichts. Platzhalter für die Planung; eine OGS-Kraft startet den Termin |
| Nur ansehen | `view` | Einladung | Systemrolle `guest` („Gast“) | Kinder, Räume und AGs ansehen, nichts ändern |
| Eigene Termine betreuen | `supervise` | Einladung | Systemrolle `ag-leitung` („AG-Leitung“) | eigene Termine starten, Kinder ein- und auschecken, beenden |

- **Kein Zugang** braucht keine E-Mail-Adresse. Die beiden anderen Stufen
  verschicken eine Einladung und brauchen eine.
- **AG-Leitung** ist eine Systemrolle wie „Lehrkraft“: Jede Schule hat sie ohne
  Einrichtung, keine Schule muss sie anlegen oder pflegen. Basisrolle `user`,
  damit die Person ein Betreuungsprofil hat und planbar ist. Einziges Recht:
  `schedules:read` (offene Restrisiken dieses Rechts: Folge-Issue aus #4607).
  Die Rolle macht niemanden extern; eine interne Kraft kann sie auch haben.
- **Gast** bleibt die Rolle fürs reine Ansehen, für Externe genauso wie für
  Grundschul-Personal. Sie verliert `users:read` und jedes Schreibrecht und
  bekommt reine Leserechte: `students:read`, `groups:read`, `rooms:read`,
  `activities:read` (Migration 1.15.479). `students:read` ist neu und öffnet
  nur die Leserouten der Kinder (Liste, Detail, aktueller Ort, Klassen, Foto,
  OGS-Gruppenansicht), die bisher allein an `users:read` hingen. Damit sieht
  ein Gast keine Elternnachrichten, kein Elternverzeichnis, keine
  Telefonnummern der Eltern, keine Notizen und keine Exporte mehr.
  Auch die JSON-Antworten der Kinder-Liste und Kinderseite geben mit
  `students:read` allein keine Wohnadresse, Gesundheitsinformationen,
  Elternnotizen, keine freien Abholhinweise oder Ankunfts- und Abholnotizen aus.
  Diese Felder brauchen `users:read`. Wohnadressen und der Vollzugriff auf
  Tagespläne brauchen weiterhin auch den bisherigen Personal- oder
  Admin-Zugriff. Das gilt auch für die Tagespläne und Notizen der
  Gruppenansicht. Die Abwesenheitsübersicht (`/students/status-days`) bleibt
  hinter `users:read`, ebenso die Abwesenheiten eines einzelnen Kindes.
  Auch Krankheits-, Entschuldigungs- und Klassenfahrtstatus samt Zeitstempeln
  in Kinderliste, Kinderseite und Gruppenansicht brauchen `users:read`,
  unabhängig vom angefragten Datum.
  Kinderfotos bleiben für Gäste sichtbar, wenn die Schule sie aktiviert hat.
  Die Beschreibung „Eingeschränkter Zugriff für nicht authentifizierte Benutzer“
  war falsch; Gast ist ein angemeldetes Konto, das nur liest.

Nicht gewählt:

- **Externe über moto schule (Lehrkraft) handeln lassen:** moto schule ist für
  Leute, die in der Schule arbeiten.
- **Eine Schulrolle „Extern“ je Schule:** Schulen passen Rollen nicht an (in
  Produktion hat am 23.10.2026 keine Schule eine eigene Rolle angelegt). Eine
  je Schule angelegte Rolle, die über ihren Namen gefunden wird, bricht beim
  Umbenennen und bringt einen Fehlerfall ohne Nutzen.
- **Gast und Extern zu einem Begriff verschmelzen:** Gast ist ein Zugang,
  Extern eine Herkunft. Eine externe Person kann Gast sein, muss es aber nicht.

## Begriff „Gastprofil“ entfällt

Die Tabelle `users.guests` heißt ab Migration 1.15.477
`users.external_caregivers`; Sequenz, Indizes, Constraints, RLS-Policy und
Trigger folgen dem Namen. Zeilen, Row-Level-Security, Rechte der
Datenbankrollen und Fremdschlüssel bleiben, weil PostgreSQL sie an die Tabelle
bindet. In Produktion hatte die Tabelle am 23.10.2026 keine Zeilen.

Im Code tragen die beiden Bedeutungen des Merkmals eigene Namen:
`HasExternalCaregiverRecord` ist der rohe Befund (aktiver Datensatz),
`IsExternalCaregiver` die Regel `schoolmembership.ExternalCaregiver`
(aktiver Datensatz und bekannte Person). Bisher stand beides im Feld `IsGuest`.

Die Architekturprüfung erkennt eine Umbenennung: Eine neue Migration mit
`ALTER TABLE schema.alt RENAME TO neu` darf `schema.neu` in `policy.json`
eintragen, wenn die Basis `schema.alt` besitzt, der Kandidat `schema.alt`
streicht und der Besitzer gleich bleibt (School Membership). Sonst bliebe eine
Umbenennung eine neue Zuordnung. Shrink-only-Register ändern sich nicht.

## Folgen

- Die bestehenden 8 Gast-Konten in Produktion (3 Schulen, laut Daten
  Grundschul-Personal, ohne Schreibvorgänge) sehen nach der Migration keine
  Elterndaten und Nachrichten mehr.
- Der Kontostand einer externen Person bleibt `account`,
  `invitation_pending` oder `none`. `none` ist ein gewollter Zustand, kein
  Mangel: Platzhalter werden für neue Zuordnungen angeboten.
- Wer über die allgemeine Einladung „Gast“ oder „AG-Leitung“ bekommt, ist
  ohne den Datensatz nicht extern und zählt als normales Personal.
- Offen, nicht Teil dieser Entscheidung: Anlegen, Ändern und Löschen von
  Aktivitäten (`/api/activities`) prüft kein Recht, nur einen
  Personaldatensatz. Den haben auch Gast und AG-Leitung. Das galt schon vorher
  für jede Rolle und braucht ein eigenes Issue.

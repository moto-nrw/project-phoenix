---
status: accepted
---

# Träger-Werte liegen zwischen Standard und Schule

Entscheidung vom 23.10.2026 zu [#4646](https://github.com/moto-nrw/project-phoenix/issues/4646),
Etappe 0 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 8, 9 und 14. Der Product Owner hat sie am 23.10.2026 angenommen.
Sie baut auf
[ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md)
auf und legt die Träger-Transaktion fest, die ADR 0084 für Tabellen des
Trägers offen lässt.

Ein Wert einer Einstellung gilt so: Standard der Registry, dann der Träger-Wert,
dann die Schul-Abweichung. Die Auflösung läuft live bei jedem Lesen, ohne Kopie
in die Schulen. Ob eine Einstellung auf Träger-Ebene existiert, steht nur in
ihrer Definition. Träger-Werte liegen in einer eigenen Tabelle mit RLS nach
Träger, geschrieben in einer Träger-Transaktion. Umgebungsvariablen sind auf
keiner Ebene ein Fallback.

## Kontext

Das Settings-System kennt zwei Ebenen: Standard der Registry und Zeile in
`config.setting_values` je `tenant_id`, mit RLS über `app.current_tenant_id`.
Eine neue Schule bekommt keine Zeilen und läuft auf den Standards
([ADR 0043](0043-einrichtungs-assistent-zustand-bei-settings-fortschritt-als-projektion.md)).
Änderungen landen in `config.setting_audit` je Schule; scheitert der Eintrag,
wird das nur geloggt.

Der Träger hat nur `platform.organizations` mit einer JSONB-Spalte `settings`,
die nichts liest (#934). Jede Schule hat genau einen Träger
(`platform.schools.organization_id NOT NULL`).

Einige Schlüssel haben Seiteneffekte in der Tenant-Transaktion oder stecken in
Sperren je Schule (MFA-Policy, Abholänderung, Slot-Cutoff,
Klassen-Sammlung, Stundenkonto). Passwort-Felder liegen als Klartext-JSONB.

## Entscheidung

### 1. Was eine Einstellung ist

- Das Settings-System hält nur skalare Werte mit Standard in der Registry.
- Tarife, Beitragsregeln, Vorlagen, Nummernkreise (Muster und Zähler) und
  Bankverbindungen samt Gläubiger-ID sind fachliche Objekte mit Träger-ID bei
  ihrem Fach-Owner (ADR 0084 §2). Dasselbe gilt für Regeln zur Platzvergabe,
  Kapazitäten und Anmeldefristen: Sie sind Felder der Anmeldephase oder des
  Ferienzeitraums.
- Logo und Briefkopf-Grafik sind Träger-Stammdaten bei Organisation & Tenancy,
  mit eigenen Feldern. Farben und Texte des Brandings sind Einstellungen.
- `platform.organizations.settings` wird mit dem ersten Bau-Ticket gelöscht.
  Logo und Login-Bild der Schule bleiben in `platform.schools.settings`
  (#1918).

### 2. Auflösung

- Reihenfolge: Standard der Registry, Träger-Wert, Schul-Abweichung. Die
  spätere Ebene gewinnt.
- Kein Fallback über Umgebungsvariablen, auf keiner Ebene.
- Die Auflösung läuft bei jedem Lesen. Eine neue Schule des Trägers erbt
  dadurch sofort alle Träger-Werte (Story 14). Es wird nichts kopiert.
- Wechselt eine Schule den Träger, gelten sofort die Werte des neuen Trägers.
  Ihre Schul-Abweichungen bleiben.
- Hatte eine Schule schon eine Abweichung, bevor der Träger einen Wert setzt,
  gewinnt weiter ihre Abweichung.
- Die Freigabe des Träger-Portals (ADR 0084 §3) regelt nur, wer schreiben darf.
  Nach einem Entzug gelten die Träger-Werte weiter. Ein Träger ohne Freigabe
  hat keine Werte, die Ebene ist für ihn unsichtbar (Story 13).

### 3. Ebenen je Definition

- Jede Definition nennt ihre Ebenen: `schule` (Standard, heutiges Verhalten),
  `traeger` (nur beim Träger, keine Abweichung) oder `beide`.
- Eine Sperre zur Laufzeit, mit der der Träger Abweichungen verbietet, gibt es
  nicht.
- Beim Start prüft die Registry: `traeger` und `beide` sind nur erlaubt, wenn
  die Definition
  1. `shared` oder `admin_only` ist,
  2. keinen Seiteneffekt-Handler hat,
  3. in keiner Sperre und keiner Prüfung über mehrere Felder steckt,
  4. nicht vom Typ `password` ist.
- Eine Ausnahme von dieser Prüfung braucht eine eigene Entscheidung.

### 4. Erste Einordnung

Alles bleibt `schule`, wenn nichts anderes festgelegt ist. Für v1 kommen auf
die Träger-Ebene nur Einstellungen, die die Etappen T0 bis T5 brauchen:

| Ebene | Einstellungen |
|---|---|
| `beide` | Rechtstexte der Anmeldung; Funktionen des Elternportals ohne Sperre; DATEV-Berater- und Mandantennummer, Lohnarten, Einheiten; Tab „Abrechnung“ |
| `traeger` (neu) | Farben und Fußzeilentext des Brandings; Vertragsweg (Story 47); Standard für Kündigungsfrist und Sonderkündigung (Story 55) |
| `schule` | alles andere, auch Zeiten, Räume, Geräte, Stundenplan, Sitzungen, Erinnerungen, Sicherheit, SFTP und alles `operator_only` oder `operator_managed` |

Die Liste je Schlüssel legt das Bau-Ticket fest.

### 5. Speicherung und Träger-Transaktion

- Träger-Werte liegen in `config.traeger_setting_values` mit
  `organization_id`, Schlüssel, Wert, Konto und Zeitstempeln. Owner ist
  `settings-platform`.
- Eine **Träger-Transaktion** setzt `app.current_org_id` aus der Träger-Sitzung.
  Tabellen des Trägers haben RLS mit `organization_id = app.current_org_id`
  zum Lesen und Schreiben.
- In der Tenant-Transaktion einer Schule darf eine zweite Policy nur die Zeilen
  des Trägers dieser Schule lesen. Schreiben aus der Schule ist ausgeschlossen.
- Diese Policies gelten nur für Tabellen, die dem Träger gehören. Die Tabellen
  der Schulen bekommen keine zweite Policy-Familie (ADR 0084, verworfene
  Option).
- Die Träger-Transaktion ist das Muster für alle Tabellen des Trägers, auch für
  Tarif, Ferienzeitraum und Vorlage. Schreiben an RLS vorbei bleibt
  ausgeschlossen.
- Der Batch-Lesepfad über mehrere Schulen liest die Träger-Werte mit.

### 6. Rechte

- **Träger-Büro:** Jede Definition nennt ihr Träger-Schreibrecht. Standard ist
  „Einstellungen“; Personal-Einstellungen verlangen „Personal“,
  Abrechnungs-Einstellungen „Abrechnung“. Lesen darf jedes Mitglied mit
  „Übersicht lesen“. Geprüft wird gegen Rechte, nie gegen Rollennamen.
- **Schule:** wie heute mit `config:update` oder `config:manage`. Bei `beide`
  sieht die Schule den wirksamen Wert und seine Herkunft („Standard“ oder „vom
  Träger, geändert am …“). Ändern erzeugt eine Schul-Abweichung, Zurücksetzen
  führt zum Träger-Wert. `traeger`-Einstellungen sieht die Schule höchstens
  lesend.
- **Über „In OGS wechseln“** schreibt das Träger-Büro eine normale
  Schul-Abweichung mit dem Vermerk „Träger-Büro“ (ADR 0084 §5).
- Das Träger-Büro sieht je Einstellung, welche Schulen abweichen. Entfernen
  kann eine Abweichung in v1 nur die Schule.
- **moto-Team:** `operator_only` und `operator_managed` gibt es nur je Schule
  ([ADR 0048](0048-loeschfristen-traegt-das-moto-team-je-schule-ein.md)). Das
  Träger-Büro schreibt sie nie, das moto-Team setzt in v1 keine Träger-Werte.
  Die Settings-Seite einer Schule im Operator-Portal zeigt den wirksamen Wert
  mit Herkunft. `admin_only` bleibt für Operatoren unsichtbar, auch vom Träger.

### 7. Audit

- `config.traeger_setting_audit` ist append-only, mit RLS wie in Abschnitt 5,
  und hält Träger, Schlüssel, alten und neuen Wert, Konto und Zeitpunkt.
- Der Eintrag läuft in derselben Transaktion. Scheitert er, scheitert die
  Änderung.
- Die Schulen bekommen keine Kopie in `config.setting_audit`.
- Das Träger-Portal zeigt die Änderungen je Einstellung als Liste.
- Gelöscht wird nach drei Jahren wie das Schul-Audit, spätestens mit dem Träger
  (Story 11).

### 8. Benachrichtigung

- Nach dem Commit einer Träger-Änderung geht `tenant_settings_changed` an jede
  Schule des Trägers.
- Einen prozessweiten Cache gibt es nicht. Der Minuten-Snapshot des Schedulers
  übernimmt Träger-Werte spätestens nach einer Minute.

## Verworfene Optionen

- **Kopie beim Anlegen einer Schule:** Spätere Träger-Änderungen bräuchten
  einen Fan-out über alle Schulen, und „vom Träger“ wäre nicht mehr von einer
  eigenen Abweichung zu unterscheiden.
- **Sperre zur Laufzeit:** ein dritter Zustand je Wert, offene Fragen zu
  bestehenden Abweichungen beim Sperren und mehr Fläche in der Oberfläche. Wer
  keine Abweichung erlauben will, nutzt `traeger`.
- **Admin-Transaktion mit Ratchet** für Träger-Werte wie bei `traeger-view`:
  Sie schriebe an RLS vorbei, was ADR 0084 §2 ausschließt.
- **Spalte `organization_id` in `config.setting_values`:** zwei Schlüssel in
  einer Tabelle, unklare Unique-Constraints und RLS, alle Queries betroffen.
- **`platform.organizations.settings` weiterverwenden:** ein zweiter Weg neben
  der Registry, ohne Typen, Validierung und Audit (#934).
- **Nummernkreise und Bankverbindungen als Einstellungen:** Ein Zähler muss
  atomar und lückenlos zählen, Bankverbindungen sind Listen mit Prüfung. Beides
  ist Zustand des Fach-Owners.
- **Träger-Werte für das moto-Team in v1**, etwa Löschfristen je Träger laut
  AVV: Sie bräuchten eine Träger-Seite und Träger-Audit im Operator-Portal.
  Das entscheidet eine eigene ADR, sobald Löschfristen je Träger konkret werden.

## Folgen

- Die Regel in `.claude/rules/settings-system.md` zu „zwei Ebenen“ wird mit dem
  ersten Bau-Ticket auf drei Ebenen umgestellt.
- `policy.json` bekommt `data_objects` für `config.traeger_setting_values` und
  `config.traeger_setting_audit` mit `write_owner` `settings-platform`.
- Wer eine Träger-Transaktion öffnen darf, steht wie die Admin-Einträge in
  `backend/test/transaction_boundaries.json`.
- Passwort-Felder und Schlüssel mit Sperre oder Seiteneffekt bleiben je Schule,
  bis eine eigene Entscheidung sie hebt.

# Multi-Tenancy: Anforderungen & Use Cases

Dieses Dokument beschreibt die Business-Anforderungen und Szenarien, die das Multi-Tenancy-System abdecken muss. Die technischen Dokumente (01-03) werden daraus abgeleitet.

---

## 1. Ausgangslage

Project Phoenix ist aktuell eine Single-Tenant-Anwendung fuer eine einzelne OGS. Ziel ist die Skalierung auf **100+ OGS bis Ende des Jahres**, verteilt auf mehrere Traeger.

---

## 2. Organisationsstruktur

### Hierarchie

```
Traeger (z.B. "Caritas Muenster")
|
+-- Traeger-Buero (0..N Mitarbeiter)
|   Personalplanung, Lohnabrechnungen, Steuerung
|   → sieht automatisch ALLE OGS des Traegers
|
+-- OGS-Buero Ost (0..N Mitarbeiter)
|   Operative Verwaltung: Vertretungen, Krankmeldungen, An-/Abmeldungen
|   → verwaltet OGS Altenberge + OGS Greven
|
+-- OGS Altenberge
|   +-- Betreuer (N)
|   +-- Kinder (N)
|       +-- Eltern (ueber Kind verknuepft, nicht direkt der OGS zugeordnet)
|
+-- OGS Greven
|   +-- Betreuer (N)
|   +-- Kinder (N)
|       +-- Eltern (ueber Kind verknuepft)
|
+-- OGS Emsdetten
    +-- Eigenes OGS-Buero (0..N Mitarbeiter)
    +-- Betreuer (N)
    +-- Kinder (N)
        +-- Eltern (ueber Kind verknuepft)
```

### Regeln

- 1 Traeger hat 1 bis N OGS (eine unabhaengige OGS ist selbst ihr eigener Traeger)
- Jede OGS hat N Kinder, N Betreuer
- **Buero-Mitarbeiter existieren auf zwei Ebenen** (nicht XOR, beides gleichzeitig moeglich):
  - **Traeger-Buero:** Zugriff auf ALLE OGS des Traegers (automatisch, auch neue OGS)
  - **OGS-Buero:** Zugriff auf 1 bis N spezifische OGS (individuell zugewiesen)
- Ein Buero-Mitarbeiter kann einem Traeger-Buero ODER einem OGS-Buero zugeordnet sein
- Eltern sind **nicht direkt** einer OGS zugeordnet, sondern ueber die Einschreibung ihres Kindes verknuepft
- Wenn ein Kind die OGS wechselt, wandert der Eltern-Zugriff automatisch mit
- Ein Elternteil kann Kinder in verschiedenen OGS und sogar verschiedenen Traegern haben
- Die OGS ist die **Daten-Isolationsgrenze** (ein Betreuer in OGS A darf NICHT die Daten von OGS B sehen)
- Der Traeger ist die uebergeordnete Organisation ("Umbrella")
- **Rollen sind global** (D13): Eine Rolle gilt fuer alle Tenants, auf die ein Account Zugriff hat. Per-Tenant-Rollen (z.B. "Admin bei OGS A, Betreuer bei OGS B") werden nicht implementiert — YAGNI, nachruesten trivial via `account_tenants.role_id`

---

## 3. Benutzerrollen & Zugriffsszenarien

### 3.1 Betreuer (Caregiver)

- Arbeitet im **Regelfall** an einer OGS
- Sieht nur Kinder, Gruppen, Raeume und Aktivitaeten dieser OGS
- Darf KEINE Daten anderer OGS sehen (auch nicht innerhalb des gleichen Traegers)
- **Ausnahmen innerhalb des gleichen Traegers:**
  - Ferienbetreuung: temporaerer Zugriff auf Kinder anderer OGS (siehe 4.1)
  - Vertretung/Aushilfe: Betreuer springt kurzfristig an einer anderen OGS ein (siehe 4.2)
  - Dauerhaft an mehreren OGS: z.B. Mo-Mi OGS A, Do-Fr OGS B (siehe 4.2)

### 3.2 Buero-Mitarbeiter (Office Staff)

Buero-Mitarbeiter existieren auf **zwei Ebenen** - beide koennen gleichzeitig innerhalb eines Traegers vorkommen:

**a) Traeger-Buero (Organization-Scope)**
- Strategisch/administrativ: Personalplanung, Lohnabrechnungen, Steuerung
- Hat automatisch Zugriff auf **ALLE OGS des Traegers** (auch neue, die spaeter hinzukommen)
- Braucht uebergreifende Uebersichten (z.B. Personal ueber alle OGS)
- Kann zwischen OGS wechseln oder aggregierte Ansichten sehen

**b) OGS-Buero (Tenant-Scope)**
- Operativ: Vertretungen, Krankmeldungen, An-/Abmeldungen
- Hat Zugriff auf **1 bis N spezifische OGS** (individuell zugewiesen, NICHT zwangslaeufig alle)
- Ein OGS-Buero kann auch mehrere OGS verwalten (z.B. "Buero Ost" verwaltet 3 von 5 OGS)
- Muss zwischen zugewiesenen OGS wechseln koennen ("OGS wechseln" Dropdown)

**Gemeinsam:**
- Arbeitet **innerhalb eines Traegers** (nicht traeger-uebergreifend)
- Ein Account, ein Passwort
- Zugriff wird pro Buero-Mitarbeiter festgelegt (Traeger-weit oder individuell pro OGS)

### 3.3 Operator (Platform-Admin)

- Sitzt ausserhalb der Tenant-Grenze
- Verwaltet Traeger, OGS, Subdomains
- Kann im Operator-Dashboard eine **bestimmte OGS auswaehlen** und deren Daten einsehen (z.B. Feedback, Statistiken)
- Braucht **keine Impersonation** - bleibt immer in der Operator-Plattform
- Kann Announcements / Release Notes an **alle OGS oder gezielt an bestimmte OGS** senden
- Sieht uebergreifende Statistiken und Vorschlaege

### 3.4 Eltern (Zukunft: Eltern-App)

- Haben einen eigenen Account (Email + Passwort, wie alle anderen Rollen)
- Sind **nicht direkt** einer OGS zugeordnet, sondern ueber ihre Kinder verknuepft
- Nutzen die **gleiche Subdomain** wie die OGS (z.B. `altenberge.{domain}`) - keine eigene Eltern-Subdomain
- System erkennt Eltern-Rolle beim Login und zeigt die Eltern-Ansicht
- Koennen 1 bis N Kinder haben
- Kinder koennen in **verschiedenen OGS** sein (z.B. Geschwister an unterschiedlichen Schulen)
- Kinder koennen sogar bei **verschiedenen Traegern** sein
- Bei Kindern in mehreren OGS: **OGS-Switcher** zum Wechseln zwischen den OGS-Ansichten
- Zugriff auf eine OGS ergibt sich automatisch aus der Einschreibung des Kindes
- Kommunizieren mit Betreuern / Buero der jeweiligen OGS ihres Kindes
- Sehen nur Daten des eigenen Kindes (nicht andere Kinder der OGS)

### 3.5 IoT-Geraete (PyrePortal)

- Physisch an einer OGS installiert
- Authentifizieren sich per Device-API-Key
- Backend mappt Device -> OGS (Tenant)
- Brauchen keinen expliziten Tenant-Header

---

## 4. Spezial-Szenarien

### 4.1 Ferienbetreuung (Cross-OGS)

**Situation:** Waehrend der Ferien betreuen OGS-Standorte eines Traegers gemeinsam Kinder. Kinder aus OGS Greven werden temporaer in OGS Altenberge betreut.

**Anforderungen:**
- Betreuer in OGS Altenberge muessen temporaer Kinder aus OGS Greven sehen und verwalten koennen
- Buero-Mitarbeiter muessen die gemischte Gruppe ueberblicken koennen
- Der Zugriff ist **zeitlich begrenzt** (z.B. "Sommerferien 2026: 01.07.-12.08.")
- Nach Ablauf faellt der Zugriff automatisch weg
- Moeglich innerhalb eines Traegers UND **traeger-uebergreifend** (maximale Flexibilitaet)

**Mechanismus (D4):** Tenant-Switch als Primaer-Mechanismus + gezielter Service-Level Cross-Tenant-Read fuer Ferienbetreuung. Admin erstellt Feriengruppe an Host-OGS, enrollt Kinder aus anderen OGS. Active-Service erkennt Cross-Tenant-Enrollments und holt nur die eingeschriebenen Kinder via privilegierten Read (Admin-Connection). RLS bleibt simpel: ein tenant_id pro Request, kein Array-Support.

### 4.2 Betreuer an mehreren OGS

**Situation A - Dauerhaft:** Ein Betreuer arbeitet regulaer an 2 OGS desselben Traegers (z.B. Mo-Mi in Altenberge, Do-Fr in Greven).

**Situation B - Vertretung:** Ein Betreuer springt kurzfristig an einer anderen OGS ein (z.B. Kollegin krank, Betreuer faehrt fuer einen Tag nach Greven).

**Anforderungen:**
- Ein Account, ein Passwort
- Kann zwischen OGS wechseln
- Sieht in jeder OGS nur die jeweiligen Daten
- Zugriff auf weitere OGS kann dauerhaft oder temporaer sein
- Nur innerhalb des gleichen Traegers moeglich

### 4.3 Eltern mit Kindern in verschiedenen OGS/Traegern

**Situation:** Eine Familie hat Kind A in OGS Altenberge (Caritas) und Kind B in OGS Stadtmitte (AWO).

**Anforderungen:**
- Ein Eltern-Account fuer beide Kinder
- Kann zwischen den OGS-Ansichten wechseln
- Sieht pro OGS nur die Daten des eigenen Kindes
- Traeger-uebergreifend moeglich
- Tenant-Zuordnung wird **automatisch** ueber Kind-Einschreibung gesteuert (kein manuelles Zuweisen)
- Kind verlässt OGS → Eltern-Zugriff auf diesen Tenant faellt automatisch weg

### 4.4 Subdomain-Routing

**Situation:** Jede OGS hat eine eigene Subdomain.

**Anforderungen:**
- `altenberge.{domain}` -> Login/Dashboard fuer OGS Altenberge
- `greven.{domain}` -> Login/Dashboard fuer OGS Greven
- `operator.{domain}` -> Operator Dashboard
- `{domain}` (Root) -> Tenant-Auswahl oder Landing Page
- Die konkrete Domain (z.B. `moto-app.de`, `moto.nrw`) wird spaeter entschieden

### 4.5 Neuen Tenant anlegen (Provisioning)

**Situation:** Ein Operator legt eine neue OGS an.

**Anforderungen:**
- Operator waehlt/erstellt Traeger
- Gibt OGS-Daten ein (Name, Slug, Subdomain, Adresse, etc.)
- System erstellt automatisch:
  - Subdomain-Eintrag
  - Default-Rollen und Permissions
  - Default-Einstellungen (config)
  - Einen Admin-Account fuer die neue OGS
- Operator erhaelt die Zugangsdaten fuer den OGS-Admin

---

## 5. Datenschutz / DSGVO

### Isolation

- Daten einer OGS duerfen NIEMALS unbeabsichtigt in einer anderen OGS sichtbar sein
- Dies gilt insbesondere fuer: Kinderdaten, Gesundheitsinformationen, Kontaktdaten von Erziehungsberechtigten
- Die Isolation muss auf Datenbankebene erzwungen werden (nicht nur im Application Code)

### Datenloeschung

- Ein Traeger kann verlangen, dass alle Daten einer OGS geloescht werden
- Dies muss vollstaendig und nachweisbar sein (Audit-Log)

### Audit

- Alle Cross-Tenant-Zugriffe muessen protokolliert werden
- Wer hat wann auf welche OGS zugegriffen?

---

## 6. Skalierung

- **Kurzfristig (2026):** 10-50 OGS, 3-5 Traeger
- **Mittelfristig (2027):** 100-200 OGS, 10-20 Traeger
- **Langfristig:** 500+ OGS, 50+ Traeger
- Performance darf bei 100 OGS nicht merklich leiden

---

## 7. Bewusste Abgrenzungen (nicht im initialen Rollout)

- **Traeger-weit geteilte Daten:** Alles startet per-Tenant (OGS). Gemeinsame Kategorien, Vorlagen o.ae. auf Traeger-Ebene koennen spaeter nachgeruestet werden (Beziehung Tenant → Organization existiert bereits in der DB).
- **OGS-Admin sieht Traeger-Infos:** OGS-Admins sollen ihren Traeger-Namen und Schwester-OGS (nur Namen, keine Daten) sehen koennen. Wird als Follow-up implementiert (org_id ist im JWT, minimaler Aufwand).
- **OGS-Konfiguration / Feature-Toggles:** Verschiedene OGS-Modi (z.B. mit/ohne Gruppen) werden ueber `settings JSONB` pro Tenant abgebildet. Vorlagen/Templates koennen spaeter ergaenzt werden, wenn sich Muster zeigen.

---

## 8. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-07 | Initiale Version mit allen bisherigen Anforderungen |
| 2026-02-08 | Eltern in Hierarchie, Betreuer-Edge-Cases, Operator ohne Impersonation, Cross-OGS traeger-uebergreifend, alle offenen Fragen geklaert |
| 2026-02-08 | Buero-Mitarbeiter auf zwei Ebenen (Traeger-Buero + OGS-Buero), Hierarchie-Diagramm aktualisiert |
| 2026-02-08 | D4-Mechanismus (Ferienbetreuung) und D13 (globale Rollen) ergaenzt |

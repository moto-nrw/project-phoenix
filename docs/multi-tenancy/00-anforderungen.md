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
+-- OGS Altenberge
|   +-- Betreuer (N)
|   +-- Buero-Mitarbeiter (N)
|   +-- Kinder (N)
|       +-- Eltern (ueber Kind verknuepft, nicht direkt der OGS zugeordnet)
|
+-- OGS Greven
|   +-- Betreuer (N)
|   +-- Buero-Mitarbeiter (N)
|   +-- Kinder (N)
|       +-- Eltern (ueber Kind verknuepft)
|
+-- OGS Emsdetten
    +-- Betreuer (N)
    +-- Buero-Mitarbeiter (N)
    +-- Kinder (N)
        +-- Eltern (ueber Kind verknuepft)
```

### Regeln

- 1 Traeger hat 1 bis N OGS
- Jede OGS hat N Kinder, N Betreuer, N Buero-Mitarbeiter
- Eltern sind **nicht direkt** einer OGS zugeordnet, sondern ueber die Einschreibung ihres Kindes verknuepft
- Wenn ein Kind die OGS wechselt, wandert der Eltern-Zugriff automatisch mit
- Ein Elternteil kann Kinder in verschiedenen OGS und sogar verschiedenen Traegern haben
- Die OGS ist die **Daten-Isolationsgrenze** (ein Betreuer in OGS A darf NICHT die Daten von OGS B sehen)
- Der Traeger ist die uebergeordnete Organisation ("Umbrella")

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

- Gehoert zum Traeger, arbeitet aber mit spezifischen OGS
- Kann Zugriff auf **1 bis N OGS** haben (NICHT zwangslaeufig alle OGS des Traegers!)
- Muss zwischen OGS wechseln koennen ("OGS wechseln" Dropdown)
- Braucht ggf. aggregierte Uebersichten ueber seine zugewiesenen OGS
- Zugriff wird pro Buero-Mitarbeiter individuell festgelegt

### 3.3 Operator (Platform-Admin)

- Sitzt ausserhalb der Tenant-Grenze
- Verwaltet Traeger, OGS, Subdomains
- Kann sich als OGS-Admin einloggen (Impersonation/Support)
- Sieht uebergreifende Statistiken und Vorschlaege

### 3.4 Eltern (Zukunft: Eltern-App)

- Haben einen eigenen Account
- Sind **nicht direkt** einer OGS zugeordnet, sondern ueber ihre Kinder verknuepft
- Koennen 1 bis N Kinder haben
- Kinder koennen in **verschiedenen OGS** sein (z.B. Geschwister an unterschiedlichen Schulen)
- Kinder koennen sogar bei **verschiedenen Traegern** sein
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
- Nur innerhalb eines Traeger-Verbunds moeglich (nicht traeger-uebergreifend)

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

## 7. Offene Fragen

- [ ] Koennen Buero-Mitarbeiter traeger-uebergreifend arbeiten? (z.B. Berater der fuer Caritas UND AWO taetig ist)
- [ ] Gibt es OGS die keinem Traeger zugeordnet sind? (Einzelne, unabhaengige OGS)
- [ ] Soll die Eltern-App eine eigene Subdomain haben? (z.B. `eltern.{domain}`)
- [ ] Brauchen Eltern eine andere Auth als Betreuer? (z.B. Social Login, Magic Link)
- [ ] Gibt es Daten die traeger-weit geteilt werden? (z.B. gemeinsame Aktivitaets-Kategorien, Vorlagen)
- [ ] Soll ein OGS-Admin seinen eigenen Traeger sehen koennen, oder ist das nur Operator-Sache?

---

## 8. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-07 | Initiale Version mit allen bisherigen Anforderungen |

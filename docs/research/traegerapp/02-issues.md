# Träger-Ebene im Issue-Tracker (moto-nrw/project-phoenix), Stand 2026-08-29

Recherchebasis: `gh issue list` mit den Suchbegriffen Träger, Traeger, Trägerapp, Träger-Portal,
organization, org-scope, Organisation, Karriereportal, Karriere, Bewerber, Recruiting,
Arbeitszeit, Personal, Finanzen, Abrechnung, Buchhaltung, DATEV, Lohn, Kommune, multi-school,
Verbund, Standort, Mandant, Reporting, Controlling, Bissendorf, Burbach, Caritas, AWO, Sales-Call,
plus alle offenen Epics. Bodies und Kommentare der relevanten Issues wurden gelesen.

## 1. Übersichtstabelle

| Nr | Titel | Status | Labels | Worum es geht |
|---|---|---|---|---|
| [674](https://github.com/moto-nrw/project-phoenix/issues/674) | feature: Trägerapp | OPEN | epic, ogs demanded feature, priority: medium | Dach-Epic der Trägerapp: Mehr-Standort-Übersicht, Dokumentenupload, Abrechnung, Zeiterfassungs-Aggregat |
| [935](https://github.com/moto-nrw/project-phoenix/issues/935) | Träger-Admin Rolle: Zugriff auf alle OGS eines Trägers | OPEN | feature, priority: medium | Rolle `organization_admin` mit automatischem Zugang zu allen OGS eines Trägers |
| [936](https://github.com/moto-nrw/project-phoenix/issues/936) | Onboarding: Neue OGS anlegen und initialisieren | OPEN | feature, priority: medium | Wizard für neue OGS inkl. Räume, Gruppen, Admin-Einladung, Subdomain-Prüfung |
| [934](https://github.com/moto-nrw/project-phoenix/issues/934) | Admin-UI: Träger und OGS verwalten | CLOSED | feature, priority: medium | Operator-CRUD für Träger und Schulen, umgesetzt bis auf Träger-Branding |
| [1322](https://github.com/moto-nrw/project-phoenix/issues/1322) | Epic: Schnittstellen-Integrationen (Schulsysteme, Träger, Reporting) | OPEN | epic, ogs demanded feature, feature | Drei Tracks: Schulsysteme, Träger-Systeme (BuT, Abrechnung), Reporting-Export |
| [2639](https://github.com/moto-nrw/project-phoenix/issues/2639) | Epic: Elternabrechnung, Grundlage, Rechnungen, SEPA und Träger-Export | OPEN | epic, priority: medium, SH | Klammer-Epic Elternabrechnung inkl. Datenexport an Träger oder Kommune |
| [2791](https://github.com/moto-nrw/project-phoenix/issues/2791) | Operator: Abrechnungsreport aktiv verwaltete Kinder je Stichtag | OPEN | feature, priority: medium | Stichtagsreport je Schule als Beleg für Rechnungen an Träger |
| [1459](https://github.com/moto-nrw/project-phoenix/issues/1459) | Add tenant/org subscription tiers and paywall gated features | OPEN | (keine) | Tarife und Kinderkontingent pro Tenant, offene Frage: Vertrag je Schule oder je Organisation |
| [704](https://github.com/moto-nrw/project-phoenix/issues/704) | feature: Ferienbetreuung in allen Apps implementieren | OPEN | epic, ogs demanded feature, priority: high | Ferien-Pooling über OGS-Grenzen hinweg innerhalb eines Trägers |
| [1021](https://github.com/moto-nrw/project-phoenix/issues/1021) | Accounts zusätzlichen Tenants zuweisen (admin/operator UI) | CLOSED | (keine) | API und UI, um bestehende Konten weiteren Schulen zuzuordnen |
| [1139](https://github.com/moto-nrw/project-phoenix/issues/1139) | Announcements org- und tenant-spezifisch adressieren | CLOSED | feature, priority: medium | `target_org_ids` und `target_tenant_ids` für Ankündigungen |
| [1017](https://github.com/moto-nrw/project-phoenix/issues/1017) | tenant deletion / deactivation | CLOSED | feature, priority: medium | Träger und Tenants löschbar bzw. deaktivierbar |
| [1435](https://github.com/moto-nrw/project-phoenix/issues/1435) | Operator-Dashboard zeigt gelöschte Träger in Tabellen und Dropdowns | CLOSED | bug | Gelöschte Träger tauchten in Tabellen und Dropdowns auf |
| [1975](https://github.com/moto-nrw/project-phoenix/issues/1975) | Tenant-Umschalter wechselt nicht bei slug != subdomain (stiller 404) | CLOSED | bug, priority: high | Stiller 404 beim Schulwechsel, Kundenmeldung Träger Gemeinde Burbach |
| [861](https://github.com/moto-nrw/project-phoenix/issues/861) | Export historische Anwesenheit für Berichterstattung | CLOSED | ogs demanded feature | Caritas-Anfrage: Nachweis gegenüber Behörden |
| [2606](https://github.com/moto-nrw/project-phoenix/issues/2606) | Statistik: Anwesenheits-/Fehlzeitenquoten und Raumauslastung | CLOSED | feature, priority: medium | Auswertungsseite mit Export, Grundlage für Trägerberichte |
| [2223](https://github.com/moto-nrw/project-phoenix/issues/2223) | Export-Landschaft konsolidieren | OPEN | maintenance, priority: medium | Doppelte Exporte und Layouts vereinheitlichen |
| [2635](https://github.com/moto-nrw/project-phoenix/issues/2635) | Fehlende Schnittstellen: CalDAV, SFTP-Dateiaustausch und RSS | OPEN | feature, priority: medium, SH | SFTP-Dateiaustausch als möglicher Träger-Übergabeweg |
| [1446](https://github.com/moto-nrw/project-phoenix/issues/1446) | iServ Export -> moto Import | OPEN | ogs demanded feature, feature, priority: medium | Stammdatenimport aus Schulsystem |
| [1417](https://github.com/moto-nrw/project-phoenix/issues/1417) | Zeiterfassung Admin-Übersicht Soll/Ist/Saldo + DATEV Export | CLOSED | feature, priority: medium | DATEV/XLSX-Export, ausdrücklich kein Total über mehrere Schulen |
| [1583](https://github.com/moto-nrw/project-phoenix/issues/1583) | Enrollment legal texts, consent model, AVV readiness | OPEN | priority: high | Verantwortlicher (Schule/Träger/UG) als Produktmodell fehlt |
| [2789](https://github.com/moto-nrw/project-phoenix/issues/2789) | Rechtstext-Links in den Portal-Footern | OPEN | feature, priority: medium | Datenschutzinfo des Trägers verlinken, nicht die von moto |
| [2180](https://github.com/moto-nrw/project-phoenix/issues/2180) | Startseite /home: rollenspezifischer Einstieg | OPEN | ogs demanded feature, priority: high | Rechte- statt rollennamenbasierter Aufbau, relevant für Trägerrollen |
| [1918](https://github.com/moto-nrw/project-phoenix/issues/1918) | System-wide domain and data-lifecycle audit | OPEN | maintenance, priority: low | Organization/School-Lifecycle, Slug-Duplikate, Tenant-Löschung |
| [2580](https://github.com/moto-nrw/project-phoenix/issues/2580) | Backend-Zielarchitektur und blockers-first Migration | OPEN | ready-for-agent | Modul „Organisation & Tenancy" als eigenes Fachmodul |
| [2659](https://github.com/moto-nrw/project-phoenix/issues/2659) | Expose Organization lookup and lifecycle capabilities | OPEN | ready-for-agent | Facade über `platform.organizations` |
| [2660](https://github.com/moto-nrw/project-phoenix/issues/2660) | Expose School lookup and lifecycle capabilities | OPEN | ready-for-agent | Facade über `platform.schools`, Slug-Auflösung erhalten |
| [2721](https://github.com/moto-nrw/project-phoenix/issues/2721) | Migrate roles, permissions, account tenants, operator access | OPEN | ready-for-agent | `auth.account_tenants`, Rollen und Scopes hinter eine Facade |
| [2708](https://github.com/moto-nrw/project-phoenix/issues/2708) | Cut Data Import over to owner Commands | OPEN | ready-for-agent | Import schreibt künftig über Owner-Commands, betrifft organizations/schools |
| [1179](https://github.com/moto-nrw/project-phoenix/issues/1179) | Epic: Architecture Refactor Roadmap | OPEN | epic | Dach-Epic des Architekturumbaus |
| [1944](https://github.com/moto-nrw/project-phoenix/issues/1944) | Web-Sessions und API-Grenze schrittweise vereinfachen | OPEN | maintenance, priority: low | Backend-owned Sessions, Portal-Isolation, Tenant-Wechsel |
| [1675](https://github.com/moto-nrw/project-phoenix/issues/1675) | Research and design parent billing and invoices | CLOSED | (keine) | Vorarbeit zu #2639, bewusst zurückgestellt |
| [1479](https://github.com/moto-nrw/project-phoenix/issues/1479) | Settings IA überarbeiten und Settings-Fläche reduzieren | OPEN | maintenance | Setup-Profile der Kundschaft, Organisationseinheit teils Angebot statt Gruppe |
| [1523](https://github.com/moto-nrw/project-phoenix/issues/1523) | Settings IA Folgearbeit: Gruppenmodus und Care-Konzept | OPEN | maintenance | Setting als Setup-/Organisationsentscheidung, viele offene Fragen |
| [1445](https://github.com/moto-nrw/project-phoenix/issues/1445) | epic: Intelligenter Dienstplan | OPEN | epic | Personalbedarf je Raum, Aktivität, Standort |
| [2314](https://github.com/moto-nrw/project-phoenix/issues/2314) | Support multi-school, MFA, and operator Gerätekopplung | OPEN | ready-for-agent | Gerätekopplung bei Konten mit mehreren Schulen |
| [924](https://github.com/moto-nrw/project-phoenix/issues/924) | fix: Multi-Tenant Review | CLOSED | (keine) | Vollständigkeitsprüfung der Tenant-Isolation über alle Layer |

## 2. Themen und konkrete Anforderungen

### 2.1 Zugang, Mandanten und Rollen

**#935** ist das Kernstück. Heute muss ein Träger-Admin „manuell pro OGS in `auth.account_tenants` +
`auth.account_roles` eingetragen werden. Bei einem Träger mit 10+ OGS ist das unpraktikabel."
Gefordert:

- neue Rolle `organization_admin` in `auth.roles` (oder Flag in `platform.organizations`)
- „Automatische Zuordnung: Wenn eine neue OGS unter dem Träger erstellt wird, erhalten alle
  Träger-Admins automatisch `account_tenants` + `account_roles` Einträge"
- Tenant-Switcher zeigt alle OGS des Trägers gruppiert (laut Issue bereits vorhanden)
- Träger-Admin kann Nutzer einladen, Rollen zuweisen, Einstellungen ändern, für alle OGS
- „Abgrenzung: Träger-Admin ≠ Plattform-Operator (kein Zugriff auf andere Träger)"

Technische Optionen im Issue: Option A eigene `organization_admins`-Tabelle mit FK auf
`platform.organizations`, Option B `auth.account_roles` um `organization_id` erweitern. Dazu die
Setzung: „Bestehende JWT-Claims (`tenant_id`) bleiben, Träger-Admin wechselt per Tenant-Switcher
zwischen OGS" und „RLS bleibt tenant-scoped (kein Cross-Tenant-Query nötig)".

**Codebefund dazu (wichtig für die Ideation):** Der Scope `"org"` existiert bereits halb.

- `backend/tenant/context.go:27`: `ScopeOrg = "org" // Organization-level user (sees all schools in org)`
- `backend/services/auth/auth_login.go:952`: „Org-scope check (§6.3): Träger-Büro users access any
  school in their org", geprüft wird `school.OrganizationID == callerOrgID`
- `backend/database/repositories/auth/accounts.go:22ff`: eigener Membership-Zweig für ScopeOrg
  (EXISTS-Query über `account_tenants` join `platform.schools` auf `organization_id`)
- `backend/services/auth/permission_management.go:177ff`: `ensureOrganizationRBACMembership`
- `backend/models/auth/token.go:34`: `PortalScopeOrg = "org"`, in der Session-Persistenz vorgesehen
- `backend/auth/jwt/claims.go:228`: Scope-Kommentar listet `"org"` als gültigen Wert

**Kein Login-Pfad setzt jemals `metadata.scope = ScopeOrg`.** `auth_login_school.go` setzt
`ScopeSchool`, für Org gibt es keine Entsprechung. Die Plumbing ist da, ein solches Token wird
nirgends ausgestellt. Das ist der günstigste Einstiegspunkt für eine Trägerapp und zugleich ein
Risiko: die Zweige sind heute toter, im Betrieb nie durchlaufener Code.

**#1021** (geschlossen) hat die Zuweisung bestehender Accounts zu weiteren Tenants gebaut
(`POST/DELETE/GET /api/admin/accounts/{id}/tenants`, „Tenant Access"-Bereich in der Kontoverwaltung,
Audit für DSGVO). Der Body endet mit einer offenen Frage: „Discuss how can asign? Admin of a school,
only operators?"

**#1975** (geschlossen, priority: high) zeigt die Fragilität des Mehr-Schul-Zugangs:
„Betroffen sind Konten mit Zugriff auf mehrere Schulen desselben Trägers. Kundenmeldung
(Träger Gemeinde Burbach): Admin Sven Reschke ist bei OGS Wahlbach angemeldet, sieht im Umschalter
OGS Burbach, klickt darauf, nichts passiert." Ursache: `slug` und `subdomain` sind auf
`platform.schools` „zwei getrennte, unabhängig validierte Spalten", der Umschalter sendete den Slug,
`auth_login.go` löste über `FindBySubdomain` auf, Ergebnis 404, den das Frontend verschluckte.
Gleiche Ursache hatte #1977 (Einladungs-Redirect nutzt `school.Slug` statt Subdomain).

**#2314** (offen, ready-for-agent) verlangt für die Gerätekopplung: „School users see only schools
where they hold `devices.manage` and a new Gerät has available capacity", genau eine passende Schule
wird automatisch gewählt, mehrere verlangen eine explizite Auswahl, „No eligible school produces a
clear reason without leaking inaccessible schools". Das ist das bisher sauberste Muster für
Mehr-Schul-Auswahl-UI im Repo.

**#1139** (geschlossen) hat mit `target_org_ids BIGINT[]` und `target_tenant_ids BIGINT[]` auf
`platform.announcements` ein org-weites Adressierungsmuster etabliert, inklusive Filterung über
`claims.OrgID`: „Filter by org: `AND (a.target_org_ids = '{}' OR ? = ANY(a.target_org_ids))` using
`claims.OrgID`". Vorlage für weitere org-weite Features.

**#924** (geschlossen) dokumentiert den Stand der Tenant-Isolation: Repository-Layer vollständig auf
`base.GetDB(ctx, r.db)`, Service-Layer mit dokumentierten Ausnahmen, „Operator-Routes sind
platform-scoped (kein TenantTx nötig)".

### 2.2 Trägerapp im engeren Sinn

**#674** ist bewusst offen gehalten: „Aktuell eine Blackbox für uns, wird aber nachgefragt."
Von OGS nachgefragte Features laut Body:

- Übersicht und Verwaltung mehrerer Standorte
- Dokumentenupload (BuT oder sonstige Informationen)
- Abrechnungsmöglichkeit (direkt oder Schnittstelle mit externer Software)

Zwei Kommentare schärfen das:

1. Abgrenzung zum Schnittstellen-Epic: „Die Schnittstellen-Aspekte aus diesem Epic
   (BuT-Dokumentenupload, Abrechnungs-Schnittstelle zu externer Software) sind ab jetzt in #1322
   gebündelt (Track 2: Träger-Systeme). Die Trägerapp-spezifischen Features (Multi-Standort-Übersicht,
   eigene UI) bleiben hier."
2. Neuer Track Personal (Zitat siehe Abschnitt 3).

### 2.3 Personal und Arbeitszeit

Die einzige belastbare Träger-Anforderung ist der Kommentar in **#674**: Aggregat der Zeiterfassung
über mehrere OGS in der Träger-App.

**#1417** (geschlossen) hat die Schul-Ebene gebaut: Übersicht mit Soll/Ist/Saldo/Resturlaub pro
Mitarbeitendem, „Filter: Beschäftigungstyp, Standort, Saldo-Range", Cross-MA-Export als CSV,
„DATEV LODAS ASCII", „DATEV Lohn & Gehalt ASCII", „XLSX für Träger-internen Gebrauch", Monats- und
Jahresaggregation.

Direkt kollidierend mit dem Bissendorf-Wunsch ist die dortige Scope-Entscheidung:
„**Bewusst nicht im Scope** (DSGVO / Persönlichkeitsschutz): kein Ranking ‚Top 5
Überstunden-Lieferanten', kein Vergleichs-Chart zwischen MA, kein Aufrechenbares Total über mehrere
Schulen."

**#1417 Tranche 2c** hält den Research-Bedarf fest, der für die Träger-Ebene weiter gilt:
„Welche Lohn-Systeme nutzen unsere OGS-Träger konkret? (Bissendorf + 3-5 weitere)",
„DATEV-Partner-Zertifikat anstreben?", „DATEV-Datenservices REST API (HR Imports v2.0) zusätzlich zur
ASCII-Datei?", „Sekundäre Formate (Sage, Lexware, ADDISON) je nach Träger-Feedback".

**#1445** (Dienstplan-Epic) modelliert Personalbedarf „pro Zeitraum, Raum, Aktivität, Standort oder
Gesamtbereich", bleibt aber innerhalb einer Schule.

**Karriereportal, Recruiting, Bewerbermanagement, Stellenausschreibungen kommen im Tracker
nirgends vor.** Suchen nach Karriereportal, Karriere, Bewerber, Recruiting und Stellenausschreibung
liefern null Treffer. Reines Neuland ohne Vorarbeit und ohne dokumentierte Nachfrage.

### 2.4 Finanzen und Abrechnung

Drei verschiedene Dinge heißen im Produkt ähnlich. **#2639** trennt sie ausdrücklich:
„**Nicht verwechseln:** Der Branch `feat/demo-billing` und `platform.school_invoices` betreffen
Rechnungen **moto zu Schule** (der Vertrag der Schule bei moto), vom Operator von Hand gepflegt, ohne
Zahlungsdienstleister (#1459). Und ‚Abrechnung' heißt im Produkt heute **Personalabrechnung**
(DATEV). Beides ist nicht gemeint."

Codestand laut #2639 (Prüfung vom 2026-08-28): tagesgenaue Betreuungsteilnahme nur als
Anwesenheitsdaten vorhanden, „nicht als Abrechnungsgrundlage"; tagesgenaue Essensteilnahme fehlt
vollständig (#2638); Abrechnung/Tarife/Perioden fehlt, „`care_offerings.price_cents` wird nur
angezeigt, nie summiert"; SEPA-Lastschriftmandate fehlen vollständig; Rechnungsstellung an Eltern
fehlt; „Export an Träger oder Kommune" fehlt.

Die für Trägerarbeit wichtigste Aussage:
„**Wer rechnet ab, ist offen, und wird nicht von uns entschieden.** OGS, Träger, Kommune oder ein
externes System: davon hängt ab, ob moto Rechnungen erzeugt, nur anzeigt oder nur Daten liefert.
Diese Frage beantworten die Trägergespräche, nicht eine interne Setzung."

Und zum Prozess: „Zum Thema Abrechnung mit Trägern haben bereits Gespräche stattgefunden, weitere
werden folgen. Wie Träger sich die Umsetzung vorstellen, ist damit keine Annahme mehr, die wir selbst
treffen müssen, sondern Input, der beschafft und hier festgehalten wird." Gefragt wird explizit,
„ob der Träger selbst abrechnet und von moto nur Daten erwartet, oder ob moto die Rechnung erzeugen
soll, in welchem Format und Rhythmus Daten übergeben werden sollen, und an welches System sie gehen".
**Das Issue hat bis heute null Kommentare, die Gesprächsergebnisse fehlen also.**

Weitere Randbedingungen aus #2639: SEPA ist „ein Themengebiet, kein Feature" (Gläubiger-ID,
Mandatsverwaltung, Vorabankündigung, pain.008-XML, Rücklastschriften); „IBAN ist eine neue
Datenklasse"; „Begriffe trennen", die Elternabrechnung braucht einen eigenen Namen, „bevor die erste
Oberfläche entsteht".

**#1675** (geschlossen 2026-08-05) hatte die Frage schon aufgeworfen: „Who creates invoices: OGS /
Träger / municipality / external provider / external accounting system". Schließungsgrund: „Das
Feature ist bisher nicht angefragt. Um eine Implementierung zu vermeiden, die an den Workflows der
OGS vorbeigeht wird das Issue geschlossen."

**#2791** ist der operative Gegenpol und das jüngste Träger-Issue: „Die künftige Abrechnung basiert
auf der ‚Zahl der am Stichtag im System aktiv verwalteten Kinder je Einrichtung'. Damit das belegbar
ist (erste Rechnungsprüfung eines Trägers!), braucht das Betreiberportal einen Report." Gefordert:
monatlicher Stichtagswert (Tag konfigurierbar, z. B. der 15.) je Schule mit Anzahl aktiv verwalteter
Kinder und aktiver Terminals, CSV-Historie, saubere Definition von „aktiv verwaltet".
„Reiner Operator-Scope, kein Tenant-Feature."

**#1459** stellt die für Träger entscheidende Vertragsfrage unter „offene Fragen":
„Gilt ein Vertrag pro Schule oder kann er auf Organisationsebene mehrere Schulen umfassen?"
Weitere offene Punkte dort: welche Kinder zum Kontingent zählen, ob Überschreitung nur einen Hinweis
oder eine Sperre auslöst, Benachrichtigungsschwellen, Startdatum für Tarifwechsel.

### 2.5 Berichte und Reporting

**#1322 Track 3** nennt den Anlass: „Caritas und andere OGS müssen gegenüber Behörden nachweisen,
dass Kinder nicht vor 15:00 gehen. Heute: Zettel und Ordner." Sub-Tasks: Excel-Export historischer
Anwesenheit pro Kind mit Ankunfts- und Abholzeit, Filter nach Zeitraum, Gruppe, Kind, „Optional:
PDF-Report mit Träger-Branding". Akzeptanzkriterium: „Berichterstattung gegenüber Behörden ist mit
dem Export abgedeckt (Validation mit Caritas)".

**#861** (geschlossen) ist das Detail-Issue: „Anfrage der Caritas: Export von Kindern mit täglicher
Ankunfts- und Abholzeit. OGS müssen berichterstatten und nachweisen, dass Kinder nicht vor 15.00
gehen. Aktuell ein riesiger Aufwand mit Zetteln, Ordnern, etc."

**#2606** (geschlossen) hat die Statistikseite nachgeschoben (Anwesenheitsquote je Kind, Fehlzeiten
nach krank/entschuldigt/ohne Meldung, Gruppenaggregat, Raumauslastung) mit verbindlichen Definitionen
für Betreuungstage inklusive Feiertagen, Schließtagen und Ferienperioden. Zwei Grenzen, die für
Trägerberichte relevant sind:

- „Raumdaten liegen nur für die letzten 30 Tage vor" (Aufbewahrung, `active.visits` werden gelöscht)
- „Gruppe = aktuelle Zuordnung `users.students.group_id`. Es gibt keine Verlaufshistorie; ein Kind
  wird rückwirkend in seiner heutigen Gruppe gezählt."

Ausdrücklich nicht enthalten: „Nächtliche anonyme Verdichtung (Tageskennzahlen pro Raum/Gruppe),
damit Raumauslastung über mehr als 30 Tage und Gruppenzuordnung tagesgenau möglich werden. Folge-Issue,
falls eine Schule Halbjahresauswertungen braucht." Für Trägerberichte über ein Schulhalbjahr ist das
die Lücke.

**#2223** warnt vor Wildwuchs in der Exportlandschaft, bevor Träger-Exporte dazukommen.
**#2635** bringt SFTP-Dateiaustausch als Übergabeweg ins Spiel.

### 2.6 Onboarding und Migration

**#936** fordert einen „Wizard oder Step-by-Step UI für Plattform-Operatoren / Träger-Admins" mit
Pflichtangaben Name, Slug, Subdomain, Träger-Zuordnung, Adresse, automatischer Erstellung von
Systemräumen (Mensa, Schulhof, WC), Standard-Betreuungsgruppen (OGS-Früh, OGS-Mittag,
OGS-Nachmittag) und erstem Admin-Account per Einladung, Subdomain-Verfügbarkeitsprüfung und optional
CSV-Import von Schülerlisten. Genannt ist `POST /api/admin/schools` mit transaktionalem Setup.
Der Repo-Stand weicht ab, gebaut wurde alles unter `/api/operator/*`.

**#934** ist geschlossen mit einer präzisen Restliste (Abschlusskommentar von theitger, Abgleich
gegen `development` bei `bfe78068a`): Träger-Liste mit CRUD, OGS-Liste pro Träger, Aktivieren und
Deaktivieren inklusive Wiederherstellen, Zugang nur für Operatoren, alles erledigt. Offen ist
ausschließlich: „Träger-Settings (Logo, Primärfarbe) und Vererbung auf die OGS".

Begründung dort: „`platform.organizations.settings` ist eine JSONB-Spalte, die kein Service und keine
UI liest oder schreibt. Branding läuft heute pro Schule über das Settings-System (Login-Bild); ein
Primärfarben-Feld gibt es nirgends. Eine Umsetzung über diese Spalte würde einen zweiten
Konfigurationsmechanismus neben dem Settings-Registry aufmachen." Vorschlag: „Falls trägerweites
Branding gebraucht wird, dafür ein eigenes kleines Issue, das Träger-Default plus Schul-Override
sauber im Settings-System verankert." **Dieses Folge-Issue existiert bisher nicht.**

**#1918** formuliert die Zielinvarianten: „Organization/school: `platform.organizations` and
`platform.schools` own hierarchy and the tenant boundary. Reserved slugs are manually duplicated in
frontend code; school JSON settings bypass the registry." Zielbild: „School ID is permanent and never
reused; archive before purge; tenant deletion has a full dependency/retention preview."

**#1017** (geschlossen) hielt knapp fest: „Tenants und Organisation sollten auch gelöscht /
deaktiviert werden können." **#1435** (geschlossen) war der Folgefehler: gelöschte Träger blieben im
Operator-Dashboard in Tabellen und Dropdowns sichtbar, Beispiel aus Produktion `Ogata Demo`.

### 2.7 Sonstiges mit Trägerbezug

**#704 (Ferienbetreuung)** enthält die einzige echte cross-tenant-Fachanforderung im Tracker:
„Alle Anmeldungen in einem Träger werden oft gepooled, und dann wir e.g. von 8 OGS in 2 OGS
Ferienbetreuung angeboten, aber für Kinder die sich über die 8 OGS verteilen. Ergo, Kinder müssen auch
über die ‚eigene OGS-Grenze' hinaus angezeigt werden können für die Ferienbetreuung."
Ebenfalls dort: „Betreuer/OGS-Büro/Träger müssen Anmeldungen sehen und verwalten können."

**#1583** und **#2789** betreffen die Rolle des Trägers als datenschutzrechtlich Verantwortlicher.
#2789: „Link auf die vom Träger hinterlegte Datenschutzinformation (Verantwortlicher ist die
Schule/der Träger, NICHT auf die Website-Datenschutzerklärung von moto verlinken, das wäre inhaltlich
falsch)". #1583 fordert ein Produktmodell für „responsible organization/controller name and address",
DSB-Kontakt, Aufsichtsbehörde, „role model: school, school carrier, OGS carrier/UG, joint controllers
if applicable", Rechtsgrundlagen je Formularmodul, Empfänger und Unterauftragsverarbeiter. Dort auch:
„AGB/Teilnahmebedingungen are optional depending on the school/träger/UG setup" und „If
Moto/Ganztagshelden UG hosts/processes data for schools/UGs, AVV/TOMs/subprocessor documentation must
be managed B2B/B2G with the responsible organization."

**#2180** (Startseite /home) enthält eine Randbedingung, die für jede Trägerrolle gilt: „Rollen sind
pro Schule frei anlegbar (`auth.roles` mit `base_role` aus `admin`, `user`, `guardian`). Die
Zusammensetzung der Seite darf deshalb nicht an Rollennamen hängen, sondern an Berechtigungen."

**#1479** liefert Kundenprofile, die den Zuschnitt einer Trägerapp beeinflussen, unter anderem:
„am Berg baut sich das heute mit 4 Pseudo-Gruppen ‚Jahrgang 1-4' (51 Kinder pro ‚Gruppe') selbst und
organisiert die Betreuung real über 72 Angebote. Das ist kein modellierter Modus, sondern ein
Workaround." Und: „Altenberge und Burbach haben 553 bzw. 274 Guardian-Profile als reine Kontaktdaten
und null verknüpfte Eltern-Accounts."

## 3. Belegtes Trägerfeedback und Kundenaussagen

| Quelle | Wann | Aussage |
|---|---|---|
| Träger Gemeinde Bissendorf, Sales-Call (#674 Kommentar) | 2026-05-04 | „Aggregat der Zeiterfassung soll in der Träger-App sichtbar sein für Personalübersicht über mehrere OGS hinweg. Träger Gemeinde Bissendorf hat 3 OGS, Personalblick über alle drei zentral wäre ein Verkaufsargument." |
| Träger Gemeinde Burbach, Admin Sven Reschke (#1975) | 2026-07 | Tenant-Umschalter zwischen OGS Wahlbach und OGS Burbach ohne Wirkung, kein Fehler sichtbar |
| Caritas (#861, #1322 Track 3) | 2026-02 bis 2026-04 | Export von Kindern mit täglicher Ankunfts- und Abholzeit, Nachweis gegenüber Behörden, dass Kinder nicht vor 15:00 gehen, „aktuell ein riesiger Aufwand mit Zetteln, Ordnern" |
| OGS Fiege, Ibbenbüren, Burbach (#704) | 2026-01 | „Ferienbetreuung ist ein riesen Verwaltungsaufwand. Wurde mehrfach von OGS bestätigt", Pooling über Träger-OGS hinweg |
| Nicht namentlich genannter Träger (#2791) | 2026-08-29 | „erste Rechnungsprüfung eines Trägers" verlangt belegbare Stichtagszahlen |
| Trägergespräche zur Abrechnung (#2639) | laufend, Stand 2026-08-28 | Gespräche haben stattgefunden, Ergebnisse sind im Issue noch nicht dokumentiert |
| Florian Lüttgenau, Conrad (#1322, Slack 2026-04-26 bis 2026-04-28) | 2026-04 | Research-Stand iServ, WebUntis, SCHILD, SchulCLOUD, SDUI, Zuständigkeiten Conrad und Flo |
| Schule am Berg (#1479, #2495) | 2026-08 | Organisationseinheit ist real das Angebot, nicht die Gruppe |

Nicht belegt und im Tracker nicht auffindbar: Wünsche zu Karriereportal, Personalverwaltung über
Standorte hinweg jenseits des Zeit-Aggregats, Finanzcontrolling auf Trägerebene, Träger-Dashboard mit
Kennzahlen, Verbund- oder Kommunalstrukturen oberhalb des Trägers.

## 4. Widersprüche und offene Fragen

1. **RLS-Annahme gegen Ferien-Pooling.** #935 setzt „RLS bleibt tenant-scoped (kein Cross-Tenant-Query
   nötig)", #704 verlangt Kinderlisten über OGS-Grenzen hinweg. Beides zusammen geht nicht ohne
   Entscheidung: echte org-scoped Reads oder eine Read-Projektion. #2580 nennt für so etwas
   „Read-Projektionen verbinden Daten ausschließlich lesend und tenant-sicher".
2. **Träger-Aggregat gegen die DSGVO-Entscheidung in #1417.** Dort wurde „kein aufrechenbares Total
   über mehrere Schulen" bewusst ausgeschlossen. Bissendorf will genau das. Ungeklärt, ob
   personenscharf oder nur aggregiert, und wer es sehen darf.
3. **Ist die Trägerapp ein eigenes Portal oder eine Rolle?** #935 löst es als Rolle plus
   Tenant-Switcher, #674 spricht von „eigener UI". Die Portal-Architektur (Tenant, Operator, Parents,
   School) hat ein fünftes Portal weder vorgesehen noch ausgeschlossen. Keine Entscheidung dokumentiert.
4. **`scope="org"` ist halb gebaut.** Kein Login mintet ihn, die vorhandenen Org-Zweige in Repository,
   Login-Switch und Permission-Service sind ungenutzt. Ungeklärt, ob sie das Zielbild sind oder
   Altlast, die bei der Identity-Migration (#2721) mit umzieht oder wegfällt.
5. **Wer darf eine OGS anlegen?** #936 sagt „Plattform-Operatoren / Träger-Admins", #934 sagt
   „Zugang nur für Plattform-Operatoren (nicht für Tenant-Admins)". Direkter Widerspruch.
6. **Träger-Branding hat keinen Ort.** In #934 als einziger offener Punkt markiert, mit begründeter
   Ablehnung des naheliegenden Wegs (`organizations.settings` JSONB). Das vorgeschlagene Folge-Issue
   (Träger-Default plus Schul-Override im Settings-Registry) wurde nie angelegt.
7. **Vertrag je Schule oder je Organisation?** (#1459) Nicht beantwortet, entscheidet über die
   Datenmodellierung von Tarifen, Kontingenten und über den Zuschnitt von #2791.
8. **Wer rechnet ab?** (#2639) Bewusst offen, bis die Trägergespräche ausgewertet sind. Bis dahin ist
   jeder Träger-Export-Zuschnitt Spekulation. Die Gesprächsergebnisse fehlen im Issue.
9. **Datenschutzrechtliche Rolle des Trägers** (#1583) ist im Produkt nicht modelliert, obwohl der
   Träger in vielen Konstellationen Verantwortlicher ist. Ein Trägerportal, das Daten mehrerer Schulen
   bündelt, verschärft die Frage inklusive AVV und gemeinsamer Verantwortlichkeit.
10. **`slug` gegen `subdomain`.** Zwei unabhängige Spalten auf `platform.schools`. Der Fix in #1975 hat
    das Symptom behoben, nicht die Doppelung. Jede neue trägerweite Navigation läuft in dieselbe Falle,
    #2660 fordert explizit „Preserves slug resolution".
11. **Zwei Bedeutungen von „Abrechnung"** im Produkt (Personal/DATEV gegen Elternabrechnung), dazu
    „Rechnungen" für moto zu Schule. #2639 verlangt einen eigenen, unterscheidbaren Namen, bevor die
    erste Oberfläche entsteht. Ein Träger-Menüpunkt „Abrechnung" wäre heute mehrdeutig.
12. **Reporting-Tiefe.** #2606 kann wegen 30-Tage-Aufbewahrung und fehlender Gruppenhistorie keine
    Halbjahresauswertung liefern. Ein Träger, der Quartals- oder Halbjahresberichte erwartet, braucht
    zuerst die dort ausgeklammerte nächtliche Verdichtung.
13. **Zählweise „aktiv verwaltetes Kind"** (#2791) ist noch nicht definiert, hängt aber an der
    Rechnungsstellung gegenüber Trägern.

## 5. Backend-Refactor mit Kollisionspotenzial

Der laufende Umbau berührt genau die Tabellen und Pakete, die eine Trägerapp braucht.

- **#2580** (ready-for-agent, das aktive Vorhaben) definiert „Organisation & Tenancy" als eigenes
  Fachmodul mit alleinigem Write-Owner. Kernregeln: „Jedes schreibbare Datenobjekt hat genau einen
  Runtime-Write-Owner. Fremde Module schreiben nie direkt in dessen Tabellen oder Repository."
  Und: „Öffentliche Contracts enthalten keine ORM-Typen, Repository-Typen, generischen CRUD-APIs,
  Filter-Maps oder internen Modelle." Neue Trägerfeatures dürfen also nicht mehr direkt auf
  `platform.organizations` zugreifen.
- **#2659** legt die Query/Command-Facade über `platform.organizations`, betroffene Pakete
  `api/operator`, `api/platform`, `services/platform`, `database/repositories/platform`,
  `models/platform`.
- **#2660** dasselbe für `platform.schools`, ausdrücklich „Preserves slug resolution and tenant
  boundaries". Beide fordern „Multi-table writes use one UnitOfWork; no fallback read chain or
  application dual write" und Entfernen des alten Providers im selben Deploy.
- **#2721** zieht `auth.account_permissions`, `auth.account_roles`, `auth.account_tenants`,
  `auth.permissions`, `auth.role_permissions`, `auth.roles` hinter eine Identity-&-Access-Facade,
  Ziel: „Keeps organization, tenant, platform, parent, and school scopes isolated". Genau diese
  Tabellen müsste #935 erweitern. Ein `organization_id` an `auth.account_roles` oder eine neue
  `organization_admins`-Tabelle mitten in diesen Cutover zu legen, erzeugt Konflikte und neue
  Ratchet-Keys. #2721 ist zusätzlich blockiert durch #2645, #2659, #2660, #2667, #2669, #2720.
- **#2708** (Data Import) listet `platform.organizations` und `platform.schools` unter den betroffenen
  Tabellen und betrifft damit den Onboarding-Pfad aus #936, inklusive der Anforderung „parse/validate
  a complete batch before writes".
- **#1179** ist das Dach-Epic des Architekturumbaus (dünne Handler, fette Use-Case-Schicht, strikte
  Abhängigkeitsrichtung).
- **#1944** stellt Sessions und API-Grenze um. Zielbild unter anderem: „Das Go-Backend ist alleiniger
  Besitzer von Web-Sessions", „Tenant-, Operator- und Parent-Portal bleiben technisch isoliert",
  „keine Änderung der Tenant-URLs ohne separates ADR und Produktentscheidung". Ein fünftes
  Träger-Portal oder ein org-weiter Host wäre genau so eine ADR-pflichtige Änderung.
- **#1918** fordert vor Trägerfeatures saubere Lifecycle-Invarianten für Organization und School
  (permanente IDs, Archivierung vor Purge, Abhängigkeitsvorschau bei Tenant-Löschung) und benennt die
  manuell duplizierten reservierten Slugs sowie die am Settings-Registry vorbeilaufenden
  School-JSON-Settings.

**Praktische Folge:** Ein Träger-Admin-Rollenmodell landet entweder vor dem Start der
Identity-Migration (#2721) oder erst danach auf der neuen Facade. Parallel gebaut kollidiert es mit
dem Architektur-Ratchet (`scripts/backend-architecture.sh check`, Pflicht-CI-Status „Backend
architecture ratchet"), der neue Keys blockiert und nur deren Entfernen erlaubt.

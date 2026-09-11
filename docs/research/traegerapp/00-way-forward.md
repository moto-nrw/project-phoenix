# Trägerapp: Way forward

Stand 2026-08-29. Synthese aus `01-code-bestand.md` (Code), `02-issues.md` (Tracker) und `03-markt.md` (Markt und Rechtsrahmen). Alle Aussagen dort belegt; hier nur die Schlüsse.

## 1. Was wir jetzt wissen

**Träger = `platform.organizations`.** Ja, im Modell ist der Träger die Org, die Schule der Tenant. Das Skelett für einen Org-Scope existiert (JWT `org_id`, `scope: "org"`, Org-Zweig in switch-tenant und in drei Account-Queries), aber kein Login stellt je ein Org-Token aus. Der Pfad ist tot, nie im Betrieb durchlaufen. Alles darüber (Rolle, Middleware, Endpoints, Frontend) fehlt.

**Die Entscheidung "Portal oder Rolle" ist nie gefallen.** #935 will eine Rolle plus Tenant-Switcher, #674 spricht von eigener UI, DEBATE D18 (Feb 2026) will einen eigenen read-only Mechanismus. Der Session-Cap-Code wirft org und tenant heute in einen Topf, also ging der erste Entwurf von "Rolle im Staff-Portal" aus.

**Belegte Nachfrage ist dünn.** Genau ein Träger-Wunsch mit Namen: Bissendorf (3 OGS) will das Zeiterfassungs-Aggregat über alle Standorte. Dazu Caritas (Anwesenheitsnachweis für Behörden, gelöst durch #861/#2606), Burbach (Multi-Schul-Umschalter, Bugfix #1975), Ferien-Pooling (#704). Karriereportal: null Treffer im Tracker, keine Nachfrage. Trägergespräche zur Abrechnung haben laut #2639 stattgefunden, die Ergebnisse stehen nirgends.

**Der Markt hat die Lücke nicht besetzt, aber einen ernsten Wettbewerber.** OGS Connect (do.it) hat Dienstplan mit Personalschlüssel-Logik, Personalakte mit Fristen, Zeiterfassung, Karriereportal und Kommunal-Auswertungen für Träger. AURORA (Haneke) ebenfalls Multi-Standort. Das reifste Träger-Dashboard kommt aus der Kita-Welt (Kitaversum: Ampel, Soll/Ist Personal, Springerpool). Große Träger (AWO, Caritas, Diakonie, DRK) haben mit Vivendi PEP ein tarifführendes Dienstplan- und Arbeitszeitkonten-System im Haus.

**Der rechtliche Hebel ist der Verwendungsnachweis.** Zuwendungsempfänger ist die Kommune, der Träger muss ihr belegen: Platzbelegung zum Stichtag 15.10., sonderpädagogischer Bedarf (2.054 statt 1.138 Euro), Betreuungszeiten, Personaleinsatz, Frist 31.10. (BASS 11-02 Nr. 19). Das sind Daten, die moto ohnehin hat. Dazu: LVR/LWL bestätigen 2025 amtlich, dass es für OGS keine systematische Datenerhebung zu Personal und Qualifikation gibt.

## 2. Was ich an den drei Ideen kritisch sehe

| Idee | Befund | Urteil |
|---|---|---|
| Arbeitszeiterfassung über Standorte | Einzige belegte Nachfrage (Bissendorf). Kollidiert mit der DSGVO-Scope-Entscheidung in #1417: "kein aufrechenbares Total über mehrere Schulen". Personenscharf oder aggregiert ist ungeklärt. | Bauen, aber zuerst die #1417-Entscheidung neu treffen und begründen. Personenscharfer Export nur für die Rolle, die Lohn macht. |
| "Intelligente Personalverwaltung" (Dienstplan auf Trägerebene) | Steht in jeder Leitungs-Stellenanzeige. Aber: OGS Connect und AURORA haben es, Vivendi PEP ist der Tarifstandard. Ein zweiter Dienstplan neben dem tarifführenden System ist ein Ausschlusskriterium. | Nicht als Neubau. Ist-Daten aus moto gegen einen anderswo gepflegten Soll-Plan spiegeln (Ausfall, unbesetzte Gruppen). |
| Karriereportal | Keine Nachfrage im Tracker, kein Beleg im Markt ausser OGS Connect als Nebenmodul. | Streichen, bis ein Träger danach fragt. |

Was stattdessen die Evidenz trägt (Rangfolge aus `03-markt.md` Abschnitt 6):
1. Wer ist heute wo nicht da, was fällt aus (Personalausfall je Standort, betroffene Kinder)
2. Nachweisdaten für Kommune und Verwendungsnachweis (Stichtag, Belegung, Betreuungstage, Export)
3. Standortvergleich mit wenigen Kennzahlen (Auslastung, Betreuungsstunden je Kind, Personalstunden je Kind)
4. Fristenmonitor (Führungszeugnis 5 Jahre, §8a-Schulung)
5. Anmeldungen gegen Plätze (Rechtsanspruch ab 08/2026)

Die Trägerapp verkauft an die Bereichsleitung, nicht an die Betreuungskraft. Anwesenheit ist der Datenlieferant, nicht das Versprechen.

## 3. Architektur-Empfehlung

**Eigenes Portal, nach dem Schul-Portal-Muster (#2207), read-only Aggregat, Tagesgeschäft per Tenant-Switch.** Das ist D18 plus die Portal-Isolation, die wir seit dem Schul-Portal ohnehin fahren.

Warum Portal und nicht Rolle im Staff-Portal:
- Ein Träger-Nutzer ohne `tenant_id` passt nicht in `TenantTxMiddleware` (reject-Zweig) und nicht in RLS. Er braucht sowieso eine eigene geschützte Gruppe.
- Session-Isolation (eigener Cookie, eigener Host `traeger.{domain}`) ist mit dem Schul-Portal ein eingespieltes Muster mit rund 60 Dateien Blaupause und `portal-shell.tsx` als geteiltem Rahmen.
- Produktidentität: die Bereichsleitung soll nicht in der Betreuer-Oberfläche landen und sich durchklicken.

Was das kostet und wo die Risiken liegen:
- **Cross-Tenant-Reads ohne RLS-Netz.** D18 sagt WithAdminTx plus expliziter `organization_id`-Filter. Jede vergessene Bedingung ist ein Datenleck. Gegenmassnahme: ein einziger `OrgScopeService`, alle Org-Queries in einem Repository-Paket, plus ein Ratchet-Test, der jede Query dort auf den Org-Filter prüft. Die Alternative (zweite RLS-Policy-Familie über `app.current_org_id` auf 58+ Tabellen) wurde in D18 verworfen; ich würde sie als Entscheidungs-Ticket nochmal aufmachen, weil Ratchet-Tests schwächer sind als Postgres.
- **Account-zu-Träger-Zuordnung fehlt.** `auth.account_tenants` ist schulgebunden. Ableitung aus Schulen ist mehrdeutig (Account in zwei Trägern). Braucht eine eigene Tabelle oder Spalte, und genau die Tabellen ziehen gerade unter die Identity-Facade (#2721).
- **Org-Token und switch-tenant.** Heute lässt `TenantMiddleware` `org` durch, damit ein Org-Nutzer in eine Schule wechseln kann. Entweder Guard (Org-Token nur auf `/org/*`, dann kein Switch) oder Handoff wie beim Schul-Portal (Org-Login mintet bei Bedarf ein Tenant-Token für eine Schule). Zweiteres ist sauberer und passt zum Scope-Matrix-Test.

## 4. Timing: ja, warten. Und zwar länger als eine Woche, wenn der Refactor so läuft wie geplant

Der Backend-Refactor (#2580, #2659, #2660, #2721, #2708) zieht genau die Tabellen hinter Facades, die die Trägerapp braucht: `platform.organizations`, `platform.schools`, `auth.account_tenants`, `auth.roles`. Ein `organization_admins`-Modell mitten in diesen Cutover zu legen, erzeugt Konflikte und neue Ratchet-Keys, die der CI-Status "Backend architecture ratchet" blockiert. Also: **kein Trägerapp-Code, bevor #2659/#2660/#2721 durch sind.** Die Trägerapp landet dann auf den neuen Facades, nicht daneben.

Die Wartezeit ist kein Leerlauf. Die eigentliche Lücke ist nicht Code, sondern Wissen: wir haben kein einziges Träger-Interview, keine Antwort auf "wer rechnet ab" (#2639), keine auf "Vertrag je Schule oder je Org" (#1459), und wir wissen nicht, welches Personalsystem unsere Träger im Haus haben (#1417 Tranche 2c hat das schon als Research-Bedarf notiert).

## 5. Vorschlag: die nächsten zwei Wochen

**Woche 1 (parallel zum Refactor): Discovery, kein Code.**
1. Drei Gespräche: Bissendorf (3 OGS, hat den Wunsch geäussert), Burbach (Mehr-Schul-Admin), ein Träger mit mehr als 10 Standorten (Caritas?). Leitfrage: "Wie sieht Ihr Monat aus, was geben Sie an die Kommune, welches Personalsystem nutzen Sie." Fragebogen per `/to-questionnaire`.
2. OGS Connect live sehen (Demo anfragen). Die haben gebaut, was wir überlegen.
3. Ergebnisse der bisherigen Abrechnungsgespräche in #2639 nachtragen.

**Woche 2: Entscheidungen, per `/wayfinder`.** Der Nebel ist zu gross für ein einzelnes Grilling. Wayfinder legt Entscheidungs-Tickets an und löst sie nacheinander, Ergebnis sind Entscheidungen, keine Deliverables. Die Tickets, die ich sehe:

| Entscheidung | Optionen | Blockiert |
|---|---|---|
| Portal oder Rolle | eigenes Portal (Empfehlung) / Rolle im Staff-Portal | alles |
| Cross-Tenant-Lesepfad | WithAdminTx + OrgScopeService + Ratchet (D18) / zweite RLS-Policy-Familie | jedes Aggregat |
| Account-zu-Träger-Modell | eigene Tabelle / Spalte an account_roles / Ableitung | Login, Rolle |
| Org-Token vs switch-tenant | Handoff wie Schul-Portal / Guard | Middleware |
| Personenscharf oder aggregiert (#1417 revidieren) | nur Aggregate / personenscharf für Lohnrolle | Zeiterfassung |
| Wer rechnet ab, Vertrag je Schule oder Org (#2639, #1459) | aus Gesprächen | Export, Tarife |
| Wer legt Schulen an (#934 vs #936 Widerspruch) | Operator / Träger-Admin | Onboarding |
| Erste Kennzahlen des MVP | aus Gesprächen, Kandidaten Abschnitt 2 | Spec |
| Name für "Abrechnung" (drei Bedeutungen im Produkt) | Begriffsentscheid, ADR | UI |

Danach `/to-spec`, `/to-tickets`, `/implement` auf dem refaktorierten Backend.

## 6. MVP-Skizze (Tracer Bullet, erst nach den Entscheidungen)

Ein Nutzer loggt sich unter `traeger.{domain}` ein, sieht die Liste seiner Schulen und genau zwei Aggregate:
1. **Heute:** je Schule Personal anwesend/abwesend, unbesetzte Gruppen, Kinder anwesend.
2. **Stichtag:** je Schule aktiv betreute Kinder zum konfigurierbaren Stichtag, mit sonderpädagogischem Bedarf, als CSV. Das ist der Verwendungsnachweis-Rohstoff und dieselbe Zählweise, die #2791 für das Operator-Portal definieren muss. Eine Definition, zwei Konsumenten.

Klick auf eine Schule wechselt per Handoff ins Tenant-Portal (voller RLS-Schutz). Kein Schreiben aus dem Träger-Portal heraus. Zeiterfassungs-Aggregat und DATEV-Sammelexport als zweite Tranche, nachdem #1417 revidiert ist.

Bewusst nicht im MVP: Dienstplan, Karriereportal, Träger-Branding (#934 Rest), Ferien-Pooling (#704, eigener Cross-Tenant-Fall mit AVV-Pflicht), Elternbeiträge (Kommunalsache, nicht Trägersache).

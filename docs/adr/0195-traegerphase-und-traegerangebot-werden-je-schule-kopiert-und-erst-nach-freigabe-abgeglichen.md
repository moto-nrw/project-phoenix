---
status: proposed
---

# Trägerphase und Trägerangebot werden je Schule kopiert und erst nach Freigabe abgeglichen

Vorgeschlagen am 10.11.2026 zu [#4704](https://github.com/moto-nrw/project-phoenix/issues/4704),
Etappe 3 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 21 und 22 (Kunde Iserlohn). Status bleibt „Vorgeschlagen“, bis
der Product Owner freigibt
([ADR 0081](0081-owner-granularity-follows-invariants-not-source-size.md)).
Sie setzt die [Entscheidung vom 09.11.2026](https://github.com/moto-nrw/project-phoenix/issues/4704#issuecomment-6741872522)
für Option A um, baut auf
[ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md)
Abschnitt 2 und
[ADR 0105](0105-ferienzeitraum-liegt-beim-traeger-in-der-traeger-transaktion.md)
auf und führt einen neuen Workflow-Owner ein
([backend/CLAUDE.md](../../backend/CLAUDE.md#target-architecture) Punkt 4,
[ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)
Punkt 4, [ADR 0070](0070-principle-only-the-target-is-a-rule-and-debt-leaves-with-evidence.md)).

Das Träger-Büro pflegt eine Anmeldephase und ihre Betreuungsangebote einmal
in eigenen Tabellen des Trägers. Ein Workflow legt daraus je Schule eine
gewöhnliche Phase und gewöhnliche Angebote an, in der Tenant-Transaktion
dieser Schule und mit allen Prüfungen der Schule. Eine Änderung beim Träger
erreicht eine Schule erst, wenn das Büro sie nach einer Vorschau freigibt.

## Kontext

Stand `origin/development` @ [`81a974e`](https://github.com/moto-nrw/project-phoenix/tree/81a974e04cf582a7a3d8f9dc177030cc681a792d).

- `enrollment.phases` (Enrollment) und `enrollment.care_offerings` (Care Plan)
  tragen `tenant_id` und eine Tenant-Policy. Jedes Angebot hängt an einer
  Phase seiner Schule.
- Buchungen, Auswahlen, Änderungsanträge, Mitbuchungs-Regeln und
  Stundenplanquelle hängen über den Fremdschlüssel
  `(tenant_id, care_offering_id)` mit `ON DELETE RESTRICT` am Schulangebot.
  Die Datenbank garantiert so, dass keine Buchung auf das Angebot einer
  anderen Schule zeigt.
- Phase und Angebot haben Felder, die an die Schule gebunden sind:
  Formular, Planungszeitraum, Klassen und Klassenstufen der Phase,
  Stundenplanvorlage (`activity_group_id`), Verfügbarkeitsregeln und
  Mitbuchungs-Auslöser des Angebots. Care Plan prüft jedes Schreiben gegen
  diese Daten der Schule.
- Vertragsvorlagen, Vertragsleistungen und Tarife schlüsseln nach
  `(school_id, care_offering_id)`. Den Preis der Abrechnung hält
  `parent_billing.care_tariffs`; `care_offerings.price_cents` wird nur
  angezeigt. Vertragsversionen frieren `module_name` und die Leistungen ein.
- „Vorlage“ ist schon belegt: Stundenplanvorlage, Vertragsvorlage,
  Dokumentvorlage. Diese ADR nennt das Objekt des Trägers deshalb
  **Trägerphase** und **Trägerangebot**, die Zeile der Schule **Kopie**.

## Entscheidung

### 1. Owner

1. **Trägerphase:** Enrollment. Tabellen `enrollment.traeger_phases`,
   `enrollment.traeger_phase_versions`, `enrollment.traeger_phase_schools`.
2. **Trägerangebot:** Care Plan. Tabellen `enrollment.traeger_care_offerings`,
   `enrollment.traeger_care_offering_versions`,
   `enrollment.traeger_care_offering_schools`. Care Plan bekommt damit seine
   ersten Tabellen des Trägers. Die Bedeutung eines Angebots (Tage,
   Gehzeiten, Pflichtauswahl, Betreuungstag) bleibt bei dem Owner, der sie
   auch an der Schule prüft.
3. **Kopie:** bleibt die Zeile in `enrollment.phases` bzw.
   `enrollment.care_offerings` mit ihrem bisherigen Owner. Die Zuordnung zum
   Trägerobjekt sind Spalten dieser Zeile (Abschnitt 4).
4. **Abgleich:** ein neuer Owner `traeger-catalog-rollout` vom Typ
   `workflow` unter `workflows/traegercatalog`. Er besitzt keine Daten und
   schreibt kein SQL (ADR 0066 Punkt 5, ADR 0067 Punkt 4). Er hält
   Berechtigung, Schulliste, Vorschau-Fingerabdruck, Sperrreihenfolge und die
   Folge der Transaktionen.

Anlegen und Ändern einer Trägerphase oder eines Trägerangebots schreiben nur
ihren Owner in der Träger-Transaktion und brauchen keinen Workflow
(ADR 0105 Punkt 7). Nur Vorschau, Freigabe, erneutes Übertragen und
Zurückziehen laufen über den Workflow, weil sie in Schulen und bei zwei
Ownern schreiben.

### 2. Transaktionen und RLS

1. **Träger-Transaktion:** Die sechs Tabellen tragen `organization_id`, kein
   `tenant_id`, eine Policy `organization_id = app.current_org_id` und
   `FORCE ROW LEVEL SECURITY` (ADR 0105 Punkt 2). Keine Schule liest sie;
   es gibt keine Lese-Policy für die Schule.
2. **Schul-Transaktion:** Jede Kopie entsteht und ändert sich nur in der
   Tenant-Transaktion ihrer Schule über die Befehle von Enrollment und Care
   Plan, mit RLS der Zielschule.
3. **Keine Transaktion trägt beides.** Der Workflow öffnet abwechselnd die
   Träger-Transaktion und je Schule eine Tenant-Transaktion daneben, nie
   ineinander (ADR 0105 Punkt 2, ADR 0163 Punkt 3). Die Routen von Vorschau,
   Freigabe und Zurückziehen hängen deshalb ohne `withTx` in
   `ProtectedTraegerRoutes`; die Routen zum Pflegen der Trägerobjekte nehmen
   `withTx`.
4. **Keine schulübergreifende Zeile.** Keine Zeile trägt Daten mehrerer
   Schulen außer den Auswahl-Tabellen des Trägers, die je Zeile eine
   `school_id` nennen. Ein Trigger lässt dort nur eine nicht gelöschte
   Schule des Trägers zu, wie bei der ausrichtenden OGS (ADR 0105 Punkt 5).
   Die Spalte ist kein Fremdschlüssel auf `platform.schools`.
5. **Schulliste:** Der Workflow holt die Schulen über
   `organizationtenancy` (`ListTraegerSchoolIDs`) und prüft vor jeder
   Schul-Transaktion, dass die Schule noch zum Träger der Sitzung gehört.
6. **Wächter:** Das Postgres-Paket von Care Plan prüft vor jeder Anweisung
   auf seine Trägertabellen, dass es in der Träger-Transaktion genau dieses
   Trägers läuft, und nennt den Träger in jedem Filter (ADR 0105 Punkt 3).
   Seine Schultabellen lehnen die Träger-Transaktion ab (ADR 0175).
7. **Rechte:** Pflegen, Vorschau, Freigabe und Zurückziehen verlangen
   „Anmeldung verwalten“. Ohne das Recht antwortet die Route 403.

### 3. Ports und Fähigkeiten

Der Workflow deklariert seine Ports in `workflows/traegercatalog/ports`; die
Root bindet sie an die öffentlichen Fähigkeiten der Owner.

| Port des Workflows | gebunden an | Transaktion |
|---|---|---|
| `Schools` | Organisation & Tenancy, Schulen des Trägers | keine eigene |
| `PhaseTemplates`: Entwurf, Version, Schulauswahl, Ergebnis je Schule lesen und schreiben | Enrollment, neue Fähigkeit `TraegerPhases` | Träger |
| `OfferingTemplates`: dasselbe für Angebote | Care Plan, neue Fähigkeit `TraegerOfferings` | Träger |
| `SchoolPhases`: `PreviewTraegerPhase`, `ApplyTraegerPhase`, `WithdrawTraegerPhase` | Enrollment | Schule |
| `SchoolOfferings`: `PreviewTraegerOffering`, `ApplyTraegerOffering`, `WithdrawTraegerOffering` | Care Plan | Schule |
| `Runtime`: `WithinTraeger`, `WithinSchool`, Sperre im Träger | Transaction Runtime | beides, nie zugleich |

`Preview*` schreibt nichts. Es läuft durch dieselbe Prüfung wie `Apply*` und
gibt je Feld „neu“, „ändert sich“, „weicht ab, bleibt“ oder einen
Fehlercode zurück. `Apply*` erzeugt oder ändert die Kopie über die
vorhandenen Befehle (`CreateOffering`, `UpdateOffering`, Anlegen und Ändern
der Phase) und prüft dabei alles, was eine Änderung der Schule auch prüft.

### 4. Datenmodell (Skizze)

Beim Träger, je Owner gleich gebaut:

- **Trägerobjekt** (`traeger_phases`, `traeger_care_offerings`):
  `organization_id`, `id`, Entwurf der Felder, `released_version`
  (NULL vor der ersten Freigabe), `withdrawn_at`. Ein Trägerangebot nennt
  seine Trägerphase über `(organization_id, traeger_phase_id)` mit
  `ON DELETE RESTRICT`.
- **Version** (`*_versions`): `organization_id`, Objekt, `version`, die
  freigegebenen Felder, `released_at`, `released_by`. Eine Version ändert
  sich nie.
- **Schulauswahl** (`*_schools`): `organization_id`, Objekt, `school_id`,
  dazu das letzte Ergebnis je Schule: `applied_version`, `outcome`
  (`applied`, `rejected`, `stale`, `withdrawn`), `error_code`,
  `recorded_at`. Eine Trägerphase gilt für ausgewählte Schulen oder mit
  `all_schools` für alle, auch für später hinzukommende. Ein Trägerangebot
  gilt für alle Schulen seiner Phase oder eine Teilmenge davon.

An der Kopie (Schul-Transaktion):

- `enrollment.phases` und `enrollment.care_offerings` bekommen
  `traeger_source_id BIGINT NULL`, `traeger_source_version INT NULL` und
  `traeger_deviations TEXT[] NOT NULL DEFAULT '{}'`. Ein CHECK verlangt
  beide ersten Spalten zusammen und erlaubt in `traeger_deviations` nur die
  freigegebenen Felder.
- `traeger_source_id` ist kein Fremdschlüssel: Die Schule sieht die
  Trägertabellen nicht, und der Abgleich geht über den Workflow.
- Ein eindeutiger Index `(tenant_id, traeger_source_id)` an der Phase und
  `(tenant_id, phase_id, traeger_source_id)` am Angebot verhindert eine
  zweite Kopie nach einem Teilfehler.
- Der Fremdschlüssel `(tenant_id, care_offering_id)` aller Buchungen bleibt
  unverändert.

**Felder:**

| Gruppe | Phase | Angebot |
|---|---|---|
| Vom Träger, an der Kopie gesperrt | Name, Art, Leistungszeitraum, Zielgruppe, Übersetzungen | Name, Beschreibung, Übersetzungen, Wochentage, Ferienbetreuung, Mittagessen, Pflicht, Betreuungstag (`counts_as_care`), Auswahlgruppe und -regel, Reihenfolge |
| Vom Träger, Schule darf abweichen (vorläufig) | Anmeldefenster (`enrollment_open_at`, `enrollment_close_at`) | Kapazität, Gehzeiten (`pickup_times`) |
| Nur an der Schule | Formular, Planungszeitraum, Klassen, Klassenstufen, Folgephase | Stundenplanvorlage, Verfügbarkeitsregeln, Mitbuchungs-Auslöser, Lücken der Regeltermine, angezeigter Preis |

Ändert die Schule ein freigegebenes Feld, nimmt der Befehl es in
`traeger_deviations` auf. Setzt sie es zurück, entfernt er es und schreibt
den Wert der übernommenen Version zurück. Ein gesperrtes Feld lehnt Care
Plan bzw. Enrollment mit einem eigenen Fehlercode ab.

### 5. Ablauf

1. **Entwurf:** Das Büro ändert Trägerphase und Trägerangebote in der
   Träger-Transaktion. Keine Schule merkt etwas.
2. **Vorschau:** Der Workflow liest Entwurf und Schulauswahl in der
   Träger-Transaktion. Danach ruft er je Schule in deren Tenant-Transaktion
   `PreviewTraegerPhase` und für jedes Angebot `PreviewTraegerOffering` auf.
   Die Antwort zeigt je Schule „neu“, „übernimmt Änderungen“, „weicht ab“,
   „lehnt ab (Grund)“ oder „wird zurückgezogen“, dazu einen Fingerabdruck
   über Entwurf und den gelesenen Stand jeder Schule.
3. **Freigabe:** Das Büro bestätigt mit dem Fingerabdruck. Der Workflow
   sperrt die Trägerphase in der Träger-Transaktion, prüft, dass der Entwurf
   unverändert ist, und schreibt die neuen Versionen von Phase und Angeboten.
   Erst dann gilt die Version als freigegeben.
4. **Übertragen:** Je Schule eine eigene Tenant-Transaktion. Sie liest den
   Stand der Schule unter der Sperre ihrer Kopie neu und vergleicht ihn mit
   dem Fingerabdruck (ADR 0067 Punkt 5). Weicht er ab, ändert sie nichts
   (`stale`). Sonst schreibt sie Phase und alle Angebote dieser Schule
   zusammen oder gar nicht. Lehnt eine Prüfung ab, bleibt die Schule bei
   ihrer bisherigen Version (`rejected` mit Fehlercode).
5. **Ergebnis:** Danach schreibt der Workflow das Ergebnis je Schule in einer
   eigenen Träger-Transaktion in die Schulauswahl.
6. **Wiederholen:** Für Schulen mit `stale`, `rejected` oder ohne Ergebnis
   startet das Büro eine neue Vorschau und überträgt dieselbe Version
   erneut. Bricht ein Lauf ab, bleiben die übrigen Schulen bei ihrer Version;
   die Kopie selbst nennt die übernommene Version, deshalb erzeugt eine
   Wiederholung nichts doppelt.

Teilfehler über mehrere Schulen sind nicht atomar (ADR 0067). Der Lauf ist
je Schule atomar und zeigt jede Schule mit ihrem Ergebnis.

### 6. Abweichung und Konflikt

1. Ein Feld in `traeger_deviations` behält bei jedem Abgleich den Wert der
   Schule. Die Vorschau zeigt den neuen Wert des Trägers daneben.
2. Alle anderen freigegebenen und gesperrten Felder bekommen den Wert der
   Version.
3. Felder, die nur an der Schule stehen, berührt der Abgleich nie.
4. Passt ein neuer Wert nicht zu den Daten der Schule (etwa ein Wochentag
   ohne Gehzeit, eine Pflichtauswahl gegen die Stundenplanvorlage), lehnt
   die Prüfung der Schule ab. Der Abgleich setzt dann keine Abweichung, um
   die Ablehnung zu umgehen.
5. Eine neue Kopie übernimmt alle Werte der Version. Die Felder, die nur an
   der Schule stehen, setzt die Schule danach selbst. Ein Angebot, das ohne
   sie nicht gültig ist, entsteht inaktiv, und die Vorschau sagt das.

### 7. Zurückziehen und Löschen

1. Das Büro zieht ein Trägerangebot oder eine Trägerphase zurück oder nimmt
   eine Schule aus der Auswahl. Das läuft wie eine Freigabe mit Vorschau.
2. In der Schule löscht `Withdraw*` eine Kopie ohne Buchungen, Auswahlen
   und Anträge. Eine Kopie mit solchen Zeilen wird nur inaktiv
   (`is_active = false`); `ON DELETE RESTRICT` bleibt die letzte Grenze.
   Eine Phasenkopie mit Anmeldungen wird ebenso nur inaktiv.
3. Eine Schule löscht eine Kopie nicht selbst; sonst legte der nächste
   Abgleich sie neu an.
4. Ein Trägerobjekt mit Kopien bleibt als zurückgezogen stehen, samt
   Versionen. Gelöscht wird es erst, wenn keine Schule mehr eine Kopie davon
   meldet.
5. Wird eine Schule gelöscht
   ([ADR 0092](0092-mandanten-datenlebenszyklus-nutzt-einen-gemeinsamen-katalog.md)),
   gehen ihre Kopien mit ihr. Ihre
   `school_id` in den Auswahl-Tabellen bleibt als bloße ID, bis das Büro die
   Auswahl ändert; der Workflow überspringt sie (ADR 0105 Punkt 5).

### 8. Demo und Löschfristen

- Die Demo-Laufzeit hat keine Träger-Transaktion (ADR 0105 Punkt 2) und
  bekommt keine Trägerobjekte.
- Die sechs Tabellen enthalten keine Personendaten; `released_by` ist eine
  Kontoreferenz mit `ON DELETE SET NULL`. Sie gehen mit
  `platform.organizations` (`ON DELETE CASCADE`) und stehen als
  `notPersonal` in `retention_coverage_test.go` und im Abgleich von
  `demo_erasure_coverage_test.go`.
- Die neuen Spalten der Kopie gehen mit ihrer Zeile.
  `platform.erase_closed_demo_visitors` und `visitorTraceCounts` brauchen
  keinen neuen Schritt.
- Lesen und Übertragen berühren keine Personendaten und stehen nicht im
  Zugriffsprotokoll (ADR 0084 Abschnitt 7). Die Freigabe nennt `released_by`.

### 9. Abrechnung

- Vertragsvorlage, Vertragsleistungen, Tarife und Beitragsregeln zeigen
  weiter auf das Schulangebot `(school_id, care_offering_id)`, also auf die
  Kopie. Es gibt keinen Tarif und keine Vertragsvorlage am Trägerangebot.
- Ein Trägerangebot trägt keinen Preis für die Abrechnung. Ein neuer Preis
  ist ein neuer Tarif in Parent Billing.
- Name, Betreuungstag und Wochentage sind an der Kopie gesperrt, weil
  Vertragsversion und Monatsbeitrag sie lesen. Ihre Änderung beim Träger
  erreicht ausgestellte Verträge und Rechnungen nicht; diese bleiben
  eingefroren. Laufende Buchungen bleiben bestehen.
- Eine neue Kopie hat noch keinen Tarif und keine Vertragsvorlage. Die
  Vorschau zeigt das für jede Schule als Hinweis. Ob Tarif und
  Vertragsvorlage beim Übertragen mitentstehen, ist eine offene Frage
  (unten).

## Policy und Register

Die Umsetzung ändert diese Einträge im selben Diff wie den Code:

1. `backend/architecture/policy.json`: Owner `traeger-catalog-rollout` mit
   `kind: workflow`.
2. Neue Datei `backend/architecture/policy/traeger-catalog-rollout.json`:
   Pakete `workflows/traegercatalog` (`public`), `/ports`, `/internal/application`,
   `/compose`, `/http`. Regeln nur auf `public`- und `contract`-Rollen von
   `enrollment`, `care-plan` und `organization-tenancy`, dazu `tenant-runtime`
   für `compose` und `http`, `identity-access` `http` für das Recht. Keine
   Regel von einem Fach-Owner auf den Workflow (ADR 0058, ADR 0071, ADR 0099).
3. `policy/enrollment.json`: `data_objects` für die drei Tabellen der
   Trägerphase mit `write_owner: enrollment`.
4. `policy/care-plan.json`: `data_objects` für die drei Tabellen des
   Trägerangebots mit `write_owner: care-plan`. Ob die Träger-Routen von
   Care Plan mit den bestehenden `care-plan.http.*`-Regeln auskommen, zeigt
   `scripts/backend-architecture.sh check`; jede zusätzliche Regel bleibt
   im eigenen Owner oder zeigt auf eine `public`-Rolle.
5. `backend/test/transaction_boundaries.json` und
   `backend/test/traeger_transaction.go`: die `WithinTraeger`-Stellen des
   Workflows (Entwurf und Auswahl lesen, Freigabe unter Sperre, Ergebnis
   schreiben) mit ihren Gründen. Die Pflege-Routen brauchen keine Stelle
   (ADR 0105 „Folgen“).
6. `rls_organization_coverage_test.go`: die sechs Tabellen.
7. Route-Golden, Analytics-Guard und Scope-Matrix für jede neue Route
   (ADR 0084 Abschnitt 8 Punkt 8).
8. Seed-Daten: Der Demo-Träger des Seeders bekommt eine Trägerphase mit
   einem Trägerangebot, übertragen an zwei Schulen
   (`TestSeedCoverageRatchet`).
9. `CONTEXT.md`: Einträge für Trägerphase, Trägerangebot und Kopie, sobald
   diese ADR angenommen ist.

## Verworfene Option B: gemeinsame Zeile mit `organization_id`

- Trägt `enrollment.care_offerings` Zeilen von Schule und Träger, ist das die
  zweite Policy-Familie auf einer Schultabelle, die ADR 0084 verwirft.
  PostgreSQL verknüpft permissive Policies mit OR: Ein Fehler in der
  Lese-Policy für die Schule öffnet die Angebote eines fremden Trägers für
  Schulsitzung, Eltern-App und öffentliches Anmeldeformular.
- Die Felder, die an die Schule gebunden sind, brauchen trotzdem eine Zeile
  je Schule. B wäre also A plus eine Ebene, die live erbt.
- Die Fremdschlüssel `(tenant_id, care_offering_id)` von Buchungen,
  Auswahlen, Anträgen, Mitbuchungs-Regeln und Stundenplanquelle könnten nicht
  mehr gelten. Die Garantie „Buchung nur zu Angeboten der eigenen Schule“
  wanderte von der Datenbank in den Code; 17 Produktionsdateien lesen die
  Tabelle.
- Eine Änderung des Trägers wirkte sofort in allen Schulen, auch in offenen
  Anmeldeformularen und laufenden Buchungen, ohne Prüfung je Schule, weil die
  Träger-Transaktion keine Schultabelle sieht.
- Sie bräuchte eine neue ADR gegen ADR 0084 und Datenmigrationen.

Ebenfalls verworfen:

- **Abgleich ohne Freigabe** (jede Änderung sofort an alle Schulen ohne
  Abweichung): Eine Änderung wirkte still in offenen Anmeldungen; ein Fehler
  an einer Schule wäre erst später sichtbar.
- **Workflow-Tabelle für Ergebnisse:** Ein Workflow besitzt keine Daten
  (ADR 0066 Punkt 5). Das Ergebnis steht deshalb in der Schulauswahl des
  jeweiligen Owners.
- **Trägerangebot bei Enrollment:** Enrollment müsste die Bedeutung eines
  Angebots doppelt kennen, die Care Plan an der Schule prüft.

## Offene Fragen an den Product Owner

1. Welche Felder darf eine Schule abweichen lassen? Vorschlag: Kapazität und
   Gehzeiten am Angebot, Anmeldefenster an der Phase. Meint „Zeiten“ auch
   die Wochentage?
2. Sollen Tarif und Vertragsvorlage einer neuen Kopie beim Übertragen aus
   einem Wert des Trägers mitentstehen, oder legt das Büro sie danach je
   Schule an?
3. Darf eine Schule eine Kopie selbst deaktivieren, oder nur das Büro?
4. Wechselt eine Schule den Träger: Werden ihre Kopien dann zu eigenen
   Angeboten der Schule mit freien Feldern?

## Folgen

- Kein Code des Workflows, bevor diese ADR angenommen ist und der Owner in
  `policy.json` steht.
- Die Kopie ist ein gewöhnliches Schulangebot. Anmeldeformular, Eltern-App,
  Buchung, Vertrag, Abrechnung und `traeger-view` lesen sie ohne Änderung.
- Care Plan und Enrollment wachsen je um eine Fähigkeit für den Träger und
  drei Befehle für die Schule. Ihre bestehenden Befehle bekommen die Sperre
  der Trägerfelder und die Pflege von `traeger_deviations`.
- Die Oberfläche des Träger-Portals zeigt je Schule das letzte Ergebnis mit
  Fehlergrund. Hilfe und Screenshots folgen mit der Umsetzung.

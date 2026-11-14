---
status: accepted
---

# Trägerphase und Trägerangebot werden je Schule kopiert und erst nach Freigabe abgeglichen

**Status: Akzeptiert.** Freigegeben von Yannick Wenger am 11.11.2026
([Antworten auf die offenen Fragen](https://github.com/moto-ogs/project-phoenix/issues/4704#issuecomment-6799155255)).

Vorgeschlagen am 10.11.2026 zu [#4704](https://github.com/moto-ogs/project-phoenix/issues/4704),
Etappe 3 der Spec [#4611](https://github.com/moto-ogs/project-phoenix/issues/4611),
User Stories 21 und 22 (Kunde Iserlohn). Sie setzt die
[Entscheidung vom 09.11.2026](https://github.com/moto-ogs/project-phoenix/issues/4704#issuecomment-6741872522)
für Option A um, baut auf
[ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md)
Abschnitt 2 und
[ADR 0105](0105-ferienzeitraum-liegt-beim-traeger-in-der-traeger-transaktion.md)
auf und führt einen neuen Workflow-Owner ein
([backend/CLAUDE.md](../../backend/CLAUDE.md#target-architecture) Punkt 4,
[ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)
Punkt 4, [ADR 0070](0070-principle-only-the-target-is-a-rule-and-debt-leaves-with-evidence.md)).

Das Träger-Büro pflegt eine Anmeldephase und ihre Betreuungsangebote einmal
in eigenen Tabellen des Trägers, dazu Preis und Vertragsvorlage jedes
Angebots. Ein Workflow legt daraus je Schule eine gewöhnliche Phase und
gewöhnliche Angebote an, in der Tenant-Transaktion dieser Schule und mit allen
Prüfungen der Schule, und dazu Tarif und Vertragsvorlage der Schule. Eine
Änderung beim Träger erreicht eine Schule erst, wenn das Büro sie nach einer
Vorschau freigibt. Die Schule darf an ihrer Kopie alles ändern; ein Abgleich
behält ihre Änderungen.

## Kontext

Stand `origin/development` @ [`81a974e`](https://github.com/moto-ogs/project-phoenix/tree/81a974e04cf582a7a3d8f9dc177030cc681a792d).

- `enrollment.phases` (Enrollment) und `enrollment.care_offerings` (Care Plan)
  tragen `tenant_id` und eine Tenant-Policy. Jedes Angebot hängt an einer
  Phase seiner Schule.
- Buchungen und Auswahlen hängen über den Fremdschlüssel
  `(tenant_id, care_offering_id)` mit `ON DELETE RESTRICT` am Schulangebot.
  Die Datenbank garantiert so, dass keine Buchung auf das Angebot einer
  anderen Schule zeigt.
- Phase und Angebot haben Felder, die an die Schule gebunden sind:
  Formular, Planungszeitraum, Klassen und Klassenstufen der Phase,
  Stundenplanvorlage (`activity_group_id`), Verfügbarkeitsregeln und
  Mitbuchungs-Auslöser des Angebots. Care Plan prüft jedes Schreiben gegen
  diese Daten der Schule.
- Vertragsvorlagen (`care_contract.contract_templates`) und Tarife
  (`parent_billing.care_tariffs` mit Kopf `care_price_lists`) schlüsseln nach
  `(school_id, care_offering_id)` und liegen beim Träger. Ein Tarif gilt ab
  dem 1. eines Monats bis zum nächsten Preis
  ([ADR 0158](0158-monatsbeitrag-entsteht-aus-der-wirksamen-vertragsfassung-und-den-tarifen-des-traegers.md)
  Punkt 1); ein abgerechneter Monat behält seinen Preis (Punkt 6).
  `care_offerings.price_cents` wird nur angezeigt. Vertragsversionen frieren
  `module_name` und die Leistungen ein.
- Ein Trägerwechsel einer Schule passiert heute nur über die Operator-Route
  `PUT /api/operator/schools/{id}` (`Provisioning.UpdateSchool` in
  Organisation & Tenancy, Admin-Transaktion), die `platform.schools.organization_id`
  ändert. Kein Owner erfährt davon.
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
   auch an der Schule prüft. Preis und Vertragsvorlage stehen als Werte am
   Trägerangebot; Tarif und Vertragsvorlage der Schule bleiben bei Parent
   Billing und Care Contract.
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
(ADR 0105 Punkt 7). Nur Vorschau, Freigabe mit Übertragen und Zurückziehen
laufen über den Workflow, weil sie in Schulen und bei mehreren Ownern
schreiben.

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
   `organizationtenancy` (`ListTraegerSchoolIDs`) und schreibt nur in
   Schulen dieser Liste. Eine Schule der Auswahl, die nicht mehr zum Träger
   gehört, überspringt er.
6. **Wächter:** Die Postgres-Pakete von Enrollment und Care Plan prüfen vor
   jeder Anweisung auf ihre Trägertabellen, dass sie in der
   Träger-Transaktion genau dieses Trägers laufen, und nennen den Träger in
   jedem Filter (ADR 0105 Punkt 3). Ihre Befehle an der Kopie verlangen die
   Tenant-Transaktion einer Schule.
7. **Rechte:** Pflegen, Vorschau, Freigabe und Zurückziehen verlangen
   „Anmeldung verwalten“ (`traeger_enrollment:manage`). Ohne das Recht
   antwortet die Route 403.

### 3. Ports und Fähigkeiten

Der Workflow deklariert seine Ports in `workflows/traegercatalog/ports`; die
Root bindet sie an die öffentlichen Fähigkeiten der Owner.

| Port des Workflows | gebunden an | Transaktion |
|---|---|---|
| `Schools` | Organisation & Tenancy, `ListTraegerSchoolIDs` | keine eigene |
| `PhaseTemplates`: Entwurf, Version, Schulauswahl, Ergebnis je Schule | Enrollment, neue Fähigkeit `TraegerPhases` | Träger |
| `OfferingTemplates`: dasselbe für Angebote | Care Plan, neue Fähigkeit `TraegerOfferings` | Träger |
| `SchoolPhases`: `CopyState`, `Apply`, `Withdraw` | Enrollment, neue Fähigkeit `TraegerPhaseCopies` | Schule |
| `SchoolOfferings`: `CopyStates`, `ApplyTraegerOfferings` (Stapel mit Rücknahmen) | Care Plan, neue Fähigkeit `TraegerOfferingCopies` | Schule |
| `Tariffs`: Preise aller geänderten Module einer Schule ab ihrem Monat | Parent Billing, neue Methode `SaveCareModulePrices` | Träger |
| `ContractTemplates`: Vertragsvorlagen einer Schule | Care Contract, `Intake.PrepareTemplateBatch` | Vorbereitung in Schule, Schreiben in gemeinsamer Träger-Transaktion |
| `Runtime`: `WithinExclusive`, `WithinTraeger`, `WithinSchool` | Tenant Runtime | exklusive Verbindung ohne Transaktion; Einheiten nacheinander |

**Vorschau:** Der Workflow ruft in der Tenant-Transaktion der Schule dieselben
`Apply`-Befehle auf wie beim Übertragen und verwirft die Transaktion danach.
So prüft die Vorschau genau das, was das Übertragen prüft. `Apply` gibt je
Feld „neu“, „ändert sich“, „weicht ab, bleibt“ oder einen Fehlercode
zurück. `Apply` erzeugt oder ändert die Kopie über die vorhandenen Befehle
(`CreatePhase`/`UpdatePhase`, `ApplyTraegerOfferings`) und prüft
dabei alles, was eine Änderung der Schule auch prüft. Der Angebotskatalog
führt den Stapel im Speicher zusammen und prüft Auswahlgruppen gegen diesen
Endzustand. Er lädt Prüfgrundlagen und schreibt Kopien, Herkunft und
Auslöser gesammelt; die Sperre nimmt er einmal je Schule. Abgeleitete
Vorlagen und Kinder werden vor dem Abgleich dedupliziert.

`SaveCareModulePrices` ersetzt die Preise eines ganzen Modulstapels einer Schule
ab dem jeweiligen Monat und lässt die übrigen Module der Preisliste stehen.
Es lädt Tarifstand und Abrechnungsbelege einmal, prüft den gemeinsamen Endstand
und schreibt die betroffenen Monatslisten gesammelt, ohne eine ungenutzte
Tarifansicht aufzubauen. Es nimmt
dieselbe Sperre und dieselbe Prüfung abgerechneter Monate wie
`SaveCarePriceList` (ADR 0158 Punkt 6).

### 4. Datenmodell

Beim Träger, je Owner gleich gebaut:

- **Trägerobjekt** (`traeger_phases`, `traeger_care_offerings`):
  `organization_id`, `id`, Entwurf der Felder als JSON, `released_version`
  (NULL vor der ersten Freigabe), `withdrawn_at`. Ein Trägerangebot nennt
  seine Trägerphase über `(organization_id, traeger_phase_id)` ohne
  Kaskade (`NO ACTION`, damit die Kaskade des Trägers beide nimmt).
- **Version** (`*_versions`): `organization_id`, Objekt, `version`, die
  freigegebenen Felder als JSON, `released_at`, `released_by`. Eine Version
  ändert sich nie; ein Trigger lehnt jede Änderung ab.
- **Schulauswahl** (`*_schools`): `organization_id`, Objekt, `school_id`,
  dazu das letzte Ergebnis je Schule: `applied_version`, am Angebot
  `applied_copy_id` (die damit abgerechnete Schulkopie), `outcome`
  (`applied`, `rejected`, `stale`, `withdrawn`), `error_code`,
  `recorded_at`. Eine Trägerphase gilt für ausgewählte Schulen oder mit
  `all_schools` für alle, auch für später hinzukommende. Ein Trägerangebot
  gilt für alle Schulen seiner Phase.

An der Kopie (Schul-Transaktion):

- `enrollment.phases` und `enrollment.care_offerings` bekommen
  `traeger_source_id BIGINT NULL`, `traeger_source_version INT NULL`,
  `traeger_values JSONB NULL` (die Werte der übernommenen Version, je
  Spalte) und `traeger_deviations TEXT[] NOT NULL DEFAULT '{}'`. Ein CHECK
  verlangt die ersten drei Spalten zusammen und erlaubt in `traeger_values`
  und `traeger_deviations` nur die Felder des Trägers.
- Ein Trigger berechnet `traeger_deviations` bei jedem Schreiben aus der
  Zeile: Ein Feld weicht ab, wenn sein Wert nicht dem Wert in
  `traeger_values` gleicht. Damit zählt jeder Schreibweg der Schule, auch
  das Deaktivieren, und eine Änderung zurück auf den Wert des Trägers
  entfernt die Abweichung wieder.
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
| Vom Träger, an der Kopie änderbar (jede Änderung ist eine Abweichung) | Name, Art, Leistungszeitraum, Anmeldefenster, Zielgruppe, Überbuchung, Auswahlregel der Angebote, aktiv | Name, Beschreibung, Wochentage und Wahlmodus, Ferienbetreuung, Mittagessen, Kapazität, angezeigter Preis, aktiv, Pflicht, Betreuungstag (`counts_as_care`), Auswahlgruppe und -regel, Reihenfolge, Gehzeiten (`pickup_times`) |
| Nur beim Träger, beim Übertragen an Parent Billing und Care Contract | – | Preise (Wochentag oder Staffel nach Tagen je Woche, Geschwisterpreis, zzgl. Essen), Vertragsvorlage |
| Nur an der Schule | Formular, Planungszeitraum, Klassen, Klassenstufen, Folgephase, Übersetzungen | Stundenplanvorlage, Verfügbarkeitsregeln, Mitbuchungs-Auslöser, Lücken der Regeltermine, Übersetzungen |

Es gibt keine gesperrten Felder. Die OGS-Leitung darf an der Kopie alles
ändern, was der Träger setzt; die Änderung landet in `traeger_deviations`.
Löschen darf die Schule eine Kopie nicht (Abschnitt 7); Deaktivieren ist eine
Abweichung des Felds `is_active`.

### 5. Ablauf

1. **Entwurf:** Das Büro ändert Trägerphase und Trägerangebote in der
   Träger-Transaktion. Keine Schule merkt etwas.
2. **Vorschau:** Der Workflow liest Entwurf und Schulauswahl in der
   Träger-Transaktion. Danach führt er je Schule in deren Tenant-Transaktion
   `Apply` für die Phase und alle Angebote aus und verwirft die
   Transaktion. Die Antwort zeigt je Schule „neu“, „übernimmt Änderungen“,
   „weicht ab“, „lehnt ab (Grund)“ oder „wird zurückgezogen“, dazu je Angebot
   den neuen Tarif ab Monat und ob eine Vertragsvorlage entsteht, und einen
   Fingerabdruck über Entwurf, Schulliste, den Kalendermonat der
   Tarifberechnung, die angewandten Vergleichsversionen der Abrechnung
   und den gelesenen Stand jeder Schule. Nach einem
   Monatswechsel ist eine neue Vorschau nötig.
3. **Freigabe:** Das Büro bestätigt mit dem Fingerabdruck. Der Workflow
   sperrt die Trägerphase in der Träger-Transaktion (`FOR UPDATE`), prüft,
   dass Entwurf und Schulliste unverändert sind, und schreibt neue Versionen
   von Phase und Angeboten, deren Entwurf sich seit der letzten Version
   geändert hat. Erst dann gilt die Version als freigegeben.
4. **Übertragen:** Je Schule eine eigene Tenant-Transaktion. Sie sperrt die
   Kopie der Phase, liest den Stand der Schule neu und vergleicht ihn mit dem
   Stand aus der Vorschau (ADR 0067 Punkt 5). Weicht er ab, ändert sie
   nichts (`stale`). Sonst schreibt sie Phase und alle Angebote dieser Schule
   zusammen oder gar nicht. Lehnt eine Prüfung ab, bleibt die Schule bei
   ihrer bisherigen Version (`rejected` mit Fehlercode).
5. **Tarif und Vertragsvorlage:** Nach der Schul-Transaktion schreibt der
   Workflow für jedes neue Angebot und jedes Angebot mit geändertem Preis
   die Preise der Module (`SaveCareModulePrices`), die vorbereiteten
   Vertragsvorlagen und deren angewandte Versionen in **einer gemeinsamen
   Träger-Transaktion je Schule**. Die Vorbereitung prüft die Vorlagen und
   die Modulzuordnung noch in der Schul-Transaktion. Schlägt ein
   Finanzschritt oder das Vermerken seines Standes fehl, wird die ganze
   Finanz-Transaktion verworfen. Die Tarifdaten verwenden den einmal
   bestätigten Kalendermonat auch bei einem Monatswechsel während des Laufs.
6. **Ergebnis:** Der erfolgreiche Finanzschritt schreibt `applied` und die
   Vergleichsversionen in derselben Transaktion wie Tarif und Vertragsvorlage.
   Fehler späterer Schulen oder der abschließenden Ergebnisaufzeichnung
   können diesen Stand nicht verlieren. Zuletzt schreibt der Workflow alle
   Ergebnisse gesammelt; vor jedem Kopieren sind die versuchten Schulen
   bereits als Wiederaufnahmeziele vermerkt.
7. **Wiederholen:** Für Schulen mit `stale`, `rejected` oder ohne Ergebnis
   startet das Büro eine neue Vorschau und überträgt dieselbe Version
   erneut. Bricht ein Lauf ab, bleiben die übrigen Schulen bei ihrer Version;
   die Kopie selbst nennt die übernommene Version, deshalb erzeugt eine
   Wiederholung keine zweite Kopie. Tarif und Vertragsvorlage vergleicht der
   Workflow mit der zuletzt als `applied` gemeldeten Version und schreibt sie
   bei einer Wiederholung mit demselben Ergebnis noch einmal.

Teilfehler über mehrere Schulen sind nicht atomar (ADR 0067). Kopien und
Finanzdaten bilden je Schule zwei atomare Einheiten; jede Schule zeigt ihr
Ergebnis. Zwischen Kopie und
Tarif einer Schule liegen zwei Transaktionen; bricht der Lauf dazwischen ab,
steht die Schule nicht auf `applied`, und die Wiederholung schreibt Tarif und
Vertragsvorlage nach.

**Sperrreihenfolge:** Eine verbindungsgebundene Advisory-Sperre je
Trägerphase umfasst Freigabe, alle Schulen, ihre Finanzdaten und die letzte
Ergebnisaufzeichnung. Sie hält keine Transaktion offen. So kann kein älterer
Lauf nach einem neueren Lauf Tarife oder Vergleichsversionen zurückschreiben.
Die Runtime reserviert Poolkapazität für die nacheinander geöffneten Einheiten
und gibt die Verbindung bei Erfolg, Fehler und Abbruch frei. Innerhalb des
Laufs: Trägerphase (Träger-Transaktion, nur während der Freigabe) →
Wiederholungssperre → Phasenkopie → Angebotskopien nach `id`
(Schul-Transaktion). Die Freigabe endet vor der Schul-Transaktion; beide
Transaktionen laufen nie zugleich.

### 6. Abweichung und Konflikt

1. Ein Feld in `traeger_deviations` behält bei jedem Abgleich den Wert der
   Schule. Die Vorschau zeigt den neuen Wert des Trägers daneben.
2. Alle anderen Felder des Trägers bekommen den Wert der Version.
3. Felder, die nur an der Schule stehen, berührt der Abgleich nie.
4. Passt ein neuer Wert nicht zu den Daten der Schule (etwa ein Wochentag
   ohne Gehzeit, eine Pflichtauswahl gegen die Stundenplanvorlage), lehnt
   die Prüfung der Schule ab. Der Abgleich setzt dann keine Abweichung, um
   die Ablehnung zu umgehen.
5. Eine neue Kopie übernimmt alle Werte der Version. Die Felder, die nur an
   der Schule stehen, setzt die Schule danach selbst.
6. Tarif und Vertragsvorlage der Schule pflegt das Büro im Bereich
   Abrechnung bzw. Verträge weiter von Hand. Ein Abgleich schreibt sie nur,
   wenn sich Preis bzw. Vertragsvorlage am Trägerangebot geändert haben.
   Eine neu angelegte Kopie bekommt ihre eigenen Konditionen auch dann,
   wenn nach einem Trägerwechsel noch alte Ergebniszeilen bestehen.
   `applied_copy_id` verbindet den Finanzstand mit genau dieser Kopie;
   Version und Kopie werden zusammen mit den Konditionen gespeichert.
   Solange die aktuelle Kopie nicht dieser ID entspricht, bleibt auch nach
   einem Teilfehler die Initialisierung in Vorschau und Wiederholung nötig.
   Veröffentlichte Preise und Vertragsvorlagen können im Katalog ersetzt,
   aber nicht ersatzlos entfernt werden. Pflegen und Freigeben lehnen eine
   solche Entfernung ab; das Zurückziehen des Angebots bleibt möglich.
   Ein geänderter Preis wird ein neuer Tarif ab dem 1. des Folgemonats, ein
   erster Preis gilt ab dem Monat des Leistungsbeginns, frühestens ab dem
   laufenden Monat. Ausgestellte Verträge und Rechnungen bleiben
   eingefroren; einen abgerechneten Monat ändert der neue Tarif nicht
   (ADR 0158 Punkt 6).

### 7. Zurückziehen und Löschen

1. Das Büro zieht ein Trägerangebot oder eine Trägerphase zurück oder nimmt
   eine Schule aus der Auswahl. Das läuft wie eine Freigabe mit Vorschau.
2. In der Schule löscht `Withdraw` eine Kopie ohne Buchungen und Auswahlen.
   Eine Kopie mit solchen Zeilen wird inaktiv (`is_active = false`) und
   verliert ihren Trägerbezug; sie bleibt ein gewöhnliches, inaktives
   Angebot der Schule. `ON DELETE RESTRICT` bleibt die letzte Grenze. Eine
   Phasenkopie mit Anmeldungen wird ebenso nur inaktiv.
3. Eine Schule löscht eine Kopie nicht selbst (Fehlercode
   `enrollment.traeger_copy_delete`); sonst legte der nächste Abgleich sie
   neu an. Deaktivieren darf sie.
4. Ein zurückgezogenes Trägerobjekt bleibt mit seinen Versionen als
   zurückgezogen stehen.
5. Wird eine Schule gelöscht
   ([ADR 0092](0092-mandanten-datenlebenszyklus-nutzt-einen-gemeinsamen-katalog.md)),
   gehen ihre Kopien mit ihr. Ihre `school_id` in den Auswahl-Tabellen bleibt
   als bloße ID, bis das Büro die Auswahl ändert; der Workflow überspringt
   sie (ADR 0105 Punkt 5).

### 8. Trägerwechsel einer Schule

Wechselt eine Schule über `PUT /api/operator/schools/{id}` den Träger,
werden ihre Kopien gewöhnliche Angebote und Phasen der Schule. Ein Trigger
`AFTER UPDATE OF organization_id` auf `platform.schools` setzt in derselben
Admin-Transaktion `traeger_source_id`, `traeger_source_version` und
`traeger_values` der Kopien dieser Schule auf NULL; der Abweichungs-Trigger
leert `traeger_deviations`. So erreicht die Änderung jeden Weg, der die
Spalte ändert, ohne dass Organisation & Tenancy von Enrollment oder Care
Plan abhängt. Die Zeilen des alten Trägers (Auswahl, Tarife,
Vertragsvorlagen) bleiben bei ihm; der Workflow überspringt die Schule, weil
sie nicht mehr in seiner Schulliste steht.

### 9. Demo und Löschfristen

- Die Demo-Laufzeit hat keine Träger-Transaktion (ADR 0105 Punkt 2) und
  bekommt keine Trägerobjekte.
- Die sechs Tabellen enthalten keine Personendaten; `released_by` ist eine
  Kontoreferenz mit `ON DELETE SET NULL`. Sie gehen mit
  `platform.organizations` (`ON DELETE CASCADE`) und stehen als
  `notPersonal` in `retention_coverage_test.go`.
- Die neuen Spalten der Kopie gehen mit ihrer Zeile.
  `platform.erase_closed_demo_visitors` und `visitorTraceCounts` brauchen
  keinen neuen Schritt.
- Lesen und Übertragen berühren keine Personendaten und stehen nicht im
  Zugriffsprotokoll (ADR 0084 Abschnitt 7). Die Freigabe nennt `released_by`.

### 10. Abrechnung

- Vertragsvorlage, Vertragsleistungen, Tarife und Beitragsregeln zeigen
  weiter auf das Schulangebot `(school_id, care_offering_id)`, also auf die
  Kopie.
- Preis und Vertragsvorlage stehen am Trägerangebot und entstehen beim
  Übertragen je Schule mit (Abschnitt 5 Punkt 5). Die Preisform folgt
  `parent_billing.care_tariffs`: je Wochentag oder als Staffel nach Tagen je
  Woche, mit Preis für jedes weitere Kind und dem Kennzeichen „zzgl. Essen“.
  Damit lassen sich beide Preislisten des SJR abbilden.
- Ändert der Träger Name, Betreuungstag oder Wochentage, erreicht das
  ausgestellte Verträge und Rechnungen nicht; diese bleiben eingefroren.
  Laufende Buchungen bleiben bestehen.
- Das ausgefüllte Anmeldeformular bleibt Grundlage des Vertrags, den Eltern
  digital unterschreiben (ADR 0153, ADR 0155); diese ADR ändert daran nichts.

## Policy und Register

Die Umsetzung ändert diese Einträge im selben Diff wie den Code:

1. `backend/architecture/policy.json`: Owner `traeger-catalog-rollout` mit
   `kind: workflow`.
2. Neue Datei `backend/architecture/policy/traeger-catalog-rollout.json`:
   Pakete `workflows/traegercatalog` (`public`), `/ports`, `/internal/application`,
   `/compose`, `/http`. Regeln nur auf `public`- und `contract`-Rollen von
   `enrollment`, `care-plan`, `parent-billing`, `care-contract` und
   `organization-tenancy`, dazu `tenant-runtime` für `compose` und `http`.
   Keine Regel von einem Fach-Owner auf den Workflow (ADR 0058, ADR 0071,
   ADR 0099).
3. `policy/enrollment.json`: `data_objects` für die drei Tabellen der
   Trägerphase mit `write_owner: enrollment`.
4. `policy/care-plan.json`: `data_objects` für die drei Tabellen des
   Trägerangebots mit `write_owner: care-plan`.
5. `backend/test/transaction_boundaries.json` und
   `backend/test/traeger_transaction.go`: Stellen außerhalb des Workflows,
   falls ein Owner selbst `WithinTraeger` öffnet. Der Workflow selbst steht
   nicht im Register (Workflow-Owner, ADR 0105 „Folgen“).
6. `rls_organization_coverage_test.go` findet die sechs Tabellen selbst; ein
   eigener Test belegt Policy und Trigger.
7. Route-Golden, Analytics-Guard und Scope-Matrix für jede neue Route
   (ADR 0084 Abschnitt 8 Punkt 8).
8. Seed-Daten: Der Demo-Träger des Seeders bekommt eine Trägerphase mit
   einem Trägerangebot, übertragen an zwei Schulen
   (`TestSeedCoverageRatchet`).
9. `CONTEXT.md`: Einträge für Trägerphase, Trägerangebot und Kopie.
10. Rückgaberegister (ADR 0201), Datenlebenszyklus-Katalog und
    `retention_coverage_test.go` für die sechs Tabellen und die neuen
    Spalten.

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
- **Gesperrte Felder an der Kopie:** Iserlohn hat keinen festen
  Ansprechpartner; jede Sperre bräuchte einen Weg zurück zum Büro. Die
  Abweichung ist sichtbar und bleibt erhalten; das reicht.
- **Abweichung von Hand pflegen** (jeder Befehl der Schule trägt sie ein):
  Jeder neue Schreibweg müsste daran denken. Der Trigger rechnet sie aus der
  Zeile.
- **Workflow-Tabelle für Ergebnisse:** Ein Workflow besitzt keine Daten
  (ADR 0066 Punkt 5). Das Ergebnis steht deshalb in der Schulauswahl des
  jeweiligen Owners.
- **Trägerangebot bei Enrollment:** Enrollment müsste die Bedeutung eines
  Angebots doppelt kennen, die Care Plan an der Schule prüft.
- **Trägerwechsel über einen Port aus Organisation & Tenancy:** Ein Befehl in
  der Admin-Transaktion müsste Schultabellen zweier Owner schreiben; der
  Trigger erreicht dasselbe ohne neue Abhängigkeit.

## Antworten des Product Owners (11.11.2026)

1. **Abweichende Felder:** Die OGS-Leitung darf an der Kopie alles ändern,
   was auch der Träger ändern darf. Es gibt keine gesperrten Felder. Jede
   Änderung landet in `traeger_deviations`, ein Abgleich überschreibt sie
   nicht, die Vorschau zeigt sie dem Büro (Abschnitte 4 und 6).
2. **Preis und Vertragsvorlage:** Stehen am Trägerangebot und entstehen beim
   Übertragen je Schule mit (Tarif in Parent Billing, Vertragsvorlage in
   Care Contract). Ein geänderter Preis wird beim nächsten Abgleich ein neuer
   Tarif ab Monatsbeginn an der Schule. Ausgestellte Verträge und Rechnungen
   bleiben eingefroren (Abschnitte 5 und 10).
3. **Deaktivieren:** Die Schule darf eine Kopie selbst deaktivieren (zählt als
   Abweichung). Löschen nur über das Büro (Zurückziehen, Abschnitt 7).
4. **Trägerwechsel:** Die Kopien werden gewöhnliche Angebote der Schule ohne
   Bezug zum Träger (Abschnitt 8).

Nicht Teil dieser Entscheidung: „Das ausgefüllte Anmeldeformular ist direkt
der Vertrag, nur der Träger unterschreibt.“ Heute unterschreiben die Eltern
digital (ADR 0155); eine Änderung braucht eine eigene Entscheidung.

## Folgen

- Die Kopie ist ein gewöhnliches Schulangebot. Anmeldeformular, Eltern-App,
  Buchung, Vertrag, Abrechnung und `traeger-view` lesen sie ohne Änderung.
- Care Plan und Enrollment wachsen je um eine Fähigkeit für den Träger und
  eine für die Kopie in der Schule. Ihre bestehenden Löschbefehle lehnen eine
  Kopie ab; die Abweichung pflegt der Trigger.
- Parent Billing wächst um `SaveCareModulePrices`.
- Die Oberfläche des Träger-Portals zeigt je Schule das letzte Ergebnis mit
  Fehlergrund. Hilfe und Screenshots folgen mit der Umsetzung.

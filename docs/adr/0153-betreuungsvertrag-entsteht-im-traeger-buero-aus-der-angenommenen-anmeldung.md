---
status: accepted
---

# Betreuungsvertrag entsteht im Träger-Büro aus der angenommenen Anmeldung

Freigegeben von Yannick Wenger am 07.11.2026

Vorschlag vom 06.11.2026 zu [#4678](https://github.com/moto-ogs/project-phoenix/issues/4678),
Etappe 2 der Spec [#4611](https://github.com/moto-ogs/project-phoenix/issues/4611),
User Stories 46, 47 (Weg a) und 49. Er ist der erste Bau-Schnitt des Owners
Betreuungsvertrag (`care-contract`) aus
[ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
und hält fest, was er in den Registern ändert (ADR 0057 Punkt 5, ADR 0070).

## Kontext

- ADR 0088 legt den Owner `care-contract` fest, aber es gibt noch keinen Code.
  Etappe 1 hat Ferienbuchungen in der Elternabrechnung festgehalten (ADR 0115),
  ohne Vertrag.
- Die OGS-Anmeldung liegt je Schule bei Enrollment (`enrollment.requests`,
  `request_children`). Angebote und Buchungen gehören Care Plan
  (`enrollment.care_offerings`, `care_offering_bookings`), beide im Tenant der
  Schule. Verträge liegen beim Träger.
- Keine Transaktion trägt Schule und Träger zugleich (ADR 0105). Innerhalb der
  Träger-Transaktion lässt sich keine Tenant-Transaktion öffnen.
- Die Vorlage der Dokumentart „Vertrag“ mit Platzhaltern gibt es seit #4677
  (ADR 0152). Mails des Trägers mit PDF im Anhang gibt es seit ADR 0123, das
  Postfach der Eltern-App seit ADR 0126.
- Beim SJR Pforzheim bucht eine Familie Tage einzeln und über Module hinweg:
  Montag bis 14:00, Dienstag bis 16:30, Mittwoch keine Betreuung, dazu
  getrennte Früh- und Nachmittagsbetreuung (SJR-Abgleich vom 06.11.2026,
  Zeile 1, ergänzt in #4678). In moto ist ein Modul ein Betreuungsangebot der
  Schule mit seiner Gehzeit je Wochentag (`pickup_times`); ein Kind bucht
  mehrere davon.
- Die Unterschrift kommt mit #4679: gezeichnet oder getippt über den
  Unterschrift-Baustein in Audit (#4785), dazu Papier oder PDF für
  Bestandsverträge. Abschluss bei der Annahme (Weg b, #4680), Änderung
  (#4681), Monatsbeitrag (#4684) und Kündigung (#4682) folgen.

## Entscheidung

1. **Owner.** `care-contract` ist der zwanzigste Domain-Owner, in Policy-Epoche
   98. Pakete: `modules/carecontract` (public), `compose`, `http`,
   `internal/application`, `internal/ports`, `internal/adapters/postgres`,
   `internal/adapters/rendering`. Er importiert keinen anderen Domain-Owner;
   alle Kanten laufen über Ports, die die Root bindet
   (`api/serve_mounts_care_contract.go`, `roots/owners/parent_portal_documents.go`).
2. **Tabellen** (Schema `care_contract`, Migration 1.15.770), alle mit
   `organization_id` und der Träger-Policy:
   - `contract_templates`: Vertragsvorlage je Schule und Angebot mit Laufzeit
     (unbefristet, bis Schuljahresende, feste Monate), Kündigungsfrist in
     Monaten und Termin (Monatsende, Halbjahr, Schuljahresende),
     Sonderkündigung mit Text, Satzungsbestätigung mit Titel und Link,
     Vertragspartner (leer heißt: der Träger).
   - `contracts`: ein Vertrag je Kind der Anmeldung an seiner Schule. Der
     Schlüssel `(school_id, request_child_id)` ist eindeutig; das ist die
     Anmeldung des Kindes aus ADR 0088 §6. Der Vertrag umfasst alle Module,
     die das Kind dort bucht. Der Status kennt alle Werte von ADR 0088 §2;
     dieser Schnitt erreicht nur `entwurf` und `versendet`.
   - `contract_versions`: eingefrorene Kopie von Kind, Betreuungsort,
     Monatsbeitrag, Vertragsbedingungen und Wirkdatum. Status `beantragt` bis
     ein späterer Schritt sie wirksam macht. Der Monatsbeitrag ist der
     Angebotspreis beim Anlegen, wenn das Kind genau ein Modul bucht (ADR
     0088 §2). Über mehrere Module bleibt er leer, und der Vertrag nennt
     „laut Beitragsordnung für die gebuchten Module“: der Preis eines
     Angebots gilt für den ganzen Monat, die Summe würde Familien mit
     einzelnen Tagen je Modul zu viel berechnen. #4684 rechnet sie je
     Wochentag und Modul.
   - `contract_version_services`: die gebuchten Leistungen der Version, eine
     Zeile je Wochentag und Modul mit Angebot, Name des Moduls und Endzeit an
     diesem Tag (`TIME`, aus der Gehzeit des Angebots). Ein Wochentag ohne
     Zeile hat keine Betreuung; ein Tag kann mehrere Module tragen. Endet ein
     Modul vor der Version, trägt die Zeile seinen letzten gebuchten Tag
     (`last_day`), und das PDF nennt ihn („gebucht bis …“): Der Vertrag sagt
     nie mehr zu, als die Buchung hergibt. Die Zeilen ändern sich nie.
   - `contract_parties`: die Eltern aus der Anmeldung, mit optionalem
     Elternkonto.
   - `contract_documents`: das PDF einer Version, erzeugt beim ersten Versand,
     mit SHA-256. Es ändert sich nie.
   - `contract_mailings`: jeder Versand mit Outbox-Eintrag und Adresse.
   - `contract_events`: Spur jedes Lesens und Schreibens. Sie hält die
     Vertrags-ID ohne Fremdschlüssel, damit ein verworfener Entwurf seine Spur
     behält.

   Keine Tabelle verweist per Fremdschlüssel auf Anmeldung, Kind oder Angebot
   der Schule: der Vertrag überlebt sie (ADR 0088 §9).
3. **Erzeugen (Weg a).** Das Träger-Büro sieht je Schule die angenommenen
   Kinder mit gebuchter Betreuung ohne Vertrag, mit ihren Modulen je
   Wochentag, und legt für sie Entwürfe an, einzeln oder gesammelt. Nur
   Angebote mit `counts_as_care` sind Module; AG und Mittagessen bleiben
   draußen. Je Angebot zählt die laufende oder nächste Buchung. Die Version
   nimmt nur die Buchungen, die an einem Stichtag laufen (Beginn der
   frühesten Buchung, frühestens heute); ein späterer Wechsel des Moduls
   kommt mit einer neuen Version (#4681). Die Intake-Fähigkeit liest jede Schule in ihrer
   eigenen Tenant-Transaktion (Enrollment: Anmeldung und Eltern; Care Plan:
   Buchungen und Angebote) und schreibt danach in der Träger-Transaktion. Ein
   zweiter Entwurf für denselben Schlüssel wird übersprungen
   (`ON CONFLICT DO NOTHING`). Jedes Modul braucht seine Vorlage, und die
   Vorlagen der Module eines Kindes müssen gleich sein: ein Vertrag hat eine
   Laufzeit, eine Frist und eine Satzung. Sonst entsteht kein Entwurf, und
   die Liste nennt den Grund (`template_missing`, `templates_differ`). Das
   Wirkdatum ist der früheste Beginn der Buchungen, sonst der Aktivierungstag
   des Kindes, sonst der Leistungsbeginn der Anmeldephase. Der Vertrag endet
   mit der letzten Buchung, wenn alle enden, oder früher mit einer festen
   Laufzeit.
4. **Status.** `entwurf` → `versendet` beim ersten Versand. Nur ein Entwurf
   lässt sich verwerfen; die Lösch-Policy der Tabelle lässt für die
   Request-Rolle nur Entwürfe zu. Der Workflow `contractconclusion`, der den
   Vertrag bei der Annahme anlegt, kommt mit #4680.
5. **Dokument.** Betreff, Anrede, Text und Grußformel kommen aus der Vorlage
   „Vertrag“ des Trägers (ADR 0152). Danach druckt der Vertrag immer die
   Tabelle „Vertragsangaben“, die Tabelle „Gebuchte Betreuung“ mit jedem
   Wochentag von Montag bis Freitag (Module mit Endzeit oder „keine
   Betreuung“) und, wenn gewählt, Sonderkündigung und Satzung,
   damit kein Vorlagentext eine Bedingung weglassen kann. Vor dem Versand ist
   das PDF eine Vorschau mit der aktuellen Vorlage; beim ersten Versand wird es
   festgehalten und jeder weitere Versand nutzt dasselbe PDF.
6. **Versand.** Wie ADR 0123: Mail des Trägers in der Outbox
   (`EnqueueEmail` mit `OrganizationID`, Art `care_contract`, Vertrag als
   verknüpfte Entität), PDF im Payload, in derselben Träger-Transaktion wie
   `contract_mailings`. Der Schlüssel zählt die Versände
   (`care-contract:<Vertrag>:<n>`). Wartet die letzte Mail noch, wird nicht
   erneut gesendet. Das weicht von ADR 0088 §8 ab (dort: kein Anhang); ADR
   0123 hat diese Abweichung für Schreiben des Trägers schon getroffen. Der
   einmalige Mail-Link zum Lesen und Unterschreiben kommt mit #4679.
7. **Eltern-App.** Ein versendeter Vertrag liegt im Postfach des Kontos, an
   dessen Adresse er ging (ADR 0088 §7, ADR 0126). Der Workflow
   `parent-portal` prüft wie bisher die Beziehung
   (`parent_portal.enrollments.view`) und liest über den neuen Port
   `ContractDocuments` in der Träger-Transaktion. Die Verträge kommen über
   eine eigene Route (`GET /parent/me/documents/contracts`) mit eigenem
   Abfrage-Budget; die Liste der Briefe bleibt unverändert, und ein Fehler
   beim Vertrag verdeckt keine Briefe im Backend.
8. **Zugriff und Protokoll.** Alle Routen verlangen das Träger-Recht
   „Verträge verwalten“ (`traeger_contracts:manage`). Jede Liste, jedes Detail,
   jede Vorschau, jeder Versand und jedes Verwerfen steht mit Konto in
   `contract_events`. Das Lesen der Anmeldungen und jedes Lesen eines
   Vertrags (Liste, Detail, PDF) steht außerdem im Zugriffsprotokoll der
   Schule, die Betreuungsort ist (`audit.data_access_log`, Arten
   `care_contract_applications` und `care_contracts`, ADR 0084 §7, ADR 0088
   §7). Weil das Protokoll nur eine Tenant-Policy hat, öffnen diese Lesewege
   ihre Einheiten selbst: erst die Träger-Transaktion, dann je Schule deren
   Tenant-Transaktion. Das Lesen im Postfach steht mit dem Elternkonto in
   `contract_events`.
9. **Nicht in diesem Schnitt.** Die Unterschrift (#4679) legt dieser Schnitt
   nicht an. Sie hängt später am versendeten Dokument einer Version
   (`contract_documents`) und am Status `unterschrieben`, den die Tabelle schon
   kennt; ob gezeichnet, getippt oder auf Papier, entscheidet kein Feld hier.
   Neue Versionen mit eigenem Wirkdatum kommen mit #4681, der Abschluss bei
   Annahme mit #4680. Die Tenant-Policy, mit der eine
   Schule ihre Verträge liest (ADR 0088 §7), kommt mit der ersten Schulfläche
   (#4683). Die Löschfristen der Verträge legt #4720 fest.
10. **Register.**
    - `policy.json`: Owner `care-contract`, Epoche 97 → 98, Fragment
      `policy/care-contract.json` mit acht Datenobjekten.
    - `command-effects/{reviews,evidence}/care-contract.json`: 25 geprüfte
      Bindungen (Outbox: platform/durable, Transaktionen: platform/scope,
      Zugriffsprotokoll: platform/mutation, Briefkopf: leaf, sonst query).
    - `passthrough/{reviews,evidence}/roots.json`: drei Adapter mit
      Entscheidung 0153.
    - Enrollment bekommt `ContractApplications`, Care Plan
      `ContractOfferings`; beide lesen nur in der Tenant-Transaktion der
      Schule.

## Verworfene Optionen

- **Vertrag automatisch bei jeder Annahme:** wäre Weg b (#4680) und änderte
  `Decide`, was E1 in ADR 0067 verbietet.
- **Anmeldungen über die Projektion `traeger-view` lesen:** die Projektion
  liest nur Zahlen und IDs (ADR 0151); Namen, Eltern und Buchungen gehören in
  die Tenant-Transaktion der Schule.
- **PDF nur über einen Link:** braucht den einmaligen Mail-Link, der mit der
  Unterschrift (#4679) kommt.
- **Ein Vertrag je Kind und Angebot:** passt nicht zu Tagen, die einzeln und
  über Module hinweg gebucht werden (SJR: Montag bis 14:00, Dienstag bis
  16:30). Die Familie bekäme mehrere Verträge für eine Betreuung, und der
  Monatsbeitrag je Wochentag und Modul (#4684) müsste sie wieder
  zusammensetzen.
- **Leistungen als JSON in der Version:** die Monatsrechnung (#4684) liest je
  Wochentag und Modul; eine Tabelle mit Prüfungen für Wochentag und Modul hält
  das sauber und abfragbar.
- **Bedingungen je Modul im Vertrag mischen:** ein Vertrag mit zwei Laufzeiten
  oder Fristen ist für Eltern nicht lesbar; abweichende Vorlagen halten den
  Entwurf an, bis das Träger-Büro sie angleicht.

## Folgen

- #4680 kann den Workflow `contractconclusion` auf `InsertContract` und den
  Status `aktiv` aufsetzen, #4679 die Unterschrift an `contract_documents`.
- #4684 rechnet den Monatsbeitrag je Wochentag und Modul aus den Leistungen
  der wirksamen Version mit ihrem Wirkdatum; #4681 legt dafür neue Versionen
  mit eigenen Leistungen an, statt Zeilen zu ändern.
- Eine Änderung der Vorlage ändert keinen bestehenden Vertrag.
- Ein Vertrag, dessen Anmeldung gelöscht wird, bleibt mit seiner Kopie.

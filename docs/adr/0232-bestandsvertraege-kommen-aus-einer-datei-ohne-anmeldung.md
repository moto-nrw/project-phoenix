---
status: proposed
---

# Bestandsverträge kommen aus einer Datei, ohne Anmeldung

Vorschlag vom 14.11.2026 zu [#4723](https://github.com/moto-ogs/project-phoenix/issues/4723),
Spec [#4611](https://github.com/moto-ogs/project-phoenix/issues/4611), User Stories 58
und 124. Er ergänzt
[ADR 0153](0153-betreuungsvertrag-entsteht-im-traeger-buero-aus-der-angenommenen-anmeldung.md)
Punkt 2 um einen zweiten Ursprung eines Vertrags und führt das Verfahren
`papier` aus
[ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
§3 für importierte Verträge aus. Product Owner Yannick Wenger hat am 13.11.2026
entschieden, den Import für SJR Pforzheim vorzuziehen (Kommentar in #4723).

## Kontext

- Ein Träger, der zu moto wechselt, hat Kinder und Verträge in einem anderen
  Programm, etwa OGS Connect. Ohne Import beginnt er mit einer leeren Liste;
  Elternabrechnung und Mandatsimport (#7009) brauchen die Verträge.
- Ein Vertrag hatte bisher genau einen Ursprung: das Kind einer angenommenen
  Anmeldung, Schlüssel `(school_id, request_child_id)` (ADR 0153 Punkt 2).
  Ein Bestandsvertrag hat keine Anmeldung in moto.
- Die Exportformate sind offen. Der Mandatsimport (#6198) löst das mit einer
  Zuordnung der Spalten, einer Vorschau und einem Fingerabdruck der bestätigten
  Vorschau.
- Die Debitorennummer führt die Elternabrechnung je Zahler oder je Kind
  (#7007). Eine Familie ist dort die Mail-Adresse des Rechnungsempfängers
  (ADR 0164 Punkt 4).
- ADR 0214 verknüpft ein Kind nie über Name und Geburtsdatum mit einem Kind
  einer Schule: Die Übereinstimmung beweist nicht, wer das Kind ist.

## Entscheidung

1. **Owner und Ort.** Der Import gehört `care-contract`: öffentliche Fähigkeit
   `carecontract.ContractImports`, Routen unter `/traeger/contract-imports`
   hinter dem Recht „Verträge verwalten“. Es gibt keine neue Tabelle und
   keinen neuen Owner.
2. **Ursprung eines Vertrags.** `care_contract.contracts` nennt genau einen
   Ursprung: `request_child_id` (Anmeldung) oder `import_origin` (Datei).
   `request_child_id` ist deshalb optional. `import_origin` ist Vorname und
   Nachname des Kindes ohne Groß- und Kleinschreibung, Geburtsdatum und erster
   Vertragstag; je Träger und Schule eindeutig. Die erste Version trägt die
   Herkunft `import`.
3. **Ein importierter Vertrag ist abgeschlossen.** Status `aktiv`, Version 1
   `wirksam`, Verfahren `papier`. Abschlusstag ist das Datum aus der Datei,
   sonst der Vertragsbeginn aus der Datei, höchstens heute. Die Eltern
   bekommen keine Mail und kein PDF von moto: Sie halten ihren Vertrag schon.
   Der Verlauf nennt Datei, Zeile und das Konto, das importiert hat.
4. **Module und Bedingungen wie bei jedem Vertrag.** Eine Zeile bucht Module
   ihrer Schule über deren Namen, an den genannten Wochentagen oder an den
   Tagen des Moduls. Die Vertragsbedingungen kommen aus den Bedingungen des
   Trägers für das Modul; ohne Bedingungen, oder bei verschiedenen Bedingungen
   mehrerer Module, übernimmt moto die Zeile nicht. Der Vertrag beginnt und
   endet wie ein Vertrag aus einer Anmeldung mit der Phase seines Moduls: Lief
   er schon vorher, beginnt Version 1 mit der Phase, und der frühere Tag
   bleibt als Abschlusstag stehen. Er endet früher nur mit einem Ende aus der
   Datei oder einer festen Laufzeit. So rechnet die Elternabrechnung nie einen
   Monat vor der Phase mit deren Modul ab. Eine Datei ohne Spalte für die
   Schule, das Modul oder den ersten Vertragstag bekommt sie einmal für alle
   Zeilen: Der Stammdaten-Export von OGS Connect (#7009) nennt keine davon.
5. **Keine Dubletten.** Hat ein Kind an der Schule einen Vertrag, der weder
   widerrufen noch beendet ist, bekommt es keinen zweiten. Verglichen werden
   Schule und Name; zwei verschiedene Geburtsdaten trennen zwei Kinder, ein
   fehlendes Geburtsdatum trennt nie. Ein Kind, das die Datei zweimal nennt,
   ist ein Konflikt. Einen Vertrag, den ein früherer Import angelegt hat,
   legt moto nicht noch einmal an, auch wenn er inzwischen beendet oder
   widerrufen ist; die Zeile nennt ihn als vorhanden. Der Import übernimmt nur die Vorschau, die das Büro
   bestätigt hat (Fingerabdruck); sonst `care_contract.import_changed`.
6. **Kein Abgleich mit Kindern der Schule.** Der Import setzt `student_id`
   nicht (ADR 0214). Der Vertrag hält seine eigene Kopie von Kind und Eltern.
   Folge: Für importierte Verträge gibt es keinen Soll-Ist-Vergleich je Kind
   und keine Änderung aus Buchungen (`AmendFromBookings` liest nur Verträge
   aus Anmeldungen), bis ein eigener, belegter Schritt das Kind verknüpft.
7. **Debitorennummer.** `care-contract` hat den Port
   `ContractDebtorNumbers`; die Root bindet ihn an die neue Fähigkeit
   `parentbilling.ContractImportNumbers`. Sie entscheidet und speichert in der
   Träger-Transaktion des Imports (Kommando-Teilnahme nach ADR 0067, wie
   `FamilyNumbers`): eine vorhandene Nummer bleibt, widersprüchliche Zeilen
   übernehmen keine, eine Zeile ohne Mail-Adresse der Eltern nennt keine
   Familie und übernimmt keine.
   **Geändert am 14.11.2026:** Der Stammdaten-Export von OGS Connect hat
   keine Mail-Adressen; ohne Nummer findet der Mandatsimport (#7009) keinen
   Zahler. Eine Zeile ohne Mail-Adresse ist deshalb eine eigene Familie, die
   moto über ihren Vertrag kennt (`parentbilling.ContractFamilyKey`,
   Schlüssel `contract:<Vertrag>`). Ihre Nummer wird übernommen, je Zahler
   wie je Kind. Derselbe Schlüssel gilt überall, wo die Elternabrechnung die
   Familie eines Vertrags bildet: Abweichung und Nummer der Familie, Mandat
   (Import und von Hand), Monatsrechnung, Essen, Bescheid, Nummer je Kind.
   Der Schlüssel ist keine Adresse: Niemand bekommt darüber Post, und die
   Oberfläche zeigt „Vertrag Nr. …“. Er ergänzt ADR 0164 Punkt 4, der die
   Familie über die Mail-Adresse bestimmt; die Geschwisterfolge kannte den
   Schlüssel schon (`familyOf`).
8. **Zugriff und Protokoll.** Der Import liest die Angebote jeder Schule in
   deren Tenant-Transaktion (Stammdaten, keine Personendaten), schreibt in der
   Träger-Transaktion und trägt danach in das Zugriffsprotokoll jeder Schule
   ein, deren Verträge er gezeigt oder angelegt hat (ADR 0084 §7). Die Datei
   wird nie gespeichert.

9. **Zeile ohne Eltern.** Nennt eine Zeile weder Namen noch Mail-Adresse
   der Eltern (im Export von OGS Connect die Kinder ohne Mandat), speichert
   moto den Vertrag ohne Vertragspartner und sagt das in der Vorschau. Nennt
   sie Vorname oder Mail-Adresse, aber keinen Nachnamen, ist sie
   unvollständig.

## Register

Die Entscheidung ändert diese Register (ADR 0057 Punkt 5, ADR 0070):

- **Compose-Budgets** (`backend/test/module_legacy_budget_ratchet_test.go`):
  `modules/carecontract/compose` 175 → 181 (Konstruktor `NewContractImports`
  und der Typ seiner Ports), `modules/parentbilling/compose` 129 → 134
  (Konstruktor `NewContractImportNumbers`), Summe 13477 → 13488. Beide
  Konstruktoren bauen nur ihren Owner (ADR 0068).
- **Träger-Transaktion** (`backend/test/traeger_transaction.go`,
  `transaction_boundaries.json`): `ContractImports.ImportContracts` und
  `ContractImports.ContractImportLog` öffnen die Träger-Transaktion selbst,
  weil der Import davor jede Schule in ihrer Tenant-Transaktion liest und
  danach in deren Zugriffsprotokoll schreibt (ADR 0105).
- **Kommando-Effekte und Pass-through**
  (`backend/architecture/command-effects/`, `passthrough/`): die Bindung
  `roots/owners.CareContractDebtorNumbers` mit `DebtorNumberScope` (Lesen,
  übersetzt) und `DecideImportedDebtorNumbers` (Blatt-Kommando nach ADR 0067).
- **Abfragebudget**: `api.traeger.care_contract_imports`, zwei Abfragen.

## Verworfene Optionen

- **Kinder in den Schulen anlegen.** Schreiben in People Directory über
  mehrere Schulen wäre ein eigener Workflow; die Schulen haben ihren
  Kinder-Import. Der Vertrag braucht das Kind der Schule nicht (ADR 0088 §2).
- **Kinder über Name und Geburtsdatum verknüpfen.** Widerspricht ADR 0214.
- **Importierte Verträge als Entwurf.** Das Büro müsste jeden Bestandsvertrag
  noch einmal versenden und unterschreiben lassen, obwohl er gilt.
- **Eigene Tabelle für Importe.** Der Verlauf des Vertrags trägt Datei und
  Zeile schon, wie beim Mandatsimport.
- **Familie ohne Mail-Adresse über die Debitorennummer bilden.** Geschwister
  mit derselben Nummer wären eine Familie, aber die Rechnung sucht die Nummer
  über die Familie, nicht umgekehrt, und bei einer Nummer je Kind (#7007)
  trennt sie Geschwister ohnehin.
- **Debitorennummer am Vertrag speichern.** Die Nummer gehört der
  Elternabrechnung (ADR 0088); der Mandatsimport und die Rechnung lesen sie
  dort.
- **Vertrag ohne Modul.** Die Elternabrechnung rechnet je Wochentag und
  Modul; ein Vertrag ohne Modul wäre nicht abrechenbar.

## Folgen

- Leser von `request_child_id` rechnen mit „kein Wert“: Kandidatenschlüssel
  und `AmendableContracts` lassen importierte Verträge aus, die Änderungen der
  Eltern lesen 0.
- Die Rückgabe an den Träger enthält die neue Spalte; das Lebenszyklus- und
  das Rückgaberegister sind angepasst.
- Geschwister ohne Mail-Adresse sind getrennte Familien: kein
  Geschwisterrabatt zwischen ihnen, und ein Mandat mit derselben Referenz für
  zwei von ihnen zeigt der Mandatsimport als Konflikt. Mit Mail-Adresse in
  der Datei gilt das nicht.
- Eine Familie ohne Mail-Adresse bekommt keine Mail und sieht nichts in der
  Eltern-App; Rechnungen gehen ihr auf Papier zu. Ein Vertrag ohne
  Vertragspartner hat eine Rechnung ohne Empfängernamen, bis das Büro einen
  abweichenden Rechnungsempfänger einträgt.
- Die Mail-Adresse eines Vertragspartners lässt sich heute nicht ändern. Wer
  das einführt, zieht Nummer, Abweichung und Mandat vom Schlüssel des
  Vertrags auf die Adresse um.
- Offen: Verknüpfung importierter Verträge mit Kindern der Schule (Punkt 6).

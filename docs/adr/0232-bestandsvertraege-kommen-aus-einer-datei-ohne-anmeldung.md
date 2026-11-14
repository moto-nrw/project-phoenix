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
   `wirksam` ab dem Tag aus der Datei, Verfahren `papier`. Abschlusstag ist das
   Datum aus der Datei, sonst der Vertragsbeginn, höchstens heute. Die Eltern
   bekommen keine Mail und kein PDF von moto: Sie halten ihren Vertrag schon.
   Der Verlauf nennt Datei, Zeile und das Konto, das importiert hat.
4. **Module und Bedingungen wie bei jedem Vertrag.** Eine Zeile bucht Module
   ihrer Schule über deren Namen, an den genannten Wochentagen oder an den
   Tagen des Moduls. Die Vertragsbedingungen kommen aus den Bedingungen des
   Trägers für das Modul; ohne Bedingungen, oder bei verschiedenen Bedingungen
   mehrerer Module, übernimmt moto die Zeile nicht. Der Vertrag endet wie ein
   Vertrag aus einer Anmeldung mit der Phase seines Moduls, früher nur mit
   einem Ende aus der Datei oder einer festen Laufzeit.
5. **Keine Dubletten.** Hat ein Kind an der Schule einen Vertrag, der weder
   widerrufen noch beendet ist, bekommt es keinen zweiten. Verglichen werden
   Schule und Name; zwei verschiedene Geburtsdaten trennen zwei Kinder, ein
   fehlendes Geburtsdatum trennt nie. Ein Kind, das die Datei zweimal nennt,
   ist ein Konflikt. Der Import übernimmt nur die Vorschau, die das Büro
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
8. **Zugriff und Protokoll.** Der Import liest die Angebote jeder Schule in
   deren Tenant-Transaktion (Stammdaten, keine Personendaten), schreibt in der
   Träger-Transaktion und trägt danach in das Zugriffsprotokoll jeder Schule
   ein, deren Verträge er gezeigt oder angelegt hat (ADR 0084 §7). Die Datei
   wird nie gespeichert.

## Verworfene Optionen

- **Kinder in den Schulen anlegen.** Schreiben in People Directory über
  mehrere Schulen wäre ein eigener Workflow; die Schulen haben ihren
  Kinder-Import. Der Vertrag braucht das Kind der Schule nicht (ADR 0088 §2).
- **Kinder über Name und Geburtsdatum verknüpfen.** Widerspricht ADR 0214.
- **Importierte Verträge als Entwurf.** Das Büro müsste jeden Bestandsvertrag
  noch einmal versenden und unterschreiben lassen, obwohl er gilt.
- **Eigene Tabelle für Importe.** Der Verlauf des Vertrags trägt Datei und
  Zeile schon, wie beim Mandatsimport.
- **Vertrag ohne Modul.** Die Elternabrechnung rechnet je Wochentag und
  Modul; ein Vertrag ohne Modul wäre nicht abrechenbar.

## Folgen

- Leser von `request_child_id` rechnen mit „kein Wert“: Kandidatenschlüssel
  und `AmendableContracts` lassen importierte Verträge aus, die Änderungen der
  Eltern lesen 0.
- Die Rückgabe an den Träger enthält die neue Spalte; das Lebenszyklus- und
  das Rückgaberegister sind angepasst.
- Offen: Verknüpfung importierter Verträge mit Kindern der Schule (Punkt 6),
  Debitorennummer für Zeilen ohne Mail-Adresse der Eltern (Punkt 7): Sie
  braucht einen anderen Schlüssel der Familie als die Mail-Adresse.

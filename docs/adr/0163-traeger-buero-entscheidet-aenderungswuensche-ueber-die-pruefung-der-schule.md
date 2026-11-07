---
status: proposed
---

# Das Träger-Büro entscheidet Änderungswünsche über die Prüfung der Schule

Vorgeschlagen am 07.11.2026 zu [#5979](https://github.com/moto-nrw/project-phoenix/issues/5979),
Folge von [#4681](https://github.com/moto-nrw/project-phoenix/issues/4681) und PR #5953,
Etappe 2 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611).
Baut auf [ADR 0153](0153-betreuungsvertrag-entsteht-im-traeger-buero-aus-der-angenommenen-anmeldung.md)
und [ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)
(Teilnahme an einem Befehl) auf. Kein neuer Owner, keine Migration.

## Kontext

- Eltern beantragen in der Eltern-App andere Tage oder Module
  (`enrollment.offering_change_requests`, Care Plan). Bisher entscheidet nur
  die Schule. #4681 verlangt „Freigabe oder Ablehnung durch Schule oder
  Träger“.
- Nach einer Freigabe folgt der aktive Vertrag den gebuchten Leistungen mit
  einer neuen Version (`AmendFromBookings`, ADR 0153 Punkt 3, #5953). Ein
  zweiter Weg zu einer Version würde Buchung und Vertrag auseinanderlaufen
  lassen.
- Die Prüfung der Schule (`OfferingChangeCapability.Decide`) sperrt Kind und
  Antrag, prüft den Status unter der Sperre und schreibt Buchungen,
  Entscheidung, Elternnachricht und Spur in der Tenant-Transaktion der Schule.
  Ihre Reichweite kommt aus den Rechten der Schul-Sitzung.

## Entscheidung

1. **Ein Entscheidungsweg.** Das Träger-Büro ruft dieselbe Prüfung der Schule
   auf, in der Tenant-Transaktion der Vertragsschule
   (`carecontract.ContractChangeDecisions`, Route
   `/traeger/contract-changes`). Care Contract bindet sie über den eigenen
   Port `SchoolChangeRequests`; die Root bindet ihn an Care Plan. Das ist
   Teilnahme an einem Befehl nach ADR 0067: Care Contract schreibt in dieser
   Transaktion nur den Eintrag im Zugriffsprotokoll der Schule, Care Plan
   behält Validierung, Sperren und Schreiben.
2. **Berechtigung vor der Schul-Transaktion.** Die Route verlangt
   „Verträge verwalten“. Care Contract liest den Vertrag in der
   Träger-Transaktion (Filter auf `organization_id` und RLS) und prüft, dass
   die Vertragsschule noch zu den Schulen des Trägers gehört. Erst dann
   öffnet es die Schul-Transaktion. Keine Transaktion trägt beides.
3. **Reichweite des Büros.** Die Bindung markiert den Kontext als
   Entscheidung des Träger-Büros. Nur diese Markierung gibt der
   Prüfungsreichweite die ganze Schule; der Schlüssel ist privat in der Root.
   Care Plan prüft zusätzlich unter der Antragssperre, dass der Antrag zum
   Kind und zur Anmeldung des Vertrags gehört
   (`OfferingChangeDecisionInput.StudentID` und `RequestChildID`).
4. **Kein doppeltes Freigeben.** Schule und Büro sperren dieselbe Antragszeile.
   Wer die Sperre als Zweiter bekommt, sieht einen entschiedenen Antrag und
   bekommt den Fehlercode `students.change_request_not_pending`; nur eine
   Freigabe ändert Buchungen.
5. **Die Version folgt wie bisher.** Der Worker gleicht nach dem Abschluss
   die aktiven Verträge ab; die Freigabe des Büros erzeugt dieselben Buchungen
   und damit dieselbe Version wie die der Schule, unter der Vertragssperre aus
   #4680 und den Regeln der Zeitleiste aus #5953.
6. **Audit.** Lesen und Entscheiden schreiben je einen Eintrag ins
   Zugriffsprotokoll der Schule (`care_contract_changes`) in derselben
   Schul-Transaktion; die Spur der Elternanfragen nennt das Konto des Büros
   als Entscheider.

## Folgen

- `modules/carecontract/compose` wächst um den Konstruktor
  `NewContractChanges` (+5 LOC); das Budget und die Summe steigen genau darum.
- Eine neue Träger-Transaktionsstelle
  (`*ContractChanges.contract`, Lesen des Vertrags) steht in
  `test/transaction_boundaries.json` und `test/traeger_transaction.go`.
- Ein vollständiger Abmeldewunsch bleibt eine Kündigung (#4682). Das Büro kann
  ihn hier nur ablehnen; das Freigeben verlangt die Bestätigung der
  Komplett-Abmeldung, die nur die Schule gibt.

## Prüfnachweis der Befehlsbindungen

- `SchoolChangeRequests.DecideChange` nimmt weiter an Care Plans geschütztem
  `Decide` teil (Leaf-Befehl nach ADR 0067). Kind und Anmeldung werden unter
  derselben Antragssperre geprüft. Der Port reicht die berechneten Folgen für
  Regeltermine und Abholzeiten zurück; er koordiniert keine weiteren Schreibwege.
- `PendingChanges` liest den Antrag derselben Anmeldung. Für ein bestätigtes
  Datum nutzt es Care Plans nicht schreibendes `PreviewDecision` samt Vergleich.
  Versionsbildung und Buchung bleiben beim bestehenden Entscheidungsweg.
- Die Bindung von `SchoolAccessLog.RecordAccess` ergänzt nur die Ressourcenart
  der Vertragsänderungen. Der Audit-Eintrag bleibt in der Schul-Transaktion.
- Die geänderten Care-Plan-Prüfungsbindungen unterscheiden den privaten
  Träger-Kontext von der unveränderten Reichweite der Schule. Der Kontext wird
  erst nach Prüfung von Vertragsrecht, Träger und Vertragsschule gesetzt.
  Die bekannten unaufgelösten Test-Callables bleiben als solche erfasst;
  weder ihre Klassifikation noch eine Architekturregel wird erweitert.
- Die Cross-Owner-Tests beziehen fremde Vertragswerte über den typisierten
  Test-Einstieg in `api`, nicht durch einen Import des fremden Owners.

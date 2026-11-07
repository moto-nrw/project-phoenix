---
status: accepted
---

# Widerruf und Kündigung beenden den Betreuungsvertrag unter seiner Zeilensperre

Freigegeben von Yannick Wenger am 07.11.2026

Vorschlag vom 07.11.2026 zu [#4682](https://github.com/moto-nrw/project-phoenix/issues/4682),
Etappe 2 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 53, 54 und 55. Er baut auf
[ADR 0153](0153-betreuungsvertrag-entsteht-im-traeger-buero-aus-der-angenommenen-anmeldung.md)
(Betreuungsvertrag) auf und führt keinen neuen Owner ein. Rechtsgrundlage:
[Recherche und Entscheidung in #4679](https://github.com/moto-nrw/project-phoenix/issues/4679#issuecomment-6678923573),
Ergänzung in #4682. Keine Rechtsberatung.

## Kontext

- Ein online geschlossener Betreuungsvertrag ist ein Fernabsatzvertrag:
  14 Tage Widerrufsrecht ab Abschluss, Beginn der Frist erst mit Belehrung,
  ohne Belehrung Ende nach 12 Monaten und 14 Tagen (§§ 312g, 355, 356 BGB).
  Die Kündigung darf höchstens Textform verlangen (§ 309 Nr. 13 BGB), und
  die Webseite braucht den Kündigungsbutton (§ 312k BGB).
- Der Monatsbeitrag (#4684) rechnet aus `VersionsInForce`. Was nach dem
  Vertragsende oder nach einem Widerruf noch dort steht, wird abgerechnet.
- Der Abschluss (#4680), die Änderung (#4681) und die Unterschrift (#4679)
  ändern den Status desselben Vertrags. Sie nehmen zuerst die Zeilensperre
  des Vertrags und lesen danach die Versionen.

## Entscheidung

1. **Zwei neue Status.** `gekuendigt` mit dem letzten Tag `ends_on` und
   `widerrufen`. Beide folgen nur auf `aktiv`. Die Versionen bleiben, wie sie
   sind; der Vertrag schneidet sie ab. `VersionsInForce` liefert bei
   `gekuendigt` keinen Tag nach `ends_on` und bei `widerrufen` keinen Tag.
2. **Ein Übergang unter der Zeilensperre.** `declare` nimmt die Sperre des
   Vertrags, liest danach seine Versionen und Erklärungen und schreibt dann.
   Eine zweite Erklärung oder eine Änderung wartet und sieht den beendeten
   Vertrag. Die Rennen laufen mit zwei echten Transaktionen.
3. **Fristen aus der Vertragsfassung.** Kündigungsfrist, Kündigungstermin und
   Sonderkündigung zum Halbjahr kommen aus den eingefrorenen Bedingungen der
   Version. So gilt, was die Eltern vereinbart haben, nicht die heutige
   Vorlage. Die Widerrufsfrist ist gesetzlich und keine Einstellung.
4. **Belehrung im PDF.** Jedes Vertrags-PDF trägt ab dieser Änderung die
   Widerrufsbelehrung und die Angabe, ob die Eltern den Beginn vor Ende der
   Frist verlangt haben. Neue PDFs tragen `withdrawal_notice`; alte PDFs
   erhalten diesen Nachweis nicht nachträglich. Ein Versandauftrag startet
   keine Frist. Erst ein positiver Zustellbeleg oder der PDF-Abruf in der
   Eltern-App setzt `withdrawal_informed_at`, unabhängig vom Abschlussdatum.
   Delivery liefert Zustellbelege durch den konsumereigenen Port; Care
   Contract übernimmt vorhandene Zustellbelege beim Lesen der Optionen oder
   beim Prüfen einer Erklärung dauerhaft. Bereits übernommene Nachweise
   bleiben nach dem Löschen der Outbox-Mail erhalten. Ohne positiven
   Nachweis bleibt konservativ die längere Frist, auch bei reinem SMTP ohne
   Zustellbeleg. Ein früher auf Papier geschlossener Vertrag hat keine.
   Ein rechtzeitig abgesandter Widerruf bleibt auch bei späterem Eingang gültig:
   `dispatched_on` hält den Absende-, `received_on` den Eingangstag fest.
5. **Bestätigung in Textform.** Die Texte sind Träger-Einstellungen
   (`care_contract.termination_confirmation`,
   `care_contract.withdrawal_confirmation`, ADR 0086, kein Env-Fallback). Der
   ausgefüllte Text steht mit der Erklärung in `care_contract.contract_notices`
   und geht als Mail des Trägers an die Eltern.
6. **Eltern mit Beziehungsrecht.** Die Eltern-App erlaubt die Erklärung nur
   für ein Kind, dessen Beziehung `parent_portal.enrollment.submit` hat, und
   nur für einen Vertrag an die eigene Adresse. Wer die Familie mit einer
   Anmeldung binden darf, darf den Vertrag auch beenden; das Leserecht des
   Posteingangs allein genügt nicht.
7. **Häkchen als freiwilliger Block.** Das Anmeldeformular trägt die
   Belehrung und das Häkchen „Betreuung soll vor Ende der Widerrufsfrist
   beginnen“ als eigene Blöcke. Enrollment liefert dafür auch angehakte
   freiwillige Blöcke mit `Required`; die Vertragsbedingungen zählen weiter
   nur als Pflicht-Block.

## Folgen

- Fällt das Ende der Widerrufsfrist auf einen Samstag, Sonntag oder einen
  bundesweiten Feiertag, endet sie am nächsten Werktag (§ 193 BGB). Regionale
  Feiertage kennt nur School Calendar; dort endet die Frist einen Tag früher,
  als sie könnte. Eine späte Belehrung verlängert die Frist nie über 12 Monate
  und 14 Tage nach dem Abschluss hinaus.

- Die Komposition von `modules/carecontract/compose` wächst um die
  Konstruktoren der Erklärungen (+35 LOC) und `workflows/parentportal/compose`
  um den Port (+1 LOC). Zwei neue Träger-Transaktionen der Erklärungen im
  Büro stehen im Register der Transaktionsgrenzen.
- Eine Änderung der Buchung nach der Kündigung erzeugt keine neue Version:
  der Lauf der Änderungen liest nur aktive Verträge.
- Wertersatz nach einem Widerruf bei verlangtem früherem Beginn
  (§ 357a Abs. 2 BGB) rechnet die Abrechnung, nicht dieser Owner.

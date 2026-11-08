---
status: proposed
---

# Das SEPA-Mandat gehört dem Zahler der Familie, und die Rechnung hält es fest

**Status: Vorgeschlagen.**

Vorschlag vom 08.11.2026 zu [#4695](https://github.com/moto-nrw/project-phoenix/issues/4695),
Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611)
(User Stories 82, 84, 85). Er baut auf
[ADR 0167](0167-zahlungsweg-faelligkeit-und-empfaenger-entscheidet-eine-regel-und-die-rechnung-haelt-sie-fest.md)
(Zahlungsweg, Fälligkeit, Familie = Mail-Adresse der Anmeldung) und
[ADR 0119](0119-abrechnungslauf-stellt-unveraenderliche-rechnungen-aus-einem-nummernkreis-aus.md)
(ausgestellte Rechnungen ändern sich nie) auf. Er führt keinen Owner ein; die
drei neuen Tabellen gehören der Elternabrechnung
(`backend/architecture/policy/parent-billing.json`).

## Kontext

Der SJR Pforzheim zieht ab dem 01.01.2027 die Monatsbeiträge per Lastschrift
ein. Dafür braucht der Träger je Zahler ein SEPA-Mandat (Referenz, Datum der
Unterschrift, Konto), seine Gläubiger-Identifikationsnummer und eine
Vorabankündigung (Pre-Notification) auf jeder Rechnung. moto zieht kein Geld
ein. Zwei Folge-Issues schreiben und lesen dieselben Mandate: der Import der
bestehenden Mandate aus OGS Connect ([#6198](https://github.com/moto-nrw/project-phoenix/issues/6198))
und die Lastschriftdatei PAIN.008 ([#4697](https://github.com/moto-nrw/project-phoenix/issues/4697)).
Später erteilen Eltern das Mandat digital ([#4696](https://github.com/moto-nrw/project-phoenix/issues/4696)).

## Entscheidung

1. **Ein Owner, eine Capability.** `parentbilling.SEPAMandates` ist die
   einzige Stelle, die Mandate schreibt und liest. Import, Elternportal und
   Lastschriftdatei nutzen sie; `Source` (`office`, `import`, `parent`) hält
   die Herkunft fest, `SEPAMandateDebits` öffnet die IBAN für die Datei.
2. **Mandat je Zahler der Familie** (`parent_billing.sepa_mandates`). Schlüssel
   ist wie bei der Abweichung die Familie (`parentbilling.FamilyKey`, Mail
   der Vertragspartei); das Mandat hängt am Vertrag, für den es erteilt wurde,
   und geht mit ihm. Je Familie ist höchstens ein Mandat in Kraft
   (`uq_sepa_mandates_in_force`). Ein Widerruf beendet es zum Tag des
   Eingangs; das widerrufene Mandat bleibt als Historie und ändert sich nie
   wieder (Trigger `refuse_sepa_mandate_rewrite`). Referenz, Familie und
   Datum der Unterschrift ändern sich nie; das Konto eines Mandats in Kraft
   darf sich ändern (Mandatsänderung).
3. **Art und Folge.** `kind` ist Dauer- (`recurring`) oder Einzelmandat
   (`one_off`), `sequence` die nächste Einziehung (`first`, `recurring`). Der
   Sequenztyp der Datei folgt daraus: OOFF, FRST, RCUR. Übernommene Mandate,
   die schon genutzt wurden, kommen als `recurring` (#6198). Den Wechsel
   FRST → RCUR nach dem ersten Einzug setzt #4697: Er schreibt
   `last_collected_on` und die Aktion `collect` in den Trail.
4. **Bankdaten.** IBAN und Kontoinhaber werden mit dem Feld-Cipher
   (AES-256-GCM, Schlüsselring `HEALTH_DATA_KEYS` aus #4722) versiegelt,
   gebunden an Feld, Träger und Mandatsreferenz. Lesbar ist von der IBAN nur
   die Maske (Land und letzte vier Zeichen). Keine Route gibt eine IBAN aus;
   keine Fehlermeldung und kein Log nennt sie. Das SQL-Log von `DB_DEBUG`
   schwärzt Bytea-Literale und IBAN-förmige Werte. IBAN-, BIC-, Gläubiger-ID- und Referenzprüfung stehen einmal
   im Shared Kernel (`sharedkernel/sepa`); die Bankverbindungen des Trägers
   (Organisation & Tenancy) nutzen dieselbe Prüfung.
5. **Audit.** Jedes Lesen und Ändern steht ohne Namen und Werte in
   `parent_billing.sepa_mandate_events` (nur angefügt: `phoenix_tenant` hat
   `SELECT, INSERT`). Ein Lauf, der die Maske zeigt, schreibt `read` mit der
   Quelle `run`; das Öffnen für die Datei `reveal`. Auch eine Änderung ohne
   Unterschied und das Lesen des Trails selbst schreiben `read`, weil ihre
   Antwort das Konto zeigt.
6. **Gläubiger und Vorlauf** (`parent_billing.sepa_creditors`): die
   Gläubiger-ID, der Vorlauf der Vorabankündigung (Standard 14 Tage, 1–30
   laut Vereinbarung) und das Verhalten ohne Mandat. Sie stehen bei den
   Abrechnungsvorgaben neben den Bankverbindungen. Ein Mandat wird der
   Gläubiger-ID erteilt, die beim Erfassen gilt (`creditor_id`, unveränderlich);
   ohne Gläubiger-ID lässt sich keines erfassen
   (`parent_billing.sepa_creditor_missing`). Ändert der Träger seine
   Gläubiger-ID, bleiben die alten Mandate, wie sie sind, und brauchen ein
   neues Mandat.
7. **Eine Regel.** `parentbilling.ResolveDebit` ergänzt das Ergebnis von
   `ResolvePayment`: Eine Lastschrift trägt das Mandat in Kraft, das am
   Rechnungsdatum unterschrieben war und derselben Gläubiger-ID gilt, die der
   Träger jetzt hat. Sonst trägt sie den Grund (`debit_gap`:
   `mandate_missing`, `creditor_missing`, `creditor_changed`).
8. **Ohne gültiges Mandat: Hinweis plus Überweisung** (Einstellung
   `missing_mandate`, Standard `transfer`). Die Rechnung geht als Überweisung
   mit dem Datum, das `ResolvePayment` einer Überweisung gibt; die
   Prüfphase (Monats- und Ferienlauf) zeigt die Familie als Hinweis. Mit `block` lehnt die Freigabe
   den Lauf ab (`parent_billing.direct_debit_without_mandate`). Beides steht im
   Fingerabdruck, weil die Zahlung der Rechnung Teil davon ist.
9. **Snapshot.** Die Freigabe kopiert Mandats-ID, Referenz, Datum, Gläubiger-ID,
   maskierte IBAN und Sequenztyp auf die Rechnung (Monats- und Ferienrechnung),
   außerdem den Grund einer Rückfallüberweisung. Das Schreiben druckt daraus die
   Vorabankündigung: Betrag, Einzugsdatum, Konto (maskiert), Gläubiger-ID,
   Mandatsreferenz. Ältere Rechnungen behalten NULL.

## Folgen

- Eine Familie ohne Mail-Adresse in der Anmeldung kann kein Mandat haben
  (`parent_billing.family_unknown`), wie bei ADR 0167.
- Die Prüfphase meldet einen Vorlauf, der kürzer ist als eingestellt
  (Rechnungsdatum bis Einzug, `notice_short`). Das Einzugsdatum selbst bleibt
  die Fälligkeit aus ADR 0167; der Hinweis hält nichts an.
- Ein SEPA-Mandat verfällt nach 36 Monaten ohne Einzug. Das braucht den
  letzten Einzug aus #4697 und folgt dort; `last_collected_on` steht dafür
  schon im Modell.
- Ein eigener Schlüsselring für Bankdaten wäre sauberer getrennt, verlangt
  aber eine neue Pflichtvariable in jeder SOPS-Umgebung. Er bleibt ein
  möglicher späterer Schritt (Rotation über die Schlüssel-ID des Cipher).

## Register

- Compose-LOC `modules/parentbilling/compose`: der Konstruktor
  `NewSEPAMandates` (Entscheidung 0171).
- Query-Budget `api.traeger.sepa_mandates` (exakt, flach in der Zahl der
  Mandate): Träger-Bindung, Mandate, Parteien der Verträge, Eintrag im Trail.

## Verworfene Optionen

- **Gläubiger-ID in den Stammdaten von Organisation & Tenancy:** Jeder Lauf
  bräuchte einen weiteren Port und eine weitere Abfrage; die Gläubiger-ID
  dient nur der Lastschrift und gehört zur Abrechnung.
- **Mandat am Elternteil der Schule (`users.guardian_financial_data`):** Die
  Abrechnung des Trägers kennt keine Bezugspersonen der Schule; die Familie
  ist die Vertragspartei (ADR 0167).
- **Ausstellung immer blockieren:** Ein fehlendes Mandat hielte den ganzen
  Lauf an; als Einstellung bleibt es wählbar.
- **IBAN im Klartext neben einer Maske:** widerspricht dem Schutz at rest.

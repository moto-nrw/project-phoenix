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

## Ergänzung #6252: Getrennte Schlüsselringe

[ADR 0183](0183-bankdaten-haben-einen-eigenen-schluesselring-und-einen-wiederholbaren-rekey.md)
ersetzt die gemeinsame Schlüsselverwendung aus Punkt 4 und dem entsprechenden
Folgepunkt: Bankdaten nutzen einen eigenen Pflichtring mit explizitem,
vorübergehendem Legacy-Lesepfad und wiederholbarem Rekey. Fachliche Mandats-
Unveränderlichkeit und Audit bleiben erhalten.

## Ergänzung 2026-11-09 (#4696): Eltern erteilen das Mandat in der Eltern-App

Entscheidung Yannick vom 09.11.2026 zu
[#4696](https://github.com/moto-nrw/project-phoenix/issues/4696) (Option B der Recherche):

1. **Nur in der Eltern-App, kein Link mit Token.** Erteilen, Lesen und Widerrufen
   prüfen dasselbe wie das Unterschreiben im Postfach: Beziehung mit
   `parent_portal.enrollments.view` zum Lesen, `parent_portal.enrollment.submit`
   zum Erteilen und Widerrufen; der Vertrag ging an die Adresse des Kontos, und
   die Familie des Mandats (Rechnungsempfänger des Vertrags) ist diese Adresse.
2. **Schalter des Trägers** `parent_billing.online_mandate`, Standard aus. Ob die
   Bank ein online bestätigtes Mandat (Textform) annimmt, klärt der Träger mit
   seiner Bank. Aus heißt: Lesen antwortet nur „aus“, Schreiben wird abgelehnt.
3. **Nachweis.** moto rendert den Mandatstext (Version `sepa-core-de-1`, Pflicht-
   angaben nach EPC SDD Core Rulebook DS-01 4.7.2), die Eltern bestätigen dessen
   SHA-256. Beim Erteilen rendert moto den Text erneut und speichert nur bei
   gleicher Prüfsumme. `parent_billing.sepa_mandate_consents` hält Version,
   Prüfsumme, Kanal `parent_account`, Konto, Zeitpunkt, die Bestätigung als
   Kontoinhaber und den exakt bestätigten Text, versiegelt im Bank-Schlüsselring.
   Spätere Kontokorrekturen ändern diesen Text nicht. Die Anfragerolle darf
   Nachweise nur lesen und einfügen; der Trigger schützt alle Nachweisfelder.
   Nur der privilegierte Rekey darf das Chiffrat unter gleichem AAD umschlüsseln,
   ohne Text, Prüfsumme oder andere Nachweisfelder zu ändern. Der Text nennt
   die IBAN maskiert. Damit eine andere IBAN mit gleicher Maske nicht mit
   derselben Bestätigung durchgeht, gibt die Vorschau die volle IBAN als
   versiegeltes Token zurück (Bankring, AAD: Träger, Konto, Referenz,
   Prüfsumme); die Erteilung öffnet es und vergleicht. Der versiegelte
   Nachweis enthält zusätzlich die volle IBAN und bindet so genau ein Konto;
   Eltern sehen nur den bestätigten Text mit Maske.
   Der Postfachhinweis wird gesammelt ohne Entschlüsselung oder Nachweisabruf
   gelesen. Die Gültigkeitsanzeige folgt denselben Regeln wie der Einzug.
4. **Vorläufig** nur für ein Konto, dessen Inhaber der Elternteil bestätigt. Ein
   Widerruf endet am Tag des Widerrufs; der nächste Lauf zieht nicht mehr ein,
   Lastschriften ohne eingereichte Datei bleiben draußen (`DebitLeftRevoked`).
5. **Keine Kopplung an `guardian_financial_data`** (siehe Verworfene Optionen).

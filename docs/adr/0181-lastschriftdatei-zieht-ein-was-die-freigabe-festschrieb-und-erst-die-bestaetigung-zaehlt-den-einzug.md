---
status: proposed
---

# Die Lastschriftdatei zieht ein, was die Freigabe festschrieb, und erst die Bestätigung zählt den Einzug

**Status: Vorgeschlagen.**

Vorschlag vom 08.11.2026 zu [#4697](https://github.com/moto-nrw/project-phoenix/issues/4697),
Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611) (User Story 86).
Er baut auf [ADR 0171](0171-sepa-mandat-gehoert-dem-zahler-der-familie-und-die-rechnung-haelt-es-fest.md)
(Mandat, Gläubiger-ID, Vorabankündigung),
[ADR 0167](0167-zahlungsweg-faelligkeit-und-empfaenger-entscheidet-eine-regel-und-die-rechnung-haelt-sie-fest.md)
(Fälligkeit je Zahlungsweg) und
[ADR 0119](0119-abrechnungslauf-stellt-unveraenderliche-rechnungen-aus-einem-nummernkreis-aus.md)
(ausgestellte Rechnungen ändern sich nie) auf. Kein neuer Owner: die Tabelle
`parent_billing.sepa_debits` gehört der Elternabrechnung.

## Kontext

Der SJR Pforzheim zieht ab dem 01.01.2027 per Lastschrift ein. Die Kämmerei
lädt je ausgestelltem Lauf eine SEPA-Lastschriftdatei herunter und reicht sie
bei ihrer Bank ein; moto zieht kein Geld ein. Die Datei darf nur gültige
Mandate enthalten, muss erste und Folgelastschrift unterscheiden und die
Kontodaten verwenden, die bei der Ausstellung galten. Ein Mandat verfällt nach
36 Monaten ohne Einzug. Laut Entscheidung im Issue entsteht die Datei zunächst
ohne Eintrag im Exportprotokoll; der folgt mit
[#4693](https://github.com/moto-nrw/project-phoenix/issues/4693).

## Entscheidung

1. **Format: pain.008.001.08 nach Anlage 3 des DFÜ-Abkommens.** Die Deutsche
   Kreditwirtschaft (DK) schreibt für SEPA-Lastschriften seit November 2025
   die ISO-20022-Version 2019 vor: pain.008.001.08 mit dem technischen
   Validierungs-Subset `pain.008.001.08_GBIC_5` (Anlage 3, Version 3.9; die ab
   15.11.2026 gültige Version 26.11 ändert die Subsets nicht). Das Vorgänger-Subset
   GBIC_4 wird nur noch bis November 2026 angenommen, pain.008.001.02 gar nicht
   mehr. Die Datei nutzt SEPA Core (`CORE`), Entgeltregel `SLEV`, je Sequenztyp
   und Einzugstag einen Sammler (`PmtInf`). Die `EndToEndId` ist die
   Rechnungsnummer, wenn sie ungekürzt im SEPA-Zeichensatz steht, sonst
   `MOTO-D<ID der Lastschrift>`, bei einer Bescheidrate
   `MOTO-N<Bescheid-ID>-<JJJJMM>`. Zwei gleiche IDs in einer Datei lehnt moto
   ab (`sepa_end_to_end_duplicate`), statt eine Datei zu schreiben, deren
   Einzüge die Bank nicht unterscheiden kann. Kennungen halten den SEPA-Zeichensatz; Namen und
   Verwendungszweck den DK-Zeichensatz mit Umlauten und ß, andere Akzente
   fallen weg. Ohne BIC steht `NOTPROVIDED` (IBAN-only im EWR).
2. **Prüfung gegen das Schema.** Ein Test validiert jede Datei mit `xmllint`
   gegen das offizielle ISO-Schema `pain.008.001.08.xsd` (Fixture im Repo,
   frei nutzbar nach den ISO-20022-Nutzungsbedingungen). Das DK-Subset GBIC_5
   von ebics.de ist nur zum persönlichen Gebrauch freigegeben und liegt nicht
   im Repo; mit `SEPA_DK_XSD` prüft derselbe Test zusätzlich dagegen (lokal
   ausgeführt).
3. **Festschreiben bei der Freigabe.** Die Freigabe eines Monats- oder
   Ferienlaufs schreibt je Rechnung per Lastschrift mit Mandat und Betrag über
   0 € eine Zeile in `sepa_debits`: Betrag (was die Familie nach dem
   Kostenträger-Anteil zahlt, ADR 0167), Fälligkeit aus der Regel bzw. dem
   Lauf, Sequenztyp, Gläubiger-ID, Mandatsreferenz, Datum der Unterschrift und
   die versiegelten Kontodaten des Mandats (Kontoinhaber, IBAN, BIC). Die
   Siegel werden ungeöffnet kopiert: Sie sind an Träger und Mandatsreferenz
   gebunden, und die Referenz ändert sich nie. Eine spätere Mandatsänderung
   erreicht die Datei eines ausgestellten Laufs nicht. Den Sequenztyp liest
   die Freigabe unter Sperre des Mandats (`FOR UPDATE`), nicht aus der
   Vorschau: Bestätigt die Kämmerei gleichzeitig einen anderen Lauf, wartet
   die Freigabe und schreibt RCUR. Der Trigger
   `refuse_sepa_debit_rewrite` hält Betrag, Konto und Rechnung fest.
4. **Nur gültige Mandate.** Beim Erzeugen der Datei bleibt eine offene
   Lastschrift draußen und wird mit Grund angezeigt, wenn ihre Rechnung
   storniert oder voll gutgeschrieben ist, das Mandat widerrufen ist, die
   Gläubiger-ID des Trägers inzwischen eine andere ist oder das Mandat am
   Einzugstag verfallen ist. Eine Teilgutschrift einer Ferienrechnung senkt den
   Betrag. Ausgeschlossene Rechnungen der Prüfphase werden nie ausgestellt und
   haben keine Zeile; Kostenträger-Anteile sind nicht Teil des Betrags.
5. **Download ändert nichts; erst die Bestätigung zählt den Einzug.** Der
   Download liest nur und schreibt `reveal` in den Verlauf jedes Mandats,
   dessen Konto er öffnet. Die Datei ist eine reine Funktion der offenen
   Lastschriften, ihrer Mandate und des Tages; ihre Nachrichten-ID ist ein
   Fingerabdruck daraus. Ein doppelter Download liefert dieselbe Datei mit
   derselben ID; reicht die Kämmerei sie zweimal ein, lehnt die Bank die
   zweite als Dublette ab. Die ausdrückliche Bestätigung „Bei der Bank
   eingereicht“ nennt die ID der Datei, die die Kämmerei gesehen hat. In einer
   Transaktion (mit Sperre der Zeilen) werden die Lastschriften `submitted`,
   jedes Mandat bekommt `last_collected_on` (spätester Einzugstag), ein
   Dauermandat wechselt von FRST auf RCUR (`sequence = recurring`), offene
   FRST-Lastschriften desselben Mandats in anderen Läufen werden RCUR, und der
   Verlauf bekommt `collect`. Hat sich die Datei seit dem Download geändert,
   lehnt die Bestätigung mit `sepa_debits_changed` ab; eine zweite Bestätigung
   findet nichts Offenes (`sepa_debits_empty`).
   Verworfen: **Wechsel beim Download.** Ein zweiter Download nach einem
   Fehler der Bank hätte die Folgelastschrift als RCUR geschrieben, obwohl die
   erste nie eingezogen wurde, und `last_collected_on` hätte einen Einzug
   behauptet, den es nicht gab.
6. **Einzugstag.** Eine Lastschrift wird an ihrer Fälligkeit eingezogen,
   frühestens morgen, weil die Bank einen vergangenen Tag ablehnt. Der
   verschobene Tag geht in die Nachrichten-ID ein und wird angezeigt.
7. **Verfall.** Ein Mandat gilt bis 36 Monate nach seinem letzten Einzug,
   ohne Einzug bis 36 Monate nach der Unterschrift. Ein Mandat, das schon
   anderswo genutzt wurde (Folge `recurring`, vom Büro erfasst oder
   übernommen, #6198) und in moto noch nicht eingezogen ist, zählt ab dem Tag
   der Übernahme: moto kennt seinen letzten Einzug nicht und darf es nicht
   allein wegen eines alten Unterschriftsdatums als verfallen werten.
   `ResolveDebit` prüft am Rechnungsdatum und behandelt ein verfallenes
   Mandat wie ein fehlendes (`debit_gap = mandate_expired`: Hinweis plus
   Überweisung oder Sperre der Freigabe, ADR 0171 Punkt 8); die Datei prüft
   den Einzugstag noch einmal. Die Mandatsliste zeigt „Gültig bis“ und
   „Verfallen“.
8. **Einzugshistorie.** `sepa_debits` ist die Historie je Mandat und
   Rechnung: Rechnungsnummer, Betrag, Einzugstag, Sequenztyp, Stand und wann
   eingereicht. Die Mandatsseite zeigt sie (`GET
   /traeger/sepa-mandates/{id}/collections`), der Lauf seine Datei (`GET
   /traeger/sepa-debits/{care-runs|holiday-runs}/{id}`).
9. **Bankdaten.** Die Datei verlässt den Server nur als Antwort auf den
   Download (`Cache-Control: no-store`) und wird nirgends abgelegt. Keine
   andere Antwort, kein Fehler und kein Log nennt eine IBAN der Familien; die
   Übersicht zeigt das Konto des Trägers nur maskiert.

## Bescheidraten (Jahresbescheid, #6243)

Mit dem Jahresbescheid (#6243, ADR 0182) stellt der Monatslauf für Monate mit
Bescheid keine Rechnung aus. Ein Monat ist je Vertrag entweder Bescheid- oder
Rechnungsmonat, eine Doppelung zwischen Rechnung und Rate gibt es also nicht.
Ein Monatslauf ohne Rechnung lässt sich nicht freigeben (`billing_run_empty`).
Deshalb haben die Raten **eine eigene Datei je Fälligkeitsmonat**
(`run_kind = 'notice'`, Lauf-ID `JJJJMM`, `GET
/traeger/sepa-debits/notice-months/{JJJJMM}`). Die Kämmerei findet sie auf einer eigenen Seite des freigegebenen
Jahreslaufs (`/traeger/abrechnung/jahresbescheide/{id}/lastschrift`), die der
Kopf des Laufs verlinkt; die Seite des Laufs lädt die Karte nicht mit.

- Die einzige Quelle der Raten ist dieselbe Lesung und Auflösung wie in
  `CareFeeNotices.EffectiveNoticeDues(ctx, organizationID, month)`: die
  Store-Abfrage `EffectiveNoticeDues` und `effectiveDues`. Die Datei löst Erst-
  und Änderungsbescheide nicht selbst auf. In die Datei
  kommen nur Raten per Lastschrift mit einem Betrag über 0 €. Der Betrag ist
  der Familienanteil nach dem Kostenträger.
- Bis zur Bestätigung wird eine Rate live geplant. Das gilt für das gültige
  Mandat der Familie (über `family_email`), den Sequenztyp und das versiegelte
  Konto. Ohne gültiges Mandat bleibt die Rate mit `mandate_missing` draußen.
  Widerruf, Verfall, eine andere Gläubiger-ID und ein schon genutztes
  Einmalmandat behandelt die Datei wie bei Rechnungen. Eine Änderung des
  Mandats (`updated_at`) geht in die Nachrichten-ID ein.
- Erst die Bestätigung schreibt die Zeilen (`submitted`, `care_notice_id`,
  `notice_month`). Dabei sperrt sie das Mandat und kopiert sein Siegel
  ungeöffnet. Danach gelten `last_collected_on`, FRST → RCUR und die
  Einzugshistorie wie bei Rechnungen. Der eindeutige Index
  `uq_sepa_debits_notice_month (organization_id, care_contract_id,
  notice_month)` verhindert einen zweiten Einzug derselben Rate. Eine
  eingereichte Rate fehlt in der nächsten Datei des Monats.
- Die Checks verlangen: Eine Bescheidzeile ist immer `submitted`, sie hat eine
  Bescheid-ID, ihr Monat ist ein Monatserster, und die Lauf-ID ist ihr Monat.

## Migration zurück

Das Down der Migration 1.15.826 setzt `debit_gap = mandate_expired` auf
`mandate_missing` zurück, in `care_invoices` und in `documents`. Erst danach
stellt es die alten Checks wieder her. Für diese Zeit sind die Schutz-Trigger
der ausgestellten Rechnungen ausgeschaltet. Ein Test prüft die Folge Up →
Daten → Down → Up.

## Folgen

- Der Eintrag im Exportprotokoll folgt mit #4693; Download und Bestätigung
  bleiben die beiden Stellen, an denen er ansetzt.
- Rücklastschriften (pain.002/camt.054) liest moto nicht; die Bestätigung
  zählt den eingereichten Einzug.
- Für Konten außerhalb des EWR verlangt das Regelwerk zusätzlich die
  Anschrift des Zahlers. moto schreibt sie nicht; solche Mandate sind beim SJR
  nicht bekannt.
- Bestehende freigegebene Läufe mit Lastschrift bekommen ihre Zeilen von der
  Migration aus dem Mandat, wie es jetzt ist; für sie gab es nie eine Datei.

## Register

- Compose-LOC `modules/parentbilling/compose`: der Konstruktor
  `NewSEPADebits` (+5).
- Query-Budgets `api.traeger.sepa_debits` und `api.traeger.sepa_mandate_collections`.

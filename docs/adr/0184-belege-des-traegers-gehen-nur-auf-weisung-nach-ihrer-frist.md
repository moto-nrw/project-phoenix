---
status: proposed
---

# Belege des Trägers gehen nur auf Weisung nach ihrer Frist

**Status: Vorgeschlagen.** Vorschlag vom 09.11.2026 zu
[#4720](https://github.com/moto-nrw/project-phoenix/issues/4720), Slice 2
„Frist je Beleg“, Teil von #4611. Grundlage ist das Löschkonzept
Finanz- und Vertragsunterlagen der Träger im Legal-Repo
(`vertragspaket/intern/loeschkonzept-traeger-finanzdaten.md`, Stand
09.11.2026, Abschnitte 2, 3 und 6). Die Entscheidung wendet
[ADR 0073](0073-inhalte-der-schule-folgen-der-frist-ihrer-datenart.md) und
[ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
Abschnitt 9 auf die Belege an und folgt dem Löschprotokoll des Trägers aus
[ADR 0118](0118-ferienanmeldungen-gehen-nach-drei-monaten-mit-eintrag-im-loeschprotokoll-des-traegers.md).
Sie führt keinen neuen Owner ein.

## Kontext

Rechnungen, Gutschriften, Stornobelege, Monatsrechnungen, Jahresbescheide,
Betreuungsverträge, SEPA-Mandate und Lastschriften blieben bis zur Löschung
des Trägers. Der Träger ist Verantwortlicher und bestimmt die Frist; moto
löscht nur auf dokumentierte Weisung. Für Hardt und SJR liegt noch keine
Weisung vor.

## Entscheidung

1. **Owner.** Die Elternabrechnung (`parent-billing`) hält die Aufbewahrung
   aller Belege eines Trägers: Weisungen je Belegart, Aussetzungen, den
   Aufbewahrungsdatensatz je Beleg und das Löschprotokoll
   (`parent_billing.beleg_retention_instructions`, `…_suspensions`,
   `beleg_retentions`, `beleg_deletions`). Sie besitzt fünf der sechs
   Belegarten selbst. Die Richtung `parent-billing → care-contract` steht in
   ADR 0088 Abschnitt 10 und besteht schon über `parentbilling.CareContracts`.
2. **Betreuungsvertrag.** Care Contract bietet `ContractRetention`:
   `RetentionFacts` (Abschluss, Ende und dessen Tag, keine Partei, kein Kind)
   und den geschützten Befehl `DeleteEndedContracts`. Der Befehl tritt der
   Admin-Transaktion des Aufrufers bei, löscht nur beendete, gekündigte oder
   widerrufene Verträge, deren letzter Tag am Stichtag vorbei ist, samt Spur,
   und meldet die gelöschten IDs. Das ist eine Befehlsbeteiligung nach
   ADR 0067 (Nachtrag #4538): eigene Prüfung beim Owner, keine Sperrfolge,
   kein Nachlesen unter fremden Sperren, nichts nach dem Commit. Die Root
   bindet den Port `parentbilling.RetainedContracts`.
3. **Fristen.** Rechnungen, Storno- und Gutschriftbelege, Monatsrechnungen
   (mit ihrem Storno: das spätere Ausstellungsdatum zählt) und Lastschriften
   (Einreichungstag): Jahre nach Ende des Ausstellungsjahres; vorgeschlagen
   sind 8 (Beleg aus 2026 frühestens am 01.01.2035). Jahresbescheide und
   Verträge: keine Voreinstellung, die Weisung nennt 6 oder 8 Jahre und das
   Ereignis (Ausstellung; Vertrag: Abschluss oder Ende). Ein Vertrag in Kraft
   bleibt immer. SEPA-Mandat: gültig bleibt es; nach Erlöschen (Widerruf oder
   36 Monate ohne Einzug) mindestens 14 Monate ab Einreichung der letzten
   eingezogenen Lastschrift. Ohne Einzug oder ohne Tag des Ereignisses gibt es
   kein berechnetes Datum.
4. **Klärungsliste.** Ohne Weisung der Belegart oder ohne berechenbares Datum
   steht der Beleg auf der Klärungsliste mit Aufnahmetag, Prüfdatum 30 Tage
   danach und Zuständigkeit (Florian, Abstimmung mit dem Träger). Ein
   überschrittenes Prüfdatum zeigt die Eskalation. Kein Beleg auf der Liste
   wird gelöscht.
5. **Aussetzung.** Eine Aussetzung für einen Beleg oder eine ganze Belegart
   hält Grund, Umfang, Zuständigkeit und das nächste Prüfdatum fest und hält
   die Löschung an, bis sie aufgehoben ist.
6. **Lauf.** Der stündliche Aufräumjob bewertet einmal am Tag jeden Beleg und
   schreibt den Datensatz neu. Danach löscht er nur Belege mit Weisung, deren
   Frist abgelaufen ist und von denen nichts mehr aufbewahrt wird, was die
   Löschung per Cascade mitnähme: Eine Lastschrift hält ihre Rechnung, ihren
   Bescheid und ihr Mandat; ein Storno oder eine Gutschrift vor Fristende hält
   die Rechnung, auf die sie verweist; ein Änderungsbescheid hält seinen
   Bescheid; eine ersetzende Monatsrechnung hält die ersetzte; Mandat oder
   Lastschrift halten ihren Vertrag. Freigegebene Läufe ohne Beleg gehen mit
   ihren Briefangaben. Gelöscht wird, nicht ausgeblendet.
7. **Bankdaten und Rechte.** Die SEPA-Tabellen bleiben für `phoenix_admin`
   geschlossen. Lesen und Löschen der Belege laufen über die SECURITY-DEFINER-
   Funktionen `parent_billing.beleg_retention_facts`, `delete_due_belege` und
   `due_retained_contracts`, die nur `phoenix_admin` aufrufen darf; sie lesen
   keine Bankdaten und weisen einen Stichtag nach morgen ab. Die Anfrage-Rolle
   hat kein Recht auf die neuen Tabellen.
8. **Protokoll.** Jede Löschung schreibt Träger, Belegart, Tag und Anzahl in
   `parent_billing.beleg_deletions`, ohne Inhalt. Einträge gehen nach drei
   Jahren oder mit dem Träger.
9. **Produktion.** Ohne Weisung löscht der Lauf nichts; er füllt nur die
   Klärungsliste. Weisungen trägt das moto-Team im Operator-Portal ein
   (#4720, eigener PR). Eine Oberfläche für den Träger gibt es nicht.

## Verworfene Optionen

- **Aufbewahrung je Owner** (Care Contract hält Weisungen und Datensätze der
  Verträge): zwei Weisungstabellen, zwei Klärungslisten und zwei Läufe für
  eine Zuständigkeit; ein Vertrag ließe sich löschen, während Mandat und
  Lastschriften der Elternabrechnung per Cascade mitgingen.
- **Neuer Workflow-Owner:** braucht eine eigene ADR und koordiniert nichts,
  was ein beteiligter Befehl nicht schon leistet.
- **`phoenix_admin` Rechte auf die SEPA-Tabellen geben:** öffnete die
  versiegelten Bankdaten für jede Admin-Transaktion.

## Folgen

- Pass-through-Evidenz: `BelegRetention.RecordSuspension` und
  `.LiftSuspension` prüfen und bewerten den Träger sofort neu;
  `retainedContracts.ContractRetentionFacts` übersetzt den Vertrag von Care
  Contract in den Port. Die Löschung bindet die Root ohne Weiterleitung.
- Budgets: Pass-throughs `modules/parentbilling` 9 → 11 (Gesamt 1404 →
  1406) für die beiden Aussetzungsbefehle; Compose-LOC
  `modules/parentbilling/compose` 96 → 102, `modules/carecontract/compose`
  178 → 182, `modules/organizationtenancy/compose` 309 → 320 (Gesamt
  13158 → 13179) für die drei Konstruktoren.
- Operator-Audit: Lesen über die Lese-Audit-Sicht `beleg_retention`, jede
  Änderung über die eigene Capability `BelegRetentionChangeAudit` von
  Organisation & Tenancy in derselben Admin-Transaktion wie die Änderung. Sie
  steht neben `Provisioning`, dessen Methodenmenge unverändert bleibt.
- Offen: Rückgabe und Hinweis vor Trägerlöschung (Slice 3); Weisungen von
  SJR und Hardt; eine Kündigung mit offener außerordentlicher Entscheidung
  und ein `beendet` ohne letzten Tag stehen auf der Klärungsliste.

# Aufbewahrungs- und Löschfristen für personenbezogene Daten

**Stand:** 12.10.2026\
**Quellen:** DSGVO, BGB, AO, HGB, UStG, EStG, SGB IV, SGB VII, SGB VIII, IfSG, ArbZG, MiLoG, AGG, BZRG, BDSG (gesetze-im-internet.de, recht.bund.de); SchulG NRW, VO-DV I, OGS-Erlass 2026, KiBiz, KomHVO NRW, ANBest-P/-G NRW, JustG NRW (recht.nrw.de, bass.schule.nrw); BMF (GoBD); DSK (SDM, Kurzpapier 11); EDSA; BfDI, LDI NRW, BayLDA, BayLfD und weitere Landesbehörden; BSI (Mindeststandard Protokollierung, IT-Grundschutz); DGUV und Unfallkasse NRW. DIN 66398 und ISO/IEC 27555 nur über Abstracts und Sekundärliteratur, gekennzeichnet.\
**Scope:** Deutschland, Schwerpunkt Offener Ganztag (OGS) an Grundschulen in NRW. Für Schuldaten ist moto Auftragsverarbeiter (Art. 28 DSGVO), für eigene Plattformdaten (Operator-Konten, Betriebslogs, eigene Buchhaltung) Verantwortlicher. Die Notiz sammelt gesetzliche Mindest- und Höchstfristen, Behördenaussagen und übliche Praxis je Datenart.\
**Hinweis:** Dies ist keine Rechtsberatung. Über die Fristen entscheidet der Verantwortliche (Träger oder Schule), bei Bedarf mit seiner Datenschutzberatung.

Verwandte Notiz: [kita-guardian-privacy-and-change-workflows.md](kita-guardian-privacy-and-change-workflows.md) behandelt Sichtbarkeit von Abwesenheitsgründen und Freitext zwischen Sorgeberechtigten, aber keine Fristen. Ihr Punkt, dass Freitext zu einer Abwesenheit Gesundheits- oder Familiendaten enthalten kann, gilt hier für die Speicherdauer entsprechend (Abschnitt 2).

## Kurzurteil

1. **Für die OGS-Kerndaten gibt es keine gesetzliche Aufbewahrungsfrist.** Anwesenheit, Raumbesuche, Krankmeldungen, Nachrichten und Kinderstammdaten eines freien Trägers unterliegen nur der Speicherbegrenzung (Art. 5 Abs. 1 lit. e, Art. 17 DSGVO). Die einzigen schulrechtlichen Fristen (VO-DV I § 9: Klassenbuch 10 Jahre, übrige Schülerdaten 5 Jahre) gelten für Schulen, nicht für Träger. Auch der seit 01.08.2026 geltende OGS-Erlass regelt weder Anwesenheitsdokumentation noch Löschfristen.
2. **Harte Mindestfristen kommen aus Steuer-, Sozialversicherungs-, Arbeits- und Zuwendungsrecht:**
   - Buchungsbelege und Rechnungen 8 Jahre (Bürokratieentlastungsgesetz IV, seit 01.01.2025, auch für Altbelege mit noch laufender Frist).
   - Bücher und Jahresabschlüsse 10 Jahre, Geschäftsbriefe 6 Jahre, Lohnkonto 6 Jahre.
   - Arbeitszeitnachweise mindestens 2 Jahre.
   - Zuwendungsbelege 5 Jahre nach Vorlage des Verwendungsnachweises.
   - Kommunen nach KomHVO NRW: Bücher 10 Jahre, Belege 6 Jahre.
3. **Harte Höchstgrenzen gibt es bei Personaldaten:**
   - Erweitertes Führungszeugnis: nur drei Datenpunkte speichern, kein Zeugnis und keine Kopie, Löschung spätestens **6 Monate** nach der letzten Tätigkeit (§ 72a Abs. 5 SGB VIII, § 30a Abs. 3 BZRG).
   - Masernschutz: nur ein Vermerk „liegt vor, Datum“, keine Kopie (LDI NRW).
   - Abgelehnte Bewerbungen: höchstens 6 Monate (BfDI, BayLDA).
4. **Beweissicherung trägt keine pauschal lange Speicherung von Anwesenheitsdaten:**
   - Personenschäden von OGS-Kindern sind über die gesetzliche Unfallversicherung abgedeckt (§ 2 Abs. 1 Nr. 8 lit. b SGB VII), zivilrechtliche Ansprüche wegen Fahrlässigkeit weitgehend ausgeschlossen (§§ 104 bis 106 SGB VII).
   - § 207 BGB hemmt die Verjährung nur zwischen Kind und Eltern, nicht gegenüber Träger oder Personal.
   - Lange Fristen bleiben für Vorsatz und sexuellen Missbrauch (§ 197 Abs. 1 Nr. 1, § 208 BGB) und den Rückgriff des Unfallversicherungsträgers relevant.
   - DSK, BayLDA und EDSA verlangen eine Abwägung und lehnen die längste denkbare Frist als Pauschale ab.
5. **Für Logs eines SaaS-Anbieters schreibt kein Gesetz eine Frist vor.** Die Behördenanker sind:
   - BSI-Mindeststandard: „bspw. 90 Tage“ für Sicherheitsprotokolle, verbindlich nur für Bundesstellen.
   - BfDI: 5 Wochen für Logfile-Statistik.
   - Ältere Aussagen von ULD und LfDI RLP: 7 Tage für IP-Adressen zur Missbrauchsabwehr.
   - Löschprotokolle dürfen laut BayLDA bis zu 3 Jahre aufbewahrt werden.
6. **Backups müssen nicht einzeln bereinigt werden, wenn sie planmäßig auslaufen.** Nach einer Rücksicherung ist sofort erneut zu löschen (SDM Baustein 60, BayLfD). Die Backup-Laufzeit richtet sich nach der kürzesten enthaltenen Löschfrist (Leitlinie zu DIN 66398, Sekundärquelle).
7. **Vertragsende:** Art. 28 Abs. 3 lit. g DSGVO und die EU-Standardvertragsklauseln (Klausel 10 lit. d) verlangen Löschung oder Rückgabe nach Wahl des Verantwortlichen samt Bescheinigung, aber keine bestimmte Frist. Konkrete Tage legt der AVV fest; 30 bis 90 Tage sind übliche Praxis ohne gesetzliche Vorgabe.
8. **Methodik:** DIN 66398:2016 ist zurückgezogen und durch DIN EN ISO/IEC 27555:2025-09 ersetzt. Das Strukturprinzip bleibt nutzbar: Datenarten, Löschklassen als Matrix aus Regellöschfrist und Startzeitpunkt, Vorhaltefristen und dokumentierte Abweichungen.

## Fristen je Datenart

Abkürzungen: „Praxis“ heißt übliche Praxis ohne gesetzliche Vorgabe; die Herkunft der Einschätzung steht in der Spalte Quelle. „KJ-Ende“ heißt Schluss des Kalenderjahres.

| Datenart | Gesetzliche Pflicht (Mindest-/Höchstfrist) | Übliche Praxis | Startzeitpunkt der Frist | Quelle | Sicherheit |
|---|---|---|---|---|---|
| 1a. Tagesanwesenheit (Kommen/Gehen) | Für Träger keine. Für Schulen: Klassenbuch 10 Jahre, übrige Schülerdaten 5 Jahre (gilt nicht für OGS-Daten freier Träger). Unfalltag: Beginn und Ende des Besuchs für die Unfallanzeige | Praxis: Schuljahr plus Puffer; höchstens Regelverjährung (3 Jahre ab KJ-Ende); bei konkretem Anlass gezielt sperren | Ende des Betreuungstags bzw. Schuljahres; Schule: KJ-Ende des Abschlusses, nicht vor Ende der Schulpflicht | Art. 5 Abs. 1 lit. e DSGVO; VO-DV I § 9 Abs. 1; UVAV § 4 Abs. 3 Nr. 6; §§ 195, 199 BGB | Pflicht: hoch. Praxis: niedrig |
| 1b. Raumwechsel/Raumbesuche | Keine | Praxis: Tage bis wenige Wochen (feiner als nötig für Aufsicht über den Tag) | Erhebung | Art. 5 Abs. 1 lit. c, e DSGVO | Pflicht: hoch. Praxis: niedrig |
| 2. Krank-/Entschuldigt-Meldungen (Art. 9) | Keine für Träger. Schule: Grund des Schulversäumnisses ist zulässiges Datum; Atteste dürfen nicht automatisiert verarbeitet werden; Frist vermutlich „übrige Daten“ 5 Jahre | Praxis: Status (krank/entschuldigt) wie Anwesenheit; Freitext und Diagnosen gar nicht oder nur kurz | Ende Schuljahr bzw. Erhebung | Art. 9 Abs. 1 DSGVO; § 43 Abs. 2 SchulG; VO-DV I Anlage 1 A II Nr. 10, 19 und Fn. **; § 9 Abs. 1 | Pflicht: hoch. Zuordnung 5 Jahre: niedrig |
| 3. Nachrichten Eltern/Personal, Personalchat | Keine. Ausnahme: Nachrichten mit Funktion eines Geschäftsbriefs oder Buchungsbelegs 6 bzw. 8 Jahre | Praxis: 6 bis 24 Monate; Ende Schuljahr oder Ende Betreuung | Versand bzw. Ende der Unterhaltung | § 147 Abs. 1 Nr. 2, 3, Abs. 3 AO; GoBD Rz. 121 | Pflicht: hoch. Praxis: niedrig |
| 4a. Abgelehnte Anmeldungen | Keine. Rechtsbehelfsfristen: Klage 1 Monat, ohne Rechtsbehelfsbelehrung 1 Jahr (falls Verwaltungsakt); AGG-Ansprüche 2 Monate (falls privatrechtlich) | Praxis: 3 bis 12 Monate | Zugang der Ablehnung | § 74 Abs. 1, § 58 Abs. 2 VwGO; § 21 Abs. 5 AGG | mittel |
| 4b. Angenommene Anmeldungen | Werden Teil der Kinderakte; Anmeldezahl zum Stichtag 15.10. ist Förderbasis, daher möglicherweise Zuwendungsunterlage (5 Jahre nach Verwendungsnachweis) | Praxis: wie Kinderstammdaten | Ende der Betreuung bzw. Vorlage Verwendungsnachweis | OGS-Erlass Nr. 1.2; Förderrichtlinie BASS 11-02 Nr. 19 Nr. 6.2.2; ANBest-P NRW Nr. 6.8 | niedrig |
| 5. Kinderstammdaten nach Austritt | Keine für Träger. Schule: Schülerstammblatt 20 Jahre | Praxis: Löschung nach Ende der Betreuung, sobald Abrechnung und Förderung abgeschlossen; Finanzbelege getrennt nach Nr. 7 | Ende der Betreuung | Art. 17 Abs. 1 lit. a DSGVO; VO-DV I § 9 Abs. 1; KiBiz § 18 (Analogie) | Pflicht: hoch. Praxis: niedrig |
| 6a. Arbeitsvertrag, Personalakte | Keine allgemeine Frist. Kopie der Niederschrift nach Nachweisgesetz gehört zu den Entgeltunterlagen (bis Ende des Jahres nach der letzten Betriebsprüfung) | Praxis: 3 Jahre ab KJ-Ende des Austritts (Regelverjährung), danach nur, was andere Fristen tragen | Ende des Arbeitsverhältnisses (KJ-Ende) | § 8 Abs. 2 Nr. 6 BVV; § 28f Abs. 1 SGB IV; LfDI BW 2017; HmbBfDI | mittel |
| 6b. Lohnunterlagen | Lohnkonto 6 Jahre; Lohnbuchungsbelege 8 Jahre; Entgeltunterlagen bis Ende des Jahres nach der letzten Betriebsprüfung | Praxis: 8 Jahre als gemeinsamer Wert | KJ-Ende der letzten Lohnzahlung bzw. Belegentstehung | § 41 Abs. 1 S. 9 EStG; § 147 Abs. 1 Nr. 4, Abs. 3 AO; § 28f SGB IV | hoch |
| 6c. Arbeitszeitnachweise/Zeiterfassung | Mindestens 2 Jahre (Mehrarbeit über 8 Std.; Minijobs nach MiLoG). MiLoG-Aufzeichnungen zählen zu den Entgeltunterlagen. Allgemeine Erfassungspflicht nach BAG, aber keine gesetzliche Frist | Praxis: 2 Jahre; länger nur, wenn sie als Lohnbuchungsbeleg dienen | Zeitpunkt der Aufzeichnung | § 16 Abs. 2 ArbZG; § 17 Abs. 1 MiLoG; § 8 Abs. 2 Nr. 13 BVV; BMAS-FAQ | hoch |
| 6d. Erweitertes Führungszeugnis | Höchstgrenze: nur Einsichtnahme, Datum, Verurteilung ja/nein; unverzüglich löschen ohne Tätigkeit; spätestens 6 Monate nach der letzten Tätigkeit. Zeugnis nicht aufbewahren | Gesetz ist abschließend | Letzte Ausübung der Tätigkeit | § 72a Abs. 5 SGB VIII; § 30a Abs. 3 BZRG; OGS-Erlass Nr. 7.7; LDI NRW | hoch |
| 6e. Masernschutznachweis | Keine Frist im IfSG. Nur Vermerk „lag vor am …“, keine Kopie; bei befristetem Nachweis Ablaufdatum vermerken | Praxis: Dauer der Tätigkeit, dann mit der Personalakte | Ende der Tätigkeit | § 20 Abs. 9, 9a IfSG; LDI NRW; LfD Niedersachsen; HBDI; LfDI RLP | Kopierverbot: hoch. Dauer: niedrig |
| 6f. Protokoll IfSG-Belehrung | 3 Jahre | Gesetz | Erstellung des Protokolls | § 34 Abs. 5a IfSG; OGS-Erlass Nr. 7.8 | hoch |
| 6g. Bewerbungen Abgelehnter | Keine Frist im Gesetz. Behörden: höchstens 6 Monate (LfDI BW 2017: 3 Monate) | 6 Monate | Zugang der Ablehnung bzw. Rücknahme | § 15 Abs. 4 AGG; § 61b Abs. 1 ArbGG; BfDI; BayLDA | hoch |
| 7a. Buchungsbelege, Rechnungen, Elternbeitragsbelege | 8 Jahre (bis 2024: 10). Ablaufhemmung, solange Festsetzungsfrist offen | Gesetz | KJ-Ende der Belegentstehung | § 147 Abs. 1 Nr. 4, Abs. 3, 4 AO; § 257 Abs. 4 HGB; § 14b Abs. 1 UStG; Art. 97 § 19a EGAO | hoch |
| 7b. Bücher, Jahresabschlüsse | 10 Jahre | Gesetz | KJ-Ende der letzten Eintragung | § 147 Abs. 1 Nr. 1, Abs. 3 AO; § 257 HGB | hoch |
| 7c. Geschäftsbriefe (auch E-Mails mit dieser Funktion) | 6 Jahre | Gesetz | KJ-Ende des Eingangs/Versands | § 147 Abs. 1 Nr. 2, 3 AO; GoBD Rz. 121 | hoch |
| 7d. Elternbeiträge bei der Kommune | Bücher 10 Jahre, Belege und sonstige Unterlagen 6 Jahre, Jahresabschlüsse dauernd | Gesetz | 1. Januar nach dem Ratsbeschluss zum Jahresabschluss | § 59 Abs. 2 KomHVO NRW; § 51 Abs. 5 KiBiz; § 9 Abs. 3 SchulG | hoch |
| 8. Fördermittelbelege (OGS-Zuwendung) | 5 Jahre nach Vorlage des Verwendungsnachweises, sofern nicht steuerrechtlich länger. Gilt bei Weiterleitung auch für den Träger | Gesetz bzw. Zuwendungsbescheid | Vorlage des Verwendungsnachweises | ANBest-P NRW Nr. 6.8; ANBest-G NRW Nr. 7.5; BASS 11-02 Nr. 19 Nr. 6.5; MSB-FAQ Förderung | hoch |
| 9a. Anmeldeereignisse, Sicherheitslogs | Keine für private Stellen | Praxis: 30 bis 90 Tage; BSI „bspw. 90 Tage“ (Bund) | Erhebung | BSI MST PD 2.1 PD.2.1.04 c; SDM B43 M43.21.09; OPS.1.1.5.A5 | Pflicht: hoch. 90 Tage: mittel |
| 9b. Zugriffsprotokolle auf personenbezogene Daten | Keine außerhalb von Polizei/Justiz. Dort: Löschung am Ende des Folgejahres | Praxis: 1 Jahr bis Ende des Folgejahres (Analogie) | Erhebung | § 76 Abs. 4 BDSG; § 55 Abs. 4 DSG NRW (nur Analogie) | niedrig |
| 9c. Löschprotokolle | Keine. Nachweispflicht aus Art. 5 Abs. 2 DSGVO; Protokoll ohne die gelöschten Inhalte | BayLDA: bis 3 Jahre | Löschzeitpunkt | Art. 5 Abs. 2 DSGVO; SDM B60 M60.S05/S06; BayLDA „Löschung“ | mittel |
| 10a. E-Mail- und Push-Warteschlange, Zustellprotokolle | Keine | Praxis: Inhalt nach Zustellung löschen; Zustellstatus 7 bis 30 Tage | Zustellung bzw. endgültiger Fehlschlag | Art. 5 Abs. 1 lit. e DSGVO | niedrig |
| 10b. Einladungslinks, MFA-Codes (mit IP), Passkey-Challenges, abgelaufene Refresh-Tokens | Keine Speicherfrist. Gültigkeit: NIST nennt 10 Minuten für Out-of-band-Codes | Praxis: nach Ablauf oder Verbrauch löschen, IP-Bezug höchstens 7 bis 30 Tage | Ablauf bzw. Verbrauch | NIST SP 800-63B-4 Abschn. 3.1.3.2 (nur Gültigkeit) | niedrig |
| 10c. Server-Logs mit IP | Keine. Speicherung für Funktionsfähigkeit grundsätzlich zulässig, mit Abwägung | Praxis: 7 Tage voll, danach gekürzt; Statistik 5 Wochen | Erhebung | EuGH C-582/14; BGH VI ZR 135/13; BfDI „Logfile-Daten“; ULD-FAQ; LfDI RLP 2015 | mittel |
| 11. Backups | Löschpflicht erfasst Backups; keine Einzellöschung nötig bei planmäßigem Überschreiben; nach Rücksicherung sofort erneut löschen | Praxis: Rotation von wenigen Wochen bis 3 Monate | Erstellung des Backups | SDM B60 M60.S10, M60.P08; BayLfD TB 30 Nr. 12.5; BSI CON.3.A4, CON.6.A2; Hammer/Schuler 7.3.5 | mittel |
| 12. Vertragsende (AVV) | Löschen oder Rückgabe nach Wahl des Verantwortlichen, Löschung bescheinigen, keine feste Frist; Ausnahme bei eigener Speicherpflicht des Auftragsverarbeiters | Praxis: 30 bis 90 Tage, Backups laufen danach aus | Vertragsende bzw. Ende der Leistung | Art. 28 Abs. 3 lit. g DSGVO; Durchführungsbeschluss (EU) 2021/915 Klausel 10 lit. d; EDSA-Leitlinien 07/2020 Rn. 139 bis 142 | Pflicht: hoch. Praxis: niedrig |

### 1. Tagesanwesenheit und Raumbesuche

- **Rahmen seit 01.08.2026:** Der neue OGS-Erlass (MB.NRW 2026 Nr. 92) bindet die Anmeldung „für die Dauer eines Schuljahres“ und verpflichtet „in der Regel zur regelmäßigen und täglichen Teilnahme“ (Nr. 1.2), verbindlich in der Regel bis 15 Uhr (Nr. 5.1). Eine Pflicht, Anwesenheit zu dokumentieren oder aufzubewahren, enthält er nicht (Textsuche). Der alte Erlass BASS 12-63 Nr. 2 ist zum 31.07.2026 außer Kraft getreten.
- **Schulrecht als Analogie:** VO-DV I § 9 Abs. 1 nennt für „Unterlagen über die Klassenführung (Klassenbuch, Kursbuch)“ 10 Jahre und für „alle übrigen Daten“ 5 Jahre, beginnend „mit Ablauf des Kalenderjahres, in dem die Akten oder Dateien abgeschlossen worden sind, jedoch nicht vor Ablauf des Kalenderjahres, in dem die Schulpflicht endet“. Die Verordnung gilt nach § 1 Abs. 1 für Schulen und Schulaufsicht; Ganztag oder OGS kommen darin nicht vor. Nutzt eine **Schule** als Verantwortliche moto für Daten, die sie selbst als Klassenführung führen muss, wären diese Fristen direkt einschlägig. Das widerspräche kurzen Löschvoreinstellungen.
- **Kita-Recht ist nicht übertragbar:** § 47 Abs. 2 SGB VIII verlangt fünfjährige Aufbewahrung der Belegungsaufzeichnungen nur für erlaubnispflichtige Einrichtungen. Die OGS braucht laut Erlass (Abschnitt II Nr. 1) keine Betriebserlaubnis.
- **Gesetzlicher Zweck mit kurzer Dauer:** Die Unfallanzeige für Kinder verlangt „den regulären Beginn und das reguläre Ende des Besuches der Einrichtung am Tag des Unfalls“ (UVAV § 4 Abs. 3 Nr. 6), zu erstatten binnen drei Tagen (§ 193 Abs. 4 SGB VII). Das trägt keine Speicherung über Wochen.
- **moto heute:** Raumbesuche ohne Einwilligung 30 Tage (Einstellbereich 1 bis 31, `gdpr.privacy_consent_retention_days`); Betreuungsplan 365 Tage (`gdpr.timetable_retention_days`).

### Beweissicherung gegen Datenminimierung

Hier stehen zwei Positionen nebeneinander. Eine Entscheidung trifft diese Notiz nicht.

**Argumente für längere Vorhaltung**

- **Verjährungsfristen:**
  - Regelverjährung 3 Jahre ab Schluss des Jahres der Kenntnis (§§ 195, 199 Abs. 1 BGB).
  - Bei Verletzung von Leben, Körper, Gesundheit oder Freiheit höchstens 30 Jahre ab dem schädigenden Ereignis (§ 199 Abs. 2 BGB).
  - Bei vorsätzlicher Verletzung, auch der sexuellen Selbstbestimmung, 30 Jahre (§ 197 Abs. 1 Nr. 1 BGB).
- **Hemmung bei sexuellem Missbrauch:** Ansprüche wegen Verletzung der sexuellen Selbstbestimmung sind bis zum 21. Lebensjahr gehemmt (§ 208 BGB). Der Wortlaut schränkt den Schuldnerkreis nicht ein, die Hemmung gilt also auch gegenüber Dritten. Zusammen mit § 197 kann ein Anspruch bis etwa zum 51. Lebensjahr durchsetzbar sein. Bei solchen Vorwürfen ist eher relevant, wer vom Personal anwesend war (Zeiterfassung, Betreuungsplan) als Kommen und Gehen der Kinder.
- **§ 832 BGB:** Wer die Aufsicht vertraglich übernimmt, haftet für Schäden, die das Kind **Dritten** zufügt. Er entlastet sich nur, wenn er „seiner Aufsichtspflicht genügt“ hat (§ 832 Abs. 1 S. 2, Abs. 2). Ob das Kind zur fraglichen Zeit betreut war, ist dafür ein Beweismittel.
- **Rückgriff des Unfallversicherungsträgers:** Bei vorsätzlich oder grob fahrlässig verursachtem Versicherungsfall nimmt der Unfallversicherungsträger Rückgriff (§ 110 SGB VII). Die Verjährung beginnt erst mit der bindenden Feststellung der Leistungspflicht (§ 113 SGB VII).
- **Erste-Hilfe-Dokumentation als Vergleichswert:** DGUV Vorschrift 1 § 24 Abs. 6 verlangt, jede Erste-Hilfe-Leistung zu dokumentieren und 5 Jahre verfügbar zu halten. Die DGUV Information 202-089 für Kitas begründet das mit dem Nachweis „möglicher Spätfolgen“. Ob § 24 Abs. 6 für Schülerinnen und Schüler gilt, ist wegen § 1 Abs. 2 (innerer Schulbereich) unklar. Die Unfallkasse NRW nennt für Schulen den „Meldeblock“.

**Argumente für kurze Fristen**

- **Unfallversicherung:** Kinder in der OGS sind gesetzlich unfallversichert. § 2 Abs. 1 Nr. 8 lit. b SGB VII erfasst ausdrücklich „Betreuungsmaßnahmen“ unmittelbar vor oder nach dem Unterricht „im Zusammenwirken“ mit der Schule.
- **Haftungsausschluss:** Für Personenschäden haften Unternehmer und Beschäftigte nur bei Vorsatz oder Wegeunfall (§§ 104 bis 106 SGB VII). Fahrlässig verursachte Personenschäden des Kindes führen damit praktisch nicht zu Zivilprozessen gegen die Schulseite. Ob § 106 Abs. 1 Nr. 3 auch das Personal eines freien Trägers erfasst, ist offen (siehe offene Fragen).
- **§ 207 BGB greift nicht:** Die Hemmung gilt nur für Ansprüche *zwischen* Kind und Eltern bzw. Stiefeltern bis zum 21. Lebensjahr, Vormund, Betreuer, Pfleger und Beistand. Ansprüche gegen Schule, Träger oder Personal sind nicht erfasst. Auch § 210 BGB greift nicht, weil die Eltern gesetzliche Vertreter sind.
- **Datenschutzbehörden:**
  - DSK-Kurzpapier Nr. 11: Die Ausnahmen des Art. 17 Abs. 3 „berechtigen nicht zu einer zeitlich unbegrenzten Verarbeitung“.
  - BayLDA (FAQ Patientendaten) verlangt bei der 30-Jahres-Frist eine Abwägung unter Berücksichtigung der „Wahrscheinlichkeit der Geltendmachung von Ansprüchen“.
  - Die Art.-29-Gruppe (WP 158, 2009) sah keine Rechtsgrundlage, Daten „aufs Geratewohl unbefristet aufzubewahren“, weil es zu einem Rechtsstreit kommen könnte.
  - Der EDSA kritisiert im CEF-Bericht 2025 Verantwortliche, die „the longest period applicable to one processing activity to all of them“ anwenden.
- **Keine Behördenaussage für Aufsichtsnachweise:** Eine offizielle Quelle, die Anwesenheitslisten in Schule oder OGS ausdrücklich zur Beweissicherung aufbewahren lässt, wurde nicht gefunden (Schulministerium NRW, Unfallkasse NRW, DGUV, LVR/LWL geprüft).

**Mittelweg aus der Methodik (nicht entschieden):** Die Leitlinie zu DIN 66398 sieht eine kurze Regellöschfrist und eine „Aussetzung der Löschung“ als dokumentierte Abweichung vor (Abschnitt 7.3). Übersetzt heißt das: Anwesenheit nach Regelfrist löschen, bei konkretem Anlass gezielt sperren. Anlässe wären Unfallanzeige, Vorwurf oder Rückgriff, gesperrt würden nur die betroffenen Kinder und Zeiträume. Die Abwägung gehört ins Verzeichnis der Verarbeitungstätigkeiten (Art. 30 Abs. 1 lit. f DSGVO).

### 2. Krank- und Entschuldigt-Meldungen

- Gesundheitsdaten sind nach Art. 9 Abs. 1 DSGVO besonders geschützt. In der Schule müssen Eltern bei Krankheit „schriftlich den Grund für das Schulversäumnis“ mitteilen; bei Zweifeln kann die Schule ein Attest verlangen (§ 43 Abs. 2 SchulG).
- VO-DV I Anlage 1 erlaubt Schulen „Fehlzeiten: Art, Umfang“ und „Schulversäumnis: Beginn, Ende, Grund“. Nach Fußnote ** dürfen medizinische Gutachten und Atteste „nicht automatisiert verarbeitet werden“. Eine eigene Frist für Entschuldigungen gibt es nicht. Die Zuordnung zu „alle übrigen Daten“ (5 Jahre) ist eine Ableitung, keine ausdrückliche Regel.
- Für eine Plattform folgt daraus eine Unterscheidung: Der Status „krank“ oder „entschuldigt“ ist ein Anwesenheitsdatum und kann dessen Frist folgen. Freitext, Diagnosen oder hochgeladene Atteste sollten nicht entstehen oder deutlich kürzer leben. Das ist eine Ableitung aus Art. 5 Abs. 1 lit. c, e und Art. 9 DSGVO, keine Behördenaussage.

### 3. Nachrichten

- Es gibt keine gesetzliche Frist. Aufbewahrungspflichtig werden Nachrichten nur, wenn sie die Funktion eines Handels- oder Geschäftsbriefs (6 Jahre) oder Buchungsbelegs (8 Jahre) haben (§ 147 Abs. 1 Nr. 2 bis 4 AO). GoBD Rz. 121 (BMF-Schreiben vom 28.11.2019, zuletzt geändert am 14.07.2025): „E-Mails mit der Funktion eines Handels- oder Geschäftsbriefs oder eines Buchungsbelegs“ sind aufbewahrungspflichtig, eine E-Mail als bloßes „Transportmittel“ nicht. Für Elternnachrichten in der OGS dürfte das selten zutreffen, etwa bei Kündigung oder Beitragsfragen.
- **moto heute:** Personalchat 365 Tage (Einstellbereich 30 bis 730, `gdpr.staff_message_retention_days`).

### 4. Anmeldungen

- **Rechtsnatur offen:** Ob die OGS-Aufnahme ein Verwaltungsakt der Schulleitung ist, regeln die Quellen nicht eindeutig. Die FAQ des Schulministeriums verweist bei unterjähriger Anmeldung auf „kommunale Vorgaben“ und „Betreuungsverträge“. Das OVG NRW (12 A 1530/12, 2013) beschreibt drei Ebenen:
  1. kommunaler Beitragsbescheid als Verwaltungsakt,
  2. öffentlich-rechtlicher Vertrag zwischen Kommune und Träger,
  3. privatrechtliches Betreuungsverhältnis zwischen Eltern und Träger.
- **Fristen je nach Rechtsnatur:**
  - Verwaltungsakt: Klage innerhalb eines Monats (§ 74 Abs. 1 VwGO), bei fehlender Belehrung innerhalb eines Jahres (§ 58 Abs. 2 VwGO). Ein Widerspruchsverfahren findet in NRW grundsätzlich nicht statt; Ausnahmen gelten unter anderem für Verwaltungsakte des Schulrechts, „soweit sie von Schulen erlassen werden“ (§ 110 Abs. 2 S. 1 Nr. 3 lit. a JustG NRW).
  - Privatrechtlicher Vertrag: Benachteiligungsansprüche nach AGG müssen innerhalb von zwei Monaten geltend gemacht werden (§ 21 Abs. 5 AGG). Ob das AGG auf die OGS-Aufnahme anwendbar ist, wurde nicht geprüft.
- **Förderbezug:** Die Förderung bemisst sich nach den am 15. Oktober angemeldeten Kindern (BASS 11-02 Nr. 19 Nr. 6.2.2). ANBest-P NRW Nr. 6.8 erfasst „alle sonstigen mit der Zuwendung zusammenhängenden Unterlagen“. Daraus lässt sich ableiten, dass die Anmeldeliste zum Stichtag eine Zuwendungsunterlage ist. Das ist eine Ableitung, keine Behördenaussage.
- **moto heute:** abgelehnte Anmeldungen 90 Tage (Einstellbereich 7 bis 730, `enrollment.rejected_retention_days`).

### 5. Kinderstammdaten nach Austritt

- Für Träger gibt es keine Aufbewahrungspflicht. Es gilt Art. 17 Abs. 1 lit. a DSGVO. Für Schulen gelten 20 Jahre für das Schülerstammblatt (VO-DV I § 9 Abs. 1), danach Anbietung an das Archiv (§ 9 Abs. 3).
- Wenn die OGS als Aufgabe der Jugendhilfe gilt (Erlass Nr. 6.5: „Aufgaben der Kinder- und Jugendhilfe“), muss der freie Träger einen entsprechenden Schutz der Sozialdaten sicherstellen (§ 61 Abs. 3 SGB VIII). § 84 SGB X kennt eine Einschränkung statt Löschung, solange schutzwürdige Interessen beeinträchtigt würden (§ 84 Abs. 3 SGB X). Ob diese Normen für OGS-Daten gelten, ist offen.
- **Analogie:** Nach KiBiz § 18 wird die Bildungsdokumentation bei Ende der Betreuung den Eltern ausgehändigt.
- Kinderbezogene Daten in Rechnungen, Beitragsbescheiden und Förderunterlagen unterliegen den Fristen aus Nr. 7 und 8, nicht der Stammdatenfrist. Ein Löschkonzept sollte sie deshalb als eigene Datenart führen.

### 6. Personaldaten

- **Arbeitsvertrag und Personalakte:** Eine Behörde mit fester Frist für die Privatwirtschaft wurde nicht gefunden. Der HmbBfDI sagt, nach dem Ende des Arbeitsverhältnisses seien Daten „grundsätzlich“ zu löschen, soweit keine Gründe für eine vorübergehende Speicherung bestehen. Der LfDI BW (Ratgeber 2017) verweist für die Abwägung auf die dreijährige Regelverjährung ab Jahresende.
- **Lohn und Sozialversicherung:**
  - Das Lohnkonto ist „bis zum Ablauf des sechsten Kalenderjahres, das auf die zuletzt eingetragene Lohnzahlung folgt“, aufzubewahren (§ 41 Abs. 1 S. 9 EStG).
  - Entgeltunterlagen „bis zum Ablauf des auf die letzte Prüfung (§ 28p) folgenden Kalenderjahres“ (§ 28f Abs. 1 SGB IV).
  - Zu den Entgeltunterlagen gehören nach § 8 Abs. 2 BVV auch die Niederschrift nach dem Nachweisgesetz (Nr. 6) und die MiLoG-Aufzeichnungen (Nr. 13).
- **Zeiterfassung:**
  - § 16 Abs. 2 ArbZG: Die über acht Stunden werktäglich hinausgehende Arbeitszeit ist aufzuzeichnen; „Die Nachweise sind mindestens zwei Jahre aufzubewahren.“
  - § 17 Abs. 1 MiLoG verlangt dasselbe für geringfügig Beschäftigte, „beginnend ab dem für die Aufzeichnung maßgeblichen Zeitpunkt“.
  - Nach BMAS-FAQ (Stand 08.08.2025) folgt aus § 3 Abs. 2 Nr. 1 ArbSchG und BAG 1 ABR 22/21 eine allgemeine Erfassungspflicht.
  - Zur ArbZG-Reform gibt es nur einen Referentenentwurf (Lobbyregister, Datum 16.06.2026), keinen Kabinettsbeschluss. Ob er eine Aufbewahrungsfrist enthält, ist unbekannt.
  - **moto heute:** 730 Tage, wählbar 2 bis 8 Jahre (`gdpr.time_tracking_retention_days`). Das passt zur Untergrenze aus ArbZG und MiLoG.
- **Erweitertes Führungszeugnis:**
  - Der OGS-Erlass Nr. 7.7 verlangt die Vorlage beim Anstellungsträger vor Tätigkeitsbeginn.
  - § 72a Abs. 5 SGB VIII erlaubt nur drei Datenpunkte: den Umstand der Einsichtnahme, das Datum des Zeugnisses und die Information über eine einschlägige Verurteilung. Sie sind „unverzüglich zu löschen“, wenn die Tätigkeit nicht aufgenommen wird, und „spätestens sechs Monate nach der letztmaligen Ausübung“. § 30a Abs. 3 BZRG regelt dasselbe.
  - LDI NRW: Der Träger darf das Zeugnis „nicht aufbewahren, auch nicht in abgeschlossenen Schränken“.
  - Eine früher verbreitete Angabe von 3 Monaten entspricht nicht dem aktuellen Wortlaut.
- **Masernschutz:**
  - § 20 IfSG enthält keine Frist.
  - LDI NRW: Es genügt, in der Akte zu vermerken, „dass und wann der entsprechende Nachweis vorgelegen hat“; Kopie oder Einbehalten sind nicht gedeckt.
  - LfD Niedersachsen, HBDI und LfDI RLP sehen es ebenso.
  - Für befristete Nachweise das Ablaufdatum vermerken (LfD Niedersachsen), weil nach § 20 Abs. 9a ein Folgenachweis fällig wird.
- **IfSG-Belehrung:** Belehrung vor Tätigkeitsbeginn und mindestens alle zwei Jahre. Das Protokoll ist „für die Dauer von drei Jahren aufzubewahren“ (§ 34 Abs. 5a IfSG; OGS-Erlass Nr. 7.8).
- **Abgelehnte Bewerbungen:**
  - BfDI: Aufbewahrung „für maximal sechs Monate“ zulässig, danach „zurückzugeben oder aber zu vernichten“. BayLDA ebenso.
  - Herleitung: zwei Monate Geltendmachung nach § 15 Abs. 4 AGG plus drei Monate Klagefrist nach § 61b Abs. 1 ArbGG.
  - Der LfDI BW hielt 2017 mehr als drei Monate nicht für erforderlich.

### 7. Finanzdaten und Bürokratieentlastungsgesetz IV

- **Neue Fristen:** Das Bürokratieentlastungsgesetz IV vom 23.10.2024 (BGBl. 2024 I Nr. 323) senkt die Frist für Buchungsbelege von 10 auf 8 Jahre in § 147 Abs. 3 AO, § 257 Abs. 4 HGB und § 14b Abs. 1 UStG. In Kraft ist das seit dem 01.01.2025.
- **Übergang:** Die Verkürzung gilt „erstmals für alle Unterlagen, deren Aufbewahrungsfrist … in der bis einschließlich 31. Dezember 2024 geltenden Fassung noch nicht abgelaufen ist“ (Art. 97 § 19a Abs. 2 EGAO; entsprechend Art. 95 EGHGB, § 27 Abs. 40 UStG). Belege von 2016 und älter durften demnach seit dem 01.01.2025 vernichtet werden, Belege von 2017 seit dem 01.01.2026, sofern keine Ablaufhemmung greift. Diese Jahreszuordnung ist abgeleitet.
- **Kreditinstitute, Versicherungen, Wertpapierinstitute:** Die ursprüngliche Verzögerung auf 2026 wurde durch Gesetz vom 22.12.2025 (BGBl. 2025 I Nr. 369, Art. 17, 18) durch dauerhaft 10 Jahre ersetzt. Für moto und OGS-Träger ist das nicht relevant.
- **Ablaufhemmung:** Die Frist läuft nicht ab, „soweit und solange die Unterlagen für Steuern von Bedeutung sind, für welche die Festsetzungsfrist noch nicht abgelaufen ist“ (§ 147 Abs. 3 S. 5 AO).
- **Kommunen:** Erhebt die Kommune die Elternbeiträge (§ 51 Abs. 5 KiBiz über § 9 Abs. 3 SchulG), gilt für sie KomHVO NRW § 59 Abs. 2: Bücher 10 Jahre, Belege 6 Jahre, Jahresabschlüsse dauernd. Die Frist beginnt mit dem 1. Januar nach dem Ratsbeschluss über den Jahresabschluss.
- **Gemeinnützige Träger:** Ob ein Träger als Verein oder gGmbH unter § 147 AO, § 257 HGB oder beides fällt, hängt von seiner Rechtsform und Buchführungspflicht ab. Das wurde nicht im Einzelnen geprüft.

### 8. Fördermittel

- Die OGS-Förderrichtlinie (BASS 11-02 Nr. 19, gültig 01.08.2026 bis 31.07.2031, Nr. 6.5) verweist auf die VV und VVG zu § 44 LHO NRW.
- **Wortlaut ANBest-P NRW Nr. 6.8** (Fassung MB.NRW 2026 Nr. 94): Originalbelege „und alle sonstigen mit der Zuwendung zusammenhängenden Unterlagen … fünf Jahre nach Vorlage des Verwendungsnachweises aufzubewahren, sofern nicht nach steuerrechtlichen oder anderen Vorschriften eine längere Aufbewahrungsfrist bestimmt ist“.
- **ANBest-G NRW Nr. 7.5** regelt dasselbe für Gemeinden. Elektronische Aufbewahrung ist zulässig.
- Leitet die Kommune die Mittel an einen Träger weiter, sind laut Förder-FAQ des Schulministeriums „die ANBest-P … zum Bestandteil der Verpflichtung zu erklären“. Damit gilt die Frist auch beim Träger.
- Die steuerliche Frist von 8 Jahren ist meist länger und hat dann Vorrang.

### 9. Sicherheits- und Audit-Logs

- **Pflicht zur Festlegung:** SDM Baustein 43 „Protokollieren“ V2.0 (gültig seit 01.06.2026), M43.21.09: „Für die Protokolldaten müssen die Aufbewahrungsfrist und die Löschung festgelegt sein“. Abzuwägen sind eine fachliche und eine funktionale Frist, „Die fachlich begründete Löschfrist ist maßgebend.“ Technische Logs dürfen nicht „auf Vorrat gespeichert und unbeachtet“ liegen. Eine Zahl nennt der Baustein nicht.
- **BSI:**
  - Mindeststandard Protokollierung und Detektion V2.1 (11.11.2024), PD.2.1.04 c: Die Speicherdauer „SOLLTE … auf bspw. 90 Tage festgelegt werden“. Die Festlegung „obliegt weiterhin der Verantwortlichkeit der Einrichtung“. Sicherheitsrelevante Ereignisse ohne Personenbezug und nachweislich einem Angriff zugeordnete Ereignisse dürfen länger gespeichert werden. Verbindlich ist der Standard nur für Bundesstellen.
  - IT-Grundschutz OPS.1.1.5.A5: Protokolldaten „MÜSSEN nach einem festgelegten Prozess gelöscht werden“. Nach A10 dürfen Administrierende sie nicht löschen können.
- **Zugriffsprotokolle:** Feste Fristen gibt es nur für Polizei und Justiz: § 76 Abs. 4 BDSG und § 55 Abs. 4 DSG NRW verlangen die Löschung „am Ende des auf deren Generierung folgenden Jahres“. Für Schule und OGS ist das nur eine Analogie.
- **Löschprotokolle:**
  - SDM Baustein 60 (M60.S05, S06): Die Löschung soll „nachträglich nachweisbar“ sein, das Protokoll darf aber keine Daten enthalten, „für die eine Löschverpflichtung besteht“.
  - SDM Baustein 43 verlangt den Nachweis, „welches Datum mit welchem Programm in welcher Qualität gelöscht wurde“.
  - BayLDA: Löschdokumentation „für eventuelle folgende Rechtsstreitigkeiten bis zu drei Jahre“, gestützt auf Art. 6 Abs. 1 lit. f DSGVO.
  - Der EDSA empfiehlt im CEF-Bericht 2025, Löschung „demonstrate“ zu können.
- **Rolle:** Betriebslogs, die moto für eigene Sicherheitszwecke führt, verantwortet moto selbst. Audit-Logs, die die Schule oder der Träger für die eigene Rechenschaft braucht, gehören zur Auftragsverarbeitung. Diese Trennung ist eine Ableitung.

### 10. Technische Kurzzeitdaten

- **Kein Sonderrecht:** Das TDDDG schreibt für Anbieter digitaler Dienste keine Löschfrist vor. § 19 TDDDG verlangt nur technische und organisatorische Vorkehrungen; die Löschpflichten in § 9 und § 12 gelten für Telekommunikationsanbieter. Es bleibt bei Art. 5 Abs. 1 lit. e DSGVO.
- **IP-Adressen:**
  - Dynamische IP-Adressen sind personenbezogen (EuGH C-582/14 Breyer; BGH VI ZR 135/13).
  - Eine Speicherung über den Nutzungsvorgang hinaus ist nach Abwägung zulässig, „um die generelle Funktionsfähigkeit der Dienste zu gewährleisten“ (BGH VI ZR 135/13).
  - Feste Zahlen nennen nur ältere oder benachbarte Quellen:
    - BGH III ZR 391/13 (2014): 7 Tage bei einem Zugangsprovider nach altem TKG.
    - ULD-FAQ (vor der DSGVO): 7 Tage für Sicherheitszwecke „im Allgemeinen“ nicht beanstandet.
    - LfDI RLP (2015): 7 Tage ungekürzt.
    - BfDI „Logfile-Daten“: 5 Wochen für Monatsstatistik, volle IP „in der Regel nicht notwendig“.
- **Codes und Tokens:** NIST SP 800-63B-4 verlangt, Out-of-band-Authentisierung nach 10 Minuten als ungültig zu behandeln (Abschn. 3.1.3.2), und schließt E-Mail als Out-of-band-Kanal für die Authentisierung aus (3.1.3.1). Das betrifft die Gültigkeit, nicht die Speicherdauer. Nach Ablauf gibt es keinen Zweck mehr außer kurzer Missbrauchsanalyse. Die Speicherdauer danach ist übliche Praxis ohne gesetzliche Vorgabe.

### 11. Backups

- **Grundsatz:** Die Löschpflicht betrifft „grundsätzlich auch personenbezogene Daten in Sicherungskopien“ (SDM Baustein 60, S. 2).
- **Keine Sofortlöschung im Backup nötig:** Es ist nicht nötig, in „jedem Backup Daten zum gleichen Zeitpunkt wie im Originaldatenbestand“ zu löschen. Die Löschung kann mit dem „planmäßigen Überschreiben“ erfolgen. Nach einer Rücksicherung muss aber „unmittelbar eine erneute Löschung dieser Daten erfolgen“ (SDM B60, M60.S10). Dafür „MÜSSEN entsprechende Überwachungsprozesse eingerichtet werden“ (M60.P08).
- **BayLfD, 30. Tätigkeitsbericht Nr. 12.5:** Eine verzögerte Backup-Löschung ist nur bei kumulativer Erfüllung zulässig:
  1. Einzellöschung technisch unmöglich oder unzumutbar,
  2. Löschfrequenz nach Schutzbedarf im Datensicherungskonzept,
  3. Zugriff nur über die Wiederherstellungsfunktion, gestützt durch Verschlüsselung,
  4. Löschung bei Wiederherstellung,
  5. Dokumentation jeder Wiederherstellung.
- **BSI:** CON.3.A4 verlangt festzulegen, „wie lange Datensicherungen aufbewahrt werden“. CON.6.A2: Der Löschprozess „MUSS auch Datensicherungen, wenn erforderlich, berücksichtigen“.
- **EDSA (CEF-Bericht 2025):** Löschen aus Backups sei nicht immer ratsam. Dann braucht es aber Verfahren, um Löschungen auf wiederhergestellten Systemen nachzuziehen. Backups pauschal und ohne Begründung auszunehmen, kritisiert der EDSA.
- **Leitlinie zu DIN 66398 (Sekundärquelle, 7.3.5):** „Die Löschfrist der Sicherungskopie muss sich an der kürzesten Löschfrist der jeweils enthaltenen Datenarten orientieren“. Beispiele: wenige Wochen bzw. 3 Monate. Bei Raumbesuchen mit 30 Tagen heißt das: Backups mit deutlich längerer Laufzeit brauchen die Löschung bei Wiederherstellung als Ausgleich.

### 12. Vertragsende

- Art. 28 Abs. 3 lit. g DSGVO: Der Auftragsverarbeiter löscht oder gibt zurück „nach Wahl des Verantwortlichen“, sofern keine eigene Speicherpflicht nach Unions- oder nationalem Recht besteht.
- **EU-Standardvertragsklauseln** (Durchführungsbeschluss (EU) 2021/915), Klausel 10 lit. d:
  - Löschung mit Bescheinigung oder Rückgabe samt Löschung bestehender Kopien.
  - „Bis zur Löschung oder Rückgabe“ gelten die Klauseln weiter.
  - Eine Frist nennt die Klausel nicht.
- **EDSA-Leitlinien 07/2020, Rn. 139 bis 142:**
  - Die Wahl kann zu Vertragsbeginn getroffen und später geändert werden.
  - Die Löschung ist „within an agreed timescale and in an agreed manner“ zu bestätigen.
  - Alle bestehenden Kopien sind zu löschen.
  - Backups beim Auftragsverarbeiter werden dort nicht ausdrücklich behandelt.
- **Folgerung für den AVV:** Konkrete Tage für Export, Löschung und das Auslaufen der Backups legt der Vertrag fest. 30 bis 90 Tage sind übliche Praxis ohne gesetzliche Vorgabe; eine Behördenquelle dafür wurde nicht gefunden. Eigene Unterlagen von moto, etwa Rechnungen an den Kunden, sind keine Auftragsdaten und folgen der AO.

## Methodik Löschkonzept (DIN 66398)

**Normstatus.** DIN 66398:2016-05 ist bei DIN Media als zurückgezogen markiert und durch DIN EN ISO/IEC 27555:2025-09 ersetzt. Laut Änderungsvermerk entfallen dabei die Hinweise auf nationales Recht, die Anhänge A bis D und große Teile zu Verantwortlichkeiten. „Sicheres Löschen“ wird risikobasiert definiert. ISO/IEC 27555:2021 steht bei ISO auf „to be revised“. Normtexte wurden nicht gelesen. Die Begriffe unten stammen aus der frei verfügbaren Leitlinie von Hammer/Schuler (Vorläufer der DIN, v1.0.4, 2022) und aus Hammer, DuD 8/2016. Beides sind **Sekundärquellen**. Die DSK verweist in SDM Baustein 60 (M60.P01 bis P03) auf DIN 66398 als „hilfreich“ und verlangt, Löschfristen schriftlich festzulegen und ins Verzeichnis der Verarbeitungstätigkeiten aufzunehmen (Art. 30 Abs. 1 lit. f DSGVO).

**Begriffe (Hammer/Schuler):**

- **Datenart:** „Gruppe von Datenobjekten, die zu einem einheitlichen fachlichen Zweck verarbeitet wird“. Nicht die Tabelle ist die Einheit, sondern der Zweck. Beispiel: „Anwesenheit Tag“ und „Raumbesuch“ sind verschiedene Datenarten, auch wenn sie in derselben Tabelle liegen.
- **Regellöschfrist:** die Frist, nach der eine Datenart „bei regulärer Verwendung … spätestens zu löschen ist“.
- **Vorhaltefrist:** die Frist, für die eine Datenart „verfügbar sein muss“, also Mindestfristen aus AO, ArbZG oder ANBest.
- **Startzeitpunkt:** das Ereignis, ab dem die Frist läuft. Typen sind Erhebung, Ende eines Vorgangs und Ende der Beziehung zur betroffenen Person.
- **Löschklasse:** „Kombination aus Löschfrist und abstraktem Startzeitpunkt“. Die Löschklassen bilden eine Matrix aus wenigen Standardfristen und den Startzeitpunkt-Typen. Jede Datenart wird genau einer Löschklasse zugeordnet.
- **Löschregel:** Löschklasse plus konkrete Bedingung für den Startzeitpunkt, zum Beispiel „Austrittsdatum gesetzt“.
- **Abweichungen (Abschnitt 7.3):** Verdichtung oder Anonymisierung, Wechsel der Datenart (etwa Rechnung statt Anmeldung) und Aussetzung der Löschung bei konkretem Anlass. Abweichungen werden einzeln dokumentiert.
- **Backups:** Ihre Löschfrist richtet sich nach der kürzesten enthaltenen Frist (7.3.5).

**Vorgehen nach dieser Struktur:**

1. Datenarten nach Zweck bilden, nicht nach Tabellen.
2. Für jede Datenart die Vorhaltefrist (gesetzliche Mindestfrist mit Quelle) und die fachlich nötige Dauer bestimmen. Die Regellöschfrist ist das Maximum aus beiden, gerundet auf das Raster.
3. Ein kleines Fristenraster festlegen. Das Beispiel Toll Collect bei Hammer 2016: sofort, 42 Tage, 120 Tage, 1 Jahr, 4, 7 und 12 Jahre. Bei gesetzlich motivierten Fristen wurde das laufende Kalenderjahr aufgeschlagen, weil § 147 Abs. 4 AO und § 199 Abs. 1 BGB ab Jahresende zählen.
4. Startzeitpunkte festlegen. Für die OGS liegen nahe: Erhebung, Ende Betreuungstag, Ende Schuljahr, Ende der Betreuung (Austritt), Ende der Beschäftigung, Vorlage des Verwendungsnachweises, Ende Kalenderjahr, Vertragsende des Kunden. Das ist ein Vorschlag, keine Quelle.
5. Löschklassen als Matrix bilden und jede Datenart einordnen.
6. Löschregeln technisch umsetzen. Backups, Löschung bei Wiederherstellung, Löschprotokolle und die Aussetzung für konkrete Anlässe gehören dazu.
7. Ergebnis ins Verzeichnis der Verarbeitungstätigkeiten übernehmen und regelmäßig überprüfen (ErwG 39).

**Rollenverteilung bei Auftragsverarbeitung (Ableitung):** Der Verantwortliche wählt Regellöschfristen innerhalb der Grenzen. Der Auftragsverarbeiter liefert Voreinstellungen, Unter- und Obergrenzen und Startzeitpunkte und setzt die Löschung technisch um. Gesetzliche Höchstgrenzen, etwa § 72a SGB VIII, sollte das System nicht überschreitbar machen. Gesetzliche Mindestfristen, etwa § 16 ArbZG, sollte es nicht unterschreitbar machen. moto setzt dieses Muster bei der Zeiterfassung bereits um (2 bis 8 Jahre).

## Offene Fragen für den Verantwortlichen / die Datenschutzberatung

1. **Rolle des Trägers:** Nach dem OGS-Erlass verantwortet der Träger die Angebote „eigenständig“ als Aufgabe der Jugendhilfe (Nr. 6.5), sie gelten aber „als schulische Veranstaltung“ (Nr. 9.1). Wer ist für welche Daten Verantwortlicher? Gilt für Daten, die eine Schule in moto führt, VO-DV I § 9 mit 5 bzw. 10 Jahren? Weder LDI NRW noch Schulministerium haben dazu eine OGS-spezifische Aussage veröffentlicht.
2. **Sozialdatenschutz:** Gilt für freie Träger über § 61 Abs. 3 SGB VIII der Sozialdatenschutz mit § 84 SGB X?
3. **Anmeldelisten als Zuwendungsunterlage:** Sind Anmelde- oder Anwesenheitslisten zum Stichtag 15.10. „mit der Zuwendung zusammenhängende Unterlagen“ (ANBest-P NRW Nr. 6.8)? Dann wären sie 5 Jahre nach Vorlage des Verwendungsnachweises aufzubewahren.
4. **Haftungsprivileg:** Erfasst § 106 Abs. 1 Nr. 3 SGB VII auch Beschäftigte eines freien Trägers? Davon hängt ab, wie stark das Argument „Personenschäden laufen über die Unfallversicherung“ trägt.
5. **Beweissicherung:** Welche Regelfrist für Anwesenheitsdaten will der Verantwortliche, und wer löst eine Sperrung bei konkretem Anlass aus? Anlässe wären Unfall, Vorwurf oder Rückgriff.
6. **Freitext bei Krankmeldungen:** Sollen Freitext zu Krankmeldungen und Atteste überhaupt gespeichert werden, oder nur der Status?
7. **Rechtsnatur der Aufnahme:** Ist die OGS-Aufnahme vor Ort ein Verwaltungsakt (Schule oder Kommune) oder ein privatrechtlicher Vertrag mit dem Träger? Davon hängen die Fristen für abgelehnte Anmeldungen ab (1 Monat, 1 Jahr oder AGG).
8. **Elternbeiträge:** Wer erhebt sie (Kommune per Satzung oder Träger)? Davon hängt ab, ob KomHVO NRW (Belege 6 Jahre) oder AO/HGB (8 Jahre) gilt. Unter welche Buchführungsregeln fällt der jeweilige Träger?
9. **Masernnachweis:** Prüft die Schule den Masernnachweis der OGS-Kinder bei der Einschulung, oder muss der Träger eigens prüfen und vermerken?
10. **Zugriffsprotokolle:** Wie lange will der Verantwortliche Zugriffsprotokolle auf Kinderdaten zur Kontrolle vorhalten? Mangels Norm ist das eine Abwägung; Analogie wäre das Ende des Folgejahres.
11. **Logs als eigene Verarbeitung:** Welche Logs führt moto als eigene Verarbeitung (Sicherheit, Betrieb), welche im Auftrag? Das ist für AVV und Datenschutzhinweise getrennt festzulegen.
12. **Fristen im AVV:** Konkrete Fristen bei Vertragsende: Exportfenster, Löschfrist, Auslaufen der Backups, Form der Löschbescheinigung.
13. **Laufende Gesetzgebung:**
    - Die ArbZG-Reform (Referentenentwurf vom Juni 2026) kann neue Aufzeichnungs- und Aufbewahrungsfristen bringen.
    - Ein NRW-Ausführungsgesetz zum Ganztag lag bis Februar 2026 nicht vor; der spätere Stand wurde nicht vollständig geprüft.
    - Der geplanten IP-Speicherpflicht unterliegen nur Internetzugangsanbieter, keine SaaS-Plattformen. Das ist nicht verifiziert.

## Quellen

**EU**

- DSGVO, Verordnung (EU) 2016/679: https://eur-lex.europa.eu/legal-content/DE/TXT/HTML/?uri=CELEX:32016R0679
- Durchführungsbeschluss (EU) 2021/915 (Standardvertragsklauseln Art. 28 Abs. 7): https://eur-lex.europa.eu/eli/dec_impl/2021/915/oj
- EuGH C-582/14 (Breyer): https://eur-lex.europa.eu/legal-content/DE/TXT/?uri=CELEX:62014CJ0582
- EDSA, Leitlinien 07/2020 zu Verantwortlichem und Auftragsverarbeiter v2.1: https://www.edpb.europa.eu/system/files/2023-10/EDPB_guidelines_202007_controllerprocessor_final_en.pdf
- EDSA, CEF-Bericht 2025 Recht auf Löschung: https://www.edpb.europa.eu/system/files/2026-02/edpb_cef-report_2025_right-to-erasure_en.pdf
- EDSA, CEF-Bericht 2024 Auskunftsrecht: https://www.edpb.europa.eu/system/files/documents/2025-01/edpb_cef-report-2024_20250116_rightofaccess_en.pdf
- Art.-29-Gruppe, WP 158: https://datenschutz.hessen.de/sites/datenschutz.hessen.de/files/2022-11/wp158_de_0.pdf

**Bundesrecht**

- § 147 AO: https://www.gesetze-im-internet.de/ao_1977/__147.html
- Art. 97 § 19a EGAO: https://www.gesetze-im-internet.de/aoeg_1977/art_97__19a.html
- § 257 HGB: https://www.gesetze-im-internet.de/hgb/__257.html
- Art. 95 EGHGB: https://www.gesetze-im-internet.de/hgbeg/art_95.html
- § 14b UStG: https://www.gesetze-im-internet.de/ustg_1980/__14b.html
- § 27 UStG: https://www.gesetze-im-internet.de/ustg_1980/__27.html
- Bürokratieentlastungsgesetz IV, BGBl. 2024 I Nr. 323: https://www.recht.bund.de/bgbl/1/2024/323/VO.html
- Gesetz zur Modernisierung und Digitalisierung der Schwarzarbeitsbekämpfung, BGBl. 2025 I Nr. 369: https://www.recht.bund.de/bgbl/1/2025/369/VO.html
- § 41 EStG: https://www.gesetze-im-internet.de/estg/__41.html
- § 28f SGB IV: https://www.gesetze-im-internet.de/sgb_4/__28f.html
- §§ 8, 9 BVV: https://www.gesetze-im-internet.de/beitrvv/__8.html, https://www.gesetze-im-internet.de/beitrvv/__9.html
- § 16 ArbZG: https://www.gesetze-im-internet.de/arbzg/__16.html
- § 17 MiLoG: https://www.gesetze-im-internet.de/milog/__17.html
- § 15 AGG: https://www.gesetze-im-internet.de/agg/__15.html
- § 21 AGG: https://www.gesetze-im-internet.de/agg/__21.html
- § 61b ArbGG: https://www.gesetze-im-internet.de/arbgg/__61b.html
- § 47 SGB VIII: https://www.gesetze-im-internet.de/sgb_8/__47.html
- § 61 SGB VIII: https://www.gesetze-im-internet.de/sgb_8/__61.html
- § 72a SGB VIII: https://www.gesetze-im-internet.de/sgb_8/__72a.html
- § 84 SGB X: https://www.gesetze-im-internet.de/sgb_10/__84.html
- § 30a BZRG: https://www.gesetze-im-internet.de/bzrg/__30a.html
- § 20 IfSG: https://www.gesetze-im-internet.de/ifsg/__20.html
- § 34 IfSG: https://www.gesetze-im-internet.de/ifsg/__34.html
- §§ 195, 197, 199, 207, 208, 210, 832, 839 BGB: https://www.gesetze-im-internet.de/bgb/__195.html, https://www.gesetze-im-internet.de/bgb/__197.html, https://www.gesetze-im-internet.de/bgb/__199.html, https://www.gesetze-im-internet.de/bgb/__207.html, https://www.gesetze-im-internet.de/bgb/__208.html, https://www.gesetze-im-internet.de/bgb/__210.html, https://www.gesetze-im-internet.de/bgb/__832.html, https://www.gesetze-im-internet.de/bgb/__839.html
- Art. 34 GG: https://www.gesetze-im-internet.de/gg/art_34.html
- §§ 2, 104, 105, 106, 110, 113, 193 SGB VII: https://www.gesetze-im-internet.de/sgb_7/__2.html, https://www.gesetze-im-internet.de/sgb_7/__104.html, https://www.gesetze-im-internet.de/sgb_7/__105.html, https://www.gesetze-im-internet.de/sgb_7/__106.html, https://www.gesetze-im-internet.de/sgb_7/__110.html, https://www.gesetze-im-internet.de/sgb_7/__113.html, https://www.gesetze-im-internet.de/sgb_7/__193.html
- UVAV 2024: https://www.gesetze-im-internet.de/uvav_2024/BJNR0C00B0023.html
- § 35 BDSG: https://www.gesetze-im-internet.de/bdsg_2018/__35.html
- § 76 BDSG: https://www.gesetze-im-internet.de/bdsg_2018/__76.html
- §§ 12, 19 TDDDG: https://www.gesetze-im-internet.de/ttdsg/__12.html, https://www.gesetze-im-internet.de/ttdsg/__19.html
- § 8 BSIG: https://www.gesetze-im-internet.de/bsig_2025/__8.html
- §§ 58, 74 VwGO: https://www.gesetze-im-internet.de/vwgo/__58.html, https://www.gesetze-im-internet.de/vwgo/__74.html

**Rechtsprechung**

- BGH VI ZR 135/13 vom 16.05.2017: https://www.bundesgerichtshof.de/SharedDocs/Entscheidungen/DE/Zivilsenate/VI_ZS/2013/VI_ZR_135-13A.pdf?__blob=publicationFile&v=1
- BGH III ZR 391/13 vom 03.07.2014: https://www.bundesgerichtshof.de/SharedDocs/Entscheidungen/DE/Zivilsenate/III_ZS/2013/III_ZR_391-13.pdf?__blob=publicationFile&v=1
- BGH VI ZR 3/21 vom 08.02.2022: https://www.bundesgerichtshof.de/SharedDocs/Entscheidungen/DE/Zivilsenate/VI_ZS/2021/VI_ZR___3-21.pdf?__blob=publicationFile&v=1
- OVG NRW 12 A 1530/12 vom 09.07.2013: https://nrwe.justiz.nrw.de/ovgs/ovg_nrw/j2013/12_A_1530_12_Urteil_20130709.html

**NRW**

- SchulG NRW (Fassung 01.08.2026): https://recht.nrw.de/lrgv/gesetz/01082026-schulgesetz-nrw-schulg
- VO-DV I: https://recht.nrw.de/lrgv/rechtsverordnung/01082022-verordnung-ueber-die-zur-verarbeitung-zugelassenen-daten-von/
- VO-DV I Anlage 1: https://recht.nrw.de/system/files/BA/10526-47195-sgv_223_20070614_1_anlage1.pdf
- OGS-Erlass, MB.NRW 2026 Nr. 92 (gültig ab 01.08.2026): https://recht.nrw.de/system/files/pdf/state-law-ministerial-gazette/2026/04/16/4b4b69/01-08-2026-offene-ganztagsschulen-sowie-ausserunte.pdf; BASS: https://bass.schule.nrw/11042.htm
- FAQ zum OGS-Erlass: https://www.schulministerium.nrw/faq-gemeinsamer-ogs-erlass
- Förderrichtlinie OGS, BASS 11-02 Nr. 19: https://bass.schule.nrw/4938.htm
- FAQ zur OGS-Förderung: https://www.schulministerium.nrw/faq-zur-foerderung-der-offenen-ganztagsschule-nach-der-richtlinie-zuwendungen-fuer-die
- ANBest-P NRW: https://recht.nrw.de/system/files/2026-04/anlage-2-zu-nr-5.1-zu-ss-44-anbest-p.pdf
- ANBest-G NRW: https://recht.nrw.de/system/files/2026-04/anlage-1-zu-nr-5.1-vvg-anbest-g.pdf
- MB.NRW 2026 Nr. 94 (VV zu §§ 44, 79 LHO): https://recht.nrw.de/system/files/pdf/announcement/2026/04/20/6a4f87/mb-nrw-2026-nr-94.pdf
- KomHVO NRW: https://recht.nrw.de/lrgv/rechtsverordnung/01012026-kommunalhaushaltsverordnung-nordrhein-westfalen-komhvo-nrw/
- KiBiz (Fassung 01.08.2026): https://recht.nrw.de/lrgv/gesetz/01082026-gesetz-zur-fruehen-bildung-und-foerderung-von-kindern-kinderbildungsgesetz/
- JustG NRW: https://recht.nrw.de/lrgv/gesetz/24122024-gesetz-ueber-die-justiz-im-land-nordrhein-westfalen-justizgesetz-nordrhein/
- DSG NRW (Fassung 01.04.2026): https://recht.nrw.de/system/files/pdf/state-law-and-regulations/2026/01/23/c21e60/01-04-2026-datenschutzgesetz-nordrhein-westfalen.pdf
- Schulministerium, FAQ Datenschutz: https://www.schulministerium.nrw/fragen-und-antworten-zum-datenschutz
- Schulministerium, Datenschutz im Schulbereich: https://www.schulministerium.nrw/schule-bildung/recht/datenschutz-im-schulbereich

**Aufsichtsbehörden und Behörden**

- DSK, Kurzpapier Nr. 11: https://www.datenschutzkonferenz-online.de/media/kp/dsk_kpnr_11.pdf
- DSK, SDM-Methode V3.1a: https://www.datenschutz-mv.de/static/DS/Dateien/Datenschutzmodell/SDM-Methode-V31a.pdf
- SDM Baustein 60 „Löschen und Vernichten“: https://www.datenschutz-mv.de/static/DS/Dateien/Datenschutzmodell/Bausteine/SDM-V2.0_L%C3%B6schen_und_Vernichten_V1.0a.pdf
- SDM Baustein 43 „Protokollieren“ V2.0: https://www.datenschutz-mv.de/static/DS/Dateien/Datenschutzmodell/Bausteine/SDM-V3.1_Protokollieren_V2.0.pdf
- LDI NRW, Erweitertes Führungszeugnis: https://www.ldi.nrw.de/Erweitertes_Fuehrungszeugnis
- LDI NRW, Nachweis Masernschutz: https://www.ldi.nrw.de/nachweis-wirksamen-masernschutzes
- LDI NRW, Digitaler Unterricht in Schulen: https://www.ldi.nrw.de/system/files/media/document/file/ldi-nrw-digitaler-unterricht-in-schulen.pdf
- LfD Niedersachsen, Masernschutz: https://www.lfd.niedersachsen.de/startseite/themen/gesundheit/masernschutzimpfung_vor_kita_und_grundschule/
- HBDI, Masern-Impfschutz: https://datenschutz.hessen.de/datenschutz/gesundheitswesen/datenschutzkonforme-kontrolle-und-dokumentation-des-masern-impfschutzes
- LfDI RLP, Masernschutz: https://www.datenschutz.rlp.de/themen/datenschutz-beim-masernschutz
- LfDI RLP, IP-Protokollierung (2015): https://www.datenschutz.rlp.de/service/aktuelles/detail/die-kurzfristige-protokollierung-von-ip-adressen-ist-zur-missbrauchsbekaempfung-zulaessig-datenschutz
- BfDI, Beschäftigte: https://www.bfdi.bund.de/DE/Buerger/Basiswissen/Beschaeftigte/Beschaeftigte_node.html
- BfDI, Logfile-Daten: https://www.bfdi.bund.de/DE/Buerger/Inhalte/Telemedien/LogFile_Analyse.html
- BfDI, eigenes Löschkonzept: https://www.bfdi.bund.de/SharedDocs/Downloads/DE/DokumenteBfDI/AccessForAll/2023/2021_Loeschkonzept-BfDI.pdf?__blob=publicationFile&v=2
- BayLDA, FAQ: https://www.lda.bayern.de/de/faq.html
- BayLDA, Löschung: https://www.lda.bayern.de/de/thema_loeschung.html
- BayLDA, FAQ Löschen von Patientendaten: https://www.lda.bayern.de/media/FAQ_Loeschen_von_Patientendaten.pdf
- BayLfD, 30. Tätigkeitsbericht: https://www.datenschutz-bayern.de/tbs/tb30/tb30.pdf
- BayLfD, Orientierungshilfe Recht auf Löschung: https://www.datenschutz-bayern.de/datenschutzreform2018/OH_Loeschung.pdf
- LfDI BW, Ratgeber Beschäftigtendatenschutz (2017): https://www.baden-wuerttemberg.datenschutz.de/wp-content/uploads/2017/06/Arbeitnehmerdatenschutz-Handreichung.pdf
- HmbBfDI, Beschäftigtendatenschutz: https://datenschutz-hamburg.de/datenschutz/beschaeftigtendatenschutz
- ULD, FAQ IP-Adressen (vor der DSGVO): https://www.datenschutzzentrum.de/artikel/575-IP-Adressen-und-andere-Nutzungsdaten-Haeufig-gestellte-Fragen.html
- BMAS, FAQ Arbeitszeiterfassung: https://www.bmas.de/DE/Arbeit/Arbeitsrecht/Arbeitnehmerrechte/Regelungen-zur-Arbeitszeit/Fragen-und-Antworten/faq-arbeitszeiterfassung.html
- Lobbyregister, ArbZG-Referentenentwurf: https://www.lobbyregister.bundestag.de/inhalte-der-interessenvertretung/regelungsvorhabensuche/RV0028369/537576
- BMF, GoBD 2. Änderung vom 14.07.2025: https://www.bundesfinanzministerium.de/Content/DE/Downloads/BMF_Schreiben/Weitere_Steuerthemen/Abgabenordnung/2025-07-14-GoBD-2-aenderung.pdf?__blob=publicationFile&v=4
- BMF, GoBD 1. Änderung vom 11.03.2024: https://www.bundesfinanzministerium.de/Content/DE/Downloads/BMF_Schreiben/Weitere_Steuerthemen/Abgabenordnung/AO-Anwendungserlass/2024-03-11-aenderung-gobd.pdf?__blob=publicationFile&v=4
- GoBD vom 28.11.2019 (Kopie beim LfSt Bayern): https://www.lfst.bayern.de/fileadmin/RESSOURCEN/INFORMATIONEN/Steuerinfos/Weitere_Themen/Aussenpruefung/2019-11-28-GoBD-1.pdf
- BSI, Mindeststandard Protokollierung und Detektion V2.1: https://www.bsi.bund.de/SharedDocs/Downloads/DE/BSI/Mindeststandards/MST_BSI_PD_Version_2_1.pdf
- BSI, OPS.1.1.5 Protokollierung: https://www.bsi.bund.de/SharedDocs/Downloads/DE/BSI/Grundschutz/IT-GS-Kompendium_Einzel_PDFs_2023/04_OPS_Betrieb/OPS_1_1_5_Protokollierung_Edition_2023.pdf?__blob=publicationFile
- BSI, CON.3 Datensicherungskonzept: https://www.bsi.bund.de/SharedDocs/Downloads/DE/BSI/Grundschutz/IT-GS-Kompendium_Einzel_PDFs_2023/03_CON_Konzepte_und_Vorgehensweisen/CON_3_Datensicherungskonzept_Edition_2023.pdf?__blob=publicationFile
- BSI, CON.6 Löschen und Vernichten: https://www.bsi.bund.de/SharedDocs/Downloads/DE/BSI/Grundschutz/IT-GS-Kompendium_Einzel_PDFs_2023/03_CON_Konzepte_und_Vorgehensweisen/CON_6_Loeschen_und_Vernichten_Edition_2023.pdf?__blob=publicationFile
- DGUV Vorschrift 1: https://publikationen.dguv.de/widgets/pdf/download/article/2909
- DGUV Information 202-089 (Kita): https://www.unfallkasse-nrw.de/fileadmin/server/download/Regeln_und_Schriften/Informationen_Schueler-UV/202-089.pdf
- Unfallkasse NRW, FAQ Erste Hilfe: https://www.unfallkasse-nrw.de/sicherheit-und-gesundheitsschutz/themen/erste-hilfe/fragen-und-antworten.html
- NIST SP 800-63B-4: https://pages.nist.gov/800-63-4/sp800-63b.html

**Normen (Abstracts) und Sekundärquellen**

- DIN 66398 (zurückgezogen), DIN Media: https://www.dinmedia.de/de/norm/din-66398/249218525
- DIN EN ISO/IEC 27555:2025-09, DIN Media: https://www.dinmedia.de/de/norm/din-en-iso-iec-27555/390032326
- ISO/IEC 27555:2021, ISO: https://www.iso.org/standard/71673.html
- Sekundärquelle: Hammer/Schuler, Leitlinie zur Entwicklung eines Löschkonzepts v1.0.4: https://www.secorvo.de/publikationen/din-leitlinie-loeschkonzept-hammer-schuler-2012.pdf
- Sekundärquelle: Hammer, DIN 66398, DuD 8/2016: https://www.secorvo.de/publikationen/din-66398-hammer-2016.pdf

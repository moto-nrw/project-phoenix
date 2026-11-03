---
status: accepted
---

# Dokumentvorlagen des Trägers liegen bei Organisation & Tenancy, ihre Sprache bei Document Rendering

**Status: Akzeptiert.** Setzt die Zuordnung der Spec
[#4611](https://github.com/moto-nrw/project-phoenix/issues/4611) um
(„Document Rendering: Vorlagen je Träger mit Platzhaltern und Branding“) und
schließt den in
[ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
offenen Punkt T6 für die Vorlagen; Entscheidung für
[#4677](https://github.com/moto-nrw/project-phoenix/issues/4677). Ergänzt
[ADR 0112](0112-stammdaten-des-traegers-liegen-bei-organisation-und-tenancy.md)
und [ADR 0121](0121-bestaetigungsschreiben-ist-die-rechnung-im-layout-des-traegers.md).

## Kontext

Das Träger-Büro will Vorlagen für Vertrag, Rechnung, Bescheid, Bestätigung,
Kündigung, Mahnung und Bescheinigungen im Layout des Trägers pflegen
(Stories 91, 92), mit Platzhaltern wie Familienname, Kind, Summe, Fälligkeit
und Notfallnummern der ausrichtenden OGS. Unbekannte Platzhalter sollen beim
Speichern abgelehnt werden, ausgestellte Dokumente ihre Version behalten, und
das Bestätigungsschreiben der Ferienabrechnung (ADR 0121) soll dieselbe
Verwaltung nutzen.

Bis jetzt schrieb die Elternabrechnung jeden Text des Bestätigungsschreibens
selbst. Document Rendering hat keine Tabellen (ADR 0088). Briefkopf, Logo,
Ansprechperson und Bankverbindungen sind Stammdaten des Trägers bei
Organisation & Tenancy (ADR 0112). Das Glossar nennt Vorlagen eigene Objekte
des Trägers, keine Träger-Werte der Einstellungen.

## Entscheidung

1. **Die Vorlagensprache ist ein reiner Vertrag von Document Rendering**
   (`modules/documentrendering/templates`, Rolle `contract`): die sieben
   Dokumentarten, die Platzhalter jeder Art mit Beschriftung und
   Beispielwert, die Standardvorlage jeder Art, die Prüfung (Pflichtteile,
   Längen, unbekannte Platzhalter) und das Füllen. Eine Vorlage hat Betreff,
   Anrede, Brieftext und Grußformel; ein Platzhalter heißt `{{name}}`. Im
   Brieftext trennt eine Leerzeile Absätze, `# ` am Anfang macht die erste
   Zeile zur Überschrift, `**…**` setzt den Absatz fett, und ein
   Tabellen-Platzhalter allein in einer Zeile druckt eine Tabelle. Ein Absatz,
   dessen Platzhalter alle leer sind, entfällt. Das Paket hat keinen Zustand,
   keine Tabelle und kennt keinen Owner.
2. **Die Versionen gehören dem Träger bei Organisation & Tenancy**
   (`platform.traeger_document_templates`, Schreib-Owner
   `organization-tenancy`), neben Briefkopf und Bankverbindungen. Jedes
   Speichern legt die nächste Version an; ein Trigger lehnt jede Änderung ab,
   ausgenommen das `ON DELETE SET NULL` des speichernden Kontos. Ohne eigene
   Version gilt die Standardvorlage (Version 0). Lesen und Schreiben laufen in
   der Träger-Transaktion des Sitzungsträgers (ADR 0105) unter der
   Träger-Policy; jede neue Version steht mit alter und neuer Nummer im
   Protokoll der Stammdaten (`subject` `template`). Lesen braucht
   „Übersicht lesen“, Speichern und die Vorschau „Einstellungen“, wie die
   übrigen Stammdaten.
3. **Die Vorschau füllt die Vorlage mit Beispieldaten** und speichert nichts.
   Sie prüft wie das Speichern; ihre Werte sind erfunden und enthalten keine
   personenbezogenen Daten.
4. **Das Bestätigungsschreiben füllt die aktuelle Vorlage des Trägers.** Der
   Root-gebundene Port `parentbilling.BillingLetterheads` liefert sie mit dem
   Briefkopf (`TraegerDocumentTemplates.Current`, Lesen in der
   Träger-Transaktion). Die Elternabrechnung füllt jeden Platzhalter ihrer Art
   mit den Werten der Rechnung; Anschriftfeld, Infoblock und Fußzeile bleiben
   wie in ADR 0121. Die Standardvorlage ist das Schreiben von ADR 0121, Feld für
   Feld wie die Bissendorfer Vorlage; einzige Änderung: Auch der Satz „Sie
   müssen für diese Betreuung nichts bezahlen.“ steht fett, weil der
   Zahlungsabsatz der Vorlage fett ist.
5. **Ein ausgestelltes Schreiben behält seine Version.**
   `parent_billing.run_letters.template_version` hält fest, mit welcher
   Version die Schreiben eines Laufs entstanden (0 für alle bisherigen); das
   gespeicherte PDF ändert sich ohnehin nie (ADR 0121).
6. **Grenzen.** Policy-Epoche 97 erlaubt genau sechs Regeln in das Paket
   `modules/documentrendering/templates`, nur in Produktion: aus
   Organisation & Tenancy `public`, `port`, `application`, `postgres` und
   `http`, aus der Elternabrechnung `application`. Der Akzeptor
   `reviewedDocumentTemplatesRule` (`internal/architecture`) nimmt nur diese
   Form an. Beide Quellen sind Domänen, das Ziel eine Plattform; kein Zyklus
   entsteht (ADR 0058). Zwischen Organisation & Tenancy und der
   Elternabrechnung gibt es keinen Import; die Root bindet den Port.

## Folgen

- Weitere Dokumente (Vertrag #4678, Bescheid #4688, Mahnung #4820) nutzen
  dieselbe Verwaltung und füllen die Platzhalter ihrer Art. Braucht ein
  Dokument einen neuen Platzhalter, ergänzt es den Katalog seiner Art.
- Die Standardvorlagen der Arten ohne Dokument sind Startpunkte; ihr Text
  wird mit dem jeweiligen Dokument gegen die Kundenvorlagen geprüft.
- Eine Version löscht niemand einzeln. Sie geht mit dem Träger
  (Datenlebenszyklus `scope: organization`).

## Verworfene Alternativen

- **Vorlagen als Träger-Werte der Einstellungen:** Das Glossar trennt
  Vorlagen von Träger-Werten, und Einstellungen haben keine Versionen.
- **Tabellen bei Document Rendering:** Die Plattform müsste Träger-Policy,
  Protokoll und Stammdaten-Rechte nachbauen, die Organisation & Tenancy schon
  hat (ADR 0112).
- **Vorlagensprache in jedem Owner:** Organisation & Tenancy prüft die
  Platzhalter, die Elternabrechnung füllt sie. Zwei Kopien des Katalogs
  laufen auseinander.
- **Füllen über einen Root-gebundenen Port:** hielte die Elternabrechnung
  frei von Document Rendering, aber der Golden-Test des Schreibens könnte die
  Standardvorlage nicht mehr selbst prüfen.

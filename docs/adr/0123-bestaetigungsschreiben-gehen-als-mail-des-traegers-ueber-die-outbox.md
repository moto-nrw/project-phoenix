---
status: accepted
---

# Bestätigungsschreiben gehen als Mail des Trägers über die Outbox

Entscheidung vom 26.10.2026 zu [#4670](https://github.com/moto-nrw/project-phoenix/issues/4670),
Etappe 1 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 94, 95, 97 und 100. Sie ist der vierte Bau-Schnitt des Owners
Elternabrechnung aus
[ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
nach [ADR 0121](0121-bestaetigungsschreiben-ist-die-rechnung-im-layout-des-traegers.md).
Sie führt keinen Owner ein und hält fest, was der Versand in Delivery und in
den Registern ändert (ADR 0057 Punkt 5, ADR 0070).

## Kontext

Ein freigegebener Lauf hat je Rechnung ein unveränderliches
Bestätigungsschreiben als PDF (ADR 0121). Das Träger-Büro will es per Klick an
eine Familie oder als Serie an alle Familien eines Laufs senden, Zustellfehler
sehen und erneut senden, und alle Schreiben für Familien ohne Internet drucken.
Eine Eltern-App mit Postfach kommt erst mit #4671; bis dahin bekommt jede
Familie das Schreiben per Mail. Delivery kennt Mails eines Trägers ohne Schule
(ADR 0110), aber keine Anhänge und keine Statusabfrage in der
Träger-Transaktion.

## Entscheidung

1. **Outbox im selben Schreibvorgang.** Ein Versand legt in einer
   Träger-Transaktion die Mail des Trägers in Delivery ab
   (`EnqueueEmail` mit `OrganizationID`, Art `parent_billing_letter`, der Lauf
   als verknüpfte Entität) und hält den Versand in
   `parent_billing.document_mailings` fest: Rechnung, Eintrag der Outbox,
   Adresse, sendendes Konto, Zeitpunkt. Scheitert etwas, rollt beides zurück.
   Der Worker sendet erst nach dem Commit.
2. **Das PDF reist im Payload.** Die Mail trägt das gespeicherte PDF, den
   Trägernamen, den Zeitraum und die Rechnungsnummer als festen Wert im
   Payload. Der Worker braucht so keine Träger-Transaktion und keinen Zugriff
   auf die Elternabrechnung; die Mail ändert sich nach dem Versand nicht. Die
   Outbox löscht sie mit ihrer Frist für Mails ohne Schule (30 Tage nach
   Ende); der Versand in `document_mailings` bleibt mit dem Schreiben.
3. **Anhänge in Delivery.** `mail.Message` bekommt `Attachments`, der
   SMTP-Adapter hängt sie an (go-mail). Leere Anhänge lehnt er ab, statt eine
   Mail ohne Schreiben zu senden. Der Renderer der Elternabrechnung
   (`compose.NewLetterMailRenderer`) liefert Betreff, Vorlage
   `billing-letter.html`, Text und Anhang; die Root bindet ihn im Worker und
   ordnet die Art „Basis“ zu, weil eine Mail ohne Schule kein Modul bucht.
   Absender ist der Träger mit der zentralen Adresse; sein Name ist die erste
   Zeile des Briefkopfs, wie in der Grußformel des Schreibens.
4. **Status je Empfänger.** Delivery bekommt `TraegerEmailStatuses`: die
   Mails eines Trägers zu einer Entität in dessen Träger-Transaktion (Policy
   aus ADR 0110). Der Lauf zeigt je Familie den letzten Versand: „nicht
   gesendet“, „wird gesendet“ (wartet oder neuer Versuch), „gesendet“ (vom
   Mailserver angenommen), „nicht zugestellt“ (Delivery gibt auf oder bricht
   ab) und „Ergebnis nicht mehr bekannt“, wenn die Outbox die Mail schon
   gelöscht hat.
5. **Erneut senden, idempotent.** Die Serie sendet nur an Familien mit
   Adresse und ohne Versand; ein zweiter Aufruf sendet nichts doppelt. Eine
   Familie lässt sich einzeln senden, zuerst oder erneut, außer ihre letzte
   Mail wartet noch (409 `parent_billing.letter_mail_pending`). Beide sperren
   die Lauf-Zeile; der Schlüssel der Outbox zählt die Versände eines
   Schreibens (`parent-billing-letter:<Rechnung>:<n>`), so ist jeder Versand
   genau ein Eintrag.
6. **Druck.** Alle Schreiben eines Laufs gehen in der Reihenfolge der
   Rechnungsnummern in ein PDF. Document Rendering bekommt dafür
   `LetterRenderer.CombineLetters`; es importiert die Seiten der gespeicherten
   PDFs (gopdf mit gofpdi, schon im Modulgraphen) und setzt nichts neu.
7. **Zugriffsprotokoll.** Jeder Versand steht mit Konto und Adresse in
   `document_mailings`. Status, Serie und Druck sind `read` in
   `parent_billing.run_events` (`mail`: `status` oder `series`, `letter`:
   `print`). Die Aktionen der Spur bleiben unverändert.
8. **Grenzen.** Alle Routen laufen in `TraegerTxMiddleware` hinter dem Recht
   „Abrechnung“. Die Anwendung kennt nur ihren Port
   `parentbilling.BillingLetterPost`; die Root bindet ihn an Delivery und an
   `TraegerSettings.TraegerTexts`. Kein Workflow, keine Admin-Transaktion.
9. **Register.**
   - `policy.json`: ein `data_object` `parent_billing.document_mailings` mit
     `write_owner` `parent-billing`.
   - `command-effects.json`: fünf geprüfte Bindungen (Outbox: platform/durable
     nach ADR 0098; Status, Briefkopf und die zwei Bindungen von
     `CombineLetters`: query).
   - `passthrough.json`: drei Methoden mit Entscheidung 0123.
   - Datenlebenszyklus: ein Eintrag mit `scope: organization`; der Versand
     geht mit seinem Schreiben.

## Verworfene Optionen

- **Nur die Rechnungs-ID im Payload, PDF beim Senden lesen:** Der Worker
  bräuchte eine eigene Träger-Transaktion und eine Bindung an die
  Elternabrechnung. Das PDF ändert sich nie, eine Kopie im Payload ist
  einfacher und nach 30 Tagen weg.
- **Link statt Anhang:** bräuchte einen öffentlichen Download mit Token für
  Familien ohne Konto; der Anhang erreicht sie ohne neue Angriffsfläche.
- **Status nur aus der Outbox:** Nach ihrer Frist wüsste niemand mehr, dass
  ein Schreiben gesendet wurde, und eine Serie sendete es erneut.
- **Neue Aktion `mail` in der Spur des Laufs:** Der Versand braucht ohnehin
  einen eigenen Eintrag mit Outbox und Adresse; eine weitere Aktion änderte
  die Prüfregel der Spur ohne Gewinn.

## Folgen

- Das Postfach in der Eltern-App (#4671) liest dasselbe PDF; Familien mit
  Eltern-App können dann statt der Mail eine Benachrichtigung bekommen.
- Weitere Dokumentarten (Vertrag, Bescheid, Mahnung) können denselben Weg
  nehmen.

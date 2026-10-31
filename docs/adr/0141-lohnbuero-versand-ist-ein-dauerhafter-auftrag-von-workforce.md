---
status: accepted
---

# Lohnbüro-Versand ist ein dauerhafter Auftrag von Workforce

Entscheidung vom 30.10.2026 zu [#4805](https://github.com/moto-nrw/project-phoenix/issues/4805).
Sie führt keinen neuen Owner ein und ändert keine Abhängigkeitsrichtung.

## Kontext

Nach dem Monatsabschluss soll die Monatsdatei aller Mitarbeitenden ohne
weiteren Klick an das Lohnbüro gehen: per SFTP oder per E-Mail mit einem
befristeten Link. Ein Fehlschlag wird bis zu dreimal wiederholt, danach
werden die Personen mit `time_tracking:manage` benachrichtigt. Jeder Versand
steht im vorhandenen Übertragungsprotokoll `audit.export_transfers`.

Export Transfer ([ADR 0012](0012-export-transfer-is-the-tenth-platform.md))
sendet fertige Dateien und kennt keinen Inhalt; sein Dienst wiederholt bewusst
nichts. Den Inhalt und den Abschluss besitzt Workforce.

## Entscheidung

1. **Auftrag bei Workforce.** Die neue Tabelle `active.payroll_dispatches`
   gehört Workforce. Der Monatsabschluss trägt den Auftrag in seiner eigenen
   Transaktion ein ([ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)
   Punkt 6), nur wenn die Schule den Versand eingeschaltet hat und der
   Abschluss mindestens eine Person einfriert. Ein weiterer Abschluss desselben
   Monats ergibt eine Korrektur mit `_korrektur-N` im Dateinamen.
2. **Arbeit im Worker.** Der Job `payroll-dispatch` least fällige Aufträge,
   baut die Datei mit dem vorhandenen Export (Zugriffsprotokoll unter der
   Person, die den Monat abgeschlossen hat) und schreibt jedes Ergebnis in
   einer eigenen, durch den Lease geschützten Transaktion. Die Wiederholung
   (5 Minuten, 30 Minuten, 2 Stunden) liegt beim Auftrag, nicht bei Export
   Transfer: jeder Versuch ist ein eigener Journaleintrag.
3. **Vorprüfung wie im Dialog.** Ein DATEV-Format geht nur raus, wenn die
   Prüfung von `datev-report` niemanden überspringt und jede Lohnart eine
   Nummer hat. Sonst ist der Auftrag gesperrt (`blocked`); `/payroll` zeigt den
   Grund, die Benachrichtigung geht in derselben Transaktion in die Outbox.
4. **E-Mail ohne Anhang.** Die Datei bleibt 14 Tage in der Zeile; gespeichert
   ist nur der SHA-256-Fingerabdruck des Links. Der Link nennt die Schule
   (`<id>.<token>`), damit der öffentliche Abruf ohne Verwaltungsrolle in der
   Transaktion der Schule läuft. Er öffnet nur per POST hinter einer Seite mit
   Knopf, weil Mail-Scanner jedem Link per GET folgen; der erste Abruf setzt
   `downloaded_at` und löscht die Datei. Delivery stellt die Mail mit vier
   Versuchen zu; der Worker liest ihren Zustand und schreibt erst dann den
   Journaleintrag.
5. **Protokoll.** `audit.export_transfers` erhält Weg, Monat und Empfänger.
   Export Transfer bietet dafür `RecordEmail`; SFTP-Versuche tragen den Monat.
6. **Ports und Wurzel.** Die Ports stehen im öffentlichen Paket von Workforce
   (`payroll_dispatch_ports.go`), jede Bindung der Wurzel spricht genau einen
   Owner an; die Texte der Mails schreibt Workforce.
7. **Register.** `NewPayrollDispatch` und das Feld der Komposition heben das
   Compose-Budget von `modules/workforce/compose` um genau 9 Zeilen.
   `PayrollDispatch.Overview` übersetzt die Zeile auf die öffentliche Ansicht
   (die Form „Owner-Kante übersetzt auf den Vertrag“ aus
   [ADR 0075](0075-pass-throughs-keep-a-per-owner-floor.md)) und hebt das
   Pass-through-Budget von Workforce um 1. Die vier Übersetzungen der Wurzel
   stehen mit ADR 0090 im Pass-through-Register.

## Verworfene Optionen

- **Versand im Request des Abschlusses:** ein SFTP-Ausfall würde den
  Abschluss scheitern lassen oder die Datei still verlieren.
- **Wiederholung in Export Transfer:** der Dienst bliebe nicht mehr ein
  einmaliger, sichtbarer Versuch, und die manuelle Übertragung erbte eine
  Hintergrundwiederholung, die niemand angefordert hat.
- **Datei als Anhang:** Lohndaten lägen dauerhaft in fremden Postfächern.
- **Link per GET:** der Vorschauabruf eines Mail-Scanners würde den
  einmaligen Link verbrauchen.

## Folgen

- Die Schul-Löschung kennt die neue Tabelle (Migration 1.15.718); ihr
  Fremdschlüssel auf die Schule ändert den FK-Fingerabdruck.
- Die Datei eines nicht geöffneten Links verschwindet mit dem Ablauf des
  Links, die Zeile 400 Tage nach dem Ende des Auftrags.

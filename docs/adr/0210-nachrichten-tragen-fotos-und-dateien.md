---
status: accepted
---

# Nachrichten tragen Fotos und Dateien

Vorschlag vom 11.11.2026 zu
[#6685](https://github.com/moto-nrw/project-phoenix/issues/6685). Die
Freigabe erteilt Yannick Wenger mit dem Merge des Pull Requests, der diese
ADR einführt; vorher gilt sie nicht. Sie führt keinen neuen Owner ein.

## Kontext

Ein Vater der Ferienbetreuung Bissendorf wünscht Fotos im Nachrichtenverlauf
zwischen Betreuung und Eltern. Bisher trägt eine Nachricht nur Text: in der
Unterhaltung mit einem Elternteil, in der Gruppen-Nachricht an Eltern und im
Team-Chat. Anhänge gibt es nur an Elternmitteilungen (#2890); dort gehören
sie der Dateiablage.

## Entscheidung

1. **Alle drei Unterhaltungen, beide Richtungen.** Personal und Eltern
   hängen an eine Nachricht bis zu fünf Dateien mit je höchstens 10 MB:
   PDF, PNG, JPEG, DOCX, XLSX, PPTX. Eine Nachricht darf nur aus Anhängen
   bestehen.
2. **Communication besitzt die Anhänge.** `users.message_attachments` und
   `users.message_attachment_cleanup` gehören Communication, weil ein Anhang
   Teil seiner Nachricht ist: dieselben Leser, dieselbe Löschfrist. Die
   Dateiablage bleibt beim Dokumentenspeicher der Schule und den Anhängen
   der Elternmitteilungen.
3. **Erst hochladen, dann senden.** Der Upload legt in einer eigenen
   Transaktion eine unzugeordnete Zeile an und schreibt danach die Bytes in
   den privaten Objektspeicher (`message-attachments/{Schule}/{Zufallsname}`).
   Die Nachricht nennt die IDs ihrer Uploads. Ihr Store sperrt genau die
   Uploads des Absenders in derselben Unterhaltungsart und bindet sie im
   selben Insert an die Nachricht. Nachricht und Anhänge werden zusammen
   festgeschrieben; ein fremder oder schon gesendeter Upload weist das
   Senden ab.
4. **Gelesen wird über die Unterhaltung.** Personal liest einen Anhang mit
   dem Recht, das die Nachricht liest (`users:read` und Kind-Lesezugriff, im
   Team-Chat die Teilnahme, in Gruppen ab dem Beitritt). Eltern lesen ihn
   nur über eine eigene Unterhaltung zu diesem Kind oder eine
   Gruppen-Nachricht, die sie für dieses Kind erreicht hat. Jede Ablehnung
   antwortet wie eine fehlende Datei. Hochladen braucht bei Eltern
   `parent_portal.notes.write` und läuft hinter dem Kind-Schreibschutz
   (#6529).
5. **Prüfung wie beim Dateiimport.** Größe, Endung und Magic Number; ein
   Bild verliert vor dem Speichern Exif-, GPS- und sonstige Metadaten.
   Ausgeliefert wird mit `nosniff`, `no-store` und, bei der Anzeige im
   Browser, mit Sandbox-CSP.
6. **Löschfrist der Nachricht.** Ein gesendeter Anhang hat keine eigene
   Frist: er fällt per Kaskade mit seiner Nachricht (Löschfrist der
   Elternnachrichten #3959, des Team-Chats, Löschung der Unterhaltung oder
   der Schule). Ein nicht gesendeter Upload verfällt nach 24 Stunden. Ein
   Trigger reiht das Objekt jeder gelöschten Zeile in derselben Transaktion
   ein; der bestehende Aufräumlauf der Dateien entfernt die Bytes.
7. **Momentaufnahme an der Nachricht.** Die drei Nachrichtentabellen tragen
   ihre Anhänge als `attachments JSONB`. Eine Unterhaltung lädt damit ohne
   zusätzliche Abfrage; die Abfragebudgets bleiben unverändert.
8. **Keine Kopplung an die Fotoeinwilligung.** Die Fotoeinwilligung (#6650)
   regelt Fotos, die die Schule von einem Kind zeigt oder weitergibt. Eine
   Nachricht geht an die Sorgeberechtigten genau dieses Kindes; eine
   Gruppen-Nachricht erreicht dagegen mehrere Familien. Die Oberfläche
   weist beim Anhängen an eine Gruppen-Nachricht darauf hin, dass alle
   Empfänger die Datei sehen. Eine technische Sperre nach Einwilligung
   bleibt eine fachliche Entscheidung zu #6650 und ist hier nicht
   vorweggenommen.

## Budgets

Diese ADR ist der Beleg nach
[ADR 0057](0057-only-the-target-is-a-rule.md) Punkt 5 für genau diese
gemessenen Stellen:

- `moduleComposeBudgets["modules/communication/composition"]` 351 → 376:
  der Anhang-Store der drei Messenger, die Elternseite der Anhänge und der
  Aufräumlauf der Objekte (+25).
- `moduleComposeBudgets["workflows/parentportal/compose"]` 333 → 335: der
  Port der Anhänge (+2). `moduleComposeBudgetTotal` + 27.
- `demoErasureExceptionBudget` 187 → 189: `users.message_attachments` und
  `users.message_attachment_cleanup` bleiben wie die Nachrichten, zu denen
  sie gehören, bei der stillgelegten Demo-Schule (bis #5559).

Eine neue Policy-Regel braucht die Änderung nicht: die Nachrichten-Stores
sperren und binden die Uploads mit eigenen Anweisungen auf den Tabellen
ihres Owners; die gemeinsame Prüfung liegt im öffentlichen Paket.

## Verworfene Alternativen

- **Anhänge in der Dateiablage wie bei Elternmitteilungen:** die Dateiablage
  müsste die Leser jeder Unterhaltung kennen, auch Teilnahme und Beitritt im
  Team-Chat. Lesen und Löschen folgen der Nachricht; sie gehören zu ihr.
- **Bytes in der Datenbank:** vergrößert Sicherung und Rückgabepaket und
  umgeht den bestehenden privaten Objektspeicher.
- **Binden in der Dienstschicht statt im Store:** die Elternseite schreibt
  über den gemeinsamen Unterhaltungskern, der keinen Anhang-Store kennt; das
  Binden wäre dann an drei Stellen statt an einer.
- **Eigene Löschfrist für Anhänge:** zwei Fristen für eine Nachricht; ein
  Text ohne sein Foto oder ein Foto ohne seinen Text wäre die Folge.

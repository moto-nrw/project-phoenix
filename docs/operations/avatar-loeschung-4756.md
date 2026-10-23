# Profilbilder anonymisierter Konten löschen (#4756, Slice 4756-a)

## Im Code umgesetzt

Migration 1.15.486 legt eine globale, nur administrativ zugängliche Warteschlange
an. `auth.erase_unused_accounts()` trägt dort vor dem Leeren der Konto- und
Profilverweise hochgeladene Profilbilder ein. Auftrag und Anonymisierung werden
gemeinsam committed oder zurückgerollt. Externe Bild-URLs erzeugen keinen Auftrag.

Der stündliche Token-Aufräumjob bearbeitet nach der Anonymisierung höchstens
100 Aufträge in einer eigenen administrativen Transaktion. Der Schritt läuft
auch beim Worker-Start und ohne neu anonymisierte Konten. Er sperrt die
gewählten Aufträge mit `FOR UPDATE SKIP LOCKED`. Dateifehler bleiben gespeichert
und werden hinter noch nicht versuchte Aufträge gestellt. Erfolgreiche
Löschungen werden trotz anderer Dateifehler bestätigt; der Lauf meldet den
Teilfehler. Der Auftrag verschwindet sofort nach Löschung oder bestätigtem
Fehlen der Datei. Ein Abbruch vor dieser Bestätigung führt zu einer sicheren
Wiederholung. Es gibt keine maximale Zahl von Versuchen.

Die Dateilöschung bleibt mit `os.Root` in `public/uploads/avatars`, einschließlich
`global/` und der alten Unterordner `<tenantId>/`. Sie folgt keinem Symlink aus
dieser Ablage heraus. Ungültige Pfade werden nicht gelöscht und bleiben als
fehlgeschlagene Aufträge sichtbar. Fehlermeldungen enthalten nur die Auftrags-ID,
keine Dateipfade oder personenbezogenen Inhalte.

Lokale Tests decken Commit, Rollback, Wiederholung nach Dateifehler,
Wiederaufnahme mit einer frischen Komposition, schon fehlende Dateien,
Mandantenschutz, Zugriffsrechte und Pfadbegrenzung ab. Das ist kein Nachweis
für das produktive System.

## Produktiv nachzuweisen (offen, Mensch-Schritte)

1. Migration und Backend gemeinsam ausrollen. Der Worker braucht Schreibrechte
   auf aktuelle und alte Avatar-Unterordner. Kein Rollback entfernt die
   Warteschlange; bei Problemen einen Forward-Fix verwenden.
2. Nach einem regulären stündlichen Lauf prüfen, ob der Schritt
   `Erased account avatar cleanup` erfolgreich war. Bei Fehlern Rechte und
   Speicherzustand prüfen; keine Dateipfade oder personenbezogenen Inhalte in
   Tickets oder Nachweisen ablegen. Für offene Aufträge nur Mengen und Alter
   erfassen, beispielsweise mit dieser lesenden administrativen Abfrage:

   ```sql
   SELECT count(*) AS pending, min(created_at) AS oldest_pending,
          count(*) FILTER (WHERE attempted_at > '-infinity'::timestamptz) AS attempted
   FROM auth.account_avatar_file_cleanup;
   ```

3. Nach behobenem Dateifehler den nächsten Lauf auf Erfolg und sinkende Anzahl
   prüfen. Datum, ausgerollte Version und das Ergebnis ohne Personendaten im
   internen Nachweis festhalten. Keine echte Datei absichtlich beschädigen.
4. Altbestand gesondert prüfen: Vor diesem Fix anonymisierte Konten haben ihre
   Bildverweise bereits verloren. Diese Dateien lassen sich nicht nachträglich
   aus den Konten in die Warteschlange übernehmen. Ein Mensch muss aktuelle
   Referenzen und die alte Ablage abgleichen, bevor er verwaiste Dateien löscht.
   Dieser Slice führt keinen pauschalen Verzeichnis-Löschlauf ein.

## AVV/TOM-Abgleich

Keine neue Frist und keine neue Datenkategorie. Textvorschlag für den internen
TOM-Nachweis: „Löschaufträge für Profilbilder werden zusammen mit der
Anonymisierung dauerhaft gespeichert. Ein stündlicher Aufräumschritt wiederholt
fehlgeschlagene Löschungen. Produktive Wirksamkeit und bereinigter Altbestand
sind gesondert nachzuweisen.“ Der produktive Nachweis bleibt offen.

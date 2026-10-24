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
fehlgeschlagene Aufträge sichtbar. Fehlt die Speicherwurzel, bleiben die Aufträge
ebenfalls erhalten; nur eine fehlende Datei innerhalb der geöffneten Wurzel gilt
als Erfolg. Fehlermeldungen enthalten nur die Auftrags-ID,
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
4. Altbestand nach dem folgenden Verfahren abgleichen und ausschließlich über
   die Warteschlange löschen. Bereits geleerte Kontoverweise erlauben keinen
   rückwirkenden Backfill allein aus den Konten.

## Altbestand: Probelauf und Eintragen in die Warteschlange

**Nur für einen berechtigten Menschen auf dem Server.** Hier wurden keine
Produktions- oder Staging-Zugriffe ausgeführt. Die Befehle verwenden die bereits
bereitgestellte Compose-Konfiguration in `~/production/`, keine API und kein SSH.
Die drei SQL-Dateien aus `docs/operations/sql/` dieses geprüften Releases müssen
lokal auf dem Server bereitliegen: aus dem ausgerollten Release-Verzeichnis
übernehmen oder aus dem Repo-Stand genau dieser Version per `scp` übertragen.
Nicht die `.env` ausgeben oder verändern.

Geprüfter Speicherort: `environments/production.compose.yml` bindet das Volume
`phoenix-production-uploads` im Dienst `server` nach `/app/public/uploads` ein.
Die Avatarwurzel ist `/app/public/uploads/avatars`, aktuell mit `global/`, früher
mit `<tenantId>/`. Der Upload in `identityaccess/inbound/me/profile.go` schreibt
`/uploads/avatars/global/...`; der Cleaner unterstützt beide Unterordner.

Die dauerhaften Referenzquellen sind `auth.accounts.avatar`,
`users.profiles.avatar` und offene Aufträge in `auth.account_avatar_file_cleanup`.
Die Avatarspalten-Migrationen und die aktuellen Upload-/Profilpfade wurden darauf
geprüft. Auch Profile anderer Schulen und bereits anonymisierte Datensätze
werden ohne Filter einbezogen. Externe URLs sind keine lokalen Dateireferenzen.
Andere Upload-Arten liegen außerhalb dieser Wurzel und werden nicht erfasst.
Vor Anwendung auf einen späteren Release diese Quellen erneut prüfen.

1. Nur einen Durchlauf gleichzeitig durchführen. Keine parallelen
   Wiederherstellungen oder Datei-Importe, bis alle
   eingetragenen Aufträge abgearbeitet sind. Uploads, API und Worker laufen weiter:
   zufällige Dateinamen (`UploadFilenameSuffix`) verhindern die erneute Referenz
   alter Dateien, die 24-Stunden-Grenze schließt frische Uploads aus und bereits
   gelöschte Kandidaten quittiert der Cleaner mit `ErrNotExist`.

2. Den folgenden **zusammenhängenden Block** in einer Bash-Sitzung ausführen.
   Er startet eine eigene Bash mit festem Arbeitsverzeichnis `~/production`.
   Die drei SQL-Dateien müssen dort unter `docs/operations/sql/` liegen.
   `row_security = off` lässt fehlende RLS-Umgehungsrechte als Fehler auffallen,
   statt Schulen unbemerkt auszulassen. Kein Shell-Tracing und keine
   Terminal-Aufzeichnung. Die private Arbeitsablage wird bei Erfolg, Fehler,
   Abbruch oder regulärem Signal auf Host und im PostgreSQL-Container entfernt.
   Bei `SIGKILL`, Rechnerausfall oder fehlgeschlagenem Container-Aufräumen muss
   ein Mensch verbliebene `avatar-abgleich.*`-Verzeichnisse lokal entfernen.
   Die aufrufende interaktive Sitzung bleibt bei Fehlern geöffnet.
   Container-Befehle ohne Dateieingabe erhalten `</dev/null`, damit sie nicht
   den restlichen Bash-Block von stdin lesen.

3. Der Block erfasst nur reguläre Dateien, deren Änderungszeit **mehr als
   24 Stunden** zurückliegt. `find` folgt keinen Symlinks; nur genau zwei Ebenen
   unter der Wurzel, bekannte Unterordner und einfache Upload-Dateinamen werden
   übernommen. Andere Namen werden nicht automatisch gelöscht. Der Probelauf
   verändert keine Dateien und schreibt nur temporäre Tabellen/Views.

4. Mengen des Probelaufs prüfen und erst dann `ja` eingeben. Der SQL-Pfad
   validiert das Layout und gleicht erneut **alle drei Referenzquellen** ab.
   Kurze Tabellensperren schützen den Abgleich gegen DB-Schreiber; nach fünf
   Sekunden Sperrwartezeit wird abgebrochen. Vorhandene Aufträge erzeugen kein
   Duplikat. Kein `rm` an der Avatarablage verwenden. Bei unterbrochener
   Verbindung den Commit-Zustand administrativ klären, bevor der Ablauf erneut
   gestartet wird. Die Shell überschreibt keine vorhandene Eintragsliste.

5. Nach regulären Worker-Läufen Enter drücken und ausschließlich Mengen prüfen.
   Bei mehr als 100 Aufträgen sind mehrere Stunden möglich. Fehlende Wurzel,
   Rechtefehler oder ungültige Pfade lassen Aufträge offen. Nur bei
   `noch_offen = 0` mit `fertig` beenden. Bei einem Fehler abbrechen und die
   Ursache lokal beheben; Wiederherstellungen und Importe bleiben untersagt,
   solange Aufträge offen sind. Nachweis: Datum, Release und ausschließlich
   **Anzahl gefunden, eingetragen, noch offen und abgearbeitet**. `abgearbeitet`
   bedeutet: gelöscht oder innerhalb der vorhandenen Wurzel fehlend bestätigt.
   Pfadlisten und Fehlerdateien enthalten möglicherweise Konto-IDs; sie bleiben
   privat und gehören nie ins Terminal, Ticket, PR oder Versionskontrollsystem.

```bash
bash -euo pipefail <<'EOF'
set +x
umask 077
cd ~/production
compose=(docker compose --env-file "$PWD/.env" -f "$PWD/docker-compose.yml")
private=$(mktemp -d "${TMPDIR:-/tmp}/avatar-abgleich.XXXXXXXX")
container_private=
cleanup() {
  result=$?
  trap - EXIT
  if [ "$result" -ne 0 ]; then
    printf 'Abgleich abgebrochen; Ursache und gegebenenfalls Commit-Zustand lokal klären.\n' >&2
  fi
  if [ -n "$container_private" ]; then
    if ! "${compose[@]}" exec -T postgres sh -eu -c \
      'rm -rf -- "$1"' sh "$container_private" </dev/null >/dev/null 2>&1; then
      printf 'Container-Aufräumen fehlgeschlagen; temporäre Listen lokal entfernen.\n' >&2
      result=1
    fi
  fi
  rm -rf -- "$private" || result=1
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
# Auf die lokal bereitgestellten SQL-Dateien dieses Releases setzen:
sql_dir="$PWD/docs/operations/sql"
db() { "${compose[@]}" exec -T postgres psql -X -q -v ON_ERROR_STOP=1 -U postgres -d postgres "$@"; }
test -r "$sql_dir/avatar-referenzen-4756.sql"
test -r "$sql_dir/avatar-waisen-eintragen-4756.sql"
test -r "$sql_dir/avatar-waisen-status-4756.sql"

# Inventur und Abgleich, nur Mengen ausgeben.
"${compose[@]}" exec -T server sh -eu -c '
  cd /app/public/uploads/avatars
  find . -mindepth 2 -maxdepth 2 -type f -mmin +1440 -exec sh -eu -c '\''
    for file do
      relative=${file#./}
      folder=${relative%%/*}
      name=${relative#*/}
      case "$folder" in
        global) ;;
        ""|0*|*[!0-9]*) continue ;;
      esac
      case "$name" in
        ""|[!A-Za-z0-9_-]*|*[!A-Za-z0-9_.-]*) continue ;;
      esac
      printf "/uploads/avatars/%s\n" "$relative"
    done
  '\'' sh {} +
' </dev/null 2> "$private/inventory.err" | LC_ALL=C sort -u > "$private/files.txt"
db -At -f - < "$sql_dir/avatar-referenzen-4756.sql" \
  > "$private/references.unsorted" 2> "$private/references.err"
LC_ALL=C sort -u "$private/references.unsorted" > "$private/references.txt"
LC_ALL=C comm -23 "$private/files.txt" "$private/references.txt" > "$private/candidates.txt"
printf 'Alte Dateien im bekannten Layout: '; wc -l < "$private/files.txt"
printf 'Unreferenzierte Kandidaten: '; wc -l < "$private/candidates.txt"

# Probelauf im PostgreSQL-Container.
container_private=$("${compose[@]}" exec -T postgres sh -eu -c \
  'umask 077; mktemp -d /tmp/avatar-abgleich.XXXXXXXX' </dev/null)
"${compose[@]}" exec -T postgres sh -eu -c \
  'umask 077; cat > "$1/candidates.txt"' sh "$container_private" < "$private/candidates.txt"
"${compose[@]}" exec -T postgres sh -eu -c \
  'umask 077; cat > "$1/enqueue.sql"' sh "$container_private" < "$sql_dir/avatar-waisen-eintragen-4756.sql"
"${compose[@]}" exec -T postgres sh -eu -c \
  'cd "$1"; psql -X -q -v ON_ERROR_STOP=1 -v apply=false -U postgres -d postgres -f enqueue.sql' \
  sh "$container_private" </dev/null 2> "$private/preview.err"

printf 'Probelauf prüfen. Nur bei plausiblen Mengen eintragen.\n'
read -r -p 'Zum Eintragen ja eingeben: ' answer </dev/tty
[ "$answer" = ja ] || exit 0
"${compose[@]}" exec -T postgres sh -eu -c \
  'cd "$1"; test ! -e enqueued.txt; psql -X -q -v ON_ERROR_STOP=1 -v apply=true -U postgres -d postgres -f enqueue.sql' \
  sh "$container_private" </dev/null 2> "$private/apply.err"

# Mengen bis zum Abschluss verfolgen.
"${compose[@]}" exec -T postgres sh -eu -c \
  'umask 077; cat > "$1/status.sql"' sh "$container_private" < "$sql_dir/avatar-waisen-status-4756.sql"
while :; do
  "${compose[@]}" exec -T postgres sh -eu -c \
    'cd "$1"; psql -X -q -v ON_ERROR_STOP=1 -U postgres -d postgres -f status.sql' \
    sh "$container_private" </dev/null 2> "$private/status.err"
  read -r -p 'Nach Worker-Lauf Enter; nur bei noch_offen = 0 fertig eingeben: ' answer </dev/tty
  [ "$answer" != fertig ] || break
done
EOF
```

**Lokal geprüft, produktiv offen:** SQL-Export, Probelauf, erneuter
Referenzabgleich, strenge Pfadprüfung, idempotentes Eintragen und
Mengen-Nachverfolgung wurden mit synthetischen Daten lokal gegen eine isolierte
Test-DB geprüft. Auch der Dateilistenbefehl wurde im lokalen Alpine-Testcontainer
mit alten/frischen Dateien und Symlinks ausgeführt. Der produktive Altbestand
ist erst nach Durchführung dieses Verfahrens nachgewiesen.

## AVV/TOM-Abgleich

Keine neue Frist und keine neue Datenkategorie. Textvorschlag für den internen
TOM-Nachweis: „Löschaufträge für Profilbilder werden zusammen mit der
Anonymisierung dauerhaft gespeichert. Ein stündlicher Aufräumschritt wiederholt
fehlgeschlagene Löschungen. Produktive Wirksamkeit und bereinigter Altbestand
sind gesondert nachzuweisen.“ Der produktive Nachweis bleibt offen.

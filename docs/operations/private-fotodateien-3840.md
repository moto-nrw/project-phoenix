# Kinderfotos und Avatare: private Dateirechte

Slice 3840-h, Teil von #3840 und #3846. Stand: 23.10.2026.

## Im Code umgesetzt

Kinderfotos und Avatare verwenden `common.SavePrivateImage` und den vorhandenen
privaten Speicherweg `SavePrivateNamedFile`. Neue Dateien erhalten `0600`, das
Zielverzeichnis `0700`. Dateinamen, gespeicherte URLs, Validierung und Löschung
bleiben unverändert. Öffentliche Login-Bilder und PDFs nutzen weiterhin ihren
bisherigen Schreibweg.

Der lokale Speicher sichert bei einem privaten Upload auch das vorhandene
Zielverzeichnis mit `0700`. Er migriert dabei **nicht** die Dateirechte älterer
Dateien und nicht alle übergeordneten Verzeichnisse. Der unten beschriebene
Bestandsschritt bleibt daher erforderlich. Diese Änderung führt keine
Datei- oder Datenträgerverschlüsselung ein und schließt #3840 nicht ab.

### Auslieferung und Container-Grenze

Im eingecheckten Stand geprüft:

- `backend/modules/peopledirectory/inbound/students/photo.go`: Kinderfotos
  werden nach Tenant-, Berechtigungs-, Feature- und Dateinamenprüfung gelesen;
  Cache-Control bleibt `private, no-cache`.
- `backend/modules/identityaccess/inbound/me/profile.go` und
  `backend/modules/filestorage/http/files/staff_avatar.go`: Avatare werden über
  authentifizierte Backend-Handler gelesen. Der eigene Avatar behält
  `private, max-age=86400`.
- `backend/api/common/upload.go`: `ServeImage` öffnet die Datei im Backend und
  liefert sie über `http.ServeContent`, ohne zusätzliche Bildskalierungsdateien.
  Die Frontend-Proxys unter `frontend/src/app/api/students/[id]/photo/[filename]`,
  `api/me/profile/avatar/[filename]` und `api/staff/[id]/avatar` rufen das Backend
  authentifiziert auf und lesen nicht selbst vom Upload-Volume.
- `backend/api/serve_mounts_public.go` enthält öffentliche Einzelrouten für
  Login-Bilder und rechtliche Dokumente, keinen generischen Upload-Static-Mount
  und keine öffentliche Route für Kinderfotos oder Avatare. `frontend/next.config.js`
  bindet das Backend-Verzeichnis nicht ein.
- Die Compose-Dateien für Produktion, Staging und Demo unter `environments/`
  mounten das Upload-Volume nur im Backend. `backend/Dockerfile` verwendet UID
  `1001`. Ein Frontend-Nutzer mit anderer UID muss die Dateien nicht öffnen.
- Die dokumentierten Caddy-Beispiele in `environments/maintenance/README.md`
  verwenden Reverse-Proxys. Ihr `file_server` bedient nur die Wartungsseite,
  nicht das Upload-Volume. Die tatsächliche Server-Konfiguration ist damit
  **nicht** nachgewiesen.

## Produktiv durch einen Menschen nachzuweisen

Keine Serverzugriffe oder Rechteänderungen wurden für diesen Slice ausgeführt.
Owner-only-Rechte schützen vor anderen lokalen UIDs, nicht vor derselben UID,
root, Docker-Administratoren oder einem kompromittierten Backend.

1. Vor dem Rollout die tatsächlichen Volume-Mounts und Caddy-/Static-Regeln
   prüfen. Kein direkter öffentlicher Zugriff auf diese beiden Kinds und kein
   notwendiger Leser mit anderer UID dürfen bestehen. Andernfalls abbrechen
   und den Betriebsaufbau klären, nicht auf `0644` ausweichen.
2. Eigentümer der Bestandsdateien und -verzeichnisse prüfen. Der Backend-Nutzer
   muss Eigentümer sein und schreiben können. Bei abweichenden Eigentümern
   abbrechen; dieser Ablauf führt kein `chown` aus.
3. In einem geplanten Wartungsfenster Uploads und andere Schreiber pausieren.
   Den folgenden Block in einer Bash **als Backend-Nutzer**, nicht als root,
   mit dem tatsächlich geprüften Upload-Volume ausführen. In einem Container
   ist dessen Upload-Wurzel `/app/public/uploads`. Auf dem Host die zugehörige
   Mount-Wurzel verwenden. Nur `student-photos` und `avatars` werden betrachtet,
   einschließlich älterer Avatar-Unterverzeichnisse. Andere Kinds bleiben gleich.
4. Zuerst `APPLY=0` setzen und den Block ausführen. Er meldet nur Zähler, keine
   Dateinamen oder Bildinhalte. Erst nach Prüfung `APPLY=1` setzen und denselben
   Block ausführen. Anschließend mit `APPLY=0` wiederholen: alle Abweichungen
   müssen null sein. Jeder Fehler bedeutet einen unvollständigen Schritt;
   Ursache beheben und erneut prüfen. Keine pauschale Lockerung der Rechte.
5. Nach dem Rollout mit freigegebenen synthetischen Testbildern Upload und
   authentifizierten Abruf beider Kinds prüfen, einschließlich Kinderfoto-
   Revalidierung und Staff-Avatar. Unauthentifizierte Abrufe sowie ein anderer
   lokaler Nutzer dürfen keine Bildbytes erhalten. Datum, Image-/Commit-Stand,
   UID, Zähler und Prüfergebnis ohne personenbezogene Inhalte dokumentieren.

### Container-Aufruf und pausierte Schreiber

Die [Produktions-Compose](../../environments/production.compose.yml) nennt den
Backend-Dienst `server` und mountet das Upload-Volume unter
`/app/public/uploads`. Das Backend-Image enthält Bash und läuft als UID `1001`.
Die folgenden Befehle sind ausschließlich für den berechtigten Betrieb
vorbereitet, hier nicht am Server ausgeführt. Im geprüften Deployment-Verzeichnis
mit dessen bestehender Compose-/Env-Konfiguration arbeiten, damit kein anderes
Compose-Projekt oder leeres Volume verwendet wird. UID und Mount vorher mit dem
tatsächlichen Deployment abgleichen, bei Abweichungen abbrechen.

Den vollständigen Bash-Block aus dem nächsten Abschnitt in der Datei
`/tmp/private-fotodateien-3840.sh` speichern.
Das Skript enthält keine Daten oder Zugangswerte und wird über stdin übergeben;
es muss nicht ins Image kopiert werden. Ein erster Trockenlauf im laufenden
Container ist möglich, ersetzt aber nicht die Prüfung bei pausierten Schreibern:

```sh
docker compose exec -T --user 1001 \
  -e UPLOAD_ROOT=/app/public/uploads -e APPLY=0 \
  server bash -s < /tmp/private-fotodateien-3840.sh
```

Im Wartungsfenster den Dienst stoppen. Damit pausieren dessen Uploads und
eingebettete Hintergrundjobs. Etwaige zusätzliche Backend-Replikate, externe
Jobs und andere Schreiber auf dasselbe Volume ebenfalls pausieren und
automatischen Neustart durch Deployment-Automation ausschließen. PostgreSQL
muss dafür nicht gestoppt werden. Ein Einmal-Container kann das Skript auch
bei gestopptem `server` mit dessen Image, Nutzer und Volume ausführen:

```sh
docker compose stop server
docker compose run --rm --no-deps -T --user 1001 --entrypoint bash \
  -e UPLOAD_ROOT=/app/public/uploads -e APPLY=0 \
  server -s < /tmp/private-fotodateien-3840.sh
```

`--entrypoint bash` ersetzt den Image-Einstieg; `-s` ersetzt das Compose-Kommando
`./main serve`. Der Einmal-Container startet daher keinen Anwendungsschreiber;
`--no-deps` startet keine abhängigen Dienste. Nach geprüftem Trockenlauf den
`run`-Aufruf bewusst mit `-e APPLY=1` wiederholen, dann mit `-e APPLY=0`
null Abweichungen nachweisen. Bei einem Fehler die Schreiber pausiert lassen.
Erst nach erfolgreicher Bestandsprüfung und Rollout des korrigierten Images
`docker compose up -d --no-deps server` ausführen, weitere pausierte Schreiber
kontrolliert freigeben und die Auslieferung nach Schritt 5 prüfen.

### Trockenlauf und bewusst freigegebener Bestandsschritt

`UPLOAD_ROOT` auf den geprüften absoluten Pfad setzen; `APPLY=0` ist der
Trockenlauf. Die Variablen vor dem Block in derselben Bash setzen.

```bash
set -euo pipefail
: "${UPLOAD_ROOT:?Geprüfte absolute Upload-Wurzel setzen}"
: "${APPLY:?0 für Trockenlauf oder 1 für Bestandsschritt setzen}"
case "$UPLOAD_ROOT" in /*) ;; *) exit 1 ;; esac
case "$APPLY" in 0|1) ;; *) exit 1 ;; esac
test "$(id -u)" -ne 0
test -d "$UPLOAD_ROOT"
test ! -L "$UPLOAD_ROOT"

photo_dirs=()
for kind in student-photos avatars; do
  dir="$UPLOAD_ROOT/$kind"
  test ! -L "$dir"
  if [ ! -e "$dir" ]; then
    printf '%s: nicht vorhanden\n' "$kind"
    continue
  fi
  test -d "$dir"
  # Keine fremden Eigentümer, Symlinks oder Spezialdateien bearbeiten.
  unsafe=$(find "$dir" -xdev \( ! -user "$(id -u)" -o \( ! -type d -a ! -type f \) \) -exec printf x \; | wc -c)
  if [ "$unsafe" -ne 0 ]; then
    printf '%s: Eigentümer oder Dateityp ungeklärt; Abbruch\n' "$kind" >&2
    exit 1
  fi
  files=$(find "$dir" -xdev -type f ! -perm 0600 -exec printf x \; | wc -c)
  dirs=$(find "$dir" -xdev -type d ! -perm 0700 -exec printf x \; | wc -c)
  printf '%s: Dateien mit abweichendem Modus=%s, Verzeichnisse=%s\n' "$kind" "$files" "$dirs"
  photo_dirs+=("$dir")
done

if [ "$APPLY" = 1 ]; then
  for dir in "${photo_dirs[@]}"; do
    find "$dir" -xdev -type f -exec chmod 0600 {} +
    find "$dir" -xdev -type d -exec chmod 0700 {} +
  done
  printf 'Bestandsschritt ausgeführt; jetzt mit APPLY=0 nachprüfen.\n'
fi
```

Dateirechte nach Restore erneut prüfen. Sicherungen und ihr Schutz bleiben
gesondert unter #3836 nachzuweisen. Der lokale Test dieses Ablaufs ersetzt
weder den produktiven Bestandsabgleich noch die übrige Risikobewertung von #3840.

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

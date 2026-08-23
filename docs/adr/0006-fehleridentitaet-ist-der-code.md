# Die Identität eines Fehlers ist sein Code, nie sein Text

Status: accepted. Gilt für jede Fehlerantwort der Phoenix-API und für jede
Fehleranzeige in Tenant-Portal, Eltern-Portal, Operator-Portal und PyrePortal.

## Kontext

Fehler werden heute über ihren Meldungstext identifiziert. Das ist der Grund für
alle drei beobachteten Symptome.

Messung am 22.08.2026:

- `api/common.ErrResponse` trägt bereits `code`, `errors[]` und `details{}`.
  Es gibt 257 `WithCode`-Aufrufe und rund 120 verschiedene Codes, benannt in drei
  Schemata nebeneinander: `ACCOUNT_ALREADY_HAS_TENANT_ACCESS`,
  `enrollment.change_request_conflict`, `care_exception_conflict`.
- Die Backend-Texte sind gemischt deutsch und englisch:
  `errors.New("API key cannot be empty")` neben
  `errors.New("Diese Klasse ist Ihnen nicht zugewiesen")`.
- Der Tenant-Client wirft `new Error("API error 409: ...")`. Code und Details
  werden an dieser Grenze zu einem String plattgemacht.
  `getApiErrorMessage()` rät sie danach über `message.includes("403")` zurück.
- PyrePortal hält 61 Textmuster in `src/services/apiErrors.ts`, die auf die
  heutigen Backend-Strings passen. Jede Textänderung im Backend bricht den
  Kiosk still.
- Angezeigt wird über drei Wege ohne Regel: 203 Dateien mit Toast, 159 mit
  `ui/alert`, 132 mit lokalem `setError`, dazu stumme `console.error`.

Die Primärquellen sind sich in diesem Punkt einig, anders als in der Frage des
Wire-Formats:

- RFC 9457: „Consumers MUST use the 'type' URI ... as the problem type's primary
  identifier", `title` ist „advisory", und „Consumers SHOULD NOT parse the
  'detail' member for information; extensions are more suitable and less
  error-prone ways to obtain such information."
- Microsoft Graph: „code ... machine-readable value that you can take a
  dependency on", „message ... Don't take any dependency on the content of this
  value in your code. You should only code against error codes."
- Azure: `x-ms-error-code` ist Vertragsbestandteil und „cannot change in the
  future". Zusatzfelder sollen ergänzt werden, „so customers don't resort to
  parsing your error message".
- Google AIP-193: `(reason, domain)` ist die Identität, `reason` in UPPER_SNAKE,
  eindeutig innerhalb der Domain. Anfragespezifische Werte müssen zusätzlich in
  `ErrorInfo.metadata`, damit „machine actors do not need to parse error
  messages to extract information".
- Sentry warnt für die Gruppierung über den Meldungstext vor „really bad groups
  when error values are frequently changing".

Über die Form des Umschlags sind sie sich dagegen nicht einig. RFC 9457 ist
Norm, aber kaum gelebt: die IANA-Registry hat nach drei Jahren sechs Einträge,
keiner davon ein Anwendungsfehler, und keiner der vier großen Anbieter liefert
`application/problem+json`.

## Entscheidung

- Die **Identität eines Fehlers ist sein Code**, ein flacher Text der Form
  `bereich.fehlername`, klein geschrieben. Kein Programm auf keiner Seite darf
  sich auf einen Meldungstext stützen. Substring-Vergleiche gegen Fehlertexte
  verschwinden ersatzlos.
- Die Antwort trägt den **RFC-9457-Umschlag**: Medientyp
  `application/problem+json` und die Felder `type`, `title`, `status`, `detail`,
  `instance`. Vier davon werden ohnehin gebraucht, `instance` ist die
  Vorgangskennung. Der `code` bleibt Erweiterungsfeld und ist die Identität, die
  `type`-URI zeigt auf den passenden Abschnitt des Hilfe-Handbuchs.
  Begründung gegen die reine Norm: eine URI als Vergleichswert betoniert einen
  Hostnamen in jeden Vergleich im Frontend ein.
- Der Code ist **zugleich der Schlüssel im Übersetzungskatalog**. Deshalb
  `bereich.fehlername` und nicht Googles UPPER_SNAKE: next-intl-Schlüssel sind
  so aufgebaut, und eine zweite Zuordnungstabelle entfällt.
- **Übersetzt wird im Frontend.** `next-intl` mit vier Sprachkatalogen läuft
  bereits, MDN rät ausdrücklich davon ab, eine explizite Sprachwahl über
  Accept-Language zu überstimmen, und serverseitige Lokalisierung müsste
  Accept-Language durch 628 Proxy-Routen fädeln. Der Backend-Text wird
  Entwicklerdiagnose und darf englisch bleiben.
- Jeder Wert, der im Text vorkommt, wird **zusätzlich strukturiert** in
  `details` mitgeschickt. Sonst muss der Client den Satz doch wieder lesen.
- Eine **Registry-Datei im Repo ist die einzige Quelle** der Codes. Daraus
  entstehen die Go-Konstanten, die TypeScript-Union und die Parameternamen je
  Code. Der Katalog ist ein `Record<ErrorCode, ...>`, ein Code ohne Text bricht
  damit `pnpm run check`.
- Kennt der Katalog einen Code nicht, wird der Text seiner **Fehlerklasse**
  angezeigt. Eine leere oder englische Meldung darf es nicht geben.
- Die rund 120 vorhandenen Codes werden **einmalig umbenannt**, zusammen mit
  allen Konsumenten in einer Auslieferung. Danach gilt dauerhaft: neuer Code ja,
  umbenennen nein, entfernen nein.

## Folgen

- Alle Konsumenten müssen zusammen ausgeliefert werden, PyrePortal
  eingeschlossen. Der Kiosk bekommt die generierte Code-Liste eingecheckt; er
  behält seine eigenen Texte, weil ein Wandtablet anders spricht als ein Portal.
  Seine 61 Textmuster entfallen.
- Die Golden-Tests der abweichenden Wire-Formate (`api/operator` mit
  `json:"message"`, dazu `active`, `feedback`, `suggestions`) ändern sich
  bewusst mit. Das ist der in `no-test-modifications.md` vorgesehene Fall einer
  absichtlich geänderten Geschäftsregel und wurde ausdrücklich freigegeben.
- Das einmalige Umbenennen ist ein Fenster, das sich nicht wiederholt. Wird es
  nicht genutzt, bleiben drei Namensschemata dauerhaft bestehen.
- Auswertbarkeit folgt daraus als Nebenwirkung: Sentry gruppiert über den Code
  statt über den Stacktrace, und die Frage „welcher Fehler wie oft an welcher
  Schule" wird überhaupt erst stellbar.

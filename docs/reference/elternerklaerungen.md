# Elternerklärungen: Verfahren, Einsatzfälle und Formulierungen

Stand: #3430, Unterschrift #4785. Diese Seite beschreibt, was die Funktion „Einverständnis“ in
den Elternmitteilungen leistet, wofür sie reicht und wofür nicht. Im Code
heißt der Modus `declaration`. Sie ist keine
Rechtsberatung. Die Einordnung der Einsatzfälle muss vor der Freigabe in der
Website fachlich und rechtlich bestätigt werden (Abnahmekriterium 1).

## Was moto anbietet

Ein Einverständnis ist ein eigener Versandmodus einer Elternmitteilung. Die
Einrichtung legt Empfänger, Frist und Regeln fest, Eltern stimmen im
Eltern-Portal pro Kind zu oder lehnen ab.

| Baustein | Umsetzung |
|---|---|
| Verfahren | Einfache elektronische Erklärung im angemeldeten Eltern-Konto (`simple_electronic`). Wählt die Einrichtung „Mit Unterschrift“, ist die Zustimmung eine einfache elektronische Signatur mit gezeichneter Unterschrift im angemeldeten Eltern-Konto (`simple_electronic_signature`). Keine fortgeschrittene oder qualifizierte elektronische Signatur. |
| Unterschrift | Pro Einverständnis wählbar, zusätzlich zur Passwortabfrage. Mit Unterschrift ist sie für die Zustimmung Pflicht; Ablehnen und Widerrufen gehen ohne Unterschrift. Eltern unterschreiben mit Finger, Stift oder Maus. Wer nicht zeichnen kann oder nur die Tastatur nutzt, tippt den Namen; das Eltern-Portal setzt ihn als Bild in dasselbe Feld, und der Nachweis vermerkt „Name getippt“. Gespeichert werden das Bild (PNG, höchstens 200 KB und 1600 × 800 Pixel, mit sichtbarem Strich), Zeitpunkt, Konto, Name der unterschreibenden Person, SHA-256 des Bildes und die Prüfsumme der unterschriebenen Fassung, dazu eine Prüfsumme über den ganzen Eintrag. Die Antwort verweist auf die Unterschrift, und ihre eigene Prüfsumme deckt diesen Verweis ab. Das Einstellen ist wie die übrigen Regeln gesperrt, sobald jemand geantwortet hat. |
| Art | Nur Zustimmung oder Ablehnung (`consent`). Kein vorausgewähltes Einverständnis. Eine reine Lesebestätigung ist bewusst keine Art dieses Modus: dafür gibt es den Elternbrief mit Lesebestätigung. Zwei Wege für dasselbe hätten das Personal verwirrt und „Erklärung“ zu einem unklaren Sammelbegriff gemacht. Die Art wird trotzdem gespeichert, damit jeder Nachweis nennt, was gefragt war. |
| Wer antworten darf | Nur Personen mit `parent_portal.declarations.submit` für genau dieses Kind. Voreinstellung: Haupt-, Mit- und weitere Sorgeberechtigte. Abholberechtigte, Notfallkontakte und Sozialarbeit nie. Die Berechtigung wird bei jeder Antwort serverseitig erneut geprüft und für die Dauer der Antwort gesperrt. |
| Mehrere Sorgeberechtigte | Pro Einverständnis wählbar: eine berechtigte Person genügt, oder alle berechtigten Personen mit Eltern-Konto müssen antworten. Eine Ablehnung oder ein Widerruf einer berechtigten Person geht einer Zustimmung vor. |
| Erneute Anmeldung | Pro Einverständnis wählbar: Passwort des Eltern-Kontos vor jeder Antwort. Der Nachweis hält fest, ob das Passwort bestätigt wurde. Versuche laufen über die Anmelde-Drosselung. |
| Fassung | Beim Veröffentlichen wird die Fassung eingefroren: Titel, Text, Art und SHA-256 jeder Anlage. Die Prüfsumme der Fassung ist SHA-256 über diese Angaben. Eine Korrektur (zurückziehen, ändern, erneut veröffentlichen) erzeugt eine neue Fassung; frühere Antworten bleiben unverändert gespeichert, zählen aber nicht mehr für den aktuellen Stand. Anlagen sind nach der ersten Veröffentlichung fest. |
| Nachweis je Antwort | Kind, Einrichtung, Konto, Name der erklärenden Person (zum Zeitpunkt der Antwort), Berechtigungsrolle, Aktion, Verfahren, Passwortbestätigung, Zeitpunkt, Fassung, Prüfsumme der Fassung und eine Prüfsumme über den gesamten Eintrag. |
| Unveränderlichkeit | Die Tabellen für Fassungen, Antworten und Unterschriften (`audit.signatures`) erlauben den Anwendungsrollen nur Lesen und Einfügen. Ein Einverständnis mit Antworten kann nicht gelöscht werden. Statusansicht, Bericht und Eltern-Nachweis prüfen jede gespeicherte Prüfsumme erneut, bei einer Unterschrift auch Bild, Eintrag und dass sie zur Fassung der Antwort gehört. Sie zeigen die Prüfsummen nicht an, sondern einen Satz: unverändert, oder eine Warnung bei einer Abweichung. Einzige Ausnahme sind unterschriebene Antworten: Beide PDFs drucken neben dem Bild der Unterschrift die Prüfsumme der unterschriebenen Fassung. |
| Widerruf | Bei widerruflichen Einwilligungen jederzeit, auch nach Ablauf der Frist. Vor der Frist kann eine Antwort korrigiert werden; jede Änderung ist ein neuer Eintrag. |
| Nachweis für Eltern | Seite im Eltern-Portal in der Sprache der Eltern und PDF im moto-Design (deutsch) mit vollständigem Text, Dateien und eigenem Verlauf, bei unterschriebenen Antworten mit dem Bild der eigenen Unterschrift. Unterschriften anderer Personen sieht niemand im Eltern-Portal. |
| Nachweis für die Einrichtung | Statusansicht je Kind, vollständiger Verlauf, Bericht als PDF im moto-Design, CSV-Export. Der Bericht zeigt jede Unterschrift als Bild neben Fassung und Zeitpunkt; die Statusansicht nennt nur, ob und wie unterschrieben wurde. Nur der CSV-Export enthält die Prüfsummen (auch die des Unterschriftsbildes), als maschinenlesbare Ablage für Support oder einen Streitfall. |
| Löschung | Wird ein Kind nach den Aufbewahrungsregeln gelöscht, entfallen seine Nachweise samt Unterschriften mit. Wird eine Schule gelöscht, ebenso. Das Einverständnis selbst bleibt, solange es Antworten hat; danach löscht es der nächtliche Lauf nach der Frist der Elternnachrichten (#4045, siehe [Löschfristen](loeschfristen.md)). |

Grenzen des Nachweises: Eine Prüfsumme belegt, dass ein gespeicherter Eintrag
nachträglich nicht verändert wurde, soweit die Prüfsumme selbst unverändert
ist. Sie belegt weder die Identität der erklärenden Person noch eine
vertrauenswürdige Entstehungszeit. Beides stützt sich auf die Anmeldung im
Eltern-Konto (optional mit erneuter Passworteingabe) und die Serverzeit.
Das Bild der Unterschrift ist eine Zeichnung im Browser. Es ändert an diesen
Grenzen nichts: Es belegt nicht, wer gezeichnet hat, und wird nicht mit einer
Unterschrift auf Papier verglichen.

## Baustein „Unterschrift“ für weitere Abläufe

Die Unterschrift ist ein gemeinsamer Baustein der Audit Platform
(`modules/auditlog`, Tabelle `audit.signatures`). Er entscheidet nicht, wer
unterschreiben darf; das prüft der Ablauf, der ihn nutzt. Nutzer sind
Einverständnisse (#4785) und die Anmeldung (#4786), geplant die
Abholerlaubnis (#4780).

- Prüfen: `auditlog.ParseSignatureImage(method, pngBase64, typedName)` prüft
  Verfahren, PNG (Magic Number, Größe, Abmessungen, sichtbarer Strich) und
  den getippten Namen; jeder Fehler ist `auditlog.ErrSignatureInvalid`.
- Speichern: `SignatureLedger.RecordSignature(ctx, SignatureEntry)` in der
  Mandanten-Transaktion des Ablaufs, also zusammen mit dem Eintrag, der auf
  die Unterschrift verweist. `DocumentSHA256` ist die Prüfsumme genau dessen,
  was unterschrieben wurde. Der Bezug bestimmt die Löschung: `StudentID` für
  Abläufe mit Kind, `EnrollmentRequestID` für eine Anmeldung (#4786); genau
  einer ist gesetzt. `SignerAccountID` darf fehlen (öffentliche Anmeldung
  ohne Konto).
- Lesen: `SignatureLedger.Signatures(ctx, ids)` gibt nur Einträge der Schule
  der Transaktion zurück, jeweils mit `IntegrityOK`.
  `SignatureLedger.LatestEnrollmentSignatures(ctx, requestIDs)` gibt je
  Anmeldung die neueste Unterschrift dieser Schule zurück (#4786).
- Oberfläche: `frontend/src/components/ui/signature-pad.tsx` (Zeichenfläche,
  Löschen, Bestätigen, Name tippen) und `frontend/src/lib/signature.ts` für
  den Request-Teil `signature: {method, image_png, typed_name}`.

## Anmeldung mit Unterschrift (#4786)

Eine Formularvorlage kann „Unterschrift erforderlich“ setzen
(`core_requirements.signature`). Dann gilt:

- Das Formular endet nach den Rechtstexten mit dem Unterschriftsfeld. Ohne
  Unterschrift lehnt der Server das Absenden ab
  (`enrollment.signature_required`), ein ungültiges Bild ebenso
  (`enrollment.signature_invalid`). Das gilt öffentlich ohne Konto und im
  Eltern-Portal.
- Gespeichert wird über den Baustein oben, Zweck `enrollment`, Bezug die
  Anmeldung. `DocumentSHA256` ist die Prüfsumme der abgesendeten Anmeldung:
  Kontaktdaten, weitere Erziehungsberechtigte, Antworten, Zustimmungen, Kinder
  mit Angebotswahl und der Rechtstext-Schnappschuss dieser Abgabe.
- Jede Bearbeitung durch die Eltern über das Formular braucht eine neue
  Unterschrift; die neueste gehört zur aktuellen Fassung. Die Feldbearbeitung
  der Kontaktdaten auf der Statusseite ist bei solchen Vorlagen gesperrt.
- Änderungsanfragen nach einer Entscheidung entscheidet die OGS; sie tragen
  keine Unterschrift. Eine von der Verwaltung angelegte Anmeldung
  (`admin_manual`) braucht keine.
- Die Verwaltung sieht die Unterschrift in der Anmeldungsdetailansicht und lädt
  den Anmeldenachweis als PDF (`GET /api/enrollment/admin/requests/{id}?format=pdf`).
  Eltern laden ihn über den Status-Link
  (`GET /api/enrollment/requests/{token}?format=pdf`). Der Status-Link ist die
  einzige Berechtigung: Der Token bestimmt Schule und Anmeldung; jedes Lesen
  danach ist auf genau diese beschränkt. Ein unbekannter, abgelaufener oder
  veränderter Token liefert 404.
- Die Prüfsumme belegt, was unterschrieben wurde. Spätere Korrekturen der
  Verwaltung ändern die Anmeldung, nicht die gespeicherte Unterschrift; der
  Nachweis zeigt die Angaben im aktuellen Stand.

## Rechtlicher Rahmen

- Art. 25 Abs. 1 eIDAS-Verordnung: Einer elektronischen Signatur darf die
  Rechtswirkung und die Zulässigkeit als Beweismittel nicht allein deshalb
  abgesprochen werden, weil sie elektronisch ist oder die Anforderungen an eine
  qualifizierte Signatur nicht erfüllt. Nur die qualifizierte elektronische
  Signatur (QES) hat die gleiche Rechtswirkung wie eine handschriftliche
  Unterschrift (Abs. 2).
- § 126a BGB: Soll eine gesetzlich vorgeschriebene Schriftform elektronisch
  ersetzt werden, braucht es Namen und QES. Die einfache Erklärung in moto
  ersetzt keine gesetzliche Schriftform.
- § 126b BGB: Textform verlangt eine lesbare Erklärung mit Nennung der
  erklärenden Person auf einem dauerhaften Datenträger. Ob der
  Druck-/PDF-Nachweis im Einzelfall als dauerhafter Datenträger genügt, ist
  rechtlich zu prüfen.
- § 127 BGB: Für eine vertraglich oder von der Einrichtung vereinbarte Form
  können andere elektronische Signaturen genügen, sofern nichts anderes gewollt
  ist.
- § 1687 Abs. 1 BGB: Bei getrennt lebenden Eltern mit gemeinsamer Sorge
  entscheidet der Elternteil, bei dem das Kind lebt, Angelegenheiten des
  täglichen Lebens allein; Angelegenheiten von erheblicher Bedeutung brauchen
  das Einvernehmen beider. Das spricht für „eine Person genügt“ bei
  Alltagsfragen und „alle Sorgeberechtigten“ bei erheblichen Entscheidungen.
- Art. 7 DSGVO: Eine Einwilligung muss nachweisbar sein, und ihr Widerruf muss
  so einfach sein wie ihre Erteilung. Deshalb ist der Widerruf auch nach der
  Frist und nach Betreuungsende möglich.

## Einsatzfälle (Vorschlag, rechtlich zu bestätigen)

| Einsatzfall | Vorgeschlagene Einstellung | Einordnung |
|---|---|---|
| Ausflugserlaubnis (Tagesausflug) | Zustimmung, eine Person genügt, Frist | Alltagsangelegenheit; einfache Erklärung als Nachweis der Einrichtung üblich ausreichend. |
| Mehrtägige Fahrt, besondere Risiken (z. B. Schwimmen, Klettern) | Zustimmung, alle Sorgeberechtigten, Passwort | Kann erhebliche Bedeutung haben; im Zweifel beide Sorgeberechtigten. |
| Fotoeinwilligung / Veröffentlichung von Bildern | Zustimmung, widerruflich | Einwilligung nach DSGVO; Nachweis und einfacher Widerruf nötig. Die bestehende Fotoeinwilligung im Kinderprofil bleibt die maßgebliche Stelle; ein Einverständnis ergänzt sie nicht automatisch. |
| Alleingehervollmacht / Abholvollmacht | Zustimmung, Passwort; je nach Einrichtung alle Sorgeberechtigten | Einrichtungsspezifische Form prüfen; die Abholberechtigungen im Kinderprofil werden nicht automatisch geändert. |
| Kenntnisnahme von Hausordnung, Hygieneplan, Informationen | Kein Einverständnis, sondern Elternbrief mit Lesebestätigung | Reine Lesebestätigung, ohne festgehaltene Fassung und ohne Passwortabfrage. |
| Betreuungsvertrag, Kündigung, Anmeldung mit gesetzlicher oder vertraglicher Schriftform | Nicht über Einverständnisse | Braucht die vereinbarte bzw. gesetzliche Form (§ 126, § 126a BGB). Erst mit QES-Ausbau denkbar. |

## QES

Nicht Teil von #3430. Eine QES braucht einen qualifizierten
Vertrauensdiensteanbieter (Identifizierung, qualifiziertes Zertifikat,
Signaturerstellung) und die Aufbewahrung des signierten Originals samt
Validierungsnachweis. Für die Entscheidung in einem Folge-Issue zu klären:

1. Welche Einsatzfälle brauchen tatsächlich Schriftform? Ohne solche Fälle gibt
   es keinen Bedarf.
2. Anbieter (z. B. aus der EU-Vertrauensliste), Kosten pro Signatur und pro
   Identifizierung, Verfügbarkeit für Eltern ohne Online-Ausweis.
3. Integration: Weiterleitung zum Anbieter, Rücknahme des signierten
   PAdES-Dokuments, Validierung, Speicherung des Originals neben der Fassung.
4. Darstellung: Ein Druck oder Export ist nie das signierte Original.

## Formulierungen für Website und Vertrieb

Zulässig:

- „Eltern geben ihr Einverständnis, etwa für Ausflüge, direkt im
  Eltern-Portal, ohne Papier.“
- „Jede Antwort wird mit Fassung, Zeitpunkt und Person nachvollziehbar
  gespeichert und kann exportiert werden.“
- „Spätere Änderungen verändern bereits gegebene Antworten nicht.“
- „Einwilligungen können Eltern jederzeit widerrufen.“
- „Einfache elektronische Erklärung im angemeldeten Eltern-Konto, auf Wunsch
  mit erneuter Passworteingabe.“
- „Auf Wunsch unterschreiben Eltern ihre Zustimmung mit dem Finger oder der
  Maus: eine einfache elektronische Signatur mit gezeichneter Unterschrift.“
- „Die Unterschrift steht im Nachweis, zusammen mit Fassung, Zeitpunkt und
  Prüfsumme.“
- „Auf Wunsch der Schule unterschreiben Eltern die Online-Anmeldung mit dem
  Finger oder der Maus: eine einfache elektronische Signatur mit gezeichneter
  Unterschrift. Ein Anmeldenachweis mit Unterschrift steht als PDF bereit.“

Nicht zulässig:

- „rechtssicher“, „rechtsgültig für alle Dokumente“, „ersetzt die
  Unterschrift“, „digitale Unterschrift“ oder „qualifizierte Signatur“.
- „fälschungssicher“ oder „manipulationssicher“ (die Prüfsummen machen
  Änderungen erkennbar, verhindern sie nicht).
- Jede Aussage, dass Verträge oder Kündigungen darüber geschlossen werden
  können.
- „wie auf Papier unterschrieben“, „gleichwertig zur handschriftlichen
  Unterschrift“, „fälschungssichere Unterschrift“ oder „Unterschrift mit
  voller Beweiskraft“.
- „elektronische Unterschrift“ oder „E-Signatur“ ohne den vollständigen
  Namen „einfache elektronische Signatur mit gezeichneter Unterschrift“.
- „unterschriebener Betreuungsvertrag“, „Anmeldevertrag unterschreiben“ oder
  „verbindlich unterschrieben wie auf Papier“ für die Anmeldung.
- „Die Anmeldung ist damit schriftlich erfolgt“ oder jede andere Aussage, die
  Unterschrift erfülle eine Schriftform.

Quellen: [Art. 25 eIDAS](https://gesetze.legal/eu/vo_eu_2014_910/25),
[§ 126a BGB](https://www.gesetze-im-internet.de/bgb/__126a.html),
[§ 126b BGB](https://www.gesetze-im-internet.de/bgb/__126b.html),
[§ 127 BGB](https://www.gesetze-im-internet.de/bgb/__127.html),
[§ 1687 BGB](https://www.gesetze-im-internet.de/bgb/__1687.html).

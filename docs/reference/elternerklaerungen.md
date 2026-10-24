# Elternerklärungen: Verfahren, Einsatzfälle und Formulierungen

Stand: #3430, Unterschrift #4785, Abholerlaubnis #4780. Diese Seite beschreibt, was die Funktion „Einverständnis“ in
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
Einverständnisse (#4785), die Abholerlaubnis (#4780) und, geplant, die
Anmeldung (#4786).

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
- Oberfläche: `frontend/src/components/ui/signature-pad.tsx` (Zeichenfläche,
  Löschen, Bestätigen, Name tippen) und `frontend/src/lib/signature.ts` für
  den Request-Teil `signature: {method, image_png, typed_name}`.

## Abholerlaubnis und Heimweg allein (#4780)

Eltern mit `parent_portal.pickup.manage` für genau dieses Kind erteilen im
Eltern-Portal Erlaubnisse, die die OGS unter `/anfragen` bestätigt. Jede
Erlaubnis ist eine einfache elektronische Signatur mit gezeichneter
Unterschrift (Zweck `pickup_permission`, Bezug: das Kind).

| Erlaubnis | Ablauf |
|---|---|
| Abholberechtigung einer Kontaktperson | Anfrage `pickup_permission` mit Wochentagen (Standard: alle Tage). Nach der Bestätigung schreibt Care Plan `can_pickup` und `pickup_weekdays`. Nur für Kontakte ohne eigenes Eltern-Konto, keine Sozialarbeit, keine Sorgeberechtigten einer anderen Familie. Wegnehmen geht weiterhin direkt und ohne Unterschrift. |
| Abholperson für einen Tag | Teil der Anfrage „Abholzeit ändern“ (`pickup_change`) mit einer hinterlegten Kontaktperson. Nach der Bestätigung steht für diesen Tag eine Abholnotiz „Abholung durch … (Erlaubnis der Eltern)“, die die OGS überall sieht, wo die Abholung des Tages steht. Die dauerhafte Abholberechtigung ändert sich nicht. |
| Heimweg allein | Abholart „allein“ für einen Tag (`pickup_change`) oder an Wochentagen (Betreuungszeiten-Anfrage). |

Gespeichert wird der deutsche Wortlaut der Erklärung („Ich erlaube, dass …“)
in der Anfrage; die Unterschrift trägt seine SHA-256-Prüfsumme. Der Nachweis
(Seite und PDF für die Eltern, PDF für die OGS) prüft Bild, Eintrag, Kind,
Zweck und Wortlaut erneut. Eltern sehen nur den Nachweis ihrer eigenen
Unterschrift. Die Stichzeit `operations.parent_pickup_change_cutoff_time`
gilt für die Tages-Anfragen; eine Abholberechtigung an Wochentagen gilt ab
der Bestätigung durch die OGS.

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
- „schriftliche Abholvollmacht“, „ersetzt die Vollmacht auf Papier“ oder
  „rechtsverbindliche Abholerlaubnis“ für die Abholerlaubnis im Eltern-Portal.

Quellen: [Art. 25 eIDAS](https://gesetze.legal/eu/vo_eu_2014_910/25),
[§ 126a BGB](https://www.gesetze-im-internet.de/bgb/__126a.html),
[§ 126b BGB](https://www.gesetze-im-internet.de/bgb/__126b.html),
[§ 127 BGB](https://www.gesetze-im-internet.de/bgb/__127.html),
[§ 1687 BGB](https://www.gesetze-im-internet.de/bgb/__1687.html).

# Elternerklärungen: Verfahren, Einsatzfälle und Formulierungen

Stand: #3430. Diese Seite beschreibt, was die Funktion „Erklärung“ in den
Elternmitteilungen leistet, wofür sie reicht und wofür nicht. Sie ist keine
Rechtsberatung. Die Einordnung der Einsatzfälle muss vor der Freigabe in der
Website fachlich und rechtlich bestätigt werden (Abnahmekriterium 1).

## Was moto anbietet

Eine Erklärung ist ein eigener Versandmodus einer Elternmitteilung. Die
Einrichtung legt Art, Empfänger, Frist und Regeln fest, Eltern antworten im
Eltern-Portal pro Kind.

| Baustein | Umsetzung |
|---|---|
| Verfahren | Einfache elektronische Erklärung im angemeldeten Eltern-Konto (`simple_electronic`). Keine gezeichnete Unterschrift, keine fortgeschrittene oder qualifizierte elektronische Signatur. |
| Arten | Zustimmung oder Ablehnung (`consent`), reine Kenntnisnahme (`acknowledgement`). Kein vorausgewähltes Einverständnis. |
| Wer antworten darf | Nur Personen mit `parent_portal.declarations.submit` für genau dieses Kind. Voreinstellung: Haupt-, Mit- und weitere Sorgeberechtigte. Abholberechtigte, Notfallkontakte und Sozialarbeit nie. Die Berechtigung wird bei jeder Abgabe serverseitig erneut geprüft und für die Dauer der Abgabe gesperrt. |
| Mehrere Sorgeberechtigte | Pro Erklärung wählbar: eine berechtigte Person genügt, oder alle berechtigten Personen mit Eltern-Konto müssen antworten. Eine Ablehnung oder ein Widerruf einer berechtigten Person geht einer Zustimmung vor. |
| Erneute Anmeldung | Pro Erklärung wählbar: Passwort des Eltern-Kontos vor jeder Abgabe. Der Nachweis hält fest, ob das Passwort bestätigt wurde. Versuche laufen über die Anmelde-Drosselung. |
| Fassung | Beim Veröffentlichen wird die Fassung eingefroren: Titel, Text, Art und SHA-256 jeder Anlage. Die Prüfsumme der Fassung ist SHA-256 über diese Angaben. Eine Korrektur (zurückziehen, ändern, erneut veröffentlichen) erzeugt eine neue Fassung; frühere Abgaben bleiben unverändert gespeichert, zählen aber nicht mehr für den aktuellen Stand. Anlagen sind nach der ersten Veröffentlichung fest. |
| Nachweis je Abgabe | Kind, Einrichtung, Konto, Name der erklärenden Person (zum Zeitpunkt der Abgabe), Berechtigungsrolle, Aktion, Verfahren, Passwortbestätigung, Zeitpunkt, Fassung, Prüfsumme der Fassung und eine Prüfsumme über den gesamten Eintrag. |
| Unveränderlichkeit | Die Tabellen für Fassungen und Abgaben erlauben den Anwendungsrollen nur Lesen und Einfügen. Eine Erklärung mit Abgaben kann nicht gelöscht werden. Die Statusansicht prüft jede gespeicherte Prüfsumme erneut und meldet Abweichungen. |
| Widerruf | Bei widerruflichen Einwilligungen jederzeit, auch nach Ablauf der Frist. Vor der Frist kann eine Antwort korrigiert werden; jede Änderung ist ein neuer Eintrag. |
| Nachweis für Eltern | Druckansicht im Eltern-Portal mit vollständigem Text, Anlagen-Prüfsummen und eigenem Verlauf, zum Drucken oder Speichern als PDF. |
| Nachweis für die Einrichtung | Statusansicht je Kind, vollständiger Verlauf, Druckansicht, CSV-Export. |
| Löschung | Wird ein Kind nach den Aufbewahrungsregeln gelöscht, entfallen seine Nachweise mit. Wird eine Schule gelöscht, ebenso. |

Grenzen des Nachweises: Eine Prüfsumme belegt, dass ein gespeicherter Eintrag
nachträglich nicht verändert wurde, soweit die Prüfsumme selbst unverändert
ist. Sie belegt weder die Identität der erklärenden Person noch eine
vertrauenswürdige Entstehungszeit. Beides stützt sich auf die Anmeldung im
Eltern-Konto (optional mit erneuter Passworteingabe) und die Serverzeit.

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
| Fotoeinwilligung / Veröffentlichung von Bildern | Zustimmung, widerruflich | Einwilligung nach DSGVO; Nachweis und einfacher Widerruf nötig. Die bestehende Fotoeinwilligung im Kinderprofil bleibt die maßgebliche Stelle; eine Erklärung ergänzt sie nicht automatisch. |
| Alleingehervollmacht / Abholvollmacht | Zustimmung, Passwort; je nach Einrichtung alle Sorgeberechtigten | Einrichtungsspezifische Form prüfen; die Abholberechtigungen im Kinderprofil werden nicht automatisch geändert. |
| Kenntnisnahme von Hausordnung, Hygieneplan, Informationen | Kenntnisnahme, eine Person genügt | Reine Lesebestätigung. |
| Betreuungsvertrag, Kündigung, Anmeldung mit gesetzlicher oder vertraglicher Schriftform | Nicht über Erklärungen | Braucht die vereinbarte bzw. gesetzliche Form (§ 126, § 126a BGB). Erst mit QES-Ausbau denkbar. |

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

- „Eltern bestätigen Erklärungen wie Ausflugserlaubnisse direkt im
  Eltern-Portal, ohne Papier.“
- „Jede Antwort wird mit Fassung, Zeitpunkt und Person nachvollziehbar
  gespeichert und kann exportiert werden.“
- „Spätere Änderungen verändern bereits abgegebene Erklärungen nicht.“
- „Einwilligungen können Eltern jederzeit widerrufen.“
- „Einfache elektronische Erklärung im angemeldeten Eltern-Konto, auf Wunsch
  mit erneuter Passworteingabe.“

Nicht zulässig:

- „rechtssicher“, „rechtsgültig für alle Dokumente“, „ersetzt die
  Unterschrift“, „digitale Unterschrift“ oder „qualifizierte Signatur“.
- „fälschungssicher“ oder „manipulationssicher“ (die Prüfsummen machen
  Änderungen erkennbar, verhindern sie nicht).
- Jede Aussage, dass Verträge oder Kündigungen darüber geschlossen werden
  können.

Quellen: [Art. 25 eIDAS](https://gesetze.legal/eu/vo_eu_2014_910/25),
[§ 126a BGB](https://www.gesetze-im-internet.de/bgb/__126a.html),
[§ 126b BGB](https://www.gesetze-im-internet.de/bgb/__126b.html),
[§ 127 BGB](https://www.gesetze-im-internet.de/bgb/__127.html),
[§ 1687 BGB](https://www.gesetze-im-internet.de/bgb/__1687.html).

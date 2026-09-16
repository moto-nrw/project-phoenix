---
status: accepted
---

# Die Raumfreigabe ist eine Entscheidung der Schule, kein Namensmerkmal

Migration 1.15.372 ([#3064](https://github.com/moto-nrw/project-phoenix/issues/3064))
hat jeden System-Raum „Schulhof“ pauschal als offenen Raum freigegeben. Am
16.09.2026 verlor die OGS am Berg damit die Blockliste auf der Aufsichtsseite:
alle GT-Blöcke laufen dort parallel im Schulhof, und die geteilte Raumansicht
aus [#3065](https://github.com/moto-nrw/project-phoenix/issues/3065) kennt
keine Blockaktionen. Vorfall und Fakten: [#3276](https://github.com/moto-nrw/project-phoenix/issues/3276).

Wir entscheiden: Ob ein Raum offen ist, legt die Schule fest. Der Name
„Schulhof“ begründet keine Freigabe. Ein offener Raum kann gleichzeitig Ort
laufender Blöcke sein; die Ansicht zeigt beides, statt zwischen beidem zu
wählen. Die Heim-Abmeldung am NFC-Gerät bleibt eine eigene Regel, die den
kanonischen Schulhof weiterhin am Namen erkennt.

## Nutzung, die die Entscheidung trägt

Stand 16.09.2026 in Produktion, letzte 14 Tage, System-Schulhof:

| Schule | Kiosk-Freispiel-Sessions | Block-Sessions | geplante Blöcke im Schulhof, kommend |
|---|---|---|---|
| Burbach | 10 | 0 | 0 |
| am Berg | 1 | 51 (bis 5 parallel) | 606 |
| Wissingen | 0 | 11 | 211 |
| Barnstorf | 21 | 0 | 0 |

Altenberge und Bissendorf hatten keine Schulhof-Sessions. Kiosk-Schulen nutzen
den Hof als offenen Raum, Web-Schulen als Blockraum. Eine pauschale Regel
trifft die eine Hälfte falsch, gleich in welche Richtung.

## Entscheidungen

1. **Bestand.** Eine Korrektur-Migration nimmt die Freigabe dort zurück, wo der
   System-Schulhof kommende, nicht spontane Regeltermine einer regulären
   Aktivität hat. Das Kriterium beschreibt den Grund („die Schule plant Blöcke
   im Hof“), läuft auf jeder Umgebung gleich und trifft in Produktion am Berg
   und Wissingen. Kiosk-Schulen behalten die Freigabe. Keine Handarbeit in
   Produktion, keine Tenant-Liste im Code.

2. **Neue Schulen.** Der provisionierte Schulhof startet ohne Freigabe. Die
   Administration setzt sie bei der Einrichtung, wenn Kinder den Hof am Gerät
   wählen sollen. Das ist ein Schritt in der Ersteinrichtung des Hilfe-Guides.
   Damit gilt wieder „per Default aus“ aus
   [#2183](https://github.com/moto-nrw/project-phoenix/issues/2183); die
   Abweichung aus #3064 („Bestandsschulen nicht zu einem neuen Morgenschritt
   zwingen“) bleibt nur für den Bestand über Punkt 1 bestehen.

3. **Eine Ansicht statt einer Weiche.** Die Raumansicht eines offenen Raums
   zeigt im Kopf die Raumbelegung und darunter je laufende Session ein Roster
   mit Aktionen, sofern die Person den Block bedienen darf, plus einen
   Abschnitt „Ohne Angebot“ für angebotsunabhängige Aufenthalte. Eigene Blöcke
   zuerst und aufgeklappt, fremde eingeklappt mit Zahl im Kopf. Die
   Kopfaktionen „Beaufsichtigen“ und „Aufsicht abgeben“ gelten weiter für die
   Freispiel-Session des Kiosks. Die Weiche „Raum- oder Blockansicht“ im
   Aufsichts-View-Model und die Heuristiken aus
   [#3272](https://github.com/moto-nrw/project-phoenix/pull/3272) entfallen.

4. **Kiosk bei laufenden Blöcken.** Wählt ein Kind am Gerät den Schulhof,
   während dort Blöcke laufen, bucht das Gerät es in den laufenden Block, in
   dessen Tages-Roster es steht; bei mehreren Treffern in den zuletzt
   gestarteten. Steht es in keinem, entsteht ein angebotsunabhängiger
   Aufenthalt. Heute landet das Kind still in der zuletzt gestarteten Session,
   gleich welcher. Betroffen ist derzeit keine Schule, weil die Blockschulen
   praktisch ohne Geräte arbeiten; die Umsetzung ist deshalb nicht dringend.

5. **Heim-Abmeldung am Gerät bleibt.** „Nach Hause“ erscheint nach dem
   Auschecken aus dem eigenen Gruppenraum, aus dem kanonischen Schulhof oder
   überall, wenn die Schule „Nach Hause in jedem Raum anzeigen“ eingeschaltet
   hat. Der Schulhof-Fall bleibt am Namen und wird nicht an die Freigabe
   gekoppelt: Kiosk-Schulen verlassen sich darauf, und eine Freigabe darf nie
   das Recht erteilen, die OGS zu verlassen (#3064). Das ist eine von zwei
   bewussten Namensregeln, die bestehen bleiben.

6. **Der Schulhof ist immer auswählbar.** Die Freigabe entscheidet, ob der
   Schulhof ein offener Raum ist, nicht ob das Team ihn auswählen kann. In
   Raumlisten für Planung, spontane Angebote und verfügbare Räume steht der
   System-Schulhof mit und ohne Freigabe. Toiletten bleiben ausgeblendet,
   andere Systemräume ebenfalls. #3064 hatte die Sichtbarkeit an die Freigabe
   gekoppelt; ein Schulhof ohne Häkchen verschwand damit aus dem
   Betreuungsplan, obwohl dort Blöcke geplant sind. Das ist die zweite
   bewusste Namensregel.

## Verworfen

- **Freigabe für alle zurücknehmen, Opt-in auch im Bestand.** Bricht Burbach
  und Barnstorf den Kiosk-Freispiel-Weg, bis eine Admin das Häkchen setzt.
- **Weiche behalten und Heuristiken ausbauen** (Session-Wahl persistieren,
  „Betreuer hinzufügen“ bei mehreren Sessions). Jede Heuristik ist ein neuer
  Sonderfall; wer zwei Blöcke im Hof betreut, sähe weiterhin die Raumansicht.
- **Eigene Blöcke wieder als Tabs neben dem Raum-Tab.** Bringt die doppelten
  Tabs zurück, die #3065 abgeschafft hat, und versteckt, was im Raum sonst
  läuft.
- **Kiosk bucht immer einen unabhängigen Aufenthalt.** Leert den Roster einer
  Betreuungskraft, obwohl ihr Kind gerade eingescannt hat.
- **Heim-Abmeldung auf „überall oder Gruppenraum“ reduzieren.** Kiosk-Schulen
  melden Kinder vom Hof aus ab; wegnehmen würde laufenden Betrieb zerstören.
- **Sichtbarkeit an die Freigabe koppeln** (Stand nach #3064). Blendet den Hof
  genau bei den Schulen aus, die dort Blöcke planen und deshalb die Freigabe
  zurücknehmen.

## Folgen

- Das Häkchen „Offener Raum“ muss die Freigabe tatsächlich schreiben. Der
  Raum-Proxy hat es bis [#3277](https://github.com/moto-nrw/project-phoenix/pull/3277)
  verworfen, weshalb die Sofortmaßnahme aus #3272 in Produktion wirkungslos war.
- Der Kiosk-Freispiel-Weg hängt an `Name == Schulhof AND is_open_room`. Eine
  Schule ohne Freigabe hat am Gerät keinen Schulhof-Knopf mit Auto-Session; das
  ist für Blockschulen gewollt und für neue Schulen der Grund für den
  Einrichtungsschritt.
- #2183 ist damit erledigt. „Sichtbarkeitsregel pro Raum“ und „Filter im
  Planer“ aus #2183 werden nicht weiterverfolgt, bis eine Schule sie verlangt.

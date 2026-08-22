# Etappe 4: Installierbarkeit und Benachrichtigungen

> **Für agentische Bearbeiter:** ERFORDERLICHE SUB-SKILL: `superpowers:subagent-driven-development` oder `superpowers:executing-plans`.

**Ziel:** Die Eltern-App ist als eigenständige App auf dem Home-Bildschirm installierbar, Eltern werden verständlich durch die Einrichtung von Benachrichtigungen geführt, und eine neue OGS-Nachricht erreicht sie auch dann, wenn Push nicht eingerichtet ist.

**Warum vorgezogen:** Entscheidung E11. Auf iOS entsteht der App-Charakter erst nach Installation auf dem Home-Bildschirm, und **nur installiert funktionieren dort Web-Push-Benachrichtigungen überhaupt**. Ohne diese Etappe bleibt der Qualitätsmaßstab "wie eine App aus dem App Store" auf dem iPhone unerreichbar.

**Umsetzt:** #2306, #2297, #2305 (Rest), #2307.

---

## Ausgangsbefund

Erhoben am 2026-08-15. Vieles existiert bereits, es ist nur nicht verbunden:

| Baustein | Zustand |
|---|---|
| `frontend/public/sw.js` | vorhanden |
| `ServiceWorkerRegistrar` | vorhanden, mit Test |
| `frontend/public/site.webmanifest` | vorhanden, Name "MOTO", `start_url: "/"`, `scope: "/"` |
| `frontend/public/icons/icon-192x192.png`, `icon-512x512.png` | vorhanden |
| `ParentNotificationOnboarding` | vorhanden, mit Modi `enable` / `install` / `denied`, Später-Erinnerung nach 7 Tagen, eingehängt in `app/parents/auth-guard.tsx` |
| Push-Endpunkte im Backend (`/parent/me/push/*`, `/parent/me/notification-preferences`) | vorhanden |
| **Verlinkung des Manifests** | **fehlt vollständig.** Kein Layout setzt `metadata.manifest`, kein Apple-Meta ist gesetzt. |

**Daraus folgt:** Die App ist heute nicht installierbar. `needsIOSInstall()` meldet auf dem iPhone dauerhaft `true`, ohne dass Eltern etwas dagegen tun können, denn ohne Manifest liefert "Zum Home-Bildschirm" nur ein Lesezeichen und keinen eigenständigen Start. Push bleibt auf iOS damit unerreichbar. Das ist die eine Lücke, die diese Etappe schließt.

---

## Mandat und Randbedingungen

Es gilt das Mandat aus Abschnitt 4a der Spezifikation (Neubau statt Renovierung, Website als einzige Designquelle, Phosphor-Icons ohne `duotone`, kein KI-Look, OGS-Sprache mit Anrede "Sie", alle vier Kataloge). Zusätzlich:

- **Keine neuen Abhängigkeiten.** Kein `next-pwa`, kein Workbox. Next.js kann Manifest und Apple-Meta über `metadata` erzeugen, der Service Worker existiert bereits.
- **Der Proxy rewritet den Parents-Host.** `parents.{TENANT_DOMAIN}/` wird intern auf `/parents/*` abgebildet. Öffentliche URLs beginnen also **ohne** `/parents`. `start_url` und `scope` müssen die öffentliche Sicht abbilden, nicht die interne. Vor dem Schreiben `frontend/src/proxy.ts` lesen und das Verhalten bestätigen.
- **Keine Fallback-Defaults für Umgebungsvariablen.** Fehlende Konfiguration muss laut scheitern (Projektregel).
- Bestehende Tests nie ändern, um neuen Code grün zu bekommen.

---

### Aufgabe 1: Die Eltern-App installierbar machen

**Dateien:**
- Anlegen: `frontend/src/app/parents/manifest.ts`
- Ändern: `frontend/src/app/parents/layout.tsx`
- Test: `frontend/src/app/parents/manifest.test.ts`

- [ ] **Schritt 1: Test schreiben.** Das erzeugte Manifest trägt `display: "standalone"`, einen eltern-spezifischen `name` und `short_name`, `start_url` und `scope` passend zur öffentlichen Sicht des Parents-Hosts, mindestens ein 192er- und ein 512er-Icon, `lang: "de"` und `orientation: "portrait-primary"`.

- [ ] **Schritt 2: Test laufen lassen, Fehlschlag bestätigen.**

- [ ] **Schritt 3: Manifest als Route erzeugen.** Next.js liefert `app/parents/manifest.ts` als Manifest aus. Inhalt orientiert sich an `public/site.webmanifest` der Website (`/Users/flo/Developer/moto/website/public/site.webmanifest`), aber eltern-spezifisch:

```ts
import type { MetadataRoute } from "next";

/**
 * Manifest der Eltern-App. Eigenes Manifest statt des geteilten
 * public/site.webmanifest, weil Name, Startadresse und Geltungsbereich
 * eltern-spezifisch sind: Eltern installieren "moto Eltern", nicht "MOTO".
 *
 * Ohne verlinktes Manifest liefert "Zum Home-Bildschirm" auf iOS nur ein
 * Lesezeichen, die App startet nicht eigenstaendig, und Web Push funktioniert
 * dort gar nicht.
 */
export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "moto Eltern",
    short_name: "moto",
    description:
      "Betreuung Ihres Kindes im Blick: Tagesstatus, Nachrichten an die OGS, Krankmeldung und Termine.",
    lang: "de",
    dir: "ltr",
    display: "standalone",
    orientation: "portrait-primary",
    background_color: "#ffffff",
    theme_color: "#ffffff",
    categories: ["education"],
    icons: [
      { src: "/icons/icon-192x192.png", sizes: "192x192", type: "image/png", purpose: "any" },
      { src: "/icons/icon-512x512.png", sizes: "512x512", type: "image/png", purpose: "any" },
    ],
  };
}
```

`start_url` und `scope` setzt der Bearbeiter nach Prüfung des Proxy-Verhaltens; ein falscher Geltungsbereich macht die App nicht installierbar.

- [ ] **Schritt 4: Manifest und Apple-Meta im Eltern-Layout verlinken.** In `app/parents/layout.tsx` eine `metadata`-Ausgabe ergänzen, die `manifest` auf die Route zeigt und `appleWebApp` setzt (`capable: true`, `statusBarStyle: "default"`, `title: "moto Eltern"`). Ohne `appleWebApp` startet iOS die Seite trotz Manifest im Browser-Rahmen.

- [ ] **Schritt 5: Tests laufen lassen, Erfolg bestätigen, `pnpm run check`, committen.**

---

### Aufgabe 2: Anleitung zum Home-Bildschirm (#2306)

**Dateien:** Ändern `frontend/src/components/parent/parent-notification-onboarding.tsx` und die vier Kataloge unter `parentNotificationSetup`.

Der Dialog hat bereits einen Modus `install`. Er wird zu einer echten, bebilderten Anleitung ausgebaut.

- [ ] **Schritt 1: Test schreiben.** Im Modus `install` erscheinen nummerierte Schritte, das Teilen-Symbol wird benannt, und es gibt genau eine Hauptaktion sowie eine Später-Möglichkeit.

- [ ] **Schritt 2: Fehlschlag bestätigen.**

- [ ] **Schritt 3: Anleitung umsetzen.** Drei nummerierte Schritte in Elternsprache, Anrede "Sie":

> **So haben Sie moto direkt auf dem Startbildschirm**
> 1. Tippen Sie unten in Safari auf das Teilen-Symbol.
> 2. Wählen Sie "Zum Home-Bildschirm".
> 3. Tippen Sie auf "Hinzufügen".
>
> Danach öffnen Sie moto wie eine App, und wir können Ihnen Nachrichten der OGS als Mitteilung schicken.

Icons aus dem Phosphor-Modul der Eltern-App (`Export`/`ShareNetwork` für das Teilen-Symbol, `Plus` für Hinzufügen), **nicht** aus lucide. Schrittnummern als Kreise 28 px, Text 17 px. Keine ganzflächige Einfärbung, keine Verläufe.

- [ ] **Schritt 4: Texte in de, en, ru und sq**, gleiche Schlüssel überall.

- [ ] **Schritt 5: Erfolg bestätigen, `pnpm run check`, committen.**

---

### Aufgabe 3: Push-Zustand sichtbar machen (#2297)

**Dateien:** neue Zeile im "Mehr"-Bereich (Etappe 3, Aufgabe 6) bzw. auf der Benachrichtigungsseite.

Die Rückmeldung der Schule war: Eine Elterninfo kam nicht an, und niemand merkte es. Eltern müssen erkennen können, ob sie überhaupt erreichbar sind.

- [ ] **Schritt 1: Test schreiben** für drei Zustände: Benachrichtigungen aktiv (grüner Haken, "Sie erhalten Mitteilungen"), nicht eingerichtet (orange, "Sie erhalten keine Mitteilungen" plus Schaltfläche "Jetzt einrichten"), vom Gerät blockiert (grau, Hinweis auf die Geräteeinstellungen).
- [ ] **Schritt 2: Fehlschlag bestätigen.**
- [ ] **Schritt 3: Umsetzen.** Zustand über die vorhandenen Helfer aus `~/lib/push-api` ermitteln, keine eigene Erkennung schreiben.
- [ ] **Schritt 4: Erfolg bestätigen, `pnpm run check`, committen.**

---

### Aufgabe 4: E-Mail bei neuer OGS-Nachricht (#2307)

**Dateien (Backend):** `backend/services/parent/parent_messaging_service.go` bzw. der Dienst, der eine Staff-Nachricht in den Eltern-Thread schreibt; neues Template unter `backend/templates/email/`.

Die E-Mail ist der Rückfall, wenn Push nicht eingerichtet ist. Ohne sie bleibt eine Nachricht unbemerkt, genau der gemeldete Fall.

- [ ] **Schritt 1: Test schreiben.** Schreibt die OGS eine Nachricht an einen Sorgeberechtigten, wird genau eine E-Mail an dessen Adresse ausgelöst. Verwende `test.CapturingMailer` (`backend/test/mailers.go`), keinen neuen Mock. Prüfe: Betreff, Vorschau gekürzt, Link zeigt auf `PARENTS_URL`, keine Kindernamen im Log auf Info-Level.
- [ ] **Schritt 2: Fehlschlag bestätigen.**
- [ ] **Schritt 3: Umsetzen.** Versand asynchron und best-effort nach dem Commit, wie die bestehenden Benachrichtigungen im selben Dienst; ein Fehlschlag darf die Antwort nie blockieren. Template mit der geteilten Chrome (`styles.html`, `header.html`, `footer.html`).

Inhalt, Elternsprache:

> **Betreff:** Neue Nachricht von der OGS
>
> Guten Tag {Anrede},
> die OGS {Schulname} hat Ihnen eine Nachricht zu {Kindername} geschrieben:
>
> "{gekürzte Vorschau}"
>
> [ Antworten ]  ← Link in die App
>
> Sie erhalten diese E-Mail, weil Sie in moto als Sorgeberechtigte hinterlegt sind.

- [ ] **Schritt 4: Erfolg bestätigen.**

```bash
cd backend && go test ./services/parent/...
cd backend && go test ./test/ -run 'TestGDPRLogPIIRatchet|TestHermeticTestPatterns'
```

- [ ] **Schritt 5: Committen.**

---

### Aufgabe 5: Abschluss

- [ ] **Installierbarkeit tatsächlich prüfen**, nicht behaupten: Die App unter `parents.localhost:3000` aufrufen und bestätigen, dass das Manifest ausgeliefert wird (`/manifest.webmanifest` bzw. der von Next erzeugte Pfad liefert 200 mit `application/manifest+json`) und dass der Browser die Installation anbietet. Ergebnis im Bericht benennen; wenn es nicht prüfbar war, das klar sagen.
- [ ] `cd frontend && pnpm run check` und `cd backend && go test ./...`
- [ ] Aufnahmen des Einrichtungsdialogs und der Anleitung in 390×844.

---

## Selbstprüfung

| Anforderung | Aufgabe |
|---|---|
| App installierbar, eigenes Icon, Vollbild (E11) | 1 |
| Anleitung zum Home-Bildschirm für iPhone und iPad (#2306) | 2 |
| Eltern werden beim ersten Login geführt (#2305) | vorhanden, ergänzt in 2 |
| Push-Zustand sichtbar (#2297) | 3 |
| Neue OGS-Nachricht erreicht Eltern auch ohne Push (#2307) | 4 |
| Keine neuen Abhängigkeiten | alle |
| Alle Texte in Elternsprache, vier Kataloge | 2, 3 |

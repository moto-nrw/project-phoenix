# Etappe 2: Eltern-Hülle und Designgrundlage

> **Für agentische Bearbeiter:** ERFORDERLICHE SUB-SKILL: `superpowers:subagent-driven-development` oder `superpowers:executing-plans`. Die Schritte nutzen Checkbox-Syntax (`- [ ]`).

**Ziel:** Die Eltern-App bekommt eine eigene Hülle (Kopfzeile, Seitennavigation, Bottom-Navigation) und die gestalterische Grundlage, auf der alle folgenden Etappen aufsetzen.

**Architektur:** Ein `ParentShell` unter `components/parent/shell/` ersetzt für Elternrouten die geteilte `AppShell`. Navigationsziele stehen in einer einzigen Liste, aus der Bottom-Navigation und Seitennavigation gerendert werden. Icons kommen aus einem Modul, damit die Bibliothek an einer Stelle austauschbar bleibt. Die Eltern-Zweige in `Sidebar` und `MobileBottomNav` des Personal-Portals entfallen.

**Tech-Stack:** Next.js 16, React 19, Tailwind 4, `@phosphor-icons/react`, Vitest.

**Umsetzt:** #2308 (Navigation, Hülle), Vorarbeit für #2250.

---

## Mandat für die Oberfläche: Neubau, keine Renovierung

Übernommen aus Abschnitt 4a der Spezifikation. Verbindlich.

Für alles oberhalb der Datenschicht gilt freie Hand. Die Oberfläche wird **von Grund auf neu gebaut**, nicht am Bestand entlang verbessert. Maßstab ist eine gute Kita-Eltern-App.

**Erlaubt:** bestehende Eltern-Komponenten löschen und ersetzen; neue Seitenstrukturen erfinden; Texte komplett neu schreiben; Karten, Abstände, Typografie und Dichte neu festlegen.

**Leitlinien:**

- **Keine neue Designsprache. Einzige Quelle ist `moto-nrw/website`** (lokal `/Users/flo/Developer/moto/website`), Datei `src/app/globals.css`, Blöcke `@theme inline` und `@layer components`, plus die Assets unter `public/`. Vor jeder Farb-, Typo-, Schatten- oder Effektentscheidung dort nachlesen.
- **Das Paket `@moto-nrw/design-system` wird vollständig ignoriert.** Keine Farben, keine Maße, keine Komponenten. Kein `bg-sage-*`, `bg-steel-*`, `--color-brand-primary`.
- **Icons: `@phosphor-icons/react`** (bereits installiert, `^2.1.10`). Gewicht `regular`, `fill` für aktive Zustände. **Kein `duotone`.** Nur in der Eltern-App.
- **Es darf nicht nach KI aussehen.** Keine ganzflächig eingefärbten Container, keine Verläufe, kein Glühen, kein Glasmorphismus, keine übergroßen Emoji. Farbe erscheint als Akzent (farbige Kante, Icon-Feld, Statuspille), die Kartenfläche bleibt weiß. Jede Farbe steht für einen Zustand; wo kein Zustand ist, ist keine Farbe.
- **Verständlichkeit aus Größe und Kontrast**, nicht aus Buntheit.
- **Sprache ist OGS- und Kita-Sprache**, Anrede "Sie". Keine Systembegriffe, keine Verwaltungswörter, keine Anglizismen. Alle vier Kataloge (de, en, ru, sq).
- **Mobile ist der Leitfall**, Tablet und Desktop bekommen eigene Aufteilungen.

**Grenzen:** neue Primitive ins geteilte UI-Kit (`frontend/src/components/ui/`), Kalenderdaten als `"YYYY-MM-DD"`, `pnpm run check` ohne Warnung, jede sichtbare Änderung mit Vorher/Nachher-Aufnahmen belegt.

Im Zweifel: die für Eltern verständlichere Lösung schlägt die zum Bestand ähnlichere.

---

## Globale Randbedingungen

**Typografische Leiter** (verbindlich für die ganze Eltern-App):

| Ebene | Größe | Gewicht |
|---|---|---|
| Statuswert des Kindes | 24 px | 800 |
| Seitentitel | 28 px | 700 |
| Abschnittstitel | 20 px | 600 |
| Fließtext, Buttonbeschriftung | 17 px | 400 / 600 |
| Sekundärtext | 15 px | 400 |

Keine Versalien-Mikrolabels. Keine Schrift unter 15 px.

**Farben** (Website-Werte, über `moto-*`-Utilities bzw. neu ergänzte Tokens):

| Bedeutung | Hex |
|---|---|
| Kind ist da, erledigt | `#83CD2D`, dunkler `#74B825` |
| Information, erwartet, Nachricht | `#5080D8`, dunkler `#3B68C0` |
| Offene Handlung | `#F78C10`, dunkler `#E07400` |
| Krank, abgemeldet | `#DC3545`, dunkler `#D42220` |
| Neutral, keine Betreuung | `#6B7280` |

**Maße:** Touchflächen mindestens 48 px. Radien 8 / 12 / 16 / 24 px. Schatten `0 1px 2px rgba(3,7,18,0.06)` für Flächen, `0 8px 24px rgba(3,7,18,0.08)` für angehobene. Haus-Easing `cubic-bezier(0.22, 1, 0.36, 1)`.

**Prüflauf nach jeder Aufgabe:** `cd frontend && pnpm run check` muss ohne Warnung durchlaufen.

---

## Dateiübersicht

| Datei | Verantwortung |
|---|---|
| `frontend/src/styles/globals.css` | Fehlende Website-Tokens, Punkt-Textur-Utility |
| `frontend/src/components/parent/shell/parent-icons.ts` | Alle Phosphor-Icons der Eltern-App an einer Stelle |
| `frontend/src/components/parent/shell/parent-nav-items.ts` | Die eine Navigationsliste |
| `frontend/src/components/parent/shell/parent-bottom-nav.tsx` | Mobile Navigation |
| `frontend/src/components/parent/shell/parent-sidebar.tsx` | Tablet-quer und Desktop |
| `frontend/src/components/parent/shell/parent-header.tsx` | Kopfzeile mit Logo, Sprache, Konto |
| `frontend/src/components/parent/shell/parent-shell.tsx` | Setzt die Teile zusammen |
| `frontend/src/components/ui/button.tsx` | Neue Größe `touch` |
| `frontend/src/app/parents/auth-guard.tsx` | `AppShell` durch `ParentShell` ersetzen |
| `frontend/src/components/dashboard/sidebar.tsx` | Eltern-Zweig entfernen |
| `frontend/src/components/dashboard/mobile-bottom-nav.tsx` | Eltern-Zweig entfernen |
| `frontend/src/i18n/messages/{de,en,ru,sq}.json` | Navigationstexte |

---

### Aufgabe 1: Designgrundlage in CSS

**Dateien:** Ändern `frontend/src/styles/globals.css`

**Stellt bereit:** die Utilities `.moto-dot-texture`, `.moto-dot-texture--soft` und die Tokens `--color-moto-gray-150`, `--color-moto-ink`, `--color-parent-red`, `--color-parent-red-strong`.

- [ ] **Schritt 1: Tokens ergänzen**

Im bestehenden `@theme`-Block der App ergänzen, mit Kommentar auf die Quelle:

```css
  /* Aus moto-nrw/website src/app/globals.css. Die Website ist die einzige
     Designquelle; diese Werte fehlten der App bisher. */
  --color-moto-gray-150: #eef0f3;
  --color-moto-ink: #030712;
  /* Rot der Website. Die App fuehrt daneben --color-moto-red (#dc2626) fuer
     das Personal-Portal weiter; eine app-weite Angleichung ist bewusst NICHT
     Teil dieses Vorhabens. */
  --color-parent-red: #dc3545;
  --color-parent-red-strong: #d42220;
  --color-parent-red-soft: #fdf2f3;
```

- [ ] **Schritt 2: Punkt-Textur als Flächenmerkmal**

Die App hat den Punkte-Hintergrund nur als Seitengrund. Die Website nutzt ihn zusätzlich als feine Textur **innerhalb** hervorgehobener Flächen (`.navigation-menu-featured`, `.product-system-header`). Das fehlt und ist der größte ungenutzte Hebel. Im `@layer components` ergänzen:

```css
  /* Feine Punkt-Textur INNERHALB einer Flaeche, nach dem Vorbild von
     .navigation-menu-featured und .product-system-header auf der Website.
     Gleiches 14px-Raster wie der Seitengrund, nur feiner und blasser, damit
     eine hervorgehobene Karte Textur bekommt statt nur weiss zu sein. */
  .moto-dot-texture {
    background-image: radial-gradient(
      circle,
      rgba(156, 163, 175, 0.32) 0.9px,
      transparent 1px
    );
    background-size: 14px 14px;
  }

  .moto-dot-texture--soft {
    background-image: radial-gradient(
      circle,
      rgba(156, 163, 175, 0.16) 0.85px,
      transparent 0.95px
    );
    background-size: 14px 14px;
  }
```

- [ ] **Schritt 3: Prüfen und committen**

```bash
cd frontend && pnpm run check
git add frontend/src/styles/globals.css
git -c core.hooksPath=/dev/null commit -m "feat: ergaenze fehlende Website-Tokens und die Punkt-Textur"
```

---

### Aufgabe 2: Icon-Modul und Touch-Größe

**Dateien:**
- Anlegen: `frontend/src/components/parent/shell/parent-icons.ts`
- Ändern: `frontend/src/components/ui/button.tsx`
- Test: `frontend/src/components/ui/button.test.tsx` (bestehende Datei ergänzen)

**Stellt bereit:** benannte Icon-Exporte für die Eltern-App und `<Button size="touch">`.

- [ ] **Schritt 1: Icon-Modul anlegen**

```ts
/**
 * Alle Icons der Eltern-App an einer Stelle.
 *
 * Die Eltern-App nutzt Phosphor, weil die moto-Website es nutzt und die
 * Website die einzige Designquelle ist. Personal- und Operator-Portal bleiben
 * bei lucide-react. Gewicht "regular" als Standard, "fill" fuer aktive
 * Zustaende. Kein "duotone".
 *
 * Nur ueber dieses Modul importieren, damit ein Bibliothekswechsel eine
 * Datei betrifft und nicht fuenfzig.
 */
export {
  House,
  Users,
  ChatCircleText,
  CalendarBlank,
  DotsThree,
  Megaphone,
  ForkKnife,
  Check,
  CheckCircle,
  Question,
  Clock,
  FirstAid,
  CaretRight,
  CaretLeft,
  X,
  Bell,
  Translate,
  SignOut,
} from "@phosphor-icons/react";
```

Der Bearbeiter prüft jeden Namen gegen die installierte Version und ersetzt nicht vorhandene durch die nächstliegende vorhandene Entsprechung. Erfinde keine Namen.

- [ ] **Schritt 2: Test für die neue Buttongröße**

In `button.test.tsx` ergänzen: ein Button mit `size="touch"` trägt eine Mindesthöhe von 48 px (`min-h-12`) und eine Schriftgröße von 17 px.

- [ ] **Schritt 3: Test laufen lassen, Fehlschlag bestätigen**

```bash
cd frontend && pnpm vitest run src/components/ui/button.test.tsx
```

- [ ] **Schritt 4: Größe ergänzen**

In der Größen-Zuordnung von `button.tsx`:

```ts
  // Eltern-App: 48px Mindesthoehe und 17px Schrift. Die Seitengroessen
  // (sm/base/lg/xl) sind mit py-3 zu niedrig fuer eine Touch-Flaeche nach
  // Apple HIG (44pt) und Material (48dp).
  touch: "min-h-12 rounded-xl px-5 text-[17px] font-semibold",
```

- [ ] **Schritt 5: Prüfen und committen**

```bash
cd frontend && pnpm vitest run src/components/ui/button.test.tsx && pnpm run check
git add frontend/src/components/parent/shell/parent-icons.ts frontend/src/components/ui/button.tsx frontend/src/components/ui/button.test.tsx
git -c core.hooksPath=/dev/null commit -m "feat: fuehre Phosphor-Icons und eine Touch-Groesse fuer die Eltern-App ein"
```

---

### Aufgabe 3: Die eine Navigationsliste

**Dateien:**
- Anlegen: `frontend/src/components/parent/shell/parent-nav-items.ts`
- Ändern: `frontend/src/i18n/messages/{de,en,ru,sq}.json`

**Stellt bereit:** `PARENT_PRIMARY_NAV` (vier Ziele) und `PARENT_MORE_NAV` (die Einträge hinter "Mehr"), jeweils mit `href`, `tKey`, `icon`, `iconActive` und optionalem `badge`.

Navigationsziele nach Entscheidung E8 der Spezifikation:

| Position | Ziel | href |
|---|---|---|
| 1 | Start | `/parents` |
| 2 | Kinder | `/parents/children` |
| 3 | Nachrichten | `/parents/messages` |
| 4 | Kalender | `/parents/calendar` |
| 5 | Mehr | Sheet bzw. Menü |

Hinter "Mehr": Neuigkeiten, Essensplan, Benachrichtigungen, Sprache, Neue Anmeldung, Abmelden.

- [ ] **Schritt 1: Liste anlegen**, mit einem Kommentar, der festhält: der Ungelesen-Zähler der Neuigkeiten wird auf das "Mehr"-Symbol aufaddiert, damit ein ungelesener Aushang sichtbar bleibt, obwohl der Eintrag im Sheet liegt.

- [ ] **Schritt 2: Texte in allen vier Katalogen** unter `parentNav` ergänzen bzw. angleichen. Deutsch: `start` "Start", `children` "Kinder", `messages` "Nachrichten", `calendar` "Kalender", `more` "Mehr", `news` "Neuigkeiten", `mealPlan` "Essensplan", `notifications` "Benachrichtigungen", `language` "Sprache", `enroll` "Neue Anmeldung", `logout` "Abmelden". Schlüssel müssen in de, en, ru und sq identisch sein, `verify-locales` prüft das.

- [ ] **Schritt 3: Prüfen und committen**

```bash
cd frontend && pnpm run check
git add frontend/src/components/parent/shell/parent-nav-items.ts frontend/src/i18n/messages/
git -c core.hooksPath=/dev/null commit -m "feat: definiere die Navigationsziele der Eltern-App"
```

---

### Aufgabe 4: Bottom-Navigation, Seitennavigation, Kopfzeile

**Dateien:** Anlegen `parent-bottom-nav.tsx`, `parent-sidebar.tsx`, `parent-header.tsx` unter `frontend/src/components/parent/shell/`. Tests je Komponente daneben.

**Nutzt:** `PARENT_PRIMARY_NAV`, `PARENT_MORE_NAV` aus Aufgabe 3, die Icons aus Aufgabe 2, `useShellAuth()` aus `~/lib/shell-auth-context` (liefert `mode`, `homeUrl`, `profileUrl`, `logout`).

**Gestaltungsvorgaben, verbindlich:**

*Bottom-Navigation (unter 1024 px):*
- fest am unteren Rand, `padding-bottom: env(safe-area-inset-bottom)`
- fünf gleich breite Ziele, je mindestens 56 px hoch
- Icon 26 px über Label 12 px, Label immer sichtbar
- aktiv: Icon in `fill`-Gewicht und Grün `#83CD2D`, Label 600, dazu ein 3 px hoher Balken **über** dem Ziel; inaktiv: `regular`, Grau `#6B7280`
- Zähler als kleine Pille oben rechts am Icon
- weiße Fläche, oberer Rand `1px` `#E5E7EB`, kein Schatten nach oben

*Seitennavigation (ab 1024 px):*
- 264 px breit, dauerhaft sichtbar, kein Ausklappen
- Einträge 48 px hoch, Icon 22 px, Text 17 px
- aktiv: Fläche `#F3F4F6`, Radius 12 px, Icon in `fill` und Grün
- unten angeheftet: Konto und Abmelden

*Kopfzeile:*
- 64 px hoch, weiß, unterer Rand `1px` `#E5E7EB`
- links Wortmarke (`/moto-logo-wordmark.webp` aus dem Website-Repo nach `frontend/public/` übernehmen), daneben der Seitentitel in 17 px
- rechts Sprachumschalter und Konto
- **kein** Kicker, **kein** zweiter Titel im Inhaltsbereich

- [ ] **Schritt 1: Tests schreiben.** Je Komponente: alle Ziele werden gerendert, das aktive Ziel trägt den aktiven Zustand, jedes Ziel hat Icon **und** sichtbares Label, die Bottom-Navigation ist ab 1024 px nicht im Dokument und die Seitennavigation darunter nicht.
- [ ] **Schritt 2: Tests laufen lassen, Fehlschlag bestätigen.**
- [ ] **Schritt 3: Komponenten umsetzen** nach den Vorgaben oben.
- [ ] **Schritt 4: Tests laufen lassen, Erfolg bestätigen.**
- [ ] **Schritt 5: Committen.**

---

### Aufgabe 5: ParentShell einsetzen und Personal-Zweige entfernen

**Dateien:**
- Anlegen: `frontend/src/components/parent/shell/parent-shell.tsx`
- Ändern: `frontend/src/app/parents/auth-guard.tsx`
- Ändern: `frontend/src/components/dashboard/sidebar.tsx`, `mobile-bottom-nav.tsx` und deren Tests

- [ ] **Schritt 1: `ParentShell` bauen.** Struktur: Punkte-Hintergrund als fester Grund (`moto-dotted-background--app-fixed`), darüber Kopfzeile, darunter ab 1024 px die Seitennavigation links und der Inhalt rechts, darunter die Bottom-Navigation. Inhaltsbereich: `p-4` mobil, `p-6` ab 640 px, `p-8` ab 1024 px, unten `pb-[calc(5rem+env(safe-area-inset-bottom))]` solange die Bottom-Navigation sichtbar ist. Inhaltsbreite ab 1280 px auf `max-w-6xl` begrenzen, damit keine leeren Bildschirmhälften entstehen.

- [ ] **Schritt 2: In `auth-guard.tsx`** `AppShell` durch `ParentShell` ersetzen. `ParentShellProvider`, `BreadcrumbProvider` und `ParentRealtimeBridge` bleiben unverändert.

- [ ] **Schritt 3: Eltern-Zweige aus dem Personal-Portal entfernen.** In `sidebar.tsx` den gesamten `if (mode === "parent")`-Block samt `PARENT_PREVIEW_ITEMS` und den Eltern-Hooks (`useParentMessagesUnread`, `useParentNewsUnread`, `useParentFeedbackUnread`, `useParentNewsEnabled`, `useParentMealPlanEnabled`) entfernen. Ebenso in `mobile-bottom-nav.tsx` `PARENT_MAIN_ITEMS`, `PARENT_ADDITIONAL_ITEMS` und alle `mode === "parent"`-Verzweigungen.

  **Vorsicht:** Diese Dateien gehören dem Personal-Portal. Bestehende Tests, die den Eltern-Modus prüfen, werden mit dem Zweig gegenstandslos und dürfen entfernt werden; **Tests zum Personal-Modus bleiben unverändert**. Wenn ein Test zum Personal-Modus bricht, halte an und berichte den Konflikt, statt ihn anzupassen.

- [ ] **Schritt 4: Vollständiger Prüflauf.**

```bash
cd frontend && pnpm vitest run src/components/dashboard/ src/components/parent/
cd frontend && pnpm run check
```

- [ ] **Schritt 5: Aufnahmen erstellen.** Nach der `responsive-screenshots`- bzw. `ui-before-after`-Skill je eine Aufnahme der Startseite in 390×844, 834×1194 und 1440×900, gegen `origin/development` und gegen diesen Branch. Pfade am Ende nennen.

- [ ] **Schritt 6: Committen.**

---

## Selbstprüfung

| Anforderung | Aufgabe |
|---|---|
| Eigene Hülle, gelöst vom Personal-Portal | 4, 5 |
| Navigation nach E8 (Start, Kinder, Nachrichten, Kalender, Mehr) | 3, 4 |
| Nachrichten in der mobilen Hauptnavigation (#2308) | 3 |
| Mobile, Tablet und Desktop mit getrennten Mustern (#2308) | 4, 5 |
| Icons Phosphor, kein duotone | 2 |
| Touchflächen ab 48 px | 2, 4 |
| Punkte-Hintergrund als Gestaltungsmittel | 1 |
| Website als einzige Designquelle | 1, 4 |
| Keine ganzflächigen Einfärbungen, keine Verläufe | 4 |
| Alle vier Sprachkataloge | 3 |
| Keine leeren Bildschirmhälften auf Desktop | 5 |

**Bewusst nicht in dieser Etappe:** Startseite, Kinderprofil, Nachrichten, Kalender und Neuigkeiten behalten ihren heutigen Inhalt und wandern nur in die neue Hülle. Ihr Neubau folgt in Etappe 3.

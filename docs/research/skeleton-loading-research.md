# Skeleton-Loading ohne Drift — Recherche

**Frage:** Wie baut man Skeleton-Ladezustände, die exakt der final geladenen Seite entsprechen, *ohne* paralleles Skeleton-Markup von Hand zu pflegen, das auseinanderdriftet? Stand der Technik für React 19 / Next.js 16 (App Router), 2025/2026.

**Stand:** 2026-08-16. Alle Aussagen mit Primärquelle (offizielle Docs, Repos, Quellcode, Originalstudien). SEO-Listicles wurden bewusst nicht verwendet; wo populäre Zahlen kursieren, die sich in der Primärquelle nicht finden, ist das explizit vermerkt.

---

## Zusammenfassung

**Die ehrliche Kernaussage: Es gibt kein Werkzeug, das 2026 automatisch pixelgenaue Skeletons aus deinen React-Komponenten erzeugt.** Die Tool-Kategorie, die das versucht hat (DOM-Snapshot-Generatoren per Puppeteer), ist seit 2018 tot. Was es stattdessen gibt, sind vier *Disziplinen*, mit denen man Drift strukturell verhindert statt nachträglich zu reparieren.

Rangfolge der Empfehlung für eine React-19/Next-16-Codebasis mit gemeinsamem UI-Kit:

1. **Skeleton in die echte Kit-Komponente hineinlegen (`isLoading`-Prop).** Genau das ist die Philosophie von `react-loading-skeleton` ("make components with *built-in* skeleton states") und die Praxis fast aller großen Design-Systeme (Ant Design `Skeleton loading`, Chakra v3 `loading`, Mantine `visible`). Ein Skeleton, das im selben File wie das echte Markup liegt, kann per Konstruktion nicht driften: wer die Zeile ändert, sieht beide Zustände. Das ist die einzige Maßnahme mit *strukturell* null Drift-Risiko.
2. **Suspense-/Streaming-Granularität so wählen, dass möglichst wenig skeletonisiert werden muss.** Statisches Chrome (Header, Tabs, Filter) rendert echt und sofort; nur die Datenregion bekommt einen Fallback. React sagt dazu ausdrücklich: "Don't put a Suspense boundary around every component." Weniger Skeleton-Fläche = weniger Drift-Oberfläche.
3. **Skeletons ganz vermeiden, wo "vorherige Daten behalten" möglich ist.** SWR `keepPreviousData` bzw. TanStack Query `placeholderData` ersetzen bei Filter-/Suchwechseln den Skeleton durch abgedimmte Altdaten. Das ist gleichzeitig die beste UX und der beste Drift-Schutz, weil das Skeleton gar nicht erst existiert.
4. **CSS-Maskierung echter Komponenten mit Platzhalterdaten** ist die einzige Technik, die *automatisch* strukturgleich ist, weil sie das echte Markup rendert. Sie funktioniert und ist real im Einsatz (skeleton-elements, SkeletonJS, page-skeleton-webpack-plugin intern), hat aber schwere Nebenwirkungen: Bilder, Borders, Icons, verschachtelte Hintergründe und interaktive Elemente müssen einzeln entschärft werden, und Screenreader/Tests sehen echte Fake-Texte. **Als globale Strategie: nein. Als eng begrenztes Utility für Detail-/Formularflächen mit viel Text: ja, mit Vorsicht.**
5. **Automatische Skeleton-Generierung aus dem DOM (Puppeteer/Webpack): Sackgasse.** `page-skeleton-webpack-plugin` letzte npm-Version 2018, letzter Commit 2020. `vue-skeleton-webpack-plugin` letzte Version 2018. Es gibt 2023+ keinen ernstzunehmenden Nachfolger für React. Suchmaschinen und LLM-Antworten halluzinieren hier Pakete ("Skelon"), die auf npm nicht existieren.

Dazu quer: **Skeletons sind kein Selbstzweck.** Die einzige echte kontrollierte Studie mit sauberer Methodik (Viget, n=136) hat Skeletons *schlechter* als Spinner und leeren Bildschirm gemessen. NN/g rät unter 1 Sekunde ausdrücklich zu gar keinem Indikator. Ein Skeleton, der 120 ms aufblitzt, ist eine Verschlechterung.

---

## 1. CSS-Maskierung echter Komponenten mit Platzhalterdaten

### Idee

Man rendert den echten Komponentenbaum mit Dummy-Daten und legt eine Wrapper-Klasse darüber, die Text transparent macht und graue Blöcke dahinter malt. Strukturgleichheit ist damit garantiert, weil es *dieselbe* Struktur ist.

### Wer macht das wirklich so

**SkeletonJS** ist die reinste Umsetzung: eine Klasse auf dem echten Element, dann `color: transparent !important` auf alle Kinder, ein Hintergrund und eine Shimmer-Animation über ein `::after`-Pseudoelement. Konfiguration über CSS Custom Properties (`--skeleton-background-color`, `--skeleton-border-radius`, `--skeleton-animation-duration`). Quelle: [CSS Script: Convert HTML Elements to Skeleton Loaders with SkeletonJS](https://www.cssscript.com/html-skeleton-loader/).

**skeleton-elements** (Vladimir Kharlampidi, Autor von Framework7) macht dasselbe Ziel mit einem anderen Trick: die Klasse `skeleton-text` schaltet auf einen **Spezial-Webfont um, dessen Glyphen graue Rechtecke sind**. Der echte Text bleibt im DOM, wird aber als Balkenreihe gerendert und skaliert automatisch mit der echten Typografie. Dazu `skeleton-block` für Flächen und `skeleton-effect-fade|pulse|wave`. Pakete für React, Vue, Svelte, Angular. Quellen: [Docs Core](https://skeleton-elements.nolimits4web.com/core/), [GitHub](https://github.com/nolimits4web/skeleton-elements).
*Wartungsstand:* npm `skeleton-elements@4.0.1`, veröffentlicht 2022-10-12; letzter Push im Repo 2023-03-04; 158 Stars. Also gepflegt genug zum Abgucken, zu klein für eine Abhängigkeit.

**page-skeleton-webpack-plugin** benutzt intern exakt diese Technik: es "überdeckt vorhandene Elemente per kaskadierenden Styles, um Bilder und Text zu verstecken, ohne das Layout zu verändern, und zeigt sie stattdessen als graue Blöcke". Quelle: [ElemeFE README](https://github.com/ElemeFE/page-skeleton-webpack-plugin/blob/master/README.md).

**Nicht** in diese Kategorie gehört der oft verlinkte CSS-Tricks-Artikel von Max Böck, [Building Skeleton Screens with CSS Custom Properties](https://css-tricks.com/building-skeleton-screens-css-custom-properties/). Der malt das Skeleton mit gestapelten `radial-gradient`/`linear-gradient`-Backgrounds auf *einem* Element und schaltet es per `:empty`-Selektor ab. Elegant, aber es ist wieder handgepflegte Parallelgeometrie mit Magic Numbers, also genau das Drift-Problem — nur in CSS statt in JSX. Nützlich ist der Artikel wegen des Kommentars von Razvan Caliman: das Animieren großflächiger Gradients "causes constant repaints in the browser, thus degrading perf" (prüfbar per DevTools → Rendering → Paint flashing).

### Fallstricke (die den Ansatz als globale Strategie erledigen)

- **Bilder und `<img>`:** `color: transparent` greift nicht. Jedes Bild braucht eine eigene Regel, sonst blitzen echte oder kaputte Bilder im Skeleton auf.
- **Borders, Shadows, Divider:** Ein Kit-Card mit `border-gray-200` behält im Skeleton seine Kontur. Manchmal gewollt, oft nicht — jedenfalls eine Extraregel.
- **Verschachtelte Hintergründe:** Grauer Block auf grauem Block auf weißer Card ergibt Matsch. Der Kontrast muss pro Verschachtelungsebene definiert werden.
- **Interaktive Elemente:** Buttons, Links und Inputs sind weiterhin fokussierbar und klickbar. `pointer-events: none` löst die Maus, nicht die Tastatur; zusätzlich braucht es `inert` oder `tabindex=-1` auf dem Container.
- **Textauswahl:** Ohne `user-select: none` kann man Fake-Inhalte markieren und kopieren.
- **Screenreader und Tests:** Die Dummy-Daten sind echter Text im Accessibility-Tree und in `getByText`-Queries. Ohne `aria-hidden` auf der maskierten Region liest NVDA Platzhalterinhalte vor. Ohne Testabschirmung werden Tests grün, obwohl das Skeleton steht.
- **Datenherkunft:** Platzhalterdaten müssen typkonform sein. Bei einem `DataTable<T>` mit strengen Typen heißt das Fake-Fixtures pflegen — die dann ihrerseits driften können, wenn auch weniger sichtbar als Markup.
- **Font-Variante:** skeleton-elements' Ansatz lädt einen zusätzlichen Font, der nur Latin/Ziffern/Interpunktion plus optional Arabisch, Farsi, Hebräisch, Russisch abdeckt. Für deutsche Umlaute ok, aber ein zusätzlicher Netzwerk-Request auf dem kritischen Pfad — ausgerechnet beim Laden.

### Bewertung

Drift-Risiko: **sehr niedrig** (echtes Markup). Wartungsaufwand: mittel, verschiebt sich von JSX-Pflege zu CSS-Ausnahmenpflege. Runtime-Kosten: gering, außer bei großflächigen Gradient-Animationen. A11y: **schlecht per Default**, reparierbar. CLS: **exzellent**, weil identische Boxen. Passt gut für textlastige Detailansichten, schlecht für Screens mit Bildern, Charts und vielen Controls.

---

## 2. `react-loading-skeleton` — Skeleton *im* echten Component

### Philosophie

Das Repo beschreibt sich als "Create skeleton screens that automatically adapt to your app". Zwei Kernaussagen aus dem README ([GitHub](https://github.com/dvtng/react-loading-skeleton)):

- **Automatische Dimensionierung:** "Other libraries require you to meticulously craft a skeleton screen that matches the font size, line height, and margins of your content. The `Skeleton` component is automatically sized to the correct dimensions." Das Skeleton erbt Schriftgröße, Zeilenhöhe und Margins vom Ort, an dem es steht — weil es genau dort steht, wo der Text stand.
- **Eingebaute Skeleton-Zustände statt Skeleton-Screens:** Die Empfehlung lautet, "components with *built-in* skeleton states" zu bauen. Begründung im README: die Styles bleiben synchron, die Komponente repräsentiert alle ihre Zustände, und Teilbereiche können unabhängig voneinander laden.

Das ist genau die Antwort auf die Ausgangsfrage: **Zero-Drift entsteht nicht durch Generierung, sondern durch Kolokation.**

### API-Details

- `count` rendert n Zeilen und akzeptiert Dezimalwerte: `count={3.5}` ergibt drei volle und eine halbbreite Zeile (im Quellcode über `calc(width * fractionalPart)` implementiert).
- `SkeletonTheme` ist ein Context-Provider, der `baseColor`/`highlightColor` für alle darunterliegenden Skeletons setzt.
- `wrapper` erlaubt es, jedes einzelne Skeleton in eine eigene Komponente zu hüllen.
- Dokumentierte Fallstricke aus dem README selbst: in einem `display: flex`-Container hat das Skeleton **keine intrinsische Breite** (Lösung: `flex: 1` per `containerClassName`), und wegen `line-height` wird der Container leicht größer als die angegebene Höhe (Lösung: `line-height: 1`).

### Accessibility — hier wird es unangenehm

Der Quellcode ([`src/Skeleton.tsx`](https://github.com/dvtng/react-loading-skeleton/blob/master/src/Skeleton.tsx)) rendert:

```tsx
<span className={containerClassName} aria-live="polite" aria-busy={enableAnimation}>
```

Das ist eine **Live-Region auf jedem einzelnen Skeleton**. Adrian Roselli warnt in [More Accessible Skeletons](https://adrianroselli.com/2020/11/more-accessible-skeletons.html) genau vor dieser Kombination und nennt sie bei Vuetify ein "code smell for broader ARIA abuse". Bei zwanzig Skeletons in einer Tabelle bekommt man zwanzig Live-Regionen. Wer die Bibliothek einsetzt, sollte die Container-A11y selbst übersteuern.

### Wartungsstand (ehrlich)

- npm: `react-loading-skeleton@3.5.0`, veröffentlicht **2024-09-21**. Seitdem kein Release.
- GitHub: letzter Push **2026-03-05**, 4.206 Stars, 7 offene Issues.
- Null Runtime-Dependencies, `peerDependencies: react >= 16.8`.

Also: stabil, klein, gepflegt genug, aber seit knapp zwei Jahren ohne Release. Für React 19 unkritisch (kein Legacy-Context, keine Klassen-Lifecycles), aber man sollte die ~2 KB Idee eher kopieren als die Abhängigkeit einzugehen — zumal Project Phoenix mit `ui/skeleton.tsx` bereits ein eigenes Primitive hat.

### Bewertung

Drift-Risiko: **niedrig**, sofern man die Kolokations-Regel wirklich einhält. Bundle: minimal. A11y: **muss korrigiert werden**. CLS: gut, solange die Höhe der Zeile stimmt. Passt exakt zu einem Shared-Kit.

---

## 3. Automatische Skeleton-Generierung aus dem gerenderten DOM

### Was es gab

**`page-skeleton-webpack-plugin` (ElemeFE)** startet im Dev-Modus einen Headless-Chrome via Puppeteer, rendert die Route, injiziert ein Skript, das das DOM abgreift, Elemente löscht/hinzufügt und den Rest per kaskadierenden Styles in graue Blöcke verwandelt, und schreibt das Ergebnis als statisches HTML/CSS in die `shell.html`. Der Preview-Durchlauf dauert laut Doku ca. 20 Sekunden pro Seite, dann muss neu gebaut werden. Quelle: [README](https://github.com/ElemeFE/page-skeleton-webpack-plugin/blob/master/README.md).

**`vue-skeleton-webpack-plugin`** ist die Vue-spezifische Variante (Skeleton-Komponente wird per SSR in die Shell gerendert).

### Wartungsstand (harte Zahlen aus der npm-Registry und GitHub-API, abgefragt 2026-08-16)

| Paket / Repo | Letzte npm-Version | Veröffentlicht | Letzter Commit | Stars |
|---|---|---|---|---|
| `page-skeleton-webpack-plugin` | 0.10.12 | **2018-05-29** | **2020-04-21** | 2.782 |
| `vue-skeleton-webpack-plugin` | 1.2.2 | **2018-09-12** | — | — |
| `skeleton-elements` | 4.0.1 | 2022-10-12 | 2023-03-04 | 158 |
| `react-skeleton-loader` | 1.0.4 | **2019-01-03** | — | — |
| `react-loading-skeleton` | 3.5.0 | 2024-09-21 | 2026-03-05 | 4.206 |
| `react-content-loader` | 7.1.2 | **2026-01-22** | 2026-01-22 | 14.003 |

`draperjs` existiert weder auf npm (`404`) noch als auffindbares GitHub-Repo.

### Gibt es moderne Nachfolger?

**Nein.** Eine Suche nach Build-Time- oder Storybook-basierten React-Skeleton-Generatoren (2023+) liefert ausschließlich Blogposts über handgebaute Skeleton-Komponenten. Die Suchergebnisse nennen ein Paket "Skelon" als "zero-config skeleton loading generator for React" — **das Paket existiert auf npm nicht** (`registry.npmjs.org/skelon` → `{"error":"Not found"}`). Das ist ein sauberes Beispiel dafür, wie SEO-Content und LLM-Zusammenfassungen in diesem Themenfeld Pakete erfinden. Nicht darauf verlassen.

Der einzige Vertreter der Kategorie "Skeleton aus Grafik generieren", der 2026 lebt, ist [`react-content-loader`](https://github.com/danilowoz/react-content-loader) (14k Stars, Release 2026-01). Der generiert aber *nicht* aus deinem Komponentenbaum, sondern rendert handgezeichnetes SVG — es gibt einen externen Web-Editor, in dem man das SVG malt. Das ist paralleles Markup in SVG-Form: maximale Drift, weil die Geometrie in Pixeln festgeschrieben ist und bei jeder Layoutänderung neu gezeichnet werden muss. Für responsive App-Screens ungeeignet, für stabile Marketing-Illustrationen ok.

### Warum die Kategorie gestorben ist

Der Puppeteer-Snapshot-Ansatz löst ein Problem, das es im App Router nicht mehr gibt: bei einer klassischen SPA war die erste Route eine leere HTML-Shell, und man wollte in diese Shell ein statisches Skeleton backen. Next.js streamt heute stattdessen echtes Server-gerendertes HTML mit `loading.tsx`/`<Suspense>`. Der Generator hätte außerdem gegen jede Client-State-Variante, jeden Viewport und jede Rolle einen eigenen Snapshot gebraucht — bei einer mandantenfähigen App mit rollenabhängigen Ansichten ist das kombinatorisch nicht haltbar.

### Bewertung

**Sackgasse.** Nicht evaluieren, nicht einführen.

---

## 4. `isLoading` auf den echten Shared-Komponenten

Das ist der De-facto-Konsens der großen Design-Systeme. Vier Varianten, die man auseinanderhalten muss:

**a) Die Komponente rendert intern ihr eigenes Skeleton (bevorzugt).**
Ant Design `Skeleton` hat `loading` ("Display the skeleton when true"), `active` (Animation), `paragraph`/`rows`. Die "When to use"-Guidance: Skeleton "could be replaced by Spin in any situation, but can provide a better user experience", empfohlen bei längeren Ladezeiten, informationsreichen Komponenten (Listen, Cards) und Erstladung. Quelle: [Ant Design Skeleton](https://ant.design/components/skeleton).

**b) Wrapper-Komponente über echten Children.**
Chakra v3 `Skeleton` nimmt `loading` und die echten Kinder; bei `loading=false` faded der Inhalt ein, die Boxmaße bleiben erhalten. Dazu `SkeletonText` (`noOfLines`) und `SkeletonCircle`, Varianten `pulse`/`shine`/`none`. Quelle: [Chakra Skeleton](https://chakra-ui.com/docs/components/skeleton).
Mantine `Skeleton` funktioniert analog über `visible` und legt sich über die Kinder; Props `height`, `width`, `radius`, `circle`, `animate`. Ein Hinweis auf `prefers-reduced-motion` fehlt in der Doku. Quelle: [Mantine Skeleton](https://mantine.dev/core/skeleton/).

**c) Dimensionsableitung aus Children.**
MUI `Skeleton` kann Breite/Höhe aus dem umschlossenen Kind ableiten ("inferring dimensions"), Varianten `text|circular|rectangular|rounded`, Animationen `pulse` (Default), `wave`, `false`. MUI argumentiert explizit mit Wahrnehmung: das Skeleton zeigt, "what is to come", statt eines abstrakten Spinners. Quelle: [MUI Skeleton](https://mui.com/material-ui/react-skeleton/).

**d) Loading-Prop, die *kein* Skeleton rendert (die Falle).**
Ant Designs **`Table`** hat zwar `loading`, rendert aber laut API einen **Spin**, nicht ein Skeleton ("Loading status of table", Typ `boolean | SpinProps`). Quelle: [Ant Design Table](https://ant.design/components/table). Wer "unsere Tabelle hat doch schon `isLoading`" sagt, muss also nachsehen, *was* dieser Zustand rendert. In Project Phoenix ist genau das der Fall (siehe unten).

**Design-System-Guidance, die über die API hinausgeht — Shopify Polaris:**
- Skeleton statt Spinner, weil es schneller wirkt und Kontext gibt.
- Skeleton-Loading nur für **dynamische** Inhalte; für Inhalte, die sich nicht ändern, **echten Inhalt** anzeigen.
- Die Zeilenanzahl an den echten Inhalt anpassen, damit die Darstellung realistisch ist.
- **Keine Platzhalter-Titel**, die sich beim Laden ändern — das verwirrt und erzeugt einen springenden Ladeeindruck.
- Spinner nur für das, was sich nicht als Skeleton darstellen lässt, z. B. Diagramme.
Quelle: [Polaris Skeleton body text](https://polaris-react.shopify.com/components/feedback-indicators/skeleton-body-text) (Weiterleitung auf shopify.dev; Inhalt via Suchindex verifiziert).

Der Punkt "echten Inhalt für statische Inhalte" ist strategisch der wichtigste: er deckt sich exakt mit der Suspense-Granularitätsregel aus Abschnitt 5.

### Bewertung

Drift-Risiko: **niedrigst**, weil Skeleton und echtes Markup dieselbe Datei sind und derselbe Review sie sieht. Bundle: null zusätzlich. A11y: kontrollierbar an *einer* Stelle statt an fünfzig. CLS: gut, wenn die Skeleton-Zeilen dieselbe Höhe wie die echten Zeilen haben — das ist die einzige Kennzahl, die man testen sollte.

---

## 5. Suspense-/Streaming-Granularität und "Skeleton vermeiden"

### React-Docs: nicht zu granular

Aus [react.dev/reference/react/Suspense](https://react.dev/reference/react/Suspense), wörtlich:

> "Don't put a Suspense boundary around every component. Suspense boundaries should not be more granular than the loading sequence that you want the user to experience. If you work with a designer, ask them where the loading states should be placed — it's likely that they've already included them in their design wireframes."

Weitere relevante Punkte derselben Seite:
- **Default = gemeinsam enthüllen.** Alles unter *einer* Boundary erscheint zusammen, auch wenn Teile früher fertig sind.
- **Verschachtelte Boundaries = progressives Enthüllen.** Erst Biografie, dann Alben.
- **`useDeferredValue` gegen Skeleton-Flackern bei Eingaben:** alte Ergebnisse abgedimmt stehen lassen (`opacity: isStale ? 0.5 : 1`), statt in den Fallback zu fallen.
- **`startTransition` verhindert das Verstecken bereits sichtbarer Inhalte:** "A Transition doesn't wait for *all* content to load. It only waits long enough to avoid hiding already revealed content." Suspense-integrierte Router sollen Navigationen per Default in Transitions wrappen.
- **Wichtige Einschränkung:** "Suspense does not detect when data is fetched inside an Effect or event handler." Wer mit SWR im Client lädt, bekommt von Suspense nichts geschenkt — dort greift ausschließlich der `isLoading`-Pfad.

### Next.js 16: `loading.tsx` vs. verschachteltes `<Suspense>`

Aus den Next.js-16-Docs ([`loading.js`](https://nextjs.org/docs/app/api-reference/file-conventions/loading), Version 16.3.1, [Fetching Data](https://nextjs.org/docs/app/getting-started/fetching-data)):

- `loading.js` wrappt `page.js`, verschachtelte `layout.js` und `not-found.js` in eine Suspense-Boundary — **nicht** das `layout.js` desselben Segments. Es streamt also die **ganze Seite**.
- **Die Layout-Falle:** greift das Layout auf uncached/Runtime-Daten zu (`cookies()`, `headers()`, uncached fetches), zeigt `loading.js` *keinen* Fallback dafür; ohne Cache Components blockiert die Navigation, bis das Layout fertig gerendert ist. Fix: Datenzugriff ins `page.js` verschieben oder im Layout eine eigene `<Suspense>`-Boundary setzen.
- Die Doku empfiehlt daraus abgeleitet ausdrücklich: "while `loading.js` works well for streaming route segments, using `<Suspense>` closer to the runtime or uncached data access is recommended."
- Die Fallback-UI wird **geprefetcht**, Navigation ist unterbrechbar, geteilte Layouts bleiben interaktiv.
- "Creating meaningful loading states": Skeletons, Spinner **oder ein kleiner, aber sinnvoller Teil des künftigen Screens** (Titelbild, Titel).

Für die Ausgangsfrage heißt das: Je feiner die Boundaries um die *Datenregionen* liegen, desto kleiner die Skeleton-Fläche, die überhaupt gepflegt werden muss. Header, Tabs, Filterchips und leere Tabellenköpfe rendern echt.

### Skeleton komplett vermeiden: vorherige Daten behalten

**SWR `keepPreviousData`** ([Docs](https://swr.vercel.app/docs/advanced/understanding)): behält den letzten Datensatz während der Revalidierung, gedacht für "continuous user actions, e.g. real-time search when typing". Dazu die Semantik-Unterscheidung, die man richtig verdrahten muss:
- `isLoading` = Request läuft **und** es gibt noch keine Daten → Skeleton.
- `isValidating` = Request läuft, egal ob Daten da sind → dezenter Hintergrund-Indikator, **kein** Skeleton.

Wer `isValidating` an den Skeleton hängt, baut sich bei jedem Fokuswechsel ein Vollbild-Skeleton.

**TanStack Query `placeholderData`** ([Docs](https://tanstack.com/query/latest/docs/framework/react/guides/placeholder-query-data)): Platzhalter- oder Teildaten, die **nicht** in den Cache geschrieben werden; die Query startet im `success`-State mit `isPlaceholderData: true`. Mit `placeholderData: (prev) => prev` bleibt bei einem Key-Wechsel von `['todos', 1]` auf `['todos', 2]` die alte Liste stehen, "instead of having to show a loading spinner". Abgrenzung zu `initialData`: das landet dauerhaft im Cache.

Interessant für Ansatz 1: `placeholderData` ist genau der offiziell gesegnete Weg, **die echte Komponente mit Fake-Daten zu rendern** — nur eben mit *plausiblen* statt maskierten Daten (z. B. Preview-Daten aus der Listen-Query). Polaris' Warnung gilt hier direkt: keine Platzhalter-Titel, die sich beim Laden ändern.

### Bewertung

Drift-Risiko: **struktureller Gewinn** — die Technik reduziert die Menge an Skeleton, statt sie besser zu pflegen. Runtime: kein Overhead, Streaming spart TTFB. A11y: neutral bis gut (weniger Zustandswechsel = weniger Screenreader-Lärm). CLS: bestes Verhalten aller Ansätze, weil das Chrome nie neu aufgebaut wird.

---

## 6. Querschnittsbewertung

| Kriterium | 1 CSS-Maskierung | 2 `react-loading-skeleton` | 3 DOM-Generatoren | 4 `isLoading` im Kit | 5 Suspense + keepPreviousData |
|---|---|---|---|---|---|
| **Drift-Risiko** | sehr niedrig (echtes Markup) | niedrig (Kolokation) | hoch (Snapshot veraltet sofort) | **niedrigst** | entfällt teilweise (kein Skeleton nötig) |
| **Wartung** | CSS-Ausnahmen + Fake-Fixtures | ein Ort pro Komponente | Buildschritt + 20 s/Seite, tot | ein Ort pro Komponente | Boundary-Platzierung, einmalig |
| **Bundle** | ~1 KB CSS (bzw. + Webfont) | ~2 KB, 0 Deps | n/a | 0 | 0 |
| **Runtime** | Gradient-Repaints beachten | trivial | n/a | trivial | Streaming spart Zeit |
| **A11y** | schlecht per Default (Fake-Text im AT-Baum), reparierbar via `aria-hidden` + `inert` | **Live-Region pro Skeleton** — übersteuern | n/a | zentral kontrollierbar | am ruhigsten |
| **CLS** | exzellent (identische Boxen) | gut bei korrekter Zeilenhöhe | gut, aber statisch | gut | **exzellent** |
| **Passt zu Shared Kit** | als eng begrenztes Utility | ja | nein | **ja, ideal** | ja |

### Accessibility-Regeln (Primärquelle: Adrian Roselli)

[More Accessible Skeletons](https://adrianroselli.com/2020/11/more-accessible-skeletons.html) ist die belastbarste Quelle zum Thema:

- **`aria-busy` allein reicht nicht.** In Rosellis Tests respektierte nur JAWS 2020 (mit IE11) das Attribut; Narrator, NVDA, TalkBack und VoiceOver "treat this content as any other". `aria-busy="true"` ist eine Absichtserklärung, keine Wirkung.
- **Deshalb zusätzlich `aria-hidden="true"`** auf die Skeleton-Dekoration, plus ein visuell verstecktes "Wird geladen"-Text für AT. Beim Fertigladen `aria-busy` auf `false` und das Skeleton aus dem DOM (oder `aria-hidden`).
- **Anti-Pattern:** `aria-busy` + `aria-live="polite"` + `role="alert"` gleichzeitig (Vuetify) — widersprüchliche Semantik.
- **Kontrast:** die meisten Skeletons verfehlen WCAG 1.4.11 (Non-text Contrast).
- **Animationsdauer:** WCAG 2.2.2 verlangt Stoppen/Pausieren bei Animationen über 5 Sekunden.
- **Reduced Motion:** Animation nur unter `@media (prefers-reduced-motion: no-preference)`.
- Roselli hält Skeletons grundsätzlich für "Lilliputian lies" und rät, den Einsatz zu hinterfragen, wo es Alternativen gibt.

`aria-busy` gehört laut [MDN](https://developer.mozilla.org/en-US/docs/Web/Accessibility/ARIA/Reference/Attributes/aria-busy) ohnehin primär in Live-Regions und Feeds, wird vor mehreren Updates auf `true` und danach auf `false` gesetzt; es ist "semantic accessibility metadata, not visual styling".

Praktische Konsequenz: **genau eine ankündigende Region pro Ladebereich**, der Rest presentational. Skeleton-Elemente dürfen nicht fokussierbar sein und müssen beim Laden aus dem DOM verschwinden statt neben dem Inhalt zu bleiben.

---

## 7. Wahrnehmungsforschung: wann Skeletons schaden

Hier ist die Quellenlage schlechter, als es die Sekundärliteratur behauptet. Die kursierenden Zahlen "9–12 % schneller wahrgenommen" und "20 % schneller als Spinner" ließen sich in keiner Primärquelle belegen; sie stammen aus Blogposts, die sich gegenseitig zitieren und dabei die Viget-Studie in ihr Gegenteil verdrehen.

**Viget (2017), n = 136** — [A Bone to Pick with Skeleton Screens](https://www.viget.com/articles/a-bone-to-pick-with-skeleton-screens): drei gleich lange Ladeanimationen (Skeleton, Spinner, leerer Bildschirm) auf Mobilgeräten, ca. 70 Teilnehmer über Mechanical Turk, Rest über Vigets Kanäle.

| Metrik | Skeleton | Spinner | Leer |
|---|---|---|---|
| Wahrgenommene Wartezeit (s) | **2,82** | 2,41 | 2,29 |
| "Meals loaded quickly" (Zustimmung) | **59 %** | 74 % | 66 % |
| Aufgabendauer nach dem Laden (s) | **10,54** | 9,49 | 9,50 |

Fazit der Autoren: "the skeleton screen performed the worst by all metrics."

**Bill Chung** — [Everything you need to know about skeleton screens](https://uxdesign.cc/what-you-should-know-about-skeleton-screens-a820c45a571a): Gegenstudie, zwei Phasen à 80 Personen. Ergebnis: Skeletons wirken kürzer als leerer Bildschirm und Spinner, "but not by much". Innerhalb der Skeleton-Varianten: Wave schlägt Pulse (65 %), links-nach-rechts schlägt rechts-nach-links (68 %), langsam-stetig schlägt schnell (60 %). Chung schreibt selbst: "The sample sizes in this study are too small to conclude anything definitively." Seine wichtigste Designaussage: ein echter Skeleton lädt **progressiv** und ersetzt Platzhalter, sobald Daten ankommen — er ist kein statischer Splash Screen.

**NN/g** — [Skeleton Screens 101](https://www.nngroup.com/articles/skeleton-screens/):
- **unter 1 Sekunde: gar kein Indikator.** Ein Skeleton bei Sub-Sekunden-Ladezeiten "frustriert mit unnötiger visueller Störung".
- 2–10 s: Spinner für einzelne Module, Skeleton für Vollseitenladungen.
- über 10 s: Fortschrittsbalken.
- Warnung vor "frame-display skeletons" (nur Header/Footer) — Nutzer halten die Seite für kaputt.
- Animation "can potentially be distracting, annoying, or even create accessibility problems".
- "Skeleton screens do not replace performance-optimization efforts."

**Praktische Ableitung — Anti-Flash:** Skeleton erst nach einer Verzögerung von ~300–500 ms einblenden und dann für eine Mindestdauer stehen lassen, damit er nicht aufblitzt. Beides ist in keiner der Bibliotheken eingebaut und muss selbst gebaut werden (Delay-Hook bzw. `useDeferredValue`/Transition für den Navigationsfall).

Die ehrliche Gesamtaussage: **Skeletons sind wahrnehmungspsychologisch bestenfalls ein kleiner Gewinn und bei kurzen Ladezeiten ein Verlust.** Ihr belastbarer Vorteil ist nicht "gefühlt schneller", sondern **Layoutstabilität** (kein CLS) und **Kontext** (der Nutzer sieht, was kommt). Das ist Grund genug, sie zu bauen — aber kein Grund, sie überall zu bauen.

---

## Empfehlung für Project Phoenix

### Ausgangslage in der Codebasis (geprüft am 2026-08-16)

- `frontend/src/components/ui/skeleton.tsx`: minimales Primitive, `animate-pulse rounded-md bg-gray-200`, im Stil von shadcn/ui.
- `frontend/src/components/ui/page-skeletons.tsx`: **parallel gepflegtes Skeleton-Kit** mit `SkeletonRegion`, `PageHeaderSkeleton`, `TableSkeleton`, `CardSkeleton`, `CardGridSkeleton`, `DetailSkeleton`, `FormSkeleton`, `ListSkeleton`, `ListPageSkeleton`. Der Dateikommentar formuliert die Absicht korrekt ("Each primitive mirrors the markup of its real kit counterpart") — das ist aber genau die Bauform, die per Konstruktion driftet: `TableSkeleton` und `DataTable` sind zwei Dateien, die niemand zusammen ändern muss.
- Die A11y-Regel dort ist bereits richtig gelöst: genau ein `<output aria-label aria-live="polite">` pro Ladebereich, die Primitives darunter sind presentational. Das entspricht Roselli besser als `react-loading-skeleton`.
- `ui/data-table.tsx` **hat** `isLoading` (Zeile 33/63) — rendert aber eine einzelne Zeile mit dem Text "Wird geladen…" über alle Spalten, **kein** Skeleton. Das ist genau die Ant-Design-`Table`-Falle: die Prop existiert, füllt aber die Erwartung nicht.

### Konkreter Plan (in dieser Reihenfolge)

**Schritt 1 — `DataTable.isLoading` rendert Skeleton-Zeilen intern.**
Der größte Hebel bei kleinstem Diff. `TableSkeleton` wandert als private Renderfunktion *in* `data-table.tsx` und benutzt `columns` als Quelle für Spaltenzahl und -breiten. Damit ist die Tabellen-Skeleton-Geometrie strukturell nicht mehr driftbar, und jeder Aufrufer, der bereits `isLoading` durchreicht, bekommt das Upgrade gratis. Dasselbe Muster danach für `InfoCard` und die `detail-modal-components`-Felder. Vorbild: Ant Design `Skeleton loading`, Chakra v3 `loading`. Regel für neue Kit-Komponenten: **wer Daten anzeigt, besitzt seinen eigenen Ladezustand.**

**Schritt 2 — `page-skeletons.tsx` schrumpfen, nicht ausbauen.**
Sobald die Kit-Komponenten ihren Ladezustand selbst können, bleiben in `page-skeletons.tsx` nur noch: `SkeletonRegion` (die A11y-Klammer) und die Seitenkomposition (`ListPageSkeleton`), die aus echten Kit-Komponenten mit `isLoading` zusammengesetzt wird statt aus nachgebauten Divs. Jede weiterhin handgebaute Silhouette ist ein bekannter Drift-Kandidat und sollte im Datei-Kommentar als solcher markiert sein.

**Schritt 3 — Suspense-Granularität statt Vollbild-`loading.tsx`.**
`loading.tsx` streamt das gesamte Segment; laut Next-16-Doku ist `<Suspense>` nahe am uncached Datenzugriff die Empfehlung. Für die Listen-Screens heißt das: `PageHeaderWithSearch`, `NavigationTabs` und Filterchips rendern **echt** (sie hängen nicht an den Nutzdaten), nur die Tabellen-/Kartenregion bekommt eine Boundary. Das halbiert die Skeleton-Fläche und entspricht Polaris' Regel "use actual content for content that doesn't change". Zusätzlich prüfen, ob ein Layout uncached Daten liest — sonst blockiert die Navigation trotz `loading.tsx`.

**Schritt 4 — SWR-Semantik korrekt verdrahten und Skeletons wegoptimieren.**
Skeleton nur an `isLoading`, niemals an `isValidating`. Bei Filter-/Zeitraum-/Gruppenwechseln (Kindersuche, Zeiterfassung, Anwesenheiten) `keepPreviousData: true` setzen und die alte Liste abgedimmt stehen lassen — das ist die einzige Maßnahme, die Skeleton-Flackern bei Navigation vollständig beseitigt, und sie kostet keine Pflege. Für Sucheingaben zusätzlich `useDeferredValue` mit `opacity`-Dimming nach dem React-Doku-Muster.

**Schritt 5 — Anti-Flash-Regel.**
Skeleton erst nach ~300 ms einblenden, dann mindestens ~300 ms stehen lassen. NN/g ist eindeutig: unter einer Sekunde ist gar kein Indikator besser. Ein kleiner `useDelayedFlag`-Hook im Kit reicht; er gehört in `SkeletonRegion`, damit ihn niemand vergisst.

**Schritt 6 — CSS-Maskierung nur als eng begrenztes Utility, falls überhaupt.**
Wenn eine textlastige Detailfläche (Profil, Stammdaten, Formular-Review) auftaucht, für die sich `isLoading` im Kit nicht lohnt, ist eine `.moto-skeleton`-Wrapperklasse nach SkeletonJS-Vorbild vertretbar: `color: transparent`, Hintergrundblöcke, `user-select: none`, `pointer-events: none`, dazu `inert` und `aria-hidden` auf dem Container und Kit-Fake-Fixtures als Datenquelle. **Aber:** nicht als Default-Strategie, nicht auf Screens mit Bildern/Charts/Controls, und nur mit einem Test, der prüft, dass Platzhaltertext nicht im Accessibility-Tree landet. Den Webfont-Trick von skeleton-elements nicht übernehmen (zusätzlicher Request auf dem kritischen Pfad).

**Nicht tun:**
- Kein `react-loading-skeleton` als Abhängigkeit. Die Idee ist richtig und übernehmenswert, das Paket bringt eine `aria-live`-Region pro Skeleton mit, die gegen die bereits saubere `SkeletonRegion`-Regel arbeitet, und das eigene Primitive existiert schon.
- Kein `react-content-loader` (handgezeichnetes SVG = maximale Drift bei responsiven Screens).
- Keine DOM-/Build-Time-Generatoren. Tote Kategorie, und Suchergebnisse in diesem Feld erfinden Pakete.
- Keine Skeletons für Charts — Polaris und NN/g sind sich einig: dafür Spinner.

### Was man messen sollte

Drift ist testbar, wenn man den richtigen Test wählt: **nicht** Snapshot-Vergleich von Skeleton gegen echtes Markup (zu spröde), sondern **Höhenstabilität**. Ein Test, der die gerenderte Höhe der Ladezustands-Region mit der Höhe der geladenen Region bei n Zeilen vergleicht, fängt genau den Fehler, der Nutzer stört (Layout-Sprung), und toleriert kosmetische Unterschiede, die niemanden stören. Alles andere fängt der Review, sobald Skeleton und echtes Markup in derselben Datei stehen — und das ist der eigentliche Punkt dieser Recherche.

---

## Quellen

**Bibliotheken und Quellcode**
- react-loading-skeleton: [Repo](https://github.com/dvtng/react-loading-skeleton), [README](https://github.com/dvtng/react-loading-skeleton/blob/master/README.md), [Skeleton.tsx](https://github.com/dvtng/react-loading-skeleton/blob/master/src/Skeleton.tsx), [npm](https://www.npmjs.com/package/react-loading-skeleton)
- page-skeleton-webpack-plugin: [Repo](https://github.com/ElemeFE/page-skeleton-webpack-plugin), [README](https://github.com/ElemeFE/page-skeleton-webpack-plugin/blob/master/README.md)
- skeleton-elements: [Repo](https://github.com/nolimits4web/skeleton-elements), [Core-Docs](https://skeleton-elements.nolimits4web.com/core/)
- react-content-loader: [Repo](https://github.com/danilowoz/react-content-loader)
- SkeletonJS: [CSS Script](https://www.cssscript.com/html-skeleton-loader/)
- Versions-/Commit-Daten: npm-Registry (`registry.npmjs.org`) und GitHub-API, abgefragt 2026-08-16

**Design-Systeme**
- [MUI Skeleton](https://mui.com/material-ui/react-skeleton/)
- [Ant Design Skeleton](https://ant.design/components/skeleton) · [Ant Design Table](https://ant.design/components/table)
- [Chakra UI v3 Skeleton](https://chakra-ui.com/docs/components/skeleton)
- [Mantine Skeleton](https://mantine.dev/core/skeleton/)
- [shadcn/ui Skeleton](https://ui.shadcn.com/docs/components/skeleton)
- [Shopify Polaris — Skeleton body text](https://polaris-react.shopify.com/components/feedback-indicators/skeleton-body-text)

**Framework-Dokumentation**
- [React — `<Suspense>`](https://react.dev/reference/react/Suspense)
- [Next.js 16 — `loading.js`](https://nextjs.org/docs/app/api-reference/file-conventions/loading)
- [Next.js 16 — Fetching Data / Streaming](https://nextjs.org/docs/app/getting-started/fetching-data)
- [SWR — Understanding SWR / keepPreviousData](https://swr.vercel.app/docs/advanced/understanding)
- [TanStack Query — Placeholder Query Data](https://tanstack.com/query/latest/docs/framework/react/guides/placeholder-query-data)

**Accessibility**
- [Adrian Roselli — More Accessible Skeletons](https://adrianroselli.com/2020/11/more-accessible-skeletons.html)
- [MDN — aria-busy](https://developer.mozilla.org/en-US/docs/Web/Accessibility/ARIA/Reference/Attributes/aria-busy)

**Wahrnehmungsforschung**
- [Viget — A Bone to Pick with Skeleton Screens](https://www.viget.com/articles/a-bone-to-pick-with-skeleton-screens)
- [Bill Chung — Everything you need to know about skeleton screens](https://uxdesign.cc/what-you-should-know-about-skeleton-screens-a820c45a571a)
- [NN/g — Skeleton Screens 101](https://www.nngroup.com/articles/skeleton-screens/)

**CSS-Technik**
- [Max Böck / CSS-Tricks — Building Skeleton Screens with CSS Custom Properties](https://css-tricks.com/building-skeleton-screens-css-custom-properties/)

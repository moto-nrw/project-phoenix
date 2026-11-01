# Seitenleisten-Icons mit Szene

Jedes Icon einer Seitenleisten-Zeile spielt beim Überfahren mit der Maus oder
beim Tastaturfokus einmal eine **Szene**: Teile des Icons bewegen sich so, wie
es das Symbol tut. Der Uhrzeiger dreht eine Runde, die Tür schwingt auf, Haken
werden gesetzt. Das gilt für alle vier Portale (Tenant, Schule, Träger, Eltern).
Die Regel dazu für Agenten: `.claude/rules/sidebar-icon-scenes.md`.

| Datei | Inhalt |
|---|---|
| `nav-scene-icon.tsx` | `NavSceneIcon` (Phosphor-Glyph), `NavConceptIcon` (Konzept, Props wie `MotoNavIcon`), `NavSceneFallback` (alte Pfad-Icons hüpfen als Ganzes) |
| `nav-scenes.tsx` | `NAV_SCENES`: je Glyph (`displayName`, z. B. `ClockIcon`) die nachgezeichneten Teile |
| `src/styles/nav-scenes.css` | die Bewegung je Teil, gleiche `ns-*`-Klassen |
| `nav-scene-coverage.test.ts` | Wächter: jedes Glyph einer Seitenleiste hat eine Szene, jede Szene bewegt Teile |

## Neues Icon in einer Seitenleiste

Der Wächter-Test schlägt fehl, sobald eine Seitenleiste (`*sidebar*.tsx`,
`*-nav-items.ts`, `lib/staff-navigation.ts`) ein Glyph ohne Szene zeigt. Dann:

1. **Handlung finden.** Was „macht“ das Symbol? Kalender → Blatt reißt ab,
   Megafon → ruft, Ablage → Brief fällt hinein. Ein Glyph, mehrere Konzepte:
   eine Szene (Schlüssel ist das Glyph).
2. **Nachzeichnen.** Phosphor-Glyphen sind ein einziger Pfad, ihre Teile lassen
   sich nicht einzeln bewegen. Zeichne das Glyph im Raster `0 0 256 256` mit
   Kontur 16, runden Enden und Ecken nach (das Gewicht „regular“). Jedes
   bewegte Teil wird ein eigenes Element mit Klasse `ns-<teil>`. Lege das
   Original daneben und vergleiche bei 20 px.
3. **Bewegen.** In `nav-scenes.css` je Teil eine Regel
   `.ns-play .ns-<teil>{animation:ns-<name> <dauer> var(--ns-o) <verzögerung> both}`
   und die Keyframes. Ende jeder Szene = exakt der Ruhezustand.
4. **Prüfen.** `pnpm exec vitest run src/components/ui/nav-scene-icon`, dann im
   laufenden Portal überfahren: startet sofort, läuft zu Ende, kein Sprung am
   Schluss, Masken decken auch auf der aktiven (grauen) Zeile.

Ein Seitenleisten-Icon wird immer über `NavConceptIcon` oder `NavSceneIcon`
gerendert, nie direkt als Phosphor-Komponente.

## Maße und Tempo

| | Wert |
|---|---|
| Dauer einer Szene | 260–640 ms, Staffelung zwischen Teilen 55–110 ms |
| Kurve | `--ns-o` = `cubic-bezier(.2,.9,.3,1)`: steiler Start, die erste Bewegung kommt in den ersten ~80 ms. Zeiger: `cubic-bezier(.4,0,.2,1)`, Umblättern: `cubic-bezier(.45,0,.3,1)` |
| Ausschläge (256er-Einheiten) | Sprünge 12–26, Wackeln 8–15°, Zeiger eine ganze Umdrehung, Skalieren 0.2–1.5 nur kleiner Teile |
| Wirkung | gern mit Ausholen (Gegenbewegung vorweg) und kurzem Nachfedern (375° → 360°); höchstens zwei bis drei bewegte Teile |
| Laufzeit-Klasse | `ns-play` bleibt `NAV_SCENE_PLAY_MS` (900 ms) stehen; längste Szene samt Verzögerung bleibt darunter |

Die Szene ersetzt kein Feedback: Hintergrund und Farbe der Zeile wechseln wie
bisher in 150 ms, die Zeile gibt beim Klick auf 98 % nach (`sidebar-geometry.ts`).

## Techniken

- `transform-box: view-box` gilt für alle Teile: `transform-origin` und
  `translate` in `px` sind Rastereinheiten (26 Einheiten ≈ 2 px bei 20 px).
- Strich zeichnen: Klasse `ns-draw` mit `pathLength="1"`, Keyframes `ns-draw`.
- Auftauchen: Klassen `ns-solid ns-pop`, Keyframes `ns-pop`.
- Schwingen, Umblättern, Drehen um eine Achse: `scaleX` mit Ursprung an der
  Angel. 3D-Transforms (`rotateY`) auf SVG-Kindern rendern Browser unzuverlässig.
- Verdecken (Sprechblase vor Sprechblase, Ablagekante vor Brief): Klasse
  `ns-mask` füllt mit `--nav-row-bg`, der Hintergrundfarbe der Zeile aus
  `sidebar-geometry.ts`. Neue Zeilenarten setzen diese Variable mit.
- Durchrollen: `<clipPath id={uid}>` um den Bereich (`uid` aus `render`), Inhalt
  bei 45 % hinaus, bei 46 % von der Gegenseite herein.
- Drehen ohne Endsprung: Endwinkel = Symmetriewinkel (Zahnrad 90°, Sonne 45°).
- Staffelung: Klassen `ns-i0`…`ns-i9` setzen `--ns-i`, die Regel rechnet
  `calc(var(--ns-i,0)*90ms)`.

## Verhalten

- Auslöser ist der nächste Link oder Knopf um das Icon: Maus (`pointerType
  mouse`) oder Tastaturfokus (`:focus-visible`). Tipp auf dem Touchscreen,
  gesperrte Zeilen ohne Link und „Bewegung reduzieren“ spielen nichts.
- Aktiv trägt das Icon die Primärfarbe seines Konzepts
  (`data-moto-duotone-tone`), inaktiv die Farbe der Zeile.
- Die Icons sind Dekoration (`aria-hidden`); Namen tragen die Zeilen.

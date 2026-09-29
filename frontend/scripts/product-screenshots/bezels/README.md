# Apple Product Bezels

Die offiziellen Apple Product Bezels für die Geräte-Mockups der
Produkt-Screenshots (#3759). Die Dateien liegen **unverändert** hier: Apples
Richtlinien erlauben es, die Web-App auf Apple-Geräten darzustellen, aber nicht,
die Rahmen zu verändern. Deshalb setzt `frame.ts` das Rohbild unter den Bezel,
statt am Bezel etwas zu malen, und `devices.ts` hält für jedes Gerät das
Bildschirmrechteck im Bezel-Bild.

| Datei | Gerät | Bezel-Maße | Bildschirmrechteck (links, oben, Breite, Höhe) |
|---|---|---|---|
| `iphone-16-plus.png` | iPhone 16 Plus (schwarz) | 1470×2970 | 90, 87, 1290, 2796 |
| `macbook-air-13.png` | MacBook Air 13" (M5), Midnight | 3400×2240 | 420, 288, 2560, 1664 |
| `ipad-pro-11-quer.png` | iPad Pro 11" (M5), Space Black, quer | 2640×1880 | 110, 106, 2420, 1668 |

Das Bildschirmrechteck ist die transparente Fläche des Bezels. Ein anderes Bild
unter demselben Namen bricht den Lauf ab, weil die Maße nicht mehr stimmen.

Neuen Bezel ergänzen: Datei unverändert hierher kopieren, Maße und
Bildschirmrechteck in `devices.ts` eintragen, den Pipeline-Test laufen lassen
(`pnpm run test:screenshots`).

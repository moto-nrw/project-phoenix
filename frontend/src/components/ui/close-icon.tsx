import { X } from "lucide-react";

/**
 * Das Schließen-X aller Overlays (#5531): steht im Kit-Knopf `ghost` `icon`.
 * Bewusst lucide: Der Toast im Wurzel-Layout hält lucide damit im geteilten
 * Chunk. Ein eigenes SVG ließ Turbopack lucide und `next/image` für die
 * NFC-Kurzanleitung in einen zweiten Seiten-Chunk legen (+7 kB, Bundle-Ratchet).
 */
export function CloseIcon() {
  return <X className="h-5 w-5" strokeWidth={2} aria-hidden="true" />;
}

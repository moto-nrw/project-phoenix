export type ButtonVariant =
  | "primary"
  | "outline"
  | "outline_danger"
  | "danger"
  | "warning"
  | "success"
  | "surface"
  | "surface_success"
  | "ghost"
  | "ghost_danger"
  | "ghost_tone"
  | "link"
  | "link_tone"
  | "fab"
  | "send"
  | "ghost_round"
  | "dashed"
  | "filter"
  | "segment"
  | "disclosure"
  | "day_chip"
  | "day_chip_stacked"
  | "count_badge";
export type ButtonSize =
  "sm" | "md" | "field" | "base" | "lg" | "xl" | "compact" | "icon" | "card";

// Ruhiger Knopf ohne Fläche; `segment` und `disclosure` bauen darauf auf.
const GHOST =
  "bg-transparent text-gray-600 hover:bg-gray-100 hover:text-gray-900 active:bg-gray-200";

export function buttonStyles({
  variant,
  size,
  className,
  touchTarget = false,
}: Readonly<{
  variant: ButtonVariant;
  size: ButtonSize;
  className: string;
  touchTarget?: boolean;
}>) {
  const textSizes: Record<ButtonSize, string> = {
    sm: "text-sm",
    md: "text-sm",
    field: "text-sm",
    base: "text-base",
    lg: "text-lg",
    xl: "text-xl",
    compact: "text-xs",
    icon: "text-xs",
    card: "text-sm",
  };
  const sizeStyles: Record<ButtonSize, string> = {
    sm: "rounded-lg px-5 py-3",
    md: "rounded-lg px-4 py-2",
    // 40 px wie `Input controlSize="compact"`: Auth-Formulare und Zeilen, in
    // denen der Knopf neben einem 40-px-Feld steht.
    field: "h-10 rounded-lg px-4",
    base: "rounded-lg px-5 py-3",
    lg: "rounded-lg px-5 py-3",
    xl: "rounded-lg px-5 py-3",
    compact:
      "h-8 gap-1.5 rounded-md px-2.5 disabled:cursor-not-allowed aria-disabled:cursor-not-allowed",
    // `aspect-square` statt `w-8`: erzwingt die Kopfkarte ihre Bedienhöhe
    // (40/44 px), folgt die Breite — sonst wird der quadratische Knopf oval.
    icon: "aspect-square h-8 rounded-md pointer-coarse:min-w-11 disabled:cursor-not-allowed aria-disabled:cursor-not-allowed",
    card: "w-full rounded-2xl p-4",
  };
  const variantStyles: Record<ButtonVariant, string> = {
    primary:
      "bg-gray-900 text-white shadow-md hover:bg-gray-700 hover:shadow-lg",
    outline:
      "bg-transparent text-gray-700 ring-1 ring-gray-300 shadow-md hover:bg-gray-50 hover:ring-gray-400",
    outline_danger:
      "bg-moto-red-soft text-moto-red-strong ring-moto-red/30 shadow-md ring-1 hover:bg-moto-red/20 hover:ring-moto-red/50",
    danger:
      "bg-moto-red text-white shadow-md hover:bg-moto-red-strong hover:shadow-lg",
    // Bestätigung mit Nebenwirkung, die kein Löschen ist (Schließtag trotzdem
    // planen, Einzelanpassungen verwerfen): Orange statt Rot. Dunkler Text:
    // Weiß auf moto-orange erreicht nur 2,4:1 und verfehlt AA.
    warning:
      "bg-moto-orange text-gray-950 shadow-md hover:bg-moto-orange-hover hover:shadow-lg",
    // Weiß auf Grün (#5141) braucht die dunklen Füllstufen: moto-green selbst
    // trägt Weiß nur mit 1,96:1. fill 4,84:1, fill-hover 6,0:1, fill-active 7,0:1.
    success:
      "bg-moto-green-fill text-white shadow-md hover:bg-moto-green-fill-hover hover:shadow-lg active:bg-moto-green-fill-active",
    surface:
      "bg-white text-gray-700 ring-1 ring-gray-200 shadow-sm hover:bg-gray-50 hover:ring-gray-300 active:bg-gray-100",
    // Erfolgsrückmeldung in einer sonst neutralen Werkzeugleiste, etwa nach
    // dem Kopieren eines Links. Sie bleibt eine helle Fläche statt einer CTA.
    surface_success:
      "bg-moto-green/10 text-moto-green-strong ring-1 ring-moto-green/40 shadow-sm hover:bg-moto-green/15 hover:ring-moto-green/50 active:bg-moto-green/20",
    ghost: GHOST,
    // Leiser roter Knopf: Entfernen in einer Zeile oder einem Formular, wo
    // ein gefüllter roter Knopf zu laut wäre.
    ghost_danger:
      "bg-transparent text-moto-red-strong hover:bg-moto-red-soft active:bg-moto-red/20",
    // Leiser Knopf auf einer farbigen Meldung (Toast, Fehlerkasten): nimmt die
    // Textfarbe der Fläche an, rot auf Fehler, grün auf Erfolg.
    ghost_tone:
      "bg-transparent text-current hover:bg-black/5 active:bg-black/10",
    // Textknopf („Passwort vergessen?“): kein Polster, kein Schatten, aber
    // derselbe sichtbare Fokusring wie jeder Kit-Knopf.
    link: "h-auto rounded-sm bg-transparent px-0 py-0 text-gray-600 underline-offset-2 shadow-none hover:text-gray-900 hover:underline",
    // Schwebender Anlegen-Knopf: rund, 56 px, grün wie `success`. Bringt sein
    // Maß selbst mit; die Größe des Aufrufs spielt keine Rolle.
    fab: "size-14 rounded-full p-0 bg-moto-green-fill text-white shadow-lg hover:bg-moto-green-fill-hover hover:shadow-xl active:bg-moto-green-fill-active",
    // Senden neben dem Chatfeld (`MessageComposer`): rund, dunkel, ohne
    // Schatten, so hoch wie das einzeilige Feld daneben (40 px, per Finger
    // 48 px). Bringt sein Maß selbst mit.
    send: "size-10 rounded-full p-0 bg-gray-900 text-white hover:bg-gray-700 active:bg-gray-800 pointer-coarse:size-12",
    // `ghost` als Kreis: Symbolknopf in einem runden Feld (Büroklammer im
    // Chatfeld), damit die Hover-Fläche dem Rund des Feldes folgt.
    ghost_round: `${GHOST} rounded-full`,
    // Gestrichelte Hinzufügen-Fläche („Weitere Nummer hinzufügen“). Ruht
    // ohne Füllung: ein grau gefüllter Knopf passt in keine Rangstufe (#5574).
    dashed:
      "border border-dashed border-gray-300 bg-transparent text-gray-600 hover:border-gray-400 hover:bg-gray-50 hover:text-gray-700",
    // Unterstrichener Textknopf auf einer farbigen Meldung (Toast,
    // Fehlerkasten, #6182): Farbe wie `ghost_tone`, das Polster bleibt, damit
    // die Hover-Tönung den Text umrahmt. Wächst mit dem Text, damit eine lange
    // Vorgangskennung in der Meldung umbrechen kann.
    link_tone:
      "h-auto min-h-8 px-2.5 py-1 bg-transparent text-current underline underline-offset-2 hover:bg-black/5 active:bg-black/10",
    // Filterknopf im Seitenkopf (#6182): runde 40-px-Fläche, blau solange das
    // Filterfeld offen ist (`aria-expanded`), blauer Ring bei aktiven Filtern
    // (`data-filters-active`). Bringt sein Maß selbst mit.
    filter:
      // oxlint-disable-next-line ui-kit/no-hand-rolled-surface -- runde Bedienfläche des Filterknopfs, keine Karte
      "h-10 rounded-2xl border border-gray-200 bg-white text-gray-600 hover:bg-gray-50 aria-expanded:border-moto-blue aria-expanded:bg-moto-blue aria-expanded:text-white aria-expanded:hover:bg-moto-blue data-filters-active:ring-2 data-filters-active:ring-moto-blue data-filters-active:ring-offset-1",
    // Teil einer gerahmten Knopfgruppe (Zeitnavigation der Planungsleiste,
    // #6182): kein eigener Radius, füllt die Höhe der Gruppe, sonst `ghost`.
    // Ein Symbol-Segment bleibt quadratisch: mit Polster und Trennlinie würde
    // es 1 px breiter als hoch.
    segment: `h-full rounded-none px-3 data-icon-only:px-0 ${GHOST}`,
    // Einklapp-Knopf im Kopf einer Karte (#6182): wie `ghost`, tönt aber
    // auch, wenn die Maus über dem übrigen Kopf liegt, weil der ganze Kopf
    // schaltet (`SectionCollapseHead`, #5021).
    disclosure: `${GHOST} group-hover/collapse-head:bg-gray-100 group-hover/collapse-head:text-gray-900`,
    // Tag einer Wochenleiste (`PlanningDayChip`, #6182): einzeilig, 36 px
    // (per Finger 44 px wie jeder Kit-Knopf).
    // Der gewählte Tag (`aria-pressed`) ist dunkel gefüllt.
    day_chip:
      "h-9 gap-1 rounded-md px-2.5 py-0 text-sm font-normal bg-transparent text-gray-600 hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-gray-900 aria-pressed:bg-gray-900 aria-pressed:text-white aria-pressed:hover:bg-gray-900 aria-pressed:hover:text-white",
    // Derselbe Tag zweizeilig im Tagesstreifen am Telefon (`PlanningDayChip
    // stacked`): Kürzel, Datum und Zustand untereinander.
    day_chip_stacked:
      "h-auto flex-col justify-start gap-0.5 rounded-lg px-1 py-1.5 text-sm font-normal bg-transparent text-gray-600 hover:bg-gray-100 aria-pressed:bg-gray-900 aria-pressed:text-white aria-pressed:hover:bg-gray-900",
    // Zahl an der Ecke einer Kachel, die etwas öffnet (#6486): die Fläche ist
    // durchsichtig, die sichtbare Zahl ist ein `NotificationBadge` im Knopf.
    // 32 px Trefferfläche, per Finger 44 px. Bringt sein Maß selbst mit.
    count_badge:
      "h-8 min-w-8 rounded-full p-0 bg-transparent shadow-none pointer-coarse:min-w-11 hover:opacity-90 focus-visible:ring-moto-blue",
  };
  // Fingerregel (#5531): Bei Bedienung per Finger ist jeder Kit-Knopf
  // mindestens 44 px hoch (Apple HIG 44 pt), `icon` auch 44 px breit. Mit Maus
  // bleibt es bei den Kit-Maßen. Kein Knopf braucht dafür ein eigenes
  // `min-h-11`. Kein `transform` im Übergang: das Kit skaliert nicht.
  const baseStyles =
    "inline-flex items-center justify-center font-medium transition-[background-color,box-shadow,color,opacity] duration-200 pointer-coarse:min-h-11 focus-visible:ring-2 focus-visible:ring-gray-600 focus-visible:outline-none disabled:opacity-50 aria-disabled:opacity-50";

  // Die Variante steht hinter der Größe: Varianten mit eigenem Maß (`fab`,
  // `filter`, `day_chip` …) setzen sich so auch bei der Schriftgröße durch.
  return [
    baseStyles,
    sizeStyles[size],
    textSizes[size],
    variantStyles[variant],
    // Ausdrückliche 44-px-Zielgröße, wenn eine Ansicht sie unabhängig vom
    // Zeiger verlangt (Toast auf schmalen Ansichten). Ein Textknopf behält
    // seine Breite.
    touchTarget && (size === "icon" ? "h-11 w-11" : "h-11"),
    className,
  ]
    .filter(Boolean)
    .join(" ");
}

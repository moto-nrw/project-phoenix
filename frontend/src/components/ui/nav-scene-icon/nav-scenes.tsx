import type { ReactNode } from "react";

/**
 * Eine Szene: das nachgezeichnete Icon im 256er-Raster, zerlegt in Teile,
 * die sich beim Überfahren einzeln bewegen. Die Bewegung steht in
 * `src/styles/nav-scenes.css` unter denselben `ns-*`-Klassen.
 */
export interface NavScene {
  /** Was die Szene zeigt, in einem halben Satz. */
  readonly description: string;
  /** Gefülltes Original-Glyph statt Kontur (nur das Zahnrad). */
  readonly filled?: boolean;
  /** Die Teile des Icons. `uid` macht `clipPath`-IDs pro Instanz eindeutig. */
  readonly render: (uid: string) => ReactNode;
}

/**
 * Szenen je Phosphor-Glyph, Schlüssel ist dessen `displayName`. Ein
 * Konzept-Icon in der Seitenleiste ohne Eintrag hier lässt
 * `nav-scene-icon.test.ts` scheitern. Neue Szenen: README.md daneben.
 */
export const NAV_SCENES: Readonly<Record<string, NavScene>> = {
  HouseLineIcon: {
    description: "Dach hüpft, Tür geht auf",
    render: () => (
      <>
        <path className="ns-door" d="M104,216V152h48v64" />
        <path d="M40,116V216M216,116V216M16,216H240" />
        <polyline className="ns-roof" points="24,132 128,28 232,132" />
      </>
    ),
  },
  CalendarDotsIcon: {
    description: "Termine poppen nacheinander auf",
    render: () => (
      <>
        <rect x="40" y="40" width="176" height="176" rx="8" />
        <path d="M40,88H216" />
        <path className="ns-rings" d="M176,24V56M80,24V56" />
        <circle
          className="ns-solid ns-pop ns-dot ns-i0"
          cx="128"
          cy="132"
          r="12"
        />
        <circle
          className="ns-solid ns-pop ns-dot ns-i1"
          cx="172"
          cy="132"
          r="12"
        />
        <circle
          className="ns-solid ns-pop ns-dot ns-i2"
          cx="84"
          cy="172"
          r="12"
        />
        <circle
          className="ns-solid ns-pop ns-dot ns-i3"
          cx="128"
          cy="172"
          r="12"
        />
        <circle
          className="ns-solid ns-pop ns-dot ns-i4"
          cx="172"
          cy="172"
          r="12"
        />
      </>
    ),
  },
  UserIcon: {
    description: "schaut sich um, zuckt mit den Schultern",
    render: () => (
      <>
        <path
          className="ns-body"
          d="M32,216c19.37-33.47,54.55-56,96-56s76.63,22.53,96,56"
        />
        <circle className="ns-head" cx="128" cy="96" r="60" />
      </>
    ),
  },
  UsersThreeIcon: {
    description: "La-Ola von links nach rechts",
    render: () => (
      <>
        <g className="ns-wave ns-i0">
          <circle cx="60" cy="84" r="26" />
          <path d="M14,148c10-18,26-28,46-28" />
        </g>
        <g className="ns-wave ns-i1">
          <circle cx="128" cy="136" r="36" />
          <path d="M70,220c12-24,32-38,58-38s46,14,58,38" />
        </g>
        <g className="ns-wave ns-i2">
          <circle cx="196" cy="84" r="26" />
          <path d="M242,148c-10-18-26-28-46-28" />
        </g>
      </>
    ),
  },
  ListChecksIcon: {
    description: "Haken werden nacheinander gesetzt",
    render: () => (
      <>
        <path className="ns-line ns-i0" d="M128,64h88" />
        <polyline
          className="ns-draw ns-i0"
          pathLength="1"
          points="40,64 56,80 88,48"
        />
        <path className="ns-line ns-i1" d="M128,128h88" />
        <polyline
          className="ns-draw ns-i1"
          pathLength="1"
          points="40,128 56,144 88,112"
        />
        <path className="ns-line ns-i2" d="M128,192h88" />
        <polyline
          className="ns-draw ns-i2"
          pathLength="1"
          points="40,192 56,208 88,176"
        />
      </>
    ),
  },
  DoorIcon: {
    description: "Tür schwingt auf, dahinter wird es dunkel",
    render: () => (
      <>
        <rect
          className="ns-solid ns-ghost ns-dark"
          x="56"
          y="32"
          width="144"
          height="184"
        />
        <path d="M24,216H232" />
        <g className="ns-leaf">
          <path d="M56,216V40a8,8,0,0,1,8-8H192a8,8,0,0,1,8,8V216" />
          <circle className="ns-solid" cx="164" cy="132" r="12" />
        </g>
      </>
    ),
  },
  CalendarIcon: {
    description: "der nächste Tag rollt hoch",
    render: (uid) => (
      <>
        <defs>
          <clipPath id={uid}>
            <rect x="48" y="96" width="160" height="112" />
          </clipPath>
        </defs>
        <rect x="40" y="40" width="176" height="176" rx="8" />
        <path d="M40,88H216" />
        <path className="ns-rings2 ns-i0" d="M80,24V56" />
        <path className="ns-rings2 ns-i1" d="M176,24V56" />
        <g clipPath={`url(#${uid})`}>
          <path
            className="ns-day"
            d="M96,128l16-8v64M140,128a20,20,0,1,1,32,16l-32,40h36"
          />
        </g>
      </>
    ),
  },
  ClockIcon: {
    description: "Zeiger drehen eine Runde",
    render: () => (
      <>
        <circle cx="128" cy="128" r="96" />
        <path className="ns-hour" d="M128,128H176" />
        <path className="ns-min" d="M128,128V68" />
      </>
    ),
  },
  TrayIcon: {
    description: "ein Brief fällt in die Ablage",
    render: () => (
      <>
        <g className="ns-tray">
          <rect x="32" y="48" width="192" height="168" rx="16" />
        </g>
        <rect
          className="ns-ghost ns-mail ns-mask"
          x="92"
          y="-20"
          width="72"
          height="52"
          rx="6"
        />
        <g className="ns-tray">
          <path
            className="ns-mask"
            stroke="none"
            d="M40,160H80l20,20h56l20-20h40v40a8,8,0,0,1-8,8H48a8,8,0,0,1-8-8Z"
          />
          <path d="M32,152H80l20,20h56l20-20h48" />
        </g>
      </>
    ),
  },
  ChatsCircleIcon: {
    description: "hin und her, dann tippt jemand",
    render: () => (
      <>
        <path
          className="ns-b1"
          d="M100,28a68,68,0,0,0-60,100L28,164l38-10A68,68,0,1,0,100,28Z"
        />
        <g className="ns-b2">
          <path
            className="ns-mask"
            d="M168,100a62,62,0,0,1,54,92l10,34-34-10a62,62,0,1,1-30-116Z"
          />
          <circle
            className="ns-solid ns-ghost ns-typ ns-i0"
            cx="144"
            cy="162"
            r="10"
          />
          <circle
            className="ns-solid ns-ghost ns-typ ns-i1"
            cx="168"
            cy="162"
            r="10"
          />
          <circle
            className="ns-solid ns-ghost ns-typ ns-i2"
            cx="192"
            cy="162"
            r="10"
          />
        </g>
      </>
    ),
  },
  ShieldWarningIcon: {
    description: "Schild wackelt, Ausrufezeichen fällt rein",
    render: () => (
      <>
        <g className="ns-shield">
          <path d="M40,56a16,16,0,0,1,16-16H200a16,16,0,0,1,16,16v56c0,84-88,112-88,112S40,196,40,112Z" />
          <path className="ns-bang" d="M128,88v44" />
          <circle
            className="ns-solid ns-pop ns-bdot"
            cx="128"
            cy="172"
            r="12"
          />
        </g>
      </>
    ),
  },
  BookOpenIcon: {
    description: "eine Seite blättert um",
    render: () => (
      <>
        <path d="M128,88a32,32,0,0,0-32-32H24V200H96a32,32,0,0,1,32,32" />
        <path d="M128,88a32,32,0,0,1,32-32h72V200H160a32,32,0,0,0-32,32" />
        <path d="M128,88V232" />
        <path
          className="ns-ghost ns-mask ns-page"
          d="M128,88a32,32,0,0,1,32-32h72V200H160a32,32,0,0,0-32,32Z"
        />
      </>
    ),
  },
  GearIcon: {
    description: "holt aus und dreht weiter",
    filled: true,
    render: () => (
      <>
        <path
          className="ns-gear"
          d="M128,80a48,48,0,1,0,48,48A48.05,48.05,0,0,0,128,80Zm0,80a32,32,0,1,1,32-32A32,32,0,0,1,128,160Zm88-29.84q.06-2.16,0-4.32l14.92-18.64a8,8,0,0,0,1.48-7.06,107.21,107.21,0,0,0-10.88-26.25,8,8,0,0,0-6-3.93l-23.72-2.64q-1.480-1.560-3-3L186,40.54a8,8,0,0,0-3.94-6,107.71,107.71,0,0,0-26.25-10.87,8,8,0,0,0-7.06,1.49L130.16,40Q128,40,125.84,40L107.2,25.11a8,8,0,0,0-7.06-1.48A107.6,107.6,0,0,0,73.89,34.51a8,8,0,0,0-3.93,6L67.32,64.27q-1.56,1.49-3,3L40.54,70a8,8,0,0,0-6,3.94,107.71,107.71,0,0,0-10.87,26.25,8,8,0,0,0,1.49,7.06L40,125.84Q40,128,40,130.16L25.11,148.8a8,8,0,0,0-1.48,7.06,107.21,107.21,0,0,0,10.88,26.25,8,8,0,0,0,6,3.93l23.72,2.64q1.49,1.56,3,3L70,215.46a8,8,0,0,0,3.94,6,107.71,107.71,0,0,0,26.25,10.87,8,8,0,0,0,7.06-1.49L125.84,216q2.16.06,4.32,0l18.64,14.92a8,8,0,0,0,7.06,1.48,107.21,107.21,0,0,0,26.25-10.88,8,8,0,0,0,3.93-6l2.64-23.72q1.56-1.48,3-3L215.46,186a8,8,0,0,0,6-3.94,107.71,107.71,0,0,0,10.87-26.25,8,8,0,0,0-1.49-7.06Zm-16.1-6.5a73.93,73.93,0,0,1,0,8.68,8,8,0,0,0,1.74,5.48l14.19,17.73a91.57,91.57,0,0,1-6.23,15L187,173.11a8,8,0,0,0-5.1,2.64,74.11,74.11,0,0,1-6.14,6.14,8,8,0,0,0-2.64,5.1l-2.51,22.58a91.32,91.32,0,0,1-15,6.23l-17.74-14.19a8,8,0,0,0-5-1.75h-.48a73.93,73.93,0,0,1-8.68,0,8,8,0,0,0-5.48,1.74L100.45,215.8a91.57,91.57,0,0,1-15-6.23L82.89,187a8,8,0,0,0-2.64-5.1,74.11,74.11,0,0,1-6.14-6.14,8,8,0,0,0-5.1-2.64L46.43,170.6a91.32,91.32,0,0,1-6.23-15l14.19-17.74a8,8,0,0,0,1.74-5.48,73.93,73.93,0,0,1,0-8.68,8,8,0,0,0-1.74-5.48L40.2,100.45a91.57,91.57,0,0,1,6.23-15L69,82.89a8,8,0,0,0,5.1-2.64,74.11,74.11,0,0,1,6.14-6.14A8,8,0,0,0,82.89,69L85.4,46.43a91.32,91.32,0,0,1,15-6.23l17.74,14.19a8,8,0,0,0,5.48,1.74,73.93,73.93,0,0,1,8.68,0,8,8,0,0,0,5.48-1.74L155.55,40.2a91.57,91.57,0,0,1,15,6.23L173.11,69a8,8,0,0,0,2.64,5.1,74.11,74.11,0,0,1,6.14,6.14,8,8,0,0,0,5.1,2.64l22.58,2.51a91.32,91.32,0,0,1,6.23,15l-14.19,17.74A8,8,0,0,0,199.87,123.66Z"
        />
      </>
    ),
  },
  AddressBookIcon: {
    description: "blättert zum nächsten Kontakt",
    render: (uid) => (
      <>
        <defs>
          <clipPath id={uid}>
            <rect x="64" y="40" width="144" height="176" />
          </clipPath>
        </defs>
        <rect x="56" y="32" width="160" height="192" rx="8" />
        <path className="ns-tab ns-i0" d="M32,80H56" />
        <path className="ns-tab ns-i1" d="M32,128H56" />
        <path className="ns-tab ns-i2" d="M32,176H56" />
        <g clipPath={`url(#${uid})`}>
          <g className="ns-contact">
            <circle cx="136" cy="108" r="28" />
            <path d="M92,176c10-20,26-32,44-32s34,12,44,32" />
          </g>
        </g>
      </>
    ),
  },
  ArrowsLeftRightIcon: {
    description: "zwei Pfeile tauschen aus",
    render: () => (
      <>
        <g className="ns-pushR">
          <path d="M40,88H216" />
          <polyline points="176,48 216,88 176,128" />
        </g>
        <g className="ns-pushL">
          <path d="M216,168H40" />
          <polyline points="80,128 40,168 80,208" />
        </g>
      </>
    ),
  },
  BriefcaseIcon: {
    description: "wird am Griff hochgehoben und pendelt",
    render: () => (
      <>
        <g className="ns-lift">
          <rect x="32" y="72" width="192" height="144" rx="8" />
          <path d="M88,72V56a16,16,0,0,1,16-16h48a16,16,0,0,1,16,16V72" />
          <path d="M32,128H224" />
          <rect
            className="ns-solid ns-pop ns-clasp"
            x="112"
            y="116"
            width="32"
            height="24"
            rx="4"
          />
        </g>
      </>
    ),
  },
  BuildingsIcon: {
    description: "die Lichter gehen nacheinander an",
    render: () => (
      <>
        <path d="M16,216H240" />
        <path d="M40,216V48a8,8,0,0,1,8-8h72a8,8,0,0,1,8,8V216" />
        <path className="ns-grow" d="M128,96h80a8,8,0,0,1,8,8V216" />
        <rect
          className="ns-solid ns-pop ns-win ns-i0"
          x="56"
          y="72"
          width="16"
          height="16"
          rx="3"
        />
        <rect
          className="ns-solid ns-pop ns-win ns-i1"
          x="88"
          y="72"
          width="16"
          height="16"
          rx="3"
        />
        <rect
          className="ns-solid ns-pop ns-win ns-i2"
          x="56"
          y="112"
          width="16"
          height="16"
          rx="3"
        />
        <rect
          className="ns-solid ns-pop ns-win ns-i3"
          x="88"
          y="112"
          width="16"
          height="16"
          rx="3"
        />
        <rect
          className="ns-solid ns-pop ns-win ns-i4"
          x="56"
          y="152"
          width="16"
          height="16"
          rx="3"
        />
        <rect
          className="ns-solid ns-pop ns-win ns-i5"
          x="88"
          y="152"
          width="16"
          height="16"
          rx="3"
        />
        <rect
          className="ns-solid ns-pop ns-win ns-i6"
          x="152"
          y="128"
          width="16"
          height="16"
          rx="3"
        />
        <rect
          className="ns-solid ns-pop ns-win ns-i7"
          x="176"
          y="128"
          width="16"
          height="16"
          rx="3"
        />
        <rect
          className="ns-solid ns-pop ns-win ns-i8"
          x="152"
          y="168"
          width="16"
          height="16"
          rx="3"
        />
        <rect
          className="ns-solid ns-pop ns-win ns-i9"
          x="176"
          y="168"
          width="16"
          height="16"
          rx="3"
        />
      </>
    ),
  },
  BuildingOfficeIcon: {
    description: "Fenster gehen an, Tür geht auf",
    render: () => (
      <>
        <path d="M16,216H240" />
        <path d="M48,216V48a8,8,0,0,1,8-8H200a8,8,0,0,1,8,8V216" />
        <path className="ns-door2" d="M112,216V168h32v48" />
        <rect
          className="ns-solid ns-pop ns-win ns-i0"
          x="86"
          y="72"
          width="20"
          height="16"
          rx="3"
        />
        <rect
          className="ns-solid ns-pop ns-win ns-i1"
          x="150"
          y="72"
          width="20"
          height="16"
          rx="3"
        />
        <rect
          className="ns-solid ns-pop ns-win ns-i2"
          x="86"
          y="116"
          width="20"
          height="16"
          rx="3"
        />
        <rect
          className="ns-solid ns-pop ns-win ns-i3"
          x="150"
          y="116"
          width="20"
          height="16"
          rx="3"
        />
      </>
    ),
  },
  CalendarBlankIcon: {
    description: "ein Blatt wird abgerissen",
    render: () => (
      <>
        <rect x="40" y="40" width="176" height="176" rx="8" />
        <path d="M40,88H216" />
        <path className="ns-rings2 ns-i0" d="M80,24V56" />
        <path className="ns-rings2 ns-i1" d="M176,24V56" />
        <path
          className="ns-ghost ns-mask ns-sheet"
          d="M40,88H216V208a8,8,0,0,1-8,8H48a8,8,0,0,1-8-8Z"
        />
      </>
    ),
  },
  CalendarCheckIcon: {
    description: "Ringe hüpfen, der Haken wird gesetzt",
    render: () => (
      <>
        <rect x="40" y="40" width="176" height="176" rx="8" />
        <path d="M40,88H216" />
        <path className="ns-rings2 ns-i0" d="M80,24V56" />
        <path className="ns-rings2 ns-i1" d="M176,24V56" />
        <polyline
          className="ns-draw ns-i1"
          pathLength="1"
          points="92,152 116,176 164,128"
        />
      </>
    ),
  },
  CalendarPlusIcon: {
    description: "das Plus dreht sich herein",
    render: () => (
      <>
        <rect x="40" y="40" width="176" height="176" rx="8" />
        <path d="M40,88H216" />
        <path className="ns-rings" d="M176,24V56M80,24V56" />
        <path className="ns-spinIn" d="M128,128v64M96,160h64" />
      </>
    ),
  },
  ChalkboardTeacherIcon: {
    description: "an der Tafel wird geschrieben",
    render: () => (
      <>
        <path d="M112,184H208a8,8,0,0,0,8-8V56a8,8,0,0,0-8-8H48a8,8,0,0,0-8,8V112" />
        <g className="ns-nod">
          <circle cx="64" cy="152" r="24" />
        </g>
        <path d="M16,216c8-20,26-32,48-32s40,12,48,32" />
        <path className="ns-draw ns-i0" pathLength="1" d="M120,88h64" />
        <path className="ns-draw ns-i1" pathLength="1" d="M120,128h40" />
      </>
    ),
  },
  ChatCircleTextIcon: {
    description: "Blase ploppt auf, Text wird geschrieben",
    render: () => (
      <>
        <path
          className="ns-bubble"
          d="M128,32a96,96,0,0,0-84,142.5L32,224l49.5-12A96,96,0,1,0,128,32Z"
        />
        <path className="ns-draw ns-i0" pathLength="1" d="M88,112h80" />
        <path className="ns-draw ns-i1" pathLength="1" d="M88,148h52" />
      </>
    ),
  },
  ClipboardTextIcon: {
    description: "Klemme schnappt, Zeilen werden geschrieben",
    render: () => (
      <>
        <rect x="48" y="40" width="160" height="184" rx="8" />
        <path
          className="ns-mask ns-clip"
          d="M88,64V40a8,8,0,0,1,8-8h64a8,8,0,0,1,8,8V64Z"
        />
        <path className="ns-draw ns-i0" pathLength="1" d="M96,120h64" />
        <path className="ns-draw ns-i1" pathLength="1" d="M96,160h64" />
      </>
    ),
  },
  ClockCounterClockwiseIcon: {
    description: "spult die Zeit zurück",
    render: () => (
      <>
        <g className="ns-rewind">
          <path d="M67.6,192A88,88,0,1,0,65.8,62.2L32,96" />
          <polyline points="32,56 32,96 72,96" />
        </g>
        <path className="ns-back" d="M128,80v48l40,24" />
      </>
    ),
  },
  CoinsIcon: {
    description: "die hintere Münze wird geworfen",
    render: () => (
      <>
        <g className="ns-toss">
          <ellipse cx="104" cy="76" rx="64" ry="24" />
          <path d="M40,76v28c0,13.25,28.65,24,64,24s64-10.75,64-24V76" />
        </g>
        <path
          className="ns-mask"
          stroke="none"
          d="M88,152c0-13.25,28.65-24,64-24s64,10.75,64,24v32c0,13.25-28.65,24-64,24s-64-10.75-64-24Z"
        />
        <ellipse cx="152" cy="152" rx="64" ry="24" />
        <path d="M88,152v32c0,13.25,28.65,24,64,24s64-10.75,64-24V152" />
      </>
    ),
  },
  ContactlessPaymentIcon: {
    description: "das Funksignal breitet sich aus",
    render: () => (
      <>
        <circle cx="128" cy="128" r="96" />
        <path className="ns-sig ns-i0" d="M100,104a32,32,0,0,1,0,48" />
        <path className="ns-sig ns-i1" d="M128,88a56,56,0,0,1,0,80" />
        <path className="ns-sig ns-i2" d="M156,72a80,80,0,0,1,0,112" />
      </>
    ),
  },
  DatabaseIcon: {
    description: "die Schichten federn nacheinander hoch",
    render: () => (
      <>
        <path
          className="ns-layer ns-i2"
          d="M40,128v64c0,17.67,39.4,32,88,32s88-14.33,88-32V128"
        />
        <path
          className="ns-layer ns-i1"
          d="M40,64v64c0,17.67,39.4,32,88,32s88-14.33,88-32V64"
        />
        <ellipse
          className="ns-layer ns-mask ns-i0"
          cx="128"
          cy="64"
          rx="88"
          ry="32"
        />
      </>
    ),
  },
  DevicesIcon: {
    description: "das Handy hüpft heraus und zurück",
    render: () => (
      <>
        <rect x="24" y="48" width="160" height="112" rx="8" />
        <path d="M80,200h48M104,160v40" />
        <g className="ns-phone">
          <rect
            className="ns-mask"
            x="168"
            y="96"
            width="64"
            height="112"
            rx="10"
          />
          <path className="ns-solid" stroke="none" d="M192,184h16v12h-16z" />
        </g>
      </>
    ),
  },
  DotsThreeIcon: {
    description: "die Punkte springen nacheinander",
    render: () => (
      <>
        <circle className="ns-solid ns-jump ns-i0" cx="60" cy="128" r="16" />
        <circle className="ns-solid ns-jump ns-i1" cx="128" cy="128" r="16" />
        <circle className="ns-solid ns-jump ns-i2" cx="196" cy="128" r="16" />
      </>
    ),
  },
  EnvelopeIcon: {
    description: "fliegt weg, ein neuer kommt",
    render: () => (
      <>
        <g className="ns-fly">
          <rect
            className="ns-mask"
            x="32"
            y="56"
            width="192"
            height="144"
            rx="8"
          />
          <polyline points="32,64 128,144 224,64" />
        </g>
      </>
    ),
  },
  EnvelopeOpenIcon: {
    description: "ein Brief schaut heraus",
    render: () => (
      <>
        <path d="M32,96L128,32l96,64" />
        {/* Blatt und Zeile steigen zusammen; die Zeile zeichnet sich dabei. */}
        <g className="ns-peek">
          <rect
            className="ns-mask"
            x="72"
            y="64"
            width="112"
            height="88"
            rx="4"
          />
          <path className="ns-draw ns-i1" pathLength="1" d="M96,92h64" />
        </g>
        <path
          className="ns-mask"
          stroke="none"
          d="M32,96l96,64,96-64V200H32Z"
        />
        <path d="M32,96V200a8,8,0,0,0,8,8H216a8,8,0,0,0,8-8V96" />
        <path d="M32,96l96,64,96-64" />
      </>
    ),
  },
  EyeIcon: {
    description: "blinzelt und schaut sich um",
    render: () => (
      <>
        <g className="ns-blink">
          <path d="M128,56C48,56,16,128,16,128s32,72,112,72,112-72,112-72S208,56,128,56Z" />
          <circle className="ns-pupil" cx="128" cy="128" r="36" />
        </g>
      </>
    ),
  },
  FileTextIcon: {
    description: "Ecke klappt, Zeilen werden geschrieben",
    render: () => (
      <>
        <path d="M200,224H56a8,8,0,0,1-8-8V40a8,8,0,0,1,8-8h96l56,56V216A8,8,0,0,1,200,224Z" />
        <path className="ns-fold" d="M152,32V88h56" />
        <path className="ns-draw ns-i0" pathLength="1" d="M96,136h64" />
        <path className="ns-draw ns-i1" pathLength="1" d="M96,168h64" />
      </>
    ),
  },
  FolderOpenIcon: {
    description: "Mappe klappt auf, ein Blatt schaut heraus",
    render: () => (
      <>
        <path d="M32,208V64a8,8,0,0,1,8-8H93.33a8,8,0,0,1,4.8,1.6L128,80h72a8,8,0,0,1,8,8v24" />
        <rect
          className="ns-peek2 ns-mask"
          x="72"
          y="84"
          width="112"
          height="72"
          rx="4"
        />
        <path
          className="ns-flap ns-mask"
          d="M32,208l29.4-88.2A8,8,0,0,1,69,114.4H222.9a8,8,0,0,1,7.6,10.5L203,208Z"
        />
      </>
    ),
  },
  ForkKnifeIcon: {
    description: "Messer und Gabel stoßen an",
    render: () => (
      <>
        <g className="ns-fork">
          <path d="M64,32v56a32,32,0,0,0,64,0V32" />
          <path d="M96,32V224" />
        </g>
        <g className="ns-knife">
          <path d="M200,160H152s0-112,48-128V224" />
        </g>
      </>
    ),
  },
  IdentificationCardIcon: {
    description: "der Ausweis dreht sich einmal",
    render: () => (
      <>
        <g className="ns-flip">
          <rect x="24" y="40" width="208" height="176" rx="16" />
          <circle cx="96" cy="116" r="24" />
          <path d="M64,176c6-18,18-28,32-28s26,10,32,28" />
          <path d="M152,112h48M152,144h48" />
        </g>
      </>
    ),
  },
  InvoiceIcon: {
    description: "der Beleg wird gedruckt",
    render: () => (
      <>
        <g className="ns-print">
          <path d="M40,32H216V216l-22-12-22,12-22-12-22,12-22-12-22,12-22-12-22,12Z" />
        </g>
        <path className="ns-draw ns-i0" pathLength="1" d="M80,80h96" />
        <path className="ns-draw ns-i1" pathLength="1" d="M80,120h96" />
        <path className="ns-draw ns-i2" pathLength="1" d="M80,160h48" />
      </>
    ),
  },
  ListBulletsIcon: {
    description: "Punkte poppen, Zeilen fahren aus",
    render: () => (
      <>
        <circle
          className="ns-solid ns-pop ns-dot ns-i0"
          cx="56"
          cy="64"
          r="12"
        />
        <path className="ns-ext ns-i0" d="M96,64h120" />
        <circle
          className="ns-solid ns-pop ns-dot ns-i1"
          cx="56"
          cy="128"
          r="12"
        />
        <path className="ns-ext ns-i1" d="M96,128h120" />
        <circle
          className="ns-solid ns-pop ns-dot ns-i2"
          cx="56"
          cy="192"
          r="12"
        />
        <path className="ns-ext ns-i2" d="M96,192h120" />
      </>
    ),
  },
  MegaphoneIcon: {
    description: "ruft laut, die Schallwellen gehen raus",
    render: () => (
      <>
        <g className="ns-shout">
          <path d="M48,96H88l96-56V216L88,160H48a8,8,0,0,1-8-8V104A8,8,0,0,1,48,96Z" />
          <path d="M88,160l16,56" />
        </g>
        <path
          className="ns-ghost ns-wave2 ns-i0"
          d="M216,104a32,32,0,0,1,0,48"
        />
        <path
          className="ns-ghost ns-wave2 ns-i1"
          d="M232,80a64,64,0,0,1,0,96"
        />
      </>
    ),
  },
  MonitorIcon: {
    description: "der Bildschirm schaltet sich ein",
    render: () => (
      <>
        <rect x="24" y="40" width="208" height="144" rx="16" />
        <path d="M96,216h64M128,184v32" />
        <rect
          className="ns-solid ns-ghost ns-tv"
          x="48"
          y="64"
          width="160"
          height="96"
          rx="4"
        />
      </>
    ),
  },
  NotebookIcon: {
    description: "kippt an, Zeilen werden geschrieben",
    render: () => (
      <>
        <g className="ns-tilt">
          <rect x="48" y="32" width="168" height="192" rx="8" />
          <path d="M88,32V224" />
        </g>
        <path className="ns-draw ns-i0" pathLength="1" d="M120,88h56" />
        <path className="ns-draw ns-i1" pathLength="1" d="M120,128h56" />
      </>
    ),
  },
  PresentationChartIcon: {
    description: "die Balken wachsen nacheinander",
    render: () => (
      <>
        <rect x="32" y="48" width="192" height="136" rx="8" />
        <path d="M128,48V24M96,184l-24,40M160,184l24,40" />
        <path className="ns-bar ns-i0" d="M96,152V128" />
        <path className="ns-bar ns-i1" d="M128,152V104" />
        <path className="ns-bar ns-i2" d="M160,152V80" />
      </>
    ),
  },
  QuestionIcon: {
    description: "das Fragezeichen wundert sich",
    render: () => (
      <>
        <circle cx="128" cy="128" r="96" />
        <path
          className="ns-wonder"
          d="M104,100a24,24,0,1,1,36,20.8c-7,4.1-12,9.9-12,17.2v6"
        />
        <circle className="ns-solid ns-pop ns-bdot" cx="128" cy="178" r="12" />
      </>
    ),
  },
  ShieldCheckIcon: {
    description: "Schild pulsiert, der Haken wird gesetzt",
    render: () => (
      <>
        <g className="ns-pulse">
          <path d="M40,56a16,16,0,0,1,16-16H200a16,16,0,0,1,16,16v56c0,84-88,112-88,112S40,196,40,112Z" />
        </g>
        <polyline
          className="ns-draw"
          pathLength="1"
          points="92,132 116,156 164,108"
        />
      </>
    ),
  },
  SignOutIcon: {
    description: "der Pfeil schießt hinaus",
    render: () => (
      <>
        <path
          className="ns-recoil"
          d="M112,40H48a8,8,0,0,0-8,8V208a8,8,0,0,0,8,8h64"
        />
        <g className="ns-shoot">
          <path d="M112,128H216" />
          <polyline points="176,88 216,128 176,168" />
        </g>
      </>
    ),
  },
  SunIcon: {
    description: "die Strahlen drehen sich, die Sonne glüht",
    render: () => (
      <>
        <circle className="ns-glow" cx="128" cy="128" r="48" />
        <g className="ns-rays">
          <path d="M128,40V24M128,232V216M40,128H24M232,128H216M65.8,65.8,54.5,54.5M201.5,201.5l-11.3-11.3M65.8,190.2,54.5,201.5M201.5,54.5,190.2,65.8" />
        </g>
      </>
    ),
  },
  SunHorizonIcon: {
    description: "die Sonne geht unter und wieder auf",
    render: (uid) => (
      <>
        <defs>
          <clipPath id={uid}>
            <rect x="0" y="0" width="256" height="170" />
          </clipPath>
        </defs>
        <g clipPath={`url(#${uid})`}>
          <g className="ns-rise">
            <path d="M64,176a64,64,0,0,1,128,0" />
            <path d="M128,72V48M64,96,48,80M192,96l16-16" />
          </g>
        </g>
        <path d="M16,176H240M80,216h96" />
      </>
    ),
  },
  UserCheckIcon: {
    description: "nickt, der Haken wird gesetzt",
    render: () => (
      <>
        <g className="ns-nodD">
          <circle cx="104" cy="96" r="52" />
          <path d="M24,208c19-29,46-48,80-48s61,19,80,48" />
        </g>
        <polyline
          className="ns-draw"
          pathLength="1"
          points="172,124 192,144 232,104"
        />
      </>
    ),
  },
  UserCircleGearIcon: {
    description: "das Zahnrad dreht sich am Profil",
    render: () => (
      <>
        <path d="M168,212A96,96,0,1,1,212,168" />
        <circle cx="128" cy="112" r="36" />
        <path d="M68,195c14-23,36-35,60-35s32,6,40,12" />
        <g className="ns-cog">
          <circle cx="208" cy="208" r="16" />
          <path d="M208,176v8M208,232v8M176,208h8M232,208h8M185.4,185.4l5.6,5.6M225,225l5.6,5.6M185.4,230.6l5.6-5.6M225,191l5.6-5.6" />
        </g>
      </>
    ),
  },
  UserPlusIcon: {
    description: "das Plus dreht sich herein",
    render: () => (
      <>
        <g className="ns-hopS">
          <circle cx="108" cy="96" r="52" />
          <path d="M24,208c19-29,46-48,84-48s65,19,84,48" />
        </g>
        <path className="ns-spinIn2" d="M208,104v48M184,128h48" />
      </>
    ),
  },
  UserSwitchIcon: {
    description: "die Pfeile drehen sich um die Person",
    render: () => (
      <>
        <circle cx="128" cy="108" r="32" />
        <path d="M80,180c10-18,28-28,48-28s38,10,48,28" />
        <g className="ns-swirl">
          <path d="M48,96A88,88,0,0,1,200,56" />
          <polyline points="200,24 200,56 168,56" />
          <path d="M208,160A88,88,0,0,1,56,200" />
          <polyline points="56,232 56,200 88,200" />
        </g>
      </>
    ),
  },
  UsersIcon: {
    description: "zwei Köpfe unterhalten sich",
    render: () => (
      <>
        <g className="ns-lean ns-i0">
          <path d="M168,40a40,40,0,1,1,0,80M192,152c22,6,40,22,48,48" />
        </g>
        <g className="ns-lean2">
          <circle cx="96" cy="96" r="48" />
          <path d="M16,208c17-29,45-48,80-48s63,19,80,48" />
        </g>
      </>
    ),
  },
  UsersFourIcon: {
    description: "das Team hüpft reihum",
    render: () => (
      <>
        <g className="ns-round ns-i0">
          <circle cx="80" cy="64" r="22" />
          <path d="M40,124c8-16,22-24,40-24s32,8,40,24" />
        </g>
        <g className="ns-round ns-i1">
          <circle cx="176" cy="64" r="22" />
          <path d="M136,124c8-16,22-24,40-24s32,8,40,24" />
        </g>
        <g className="ns-round ns-i2">
          <circle cx="176" cy="168" r="22" />
          <path d="M136,228c8-16,22-24,40-24s32,8,40,24" />
        </g>
        <g className="ns-round ns-i3">
          <circle cx="80" cy="168" r="22" />
          <path d="M40,228c8-16,22-24,40-24s32,8,40,24" />
        </g>
      </>
    ),
  },
};

// Geräte der Produkt-Screenshots (#3759). Pro Gerät der native Viewport und
// deviceScaleFactor, damit das Rohbild exakt die Pixel des echten Displays hat,
// und der offizielle Apple Product Bezel samt Bildschirmrechteck.
//
// Die Bezels liegen unverändert in ./bezels (Apple-Richtlinie: Rahmen nicht
// verändern, nur die Web-App auf Apple-Geräten darstellen). `screen` ist das
// Rechteck im Bezel-Bild, in das das Rohbild gesetzt wird.

export const DEVICE_IDS = ["macbook", "ipad", "iphone"] as const;
export type DeviceId = (typeof DEVICE_IDS)[number];

interface Rect {
  readonly left: number;
  readonly top: number;
  readonly width: number;
  readonly height: number;
}

export interface DeviceSpec {
  readonly id: DeviceId;
  readonly label: string;
  /** CSS-Pixel des Viewports. */
  readonly viewport: { readonly width: number; readonly height: number };
  readonly deviceScaleFactor: number;
  readonly isMobile: boolean;
  readonly hasTouch: boolean;
  readonly bezel: {
    /** Dateiname in ./bezels. */
    readonly file: string;
    /** Erwartete Maße des Bezel-Bilds; ein anderes Bild bricht den Lauf ab. */
    readonly width: number;
    readonly height: number;
    readonly screen: Rect;
  };
}

export const DEVICES: Readonly<Record<DeviceId, DeviceSpec>> = {
  // MacBook Air 13": 2560×1664 px.
  macbook: {
    id: "macbook",
    label: 'MacBook Air 13"',
    viewport: { width: 1280, height: 832 },
    deviceScaleFactor: 2,
    isMobile: false,
    hasTouch: false,
    bezel: {
      file: "macbook-air-13.png",
      width: 3400,
      height: 2240,
      screen: { left: 420, top: 288, width: 2560, height: 1664 },
    },
  },
  // iPad Pro 11" quer: 2420×1668 px.
  ipad: {
    id: "ipad",
    label: 'iPad Pro 11" quer',
    viewport: { width: 1210, height: 834 },
    deviceScaleFactor: 2,
    isMobile: false,
    hasTouch: true,
    bezel: {
      file: "ipad-pro-11-quer.png",
      width: 2640,
      height: 1880,
      screen: { left: 110, top: 106, width: 2420, height: 1668 },
    },
  },
  // iPhone 16 Plus: 1290×2796 px.
  iphone: {
    id: "iphone",
    label: "iPhone 16 Plus",
    viewport: { width: 430, height: 932 },
    deviceScaleFactor: 3,
    isMobile: true,
    hasTouch: true,
    bezel: {
      file: "iphone-16-plus.png",
      width: 1470,
      height: 2970,
      screen: { left: 90, top: 87, width: 1290, height: 2796 },
    },
  },
};

/** Native Pixelmaße des Rohbilds. */
export function nativeSize(device: DeviceSpec): {
  width: number;
  height: number;
} {
  return {
    width: device.viewport.width * device.deviceScaleFactor,
    height: device.viewport.height * device.deviceScaleFactor,
  };
}

export type Portal = "tenant" | "eltern";

/** Standardgeräte je Portal; ein Shot kann sie mit `geraete` überschreiben. */
export const DEFAULT_DEVICES: Readonly<Record<Portal, readonly DeviceId[]>> = {
  tenant: ["macbook", "ipad"],
  eltern: ["iphone", "ipad"],
};

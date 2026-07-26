const UNKNOWN_DEVICE_LABEL = "Unbekanntes Gerät";

export function formatDeviceLabelFromUserAgent(
  userAgent?: string | null,
): string {
  const trimmed = userAgent?.trim();
  if (!trimmed) return UNKNOWN_DEVICE_LABEL;

  const lower = trimmed.toLowerCase();
  const browser = detectBrowser(lower);
  const os = detectOS(lower);

  if (browser && os) return `${browser} auf ${os}`;
  return browser ?? os ?? UNKNOWN_DEVICE_LABEL;
}

export function suggestCurrentDeviceLabel(): string {
  if (typeof navigator === "undefined") return UNKNOWN_DEVICE_LABEL;
  return formatDeviceLabelFromUserAgent(navigator.userAgent);
}

function detectBrowser(lowerUserAgent: string): string | null {
  if (lowerUserAgent.includes("edg/")) return "Edge";
  if (lowerUserAgent.includes("firefox/") || lowerUserAgent.includes("fxios/"))
    return "Firefox";
  if (lowerUserAgent.includes("crios/")) return "Chrome";
  if (lowerUserAgent.includes("chrome/") && !lowerUserAgent.includes("edg/")) {
    return "Chrome";
  }
  if (
    lowerUserAgent.includes("safari/") &&
    !lowerUserAgent.includes("chrome/") &&
    !lowerUserAgent.includes("crios/")
  ) {
    return "Safari";
  }
  return null;
}

function detectOS(lowerUserAgent: string): string | null {
  if (lowerUserAgent.includes("iphone")) return "iPhone";
  if (lowerUserAgent.includes("ipad")) return "iPad";
  if (lowerUserAgent.includes("android")) return "Android";
  if (lowerUserAgent.includes("mac os") || lowerUserAgent.includes("macintosh"))
    return "macOS";
  if (lowerUserAgent.includes("windows")) return "Windows";
  if (lowerUserAgent.includes("linux")) return "Linux";
  return null;
}

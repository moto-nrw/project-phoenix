import { useState, useEffect } from "react";

/**
 * Custom hook to detect mobile viewport (< 768px)
 * Uses globalThis for ES2020 compliance
 */
export function useIsMobile(): boolean {
  const [isMobile, setIsMobile] = useState(false);

  useEffect(() => {
    const checkMobile = () => setIsMobile(globalThis.innerWidth < 768);
    checkMobile();
    globalThis.addEventListener("resize", checkMobile);
    return () => globalThis.removeEventListener("resize", checkMobile);
  }, []);

  return isMobile;
}

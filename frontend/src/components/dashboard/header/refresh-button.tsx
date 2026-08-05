// Refresh button for header
// Triggers SWR cache revalidation to refresh all page data
// Animation: spin → green checkmark → back to rotate icon

"use client";

import { useState, useCallback, useRef, useEffect } from "react";
import { RotateCw, Check } from "lucide-react";
import { useSWRConfig } from "swr";
import { useTranslations } from "next-intl";

type ButtonState = "idle" | "spinning" | "success";

export function RefreshButton() {
  // parentNav is provided in every shell (full catalog in the parents portal,
  // German-only mirror via ShellNavIntlProvider in the staff/operator shells),
  // so the German label is unchanged there and only the parent portal localizes.
  const t = useTranslations("parentNav");
  const [state, setState] = useState<ButtonState>("idle");
  const pendingStopRef = useRef(false);
  const { mutate } = useSWRConfig();

  const handleRefresh = useCallback(async () => {
    if (state !== "idle") return;

    pendingStopRef.current = false;
    setState("spinning");

    try {
      await mutate(() => true);
    } finally {
      // Signal stop, animation will finish current rotation first
      pendingStopRef.current = true;
    }
  }, [state, mutate]);

  const handleAnimationIteration = useCallback(() => {
    if (pendingStopRef.current) {
      setState("success");
      pendingStopRef.current = false;
    }
  }, []);

  // Auto-transition from success back to idle
  useEffect(() => {
    if (state !== "success") return;
    const timeout = setTimeout(() => setState("idle"), 800);
    return () => clearTimeout(timeout);
  }, [state]);

  return (
    <button
      type="button"
      onClick={handleRefresh}
      disabled={state === "spinning"}
      aria-label={t("refresh")}
      title={t("refresh")}
      className="flex h-10 w-10 items-center justify-center rounded-lg text-gray-500 transition-colors duration-200 hover:bg-gray-100 hover:text-gray-700 disabled:opacity-50"
    >
      <div className="relative h-5 w-5">
        <RotateCw
          className={`absolute inset-0 h-5 w-5 ${
            state === "spinning"
              ? "animate-spin"
              : `transition-[opacity,scale] duration-200 ${state === "success" ? "scale-50 opacity-0" : ""}`
          }`}
          onAnimationIteration={handleAnimationIteration}
        />
        <Check
          className={`text-moto-green absolute inset-0 h-5 w-5 transition-[opacity,scale] duration-200 ${
            state === "success" ? "scale-100 opacity-100" : "scale-50 opacity-0"
          }`}
        />
      </div>
    </button>
  );
}

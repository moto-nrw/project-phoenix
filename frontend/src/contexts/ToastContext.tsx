"use client";

import React, { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";

type ToastType = "success" | "error" | "info" | "warning";

export interface ToastOptions {
  id?: string;
  duration?: number; // ms
}

interface ToastItemData {
  id: string;
  type: ToastType;
  message: string;
  duration: number;
}

interface ToastAPI {
  success: (message: string, options?: ToastOptions) => void;
  error: (message: string, options?: ToastOptions) => void;
  info: (message: string, options?: ToastOptions) => void;
  warning: (message: string, options?: ToastOptions) => void;
  remove: (id: string) => void;
}

const ToastContext = createContext<ToastAPI | undefined>(undefined);

export function useToast() {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within ToastProvider");
  return ctx;
}

// Visual style mapping aligned with SimpleAlert for consistency
const stylesByType: Record<ToastType, { bg: string; border: string; text: string; iconPath: string } > = {
  success: {
    bg: "bg-[#83CD2D]/10",
    border: "border-[#83CD2D]/20",
    text: "text-[#5A8B1F]",
    iconPath: "M5 13l4 4L19 7",
  },
  error: {
    bg: "bg-[#FF3130]/10",
    border: "border-[#FF3130]/20",
    text: "text-[#CC2626]",
    iconPath: "M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z",
  },
  info: {
    bg: "bg-[#5080D8]/10",
    border: "border-[#5080D8]/20",
    text: "text-[#4070C8]",
    iconPath: "M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z",
  },
  warning: {
    bg: "bg-[#F78C10]/10",
    border: "border-[#F78C10]/20",
    text: "text-[#C56F0D]",
    iconPath: "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z",
  },
};

function useReducedMotion() {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    if (typeof window === "undefined" || !window.matchMedia) return;
    const media = window.matchMedia("(prefers-reduced-motion: reduce)");
    const update = () => setReduced(media.matches);
    update();
    media.addEventListener?.("change", update);
    return () => media.removeEventListener?.("change", update);
  }, []);
  return reduced;
}

interface InternalToastTimers {
  timeoutId?: ReturnType<typeof setTimeout>;
  remaining: number;
  start: number;
}

function ToastRow({ item, onClose, reducedMotion }: { item: ToastItemData; onClose: (id: string) => void; reducedMotion: boolean }) {
  const { text, bg, border, iconPath } = stylesByType[item.type];
  const [visible, setVisible] = useState(false);
  const [exiting, setExiting] = useState(false);
  const timersRef = useRef<InternalToastTimers>({ remaining: item.duration, start: Date.now() });
  const isDesktopRef = useRef<boolean>(false);

  useEffect(() => {
    setVisible(true);
    timersRef.current.start = Date.now();

    let localTimeout: ReturnType<typeof setTimeout> | undefined;
    if (item.duration > 0) {
      localTimeout = setTimeout(() => {
        setExiting(true);
        setTimeout(() => onClose(item.id), reducedMotion ? 0 : 250);
      }, item.duration);
      timersRef.current.timeoutId = localTimeout;
    }

    if (typeof window !== "undefined") {
      isDesktopRef.current = !!(window.matchMedia && window.matchMedia("(min-width: 768px)").matches);
    }

    return () => {
      if (localTimeout) clearTimeout(localTimeout);
    };
  }, [item.duration, item.id, onClose, reducedMotion]);

  const pauseIfDesktop = () => {
    if (!isDesktopRef.current) return;
    if (timersRef.current.timeoutId) {
      clearTimeout(timersRef.current.timeoutId);
      timersRef.current.timeoutId = undefined;
      const elapsed = Date.now() - timersRef.current.start;
      timersRef.current.remaining = Math.max(0, item.duration - elapsed);
    }
  };

  const resumeIfDesktop = () => {
    if (!isDesktopRef.current) return;
    if (timersRef.current.remaining > 0) {
      timersRef.current.start = Date.now();
      timersRef.current.timeoutId = setTimeout(() => {
        setExiting(true);
        setTimeout(() => onClose(item.id), reducedMotion ? 0 : 250);
      }, timersRef.current.remaining);
    }
  };

  return (
    <div
      role="status"
      aria-live="polite"
      aria-atomic="true"
      onMouseEnter={pauseIfDesktop}
      onMouseLeave={resumeIfDesktop}
      className={`pointer-events-auto relative ${bg} ${border} ${text} rounded-2xl border p-4 shadow-lg backdrop-blur-sm transition-all ${reducedMotion ? "" : "duration-300 ease-out"} ${visible && !exiting ? "translate-y-0 opacity-100" : "translate-y-2 opacity-0"}`}
      style={{ transform: reducedMotion ? undefined : undefined }}
    >
      <div className="flex items-start gap-3">
        <div className={`flex-shrink-0 ${text}`}>
          <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d={iconPath} />
          </svg>
        </div>
        <p className={`flex-1 text-sm font-medium ${text}`}>{item.message}</p>
        <button
          aria-label="Schließen"
          onClick={() => onClose(item.id)}
          className={`flex-shrink-0 ${text} hover:opacity-70 transition-opacity`}
        >
          <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>
  );
}

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<ToastItemData[]>([]);
  const reducedMotion = useReducedMotion();

  // Track last shown timestamps for simple de-duplication
  const lastShownRef = useRef<Map<string, number>>(new Map());
  const DE_DUPE_WINDOW = 2000; // ms
  const MAX_VISIBLE = 3;

  const remove = useCallback((id: string) => {
    setItems((prev) => prev.filter((it) => it.id !== id));
  }, []);

  const push = useCallback((type: ToastType, message: string, options?: ToastOptions) => {
    if (!message) return;
    const now = Date.now();
    const last = lastShownRef.current.get(message) ?? 0;
    if (now - last < DE_DUPE_WINDOW) return; // de-dup
    lastShownRef.current.set(message, now);

    const id = options?.id ?? `${now}-${Math.random().toString(36).slice(2, 8)}`;
    const duration = options?.duration ?? 3000;

    setItems((prev) => {
      const next: ToastItemData[] = [...prev, { id, type, message, duration }];
      if (next.length > MAX_VISIBLE) {
        // remove oldest to keep at most MAX_VISIBLE visible
        next.shift();
      }
      return next;
    });
  }, []);

  const api: ToastAPI = useMemo(() => ({
    success: (m, o) => push("success", m, o),
    error: (m, o) => push("error", m, o),
    info: (m, o) => push("info", m, o),
    warning: (m, o) => push("warning", m, o),
    remove,
  }), [push, remove]);

  return (
    <ToastContext.Provider value={api}>
      {children}
      {/* Global container: mobile full width near bottom; desktop bottom-right */}
      <div
        className="pointer-events-none fixed bottom-6 left-4 right-4 md:left-auto md:right-6 z-[9998] md:max-w-sm flex flex-col items-stretch md:items-end gap-2"
      >
        {items.map((item) => (
          <ToastRow key={item.id} item={item} onClose={remove} reducedMotion={reducedMotion} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}

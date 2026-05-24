"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Hammer, Quote, ShieldCheck, UsersRound } from "lucide-react";
import { cn } from "~/lib/utils";

export const authInputClassName =
  "moto-content-surface h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-50 disabled:text-gray-500";

export const authPrimaryButtonClassName =
  "inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-400";

type AuthShellVariant =
  | "tenant"
  | "parents"
  | "tenant-select"
  | "reset"
  | "operator";

interface Testimonial {
  readonly quote: string;
  readonly author: string;
  readonly role: string;
  readonly metric: string;
}

type TestimonialCardState = "previous" | "active" | "next" | "hidden";

const motoTestimonials: Testimonial[] = [
  {
    quote:
      "Was vorher 15 Minuten Sucherei, WhatsApps und Telefonate waren, sind heute zwei Klicks. moto spart uns Zeit, die jetzt wieder bei den Kindern landet.",
    author: "OGS-Team",
    role: "Betreuungsalltag",
    metric: "Mehr Zeit für Kinder",
  },
  {
    quote:
      "moto hat uns gezeigt, wie sehr digitale Unterstützung den OGS-Alltag entlasten kann. Besonders geholfen hat uns, dass das Team unsere Abläufe wirklich versteht.",
    author: "OGS-Leitung",
    role: "Offener Ganztag",
    metric: "Spürbare Entlastung",
  },
  {
    quote:
      "Mit den steigenden Anmeldezahlen hätten wir ohne moto irgendwann nur noch hinterhergearbeitet. Jetzt landet alles geordnet an einem Ort: Anmeldungen, Änderungen und wichtige Hinweise.",
    author: "Trägerteam",
    role: "Anmeldung und Planung",
    metric: "Anmeldungen bewältigen",
  },
  {
    quote:
      "Wir dachten lange, unser Alltag sei einfach zu unterschiedlich für eine Software. moto bildet unsere Abläufe ab, ohne sie glattzubügeln.",
    author: "Betreuungsteam",
    role: "OGS-Praxis",
    metric: "Alltagstauglich",
  },
  {
    quote:
      "Abholzeiten, Krankmeldungen und Hinweise stehen nicht mehr irgendwo verteilt. Dadurch behalten Teams den Überblick und können auf Rückfragen sicher reagieren.",
    author: "Koordination",
    role: "Kommunikation",
    metric: "Alles an einem Ort",
  },
  {
    quote:
      "Das Wichtigste ist nicht, dass alles digital ist. Das Wichtigste ist, dass wir ruhiger arbeiten und trotzdem den besseren Überblick haben.",
    author: "OGS-Leitung",
    role: "Offener Ganztag",
    metric: "Kontrolle gewinnen",
  },
];

interface AuthShellProps {
  readonly eyebrow: string;
  readonly title: string;
  readonly subtitle: string;
  readonly children: ReactNode;
  readonly variant: AuthShellVariant;
  readonly brand?: ReactNode;
  readonly eyebrowClassName?: string;
  readonly titleClassName?: string;
  readonly footer?: ReactNode;
  readonly formMaxWidth?: string;
}

export function OperatorBrand() {
  return (
    <div className="flex flex-col items-center gap-3">
      <div className="relative flex h-24 w-24 items-center justify-center rounded-2xl border border-gray-800 bg-gray-950 text-white shadow-lg shadow-gray-200">
        <Hammer className="h-12 w-12" aria-hidden="true" strokeWidth={2.25} />
        <span className="absolute -right-1 -bottom-1 h-4 w-4 rounded-full border-2 border-white bg-[#83CD2D]" />
      </div>
      <div className="text-center">
        <p className="text-sm font-semibold text-gray-950">Operator Console</p>
        <p className="mt-0.5 text-xs font-medium text-gray-500">
          Interner Plattformzugang
        </p>
      </div>
    </div>
  );
}

export function AuthShell({
  eyebrow,
  title,
  subtitle,
  children,
  variant,
  brand = null,
  eyebrowClassName,
  titleClassName,
  footer,
  formMaxWidth = "max-w-[29rem]",
}: AuthShellProps) {
  return (
    <main className="moto-auth-shell-background grid min-h-screen lg:grid-cols-[minmax(0,0.9fr)_minmax(34rem,1.1fr)]">
      <section className="relative z-10 flex min-h-screen items-center justify-center px-4 py-8 sm:px-6 lg:px-10">
        <div className={cn("w-full", formMaxWidth)}>
          <div className="rounded-2xl border border-gray-200 bg-white px-6 py-10 shadow-sm sm:p-8">
            {brand && <div className="mb-8 flex justify-center">{brand}</div>}
            <div className="mb-9 text-center">
              <p
                className={cn(
                  "mb-3 text-xs font-semibold tracking-[0.16em] text-[#5080D8] uppercase",
                  eyebrowClassName,
                )}
              >
                {eyebrow}
              </p>
              <h1
                className={cn(
                  "text-3xl font-semibold tracking-normal text-gray-950 sm:text-4xl",
                  titleClassName,
                )}
              >
                {title}
              </h1>
              <p className="mt-3 text-sm leading-6 text-gray-600">{subtitle}</p>
            </div>
            {children}
          </div>
          {footer && <div className="mt-5 text-center">{footer}</div>}
        </div>
      </section>

      <AuthTestimonialPanel variant={variant} />
    </main>
  );
}

function AuthTestimonialPanel({
  variant,
}: {
  readonly variant: AuthShellVariant;
}) {
  const items = useMemo(() => {
    void variant;
    return motoTestimonials;
  }, [variant]);
  const [activeIndex, setActiveIndex] = useState(0);

  useEffect(() => {
    const interval = window.setInterval(() => {
      setActiveIndex((current) => (current + 1) % items.length);
    }, 11000);

    return () => window.clearInterval(interval);
  }, [items.length]);

  const getCardState = (index: number): TestimonialCardState => {
    const offset = (index - activeIndex + items.length) % items.length;
    if (offset === 0) return "active";
    if (offset === 1) return "next";
    if (offset === items.length - 1) return "previous";
    return "hidden";
  };

  return (
    <aside className="moto-dotted-background moto-dotted-background--split relative z-0 hidden min-h-screen overflow-hidden border-l border-gray-200 px-8 py-10 lg:flex lg:flex-col lg:justify-between xl:px-14">
      <div className="relative z-10 flex items-center justify-between">
        <div className="inline-flex items-center gap-2 rounded-full border border-gray-200 bg-white/85 px-3 py-1.5 text-xs font-medium text-gray-700 shadow-sm backdrop-blur-sm">
          <UsersRound className="h-4 w-4 text-[#83CD2D]" aria-hidden="true" />
          Für den Ganztag gemacht
        </div>
        <div className="inline-flex items-center gap-2 rounded-full border border-gray-200 bg-white/85 px-3 py-1.5 text-xs font-medium text-gray-700 shadow-sm backdrop-blur-sm">
          <ShieldCheck className="h-4 w-4 text-[#5080D8]" aria-hidden="true" />
          In Deutschland gehostet
        </div>
      </div>

      <div className="relative z-10 mx-auto flex w-full max-w-xl flex-1 flex-col justify-center py-12">
        <div className="relative h-[28rem] [perspective:1200px]">
          {items.map((testimonial, index) => (
            <TestimonialCard
              key={`${testimonial.author}-${testimonial.metric}`}
              testimonial={testimonial}
              state={getCardState(index)}
            />
          ))}
        </div>

        <div className="mt-7 flex items-center justify-center gap-2">
          {items.map((item, index) => (
            <button
              key={`${item.author}-${item.metric}`}
              type="button"
              aria-label={`Erfahrungsbericht ${index + 1} anzeigen`}
              onClick={() => setActiveIndex(index)}
              className={cn(
                "h-2.5 rounded-full transition-all duration-700 ease-[cubic-bezier(0.22,1,0.36,1)]",
                index === activeIndex
                  ? "w-8 bg-gray-950"
                  : "w-2.5 bg-gray-300 hover:bg-gray-400",
              )}
            />
          ))}
        </div>
      </div>

      <div aria-hidden="true" className="relative z-10 h-6" />
    </aside>
  );
}

function TestimonialCard({
  testimonial,
  state,
}: {
  readonly testimonial: Testimonial;
  readonly state: TestimonialCardState;
}) {
  const motion = {
    previous: {
      opacity: 0.32,
      transform:
        "translate3d(-3.5rem, 2.25rem, -120px) rotateY(-18deg) rotateZ(-1.5deg) scale(0.92)",
      zIndex: 10,
    },
    active: {
      opacity: 1,
      transform: "translate3d(0, 0, 0) rotateY(0deg) rotateZ(0deg) scale(1)",
      zIndex: 30,
    },
    next: {
      opacity: 0.32,
      transform:
        "translate3d(3.5rem, 3.5rem, -120px) rotateY(18deg) rotateZ(1.5deg) scale(0.92)",
      zIndex: 10,
    },
    hidden: {
      opacity: 0,
      transform:
        "translate3d(0, 4.5rem, -220px) rotateY(26deg) rotateZ(2deg) scale(0.86)",
      zIndex: 0,
    },
  }[state];

  return (
    <article
      aria-hidden={state !== "active"}
      className={cn(
        "absolute inset-x-0 top-0 min-h-[22rem] overflow-hidden rounded-2xl border border-gray-200 bg-white/94 p-8 shadow-xl shadow-gray-200/70 backdrop-blur-sm transition-all duration-[1250ms] ease-[cubic-bezier(0.16,1,0.3,1)] will-change-transform motion-reduce:transition-none",
        state !== "active" && "pointer-events-none",
        state === "active" && "shadow-2xl shadow-gray-300/60",
      )}
      style={{
        opacity: motion.opacity,
        transform: motion.transform,
        transformStyle: "preserve-3d",
        zIndex: motion.zIndex,
      }}
    >
      <div
        className={cn(
          "transition-all duration-700 ease-out motion-reduce:transition-none",
          state === "active"
            ? "translate-y-0 opacity-100"
            : "translate-y-3 opacity-0",
        )}
      >
        <div className="mb-6 flex items-center justify-between">
          <div className="flex h-11 w-11 items-center justify-center rounded-full bg-gray-950 text-white">
            <Quote className="h-5 w-5" aria-hidden="true" />
          </div>
          <span className="rounded-full bg-[#EAF6D8] px-3 py-1 text-xs font-semibold text-[#4E7D1B]">
            {testimonial.metric}
          </span>
        </div>
        <p className="text-xl leading-8 font-medium text-gray-950">
          "{testimonial.quote}"
        </p>
        <div className="mt-7 border-t border-gray-100 pt-5">
          <p className="text-sm font-semibold text-gray-950">
            {testimonial.author}
          </p>
          <p className="mt-1 text-sm text-gray-500">{testimonial.role}</p>
        </div>
      </div>
    </article>
  );
}

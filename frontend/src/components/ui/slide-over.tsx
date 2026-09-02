"use client";

/**
 * SlideOver — detail panel primitive built on Vaul. Slides in from the right on
 * a desktop-sized viewport and up from the bottom on a phone.
 *
 * Companion to ./drawer.tsx (bottom-sheet). The two share the same Vaul
 * primitives but differ in `direction` and the resulting Tailwind layout.
 * Use SlideOver for desktop-first detail panels that need to coexist with
 * the underlying view (planner grid → click instance → slide-over from
 * right). Use Drawer when a surface should be a bottom sheet at every size.
 *
 * Warum die Richtung mitwandert: ein von rechts einfahrendes 92vw-Panel ist auf
 * einem Handy ein Fremdkörper — dort ist das Blatt von unten die erwartete
 * Geste, und die App benutzt sie an anderer Stelle längst (das "Mehr"-Menü der
 * Bottom-Navigation). Vaul richtet Drag-Achse und Animation am `direction`-Prop
 * der Wurzel aus, deshalb ist das eine echte Breakpoint-Abfrage (matchMedia)
 * und keine reine CSS-Angelegenheit.
 *
 * Usage:
 * ```tsx
 * <SlideOver open={selected !== null} onOpenChange={(o) => !o && clear()}>
 *   <SlideOverContent>
 *     <SlideOverHeader>
 *       <SlideOverTitle>Mensa</SlideOverTitle>
 *       <SlideOverDescription>Mittwoch, 24.09.2026</SlideOverDescription>
 *     </SlideOverHeader>
 *     <div className="flex-1 overflow-y-auto px-5 py-4">…</div>
 *     <SlideOverFooter>…</SlideOverFooter>
 *   </SlideOverContent>
 * </SlideOver>
 * ```
 */

import * as React from "react";
import { Drawer as DrawerPrimitive } from "vaul";
import { X } from "lucide-react";

import { BELOW_SM } from "~/lib/hooks/use-media-query";
import { cn } from "~/lib/utils";
import {
  OVERLAY_BACKDROP_CLASS,
  OVERLAY_BACKDROP_TINT_CLASS,
} from "./overlay-styles";

/**
 * Unter sm fährt das Panel von unten ein, darüber von rechts. Vor dem ersten
 * Effect gilt "right", damit Server- und Client-Render übereinstimmen. Die
 * Auflösung wird separat mitgegeben, damit ein beim Laden bereits offenes
 * Panel einmalig die korrekte Richtung übernehmen kann.
 */
function useSlideOverDirection(): {
  direction: "right" | "bottom";
  isResolved: boolean;
} {
  const [state, setState] = React.useState<{
    direction: "right" | "bottom";
    isResolved: boolean;
  }>({ direction: "right", isResolved: false });

  React.useEffect(() => {
    const mediaQuery = window.matchMedia(BELOW_SM);
    const update = () => {
      setState({
        direction: mediaQuery.matches ? "bottom" : "right",
        isResolved: true,
      });
    };
    update();
    mediaQuery.addEventListener("change", update);
    return () => mediaQuery.removeEventListener("change", update);
  }, []);

  return state;
}

const SlideOverDirectionContext = React.createContext<"right" | "bottom">(
  "right",
);

const SlideOver = ({
  shouldScaleBackground = false,
  open: openProp,
  defaultOpen = false,
  onOpenChange,
  ...props
}: React.ComponentProps<typeof DrawerPrimitive.Root>) => {
  const { direction: preferredDirection, isResolved } = useSlideOverDirection();
  const [uncontrolledOpen, setUncontrolledOpen] = React.useState(defaultOpen);
  const open = openProp ?? uncontrolledOpen;
  const [direction, setDirection] = React.useState(preferredDirection);
  const appliedInitialDirection = React.useRef(false);

  // Vaul übernimmt `direction` nur beim Mount. Ein Richtungswechsel mit
  // `key={direction}` darf deshalb nicht passieren, solange ein Formular im
  // Panel offen ist: Das würde dessen lokalen, noch nicht gespeicherten State
  // verwerfen. Die erste Client-Auflösung ist davon ausgenommen: Zu diesem
  // Zeitpunkt konnte noch kein Nutzer im Panel arbeiten, und ein per Deep-Link
  // sofort offenes Panel muss auf einem Telefon von unten erscheinen. Nach dem
  // Schließen darf der nächste Öffnungsvorgang die dann passende Vaul-Instanz
  // verwenden.
  React.useEffect(() => {
    if (!isResolved) return;

    if (!appliedInitialDirection.current) {
      appliedInitialDirection.current = true;
      setDirection(preferredDirection);
      return;
    }

    if (!open) setDirection(preferredDirection);
  }, [isResolved, open, preferredDirection]);

  const handleOpenChange = React.useCallback(
    (nextOpen: boolean) => {
      setUncontrolledOpen(nextOpen);
      onOpenChange?.(nextOpen);
    },
    [onOpenChange],
  );

  return (
    <SlideOverDirectionContext.Provider value={direction}>
      <DrawerPrimitive.Root
        // Vaul liest die Richtung beim Mount für Drag-Achse und Animation.
        // Der Key baut die Instanz erst nach dem Schließen für einen inzwischen
        // geänderten Breakpoint neu auf.
        key={direction}
        direction={direction}
        shouldScaleBackground={shouldScaleBackground}
        open={open}
        onOpenChange={handleOpenChange}
        {...props}
      />
    </SlideOverDirectionContext.Provider>
  );
};
SlideOver.displayName = "SlideOver";

const SlideOverPortal = DrawerPrimitive.Portal;

const SlideOverOverlay = React.forwardRef<
  React.ComponentRef<typeof DrawerPrimitive.Overlay>,
  React.ComponentPropsWithoutRef<typeof DrawerPrimitive.Overlay>
>(({ className, ...props }, ref) => (
  <DrawerPrimitive.Overlay
    ref={ref}
    className={cn(
      "fixed inset-0 z-40",
      OVERLAY_BACKDROP_TINT_CLASS,
      OVERLAY_BACKDROP_CLASS,
      className,
    )}
    {...props}
  />
));
SlideOverOverlay.displayName = DrawerPrimitive.Overlay.displayName;

interface SlideOverContentProps extends React.ComponentPropsWithoutRef<
  typeof DrawerPrimitive.Content
> {
  /**
   * Panel width on desktop. Default 420px matches the timetable mockup.
   * Ignoriert, solange das Panel als Blatt von unten läuft (< sm) — dort ist
   * es immer volle Breite.
   */
  widthClass?: string;
}

const SlideOverContent = React.forwardRef<
  React.ComponentRef<typeof DrawerPrimitive.Content>,
  SlideOverContentProps
>(({ className, widthClass, children, ...props }, ref) => {
  const direction = React.useContext(SlideOverDirectionContext);
  const isSheet = direction === "bottom";
  return (
    <SlideOverPortal>
      <SlideOverOverlay />
      <DrawerPrimitive.Content
        ref={ref}
        data-date-picker-focus-trap="true"
        className={cn(
          "fixed z-50 flex flex-col bg-white shadow-2xl outline-none",
          isSheet
            ? // Blatt von unten: an drei Kanten bündig, oben abgerundet, nie
              // höher als 90vh — der Rest der Seite bleibt als Kontext sichtbar.
              "inset-x-0 bottom-0 max-h-[90vh] rounded-t-2xl"
            : // `max-w-full` fängt den einen Frame vor dem matchMedia-Effect ab:
              // ein per Deep-Link (?block=…) sofort offenes Panel würde sonst
              // kurz mit 420px auf einem schmaleren Display stehen.
              cn("top-0 right-0 h-full w-[420px] max-w-full", widthClass),
          className,
        )}
        {...props}
      >
        {isSheet && (
          // Griff wie im Drawer: signalisiert die Wischgeste, mit der Vaul das
          // Blatt schließt.
          <div
            aria-hidden
            className="mx-auto mt-3 h-1 w-12 shrink-0 rounded-full bg-gray-300"
          />
        )}
        {children}
      </DrawerPrimitive.Content>
    </SlideOverPortal>
  );
});
SlideOverContent.displayName = "SlideOverContent";

const SlideOverHeader = ({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) => (
  <div
    className={cn(
      "flex flex-col gap-1 border-b border-slate-200 px-5 py-4",
      className,
    )}
    {...props}
  />
);
SlideOverHeader.displayName = "SlideOverHeader";

const SlideOverFooter = ({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) => (
  <div
    className={cn(
      "flex flex-col gap-2 border-t border-slate-200 bg-slate-50 px-5 py-4",
      className,
    )}
    {...props}
  />
);
SlideOverFooter.displayName = "SlideOverFooter";

const SlideOverTitle = React.forwardRef<
  React.ComponentRef<typeof DrawerPrimitive.Title>,
  React.ComponentPropsWithoutRef<typeof DrawerPrimitive.Title>
>(({ className, ...props }, ref) => (
  <DrawerPrimitive.Title
    ref={ref}
    className={cn("text-lg leading-tight font-bold text-slate-900", className)}
    {...props}
  />
));
SlideOverTitle.displayName = DrawerPrimitive.Title.displayName;

const SlideOverDescription = React.forwardRef<
  React.ComponentRef<typeof DrawerPrimitive.Description>,
  React.ComponentPropsWithoutRef<typeof DrawerPrimitive.Description>
>(({ className, ...props }, ref) => (
  <DrawerPrimitive.Description
    ref={ref}
    className={cn("text-xs text-slate-500", className)}
    {...props}
  />
));
SlideOverDescription.displayName = DrawerPrimitive.Description.displayName;

const SlideOverClose = DrawerPrimitive.Close;

/**
 * SlideOverCloseButton — the shared close (X) control for slide-over headers.
 *
 * The SlideOver primitive (Vaul) ships no styled close button, so every panel
 * used to hand-roll its own — which is why the close-X drifted across the app.
 * This is the single source of truth: a round icon button matching the
 * canonical slide-over close (room-detail-panel's closeButtonClass). Renders a
 * default X; pass children to override, and aria-label to retitle it.
 */
const SlideOverCloseButton = React.forwardRef<
  React.ComponentRef<typeof DrawerPrimitive.Close>,
  React.ComponentPropsWithoutRef<typeof DrawerPrimitive.Close>
>(({ className, children, ...props }, ref) => (
  <DrawerPrimitive.Close
    ref={ref}
    aria-label="Schließen"
    className={cn(
      "inline-flex h-9 w-9 items-center justify-center rounded-full text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-300",
      className,
    )}
    {...props}
  >
    {children ?? <X className="h-5 w-5" aria-hidden="true" />}
  </DrawerPrimitive.Close>
));
SlideOverCloseButton.displayName = "SlideOverCloseButton";

export {
  SlideOver,
  SlideOverContent,
  SlideOverHeader,
  SlideOverFooter,
  SlideOverTitle,
  SlideOverDescription,
  SlideOverClose,
  SlideOverCloseButton,
};

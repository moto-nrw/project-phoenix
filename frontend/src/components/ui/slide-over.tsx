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
 *     <SlideOverBody error={saveError}>…</SlideOverBody>
 *     <SlideOverFooter>…</SlideOverFooter>
 *   </SlideOverContent>
 * </SlideOver>
 * ```
 *
 * `SlideOverHeader` is a row: title and description on the left, the close X on
 * the right. `SlideOverFooter` is a right-aligned row of actions. `size` on
 * `SlideOverContent` picks the desktop width (`md`, `lg`, `xl`); without it the
 * panel keeps its narrow 420 px.
 *
 * `SlideOverBody` is the scrolling middle; its `error` prop is the fixed slot
 * for a form's validation or save error (BAUARTEN-SPEC, Bauart 2 Regel 5), so
 * no panel places that alert on its own.
 *
 * A panel that saves passes `pending` from `useFormModalPending(open)` and
 * runs every save through its `run` (#6593). While a save runs, the panel
 * locks every way to close it (X, Escape, backdrop, swipe) and every footer
 * action, and marks itself `aria-busy`; `ui-kit/slide-over-pending` fails a
 * panel without it. `isDismissDisabled` locks only the ways to close it, as
 * on `Modal` (#6575).
 */

import * as React from "react";
import { Drawer as DrawerPrimitive } from "vaul";

import { BELOW_SM } from "~/lib/hooks/use-media-query";
import { cn } from "~/lib/utils";
import { buttonClassName } from "./button";
import { CloseIcon } from "./close-icon";
import { ICON_ONLY } from "./control-geometry";
import { DialogActions, DialogErrorSlot } from "./dialog-parts";
import { useBusyFocus } from "./hooks/useBusyFocus";
import { useSheetAboveKeyboard } from "./hooks/useSheetAboveKeyboard";
import type { FormModalPending } from "./use-form-modal-pending";
import type { FormErrorInput } from "./form-error";
import {
  OVERLAY_BACKDROP_CLASS,
  OVERLAY_BACKDROP_TINT_CLASS,
  Z_LAYER,
} from "./overlay-styles";
import { isInsideModal, SHEET_SCOPE_PROPS, useOverlay } from "./overlay-stack";

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
/** True while the panel's save runs (`pending`): footer actions are locked. */
const SlideOverBusyContext = React.createContext(false);
/** True while every way to close the panel is locked (busy or `isDismissDisabled`). */
const SlideOverDismissLockedContext = React.createContext(false);
/** Whether the surrounding panel currently locks dismissal actions. */
function useSlideOverDismissDisabled(): boolean {
  return React.useContext(SlideOverDismissLockedContext);
}
/**
 * False once the panel is closed. Vaul keeps it on screen while it slides
 * out; its footer must not start a save then that the next opening would
 * count as its own.
 */
const SlideOverOpenContext = React.createContext(true);

// Vaul's props are a union (with or without `fadeFromIndex`); a plain Omit
// would merge its members.
type DistributiveOmit<T, K extends PropertyKey> = T extends unknown
  ? Omit<T, K>
  : never;

/**
 * Vaul's `dismissible` is not offered: the lock is `isDismissDisabled`, the
 * one name for it in every kit dialog (#6575).
 */
type SlideOverProps = DistributiveOmit<
  React.ComponentProps<typeof DrawerPrimitive.Root>,
  "dismissible"
> & {
  /**
   * The pending contract from `useFormModalPending(open)` (#6593). While its
   * save runs, every way to close the panel (X, Escape, backdrop, swipe) and
   * every footer action is locked, the X shows as disabled and the panel is
   * `aria-busy`: a request that still commits must not look cancelled. Every
   * panel that saves passes it; `ui-kit/slide-over-pending` fails one
   * without it.
   */
  pending?: FormModalPending;
  /**
   * Locks every way to close the panel (X, Escape, backdrop, swipe) while an
   * operation must finish in place, without locking the footer. The one name
   * for this lock in every kit dialog, as on `Modal` (#6575).
   */
  isDismissDisabled?: boolean;
};

const SlideOver = ({
  shouldScaleBackground = false,
  open: openProp,
  defaultOpen = false,
  onOpenChange,
  pending,
  isDismissDisabled = false,
  ...props
}: SlideOverProps) => {
  const busy = pending?.pending ?? false;
  const dismissLocked = busy || isDismissDisabled;
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
      if (!nextOpen && dismissLocked) return;
      setUncontrolledOpen(nextOpen);
      onOpenChange?.(nextOpen);
    },
    [dismissLocked, onOpenChange],
  );

  return (
    <SlideOverDirectionContext.Provider value={direction}>
      <SlideOverBusyContext.Provider value={busy}>
        <SlideOverDismissLockedContext.Provider value={dismissLocked}>
          <SlideOverOpenContext.Provider value={open}>
            <DrawerPrimitive.Root
              // Vaul liest die Richtung beim Mount für Drag-Achse und Animation.
              // Der Key baut die Instanz erst nach dem Schließen für einen inzwischen
              // geänderten Breakpoint neu auf.
              key={direction}
              direction={direction}
              shouldScaleBackground={shouldScaleBackground}
              open={open}
              onOpenChange={handleOpenChange}
              dismissible={!dismissLocked}
              {...props}
            />
          </SlideOverOpenContext.Provider>
        </SlideOverDismissLockedContext.Provider>
      </SlideOverBusyContext.Provider>
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
      `fixed inset-0 ${Z_LAYER.sheetBackdrop}`,
      OVERLAY_BACKDROP_TINT_CLASS,
      OVERLAY_BACKDROP_CLASS,
      className,
    )}
    {...props}
  />
));
SlideOverOverlay.displayName = DrawerPrimitive.Overlay.displayName;

/**
 * Die Panelbreiten am Desktop. Ohne `size` bleibt es bei schmalen 420 px, wie
 * im Stundenplan. Es gibt bewusst keine freie Breite: `ui-kit/no-arbitrary-
 * slide-over-width` sperrt sie (#5602).
 */
const SLIDE_OVER_WIDTH = {
  md: "sm:w-[640px]",
  lg: "sm:w-[760px]",
  xl: "sm:w-[860px]",
} as const;

type SlideOverContentProps = Omit<
  React.ComponentPropsWithoutRef<typeof DrawerPrimitive.Content>,
  "size"
> & {
  /**
   * Panel width on desktop. Ignoriert, solange das Panel als Blatt von unten
   * läuft (< sm) — dort ist es immer volle Breite.
   */
  size?: keyof typeof SLIDE_OVER_WIDTH;
  /**
   * Ein Klick neben das Panel schließt es nicht. Für Formulare, deren
   * Eingaben ein Fehlklick sonst verwirft (#3370). X, Abbrechen, Escape und
   * die Wischgeste auf dem Telefon schließen weiterhin.
   */
  isBackdropDismissDisabled?: boolean;
};

function isSlideOverWidth(
  size: unknown,
): size is keyof typeof SLIDE_OVER_WIDTH {
  return typeof size === "string" && Object.hasOwn(SLIDE_OVER_WIDTH, size);
}

const SlideOverContent = React.forwardRef<
  React.ComponentRef<typeof DrawerPrimitive.Content>,
  SlideOverContentProps
>(
  (
    {
      className,
      size,
      isBackdropDismissDisabled = false,
      onInteractOutside,
      onEscapeKeyDown,
      children,
      ...props
    },
    ref,
  ) => {
    const direction = React.useContext(SlideOverDirectionContext);
    const busy = React.useContext(SlideOverBusyContext);
    const open = React.useContext(SlideOverOpenContext);
    const { isTopmost } = useOverlay(open, "sheet");
    const isSheet = direction === "bottom";
    const sheetRef = React.useRef<HTMLDivElement | null>(null);
    useSheetAboveKeyboard(sheetRef, isSheet);
    // Keeps focus in the panel while the save disables its own button.
    useBusyFocus(sheetRef, busy);
    return (
      <SlideOverPortal>
        <SlideOverOverlay />
        <DrawerPrimitive.Content
          ref={(node) => {
            sheetRef.current = node;
            if (typeof ref === "function") ref(node);
            else if (ref) ref.current = node;
          }}
          {...SHEET_SCOPE_PROPS}
          // Focus target while a busy panel has disabled its own controls;
          // Radix sets the same on its focus scope.
          tabIndex={-1}
          aria-busy={busy || undefined}
          className={cn(
            `fixed ${Z_LAYER.sheet} flex flex-col bg-white shadow-2xl outline-none`,
            isSheet
              ? // Blatt von unten: an drei Kanten bündig, oben abgerundet, nie
                // höher als 90vh — der Rest der Seite bleibt als Kontext sichtbar.
                "inset-x-0 bottom-0 max-h-[90vh] rounded-t-2xl"
              : // `max-w-full` fängt den einen Frame vor dem matchMedia-Effect ab:
                // ein per Deep-Link (?block=…) sofort offenes Panel würde sonst
                // kurz mit 420px auf einem schmaleren Display stehen.
                cn(
                  "top-0 right-0 h-full w-[420px] max-w-full",
                  isSlideOverWidth(size) ? SLIDE_OVER_WIDTH[size] : undefined,
                ),
            className,
          )}
          // `dismissible={false}` an der Wurzel wäre für einen reinen
          // Hintergrundschutz zu breit: Es sperrt auch X und Escape. Radix
          // schließt bei einem Außenklick nur, solange dieses Ereignis nicht
          // abgewiesen wurde. Ein Kit-Modal über dem Panel liegt im Portal
          // außerhalb davon; ein Klick oder Fokus dort schließt das Panel
          // darunter nicht (#6148).
          onInteractOutside={(
            event: Parameters<
              NonNullable<SlideOverContentProps["onInteractOutside"]>
            >[0],
          ) => {
            onInteractOutside?.(event);
            if (isBackdropDismissDisabled || isInsideModal(event.target)) {
              event.preventDefault();
            }
          }}
          onEscapeKeyDown={(event) => {
            onEscapeKeyDown?.(event);
            // An overlay above the panel (dialog, tour) takes the Escape.
            // Radix handles Escape in capture, before nested controls close.
            if (
              !isTopmost() ||
              document.querySelector(
                '[role="combobox"][aria-expanded="true"], [role="listbox"], [data-date-picker-panel]',
              )
            ) {
              event.preventDefault();
            }
          }}
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
  },
);
SlideOverContent.displayName = "SlideOverContent";

/**
 * SlideOverCloseButton — the shared close (X) control for slide-over headers.
 *
 * The SlideOver primitive (Vaul) ships no styled close button. Like every
 * overlay close X (#5531) it is the kit button `ghost` `icon` with
 * `CloseIcon`, so Modal, FormModal and SlideOver close the same way. Renders
 * the X by default; pass children to override, and aria-label to retitle it.
 */
const SlideOverCloseButton = React.forwardRef<
  React.ComponentRef<typeof DrawerPrimitive.Close>,
  React.ComponentPropsWithoutRef<typeof DrawerPrimitive.Close>
>(({ className, children, disabled = false, ...props }, ref) => {
  const dismissLocked = useSlideOverDismissDisabled();

  return (
    <DrawerPrimitive.Close
      ref={ref}
      type="button"
      aria-label="Schließen"
      disabled={disabled || dismissLocked}
      // Daran findet die Tour der ersten Schritte (#2832) das Schließen.
      data-overlay-close=""
      {...ICON_ONLY}
      className={buttonClassName({
        variant: "ghost",
        size: "icon",
        className: cn("shrink-0", className),
      })}
      {...props}
    >
      {children ?? <CloseIcon />}
    </DrawerPrimitive.Close>
  );
});
SlideOverCloseButton.displayName = "SlideOverCloseButton";

/**
 * The textual dismissal control for a `SlideOverFooter`. It closes through
 * Vaul and reads the same dismissal lock as the header X; save actions in the
 * footer remain independent.
 */
const SlideOverCancelButton = React.forwardRef<
  React.ComponentRef<typeof DrawerPrimitive.Close>,
  React.ComponentPropsWithoutRef<typeof DrawerPrimitive.Close>
>(({ className, disabled = false, ...props }, ref) => {
  const dismissLocked = useSlideOverDismissDisabled();

  return (
    <DrawerPrimitive.Close
      ref={ref}
      type="button"
      disabled={disabled || dismissLocked}
      className={buttonClassName({
        variant: "outline",
        size: "md",
        className: cn(className),
      })}
      {...props}
    />
  );
});
SlideOverCancelButton.displayName = "SlideOverCancelButton";

const DEFAULT_CLOSE = <SlideOverCloseButton />;

type SlideOverHeaderProps = React.HTMLAttributes<HTMLDivElement> & {
  /**
   * The close control on the right. Defaults to `SlideOverCloseButton`; pass
   * your own for an `aria-label` or `disabled`, `false` only where the panel
   * must not be closed from here.
   */
  close?: React.ReactNode;
};

const SlideOverHeader = ({
  className,
  close = DEFAULT_CLOSE,
  children,
  ...props
}: SlideOverHeaderProps) => (
  <div
    className={cn(
      "flex flex-row items-start justify-between gap-3 border-b border-slate-200 px-5 py-4",
      className,
    )}
    {...props}
  >
    <div className="flex min-w-0 flex-1 flex-col gap-1">{children}</div>
    {close}
  </div>
);
SlideOverHeader.displayName = "SlideOverHeader";

type SlideOverBodyProps = React.HTMLAttributes<HTMLDivElement> & {
  /**
   * The form's validation or save error. Rendered as an `Alert` at the top
   * of the body and scrolled into view when it changes. Field-level problems
   * additionally go to the field (`Input` `error`).
   */
  error?: FormErrorInput;
};

/**
 * The scrolling middle of a panel between `SlideOverHeader` and
 * `SlideOverFooter`. Carries the panel's padding and the error slot; the ref
 * reaches the scrolling element (`useApiFormError` scrolls it).
 */
const SlideOverBody = React.forwardRef<HTMLDivElement, SlideOverBodyProps>(
  ({ className, error, children, ...props }, ref) => (
    <div
      ref={ref}
      className={cn("flex-1 overflow-y-auto px-5 py-4", className)}
      {...props}
    >
      <DialogErrorSlot error={error} />
      {children}
    </div>
  ),
);
SlideOverBody.displayName = "SlideOverBody";

/**
 * The panel's action row (`DialogActions`). It locks every action in it
 * while the panel's save runs and while a closed panel slides out (#6593).
 * A dismissal action uses `SlideOverCancelButton`; an external dismissal
 * lock (`isDismissDisabled`) leaves save and submit actions usable.
 */
const SlideOverFooter = ({
  className,
  ...props
}: React.FieldsetHTMLAttributes<HTMLFieldSetElement>) => {
  const busy = React.useContext(SlideOverBusyContext);
  const open = React.useContext(SlideOverOpenContext);
  return (
    <DialogActions
      {...props}
      locked={busy || !open}
      className={cn(
        "flex-row border-t border-slate-200 bg-slate-50 px-5 py-4",
        className,
      )}
    />
  );
};
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

export {
  SlideOver,
  SlideOverContent,
  SlideOverHeader,
  SlideOverBody,
  SlideOverFooter,
  SlideOverTitle,
  SlideOverDescription,
  SlideOverCloseButton,
  SlideOverCancelButton,
};

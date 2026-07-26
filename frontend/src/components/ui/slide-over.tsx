"use client";

/**
 * SlideOver — right-side panel primitive built on Vaul.
 *
 * Companion to ./drawer.tsx (bottom-sheet). The two share the same Vaul
 * primitives but differ in `direction` and the resulting Tailwind layout.
 * Use SlideOver for desktop-first detail panels that need to coexist with
 * the underlying view (planner grid → click instance → slide-over from
 * right). Use Drawer for mobile bottom-sheets.
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

import { cn } from "~/lib/utils";

const SlideOver = ({
  shouldScaleBackground = false,
  ...props
}: React.ComponentProps<typeof DrawerPrimitive.Root>) => (
  <DrawerPrimitive.Root
    direction="right"
    shouldScaleBackground={shouldScaleBackground}
    {...props}
  />
);
SlideOver.displayName = "SlideOver";

const SlideOverPortal = DrawerPrimitive.Portal;

const SlideOverOverlay = React.forwardRef<
  React.ComponentRef<typeof DrawerPrimitive.Overlay>,
  React.ComponentPropsWithoutRef<typeof DrawerPrimitive.Overlay>
>(({ className, ...props }, ref) => (
  <DrawerPrimitive.Overlay
    ref={ref}
    className={cn(
      "fixed inset-0 z-40 bg-slate-900/30 backdrop-blur-[1px]",
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
   * Mobile (< sm) takes 92vw automatically.
   */
  widthClass?: string;
}

const SlideOverContent = React.forwardRef<
  React.ComponentRef<typeof DrawerPrimitive.Content>,
  SlideOverContentProps
>(({ className, widthClass, children, ...props }, ref) => (
  <SlideOverPortal>
    <SlideOverOverlay />
    <DrawerPrimitive.Content
      ref={ref}
      data-date-picker-focus-trap="true"
      className={cn(
        "fixed top-0 right-0 z-50 flex h-full flex-col bg-white shadow-2xl outline-none",
        "w-[92vw] sm:w-[420px]",
        widthClass,
        className,
      )}
      {...props}
    >
      {children}
    </DrawerPrimitive.Content>
  </SlideOverPortal>
));
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
 * canonical slide-over close (room-detail-modal's closeButtonClass). Renders a
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

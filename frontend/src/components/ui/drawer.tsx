"use client";

import * as React from "react";
import { Drawer as DrawerPrimitive } from "vaul";

import { cn } from "~/lib/utils";
import { useSheetAboveKeyboard } from "./hooks/useSheetAboveKeyboard";
import {
  OVERLAY_BACKDROP_CLASS,
  OVERLAY_BACKDROP_TINT_CLASS,
  Z_LAYER,
} from "./overlay-styles";

type DrawerDirection = "bottom" | "left";

// Mirror Vaul's own direction prop down to DrawerContent so consumers
// don't have to thread it twice. Bottom keeps the iOS-style drag handle;
// the side panel drops it (no swipe-to-dismiss UX on desktop).
const DrawerDirectionContext = React.createContext<DrawerDirection>("bottom");

// Vaul's props are a union (with or without snap points); omit per member.
type OmitEach<T, K extends PropertyKey> = T extends unknown
  ? Omit<T, K>
  : never;
type DrawerRootProps = OmitEach<
  React.ComponentProps<typeof DrawerPrimitive.Root>,
  "direction"
> & { readonly direction?: DrawerDirection };

const Drawer = ({
  shouldScaleBackground = true,
  direction = "bottom",
  ...props
}: DrawerRootProps) => (
  <DrawerDirectionContext.Provider value={direction}>
    <DrawerPrimitive.Root
      shouldScaleBackground={shouldScaleBackground}
      direction={direction}
      {...props}
    />
  </DrawerDirectionContext.Provider>
);
Drawer.displayName = "Drawer";

const DrawerPortal = DrawerPrimitive.Portal;

const DrawerOverlay = React.forwardRef<
  React.ComponentRef<typeof DrawerPrimitive.Overlay>,
  React.ComponentPropsWithoutRef<typeof DrawerPrimitive.Overlay>
>(({ className, ...props }, ref) => (
  <DrawerPrimitive.Overlay
    ref={ref}
    className={cn(
      `fixed inset-0 ${Z_LAYER.sheet}`,
      OVERLAY_BACKDROP_TINT_CLASS,
      OVERLAY_BACKDROP_CLASS,
      className,
    )}
    {...props}
  />
));
DrawerOverlay.displayName = DrawerPrimitive.Overlay.displayName;

// Position + shape per direction. Vaul handles the slide animation; we
// only own the static layout (which edges are pinned, which corners are
// rounded). The side panel uses square edges — review feedback:
// the rounded inner edge looked off against the page chrome, and the
// outer edge is flush against the viewport so nothing's there to round.
// Width caps live in className overrides at the call site.
const directionContentClasses: Record<DrawerDirection, string> = {
  bottom:
    "fixed inset-x-0 bottom-0 mt-24 flex h-auto max-h-[90vh] flex-col rounded-t-2xl",
  left: "fixed inset-y-0 left-0 flex h-full w-full max-w-lg flex-col",
};

const DrawerContent = React.forwardRef<
  React.ComponentRef<typeof DrawerPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof DrawerPrimitive.Content>
>(({ className, children, ...props }, ref) => {
  const direction = React.useContext(DrawerDirectionContext);
  const sheetRef = React.useRef<HTMLDivElement | null>(null);
  useSheetAboveKeyboard(sheetRef, direction === "bottom");
  return (
    <DrawerPortal>
      <DrawerOverlay />
      <DrawerPrimitive.Content
        ref={(node: HTMLDivElement | null) => {
          sheetRef.current = node;
          if (typeof ref === "function") ref(node);
          else if (ref) ref.current = node;
        }}
        className={cn(
          `${Z_LAYER.sheet} border-0 bg-white`,
          directionContentClasses[direction],
          className,
        )}
        {...props}
      >
        {direction === "bottom" && (
          // iOS-style drag handle, only on bottom sheet (matches the
          // original Vaul UX). Side panels are dismissed via close button
          // / overlay click.
          <div
            data-drawer-handle="true"
            aria-hidden="true"
            className="mx-auto mt-4 h-1 w-12 rounded-full bg-gray-300"
          />
        )}
        {children}
      </DrawerPrimitive.Content>
    </DrawerPortal>
  );
});
DrawerContent.displayName = "DrawerContent";

const DrawerHeader = ({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) => (
  <div
    className={cn("grid gap-1.5 p-4 text-center sm:text-left", className)}
    {...props}
  />
);
DrawerHeader.displayName = "DrawerHeader";

const DrawerTitle = React.forwardRef<
  React.ComponentRef<typeof DrawerPrimitive.Title>,
  React.ComponentPropsWithoutRef<typeof DrawerPrimitive.Title>
>(({ className, ...props }, ref) => (
  <DrawerPrimitive.Title
    ref={ref}
    className={cn(
      "text-lg leading-none font-semibold tracking-tight",
      className,
    )}
    {...props}
  />
));
DrawerTitle.displayName = DrawerPrimitive.Title.displayName;

const DrawerDescription = React.forwardRef<
  React.ComponentRef<typeof DrawerPrimitive.Description>,
  React.ComponentPropsWithoutRef<typeof DrawerPrimitive.Description>
>(({ className, ...props }, ref) => (
  <DrawerPrimitive.Description
    ref={ref}
    className={cn("text-sm text-gray-500", className)}
    {...props}
  />
));
DrawerDescription.displayName = DrawerPrimitive.Description.displayName;

// Vaul's Close primitive — used for explicit close buttons inside the
// drawer body (e.g. an X in the top-right of a slide-over). Wires up
// the underlying Radix Dialog close, so consumers don't have to re-pipe
// `onClose` through their own button.
const DrawerClose = DrawerPrimitive.Close;

export {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
  DrawerDescription,
};

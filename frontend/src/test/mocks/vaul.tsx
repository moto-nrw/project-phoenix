import {
  createContext,
  forwardRef,
  useContext,
  useRef,
  type ButtonHTMLAttributes,
  type HTMLAttributes,
  type MutableRefObject,
  type ReactNode,
} from "react";

/**
 * Stand-in for `vaul`, the drawer library behind the kit `SlideOver`. Vaul
 * renders nothing in jsdom, so a dialog built on it stays empty in a test.
 * The structure stays, only the animation layer goes; `Close` reports to
 * `onOpenChange` like the original. So do Escape on the panel and a click on
 * the backdrop (`[data-vaul-overlay]`), unless the panel's
 * `onEscapeKeyDown` / `onInteractOutside` rejects the event. The root shows
 * `dismissible` as `data-dismissible`; with `false` Vaul allows no swipe.
 *
 * Use: `vi.mock("vaul", () => import("~/test/mocks/vaul"));`
 */
const CloseContext = createContext<((open: boolean) => void) | undefined>(
  undefined,
);
const InteractOutsideContext = createContext<MutableRefObject<
  ((event: Event) => void) | undefined
> | null>(null);

function Root({
  children,
  open,
  onOpenChange,
  dismissible = true,
}: Readonly<{
  children: ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  dismissible?: boolean;
}>) {
  const interactOutside = useRef<((event: Event) => void) | undefined>(
    undefined,
  );
  if (open === false) return null;
  return (
    <CloseContext.Provider value={onOpenChange}>
      <InteractOutsideContext.Provider value={interactOutside}>
        <div data-dismissible={String(dismissible)}>{children}</div>
      </InteractOutsideContext.Provider>
    </CloseContext.Provider>
  );
}

function Portal({ children }: Readonly<{ children: ReactNode }>) {
  return <>{children}</>;
}

const Overlay = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  function Overlay(props, ref) {
    const onOpenChange = useContext(CloseContext);
    const interactOutside = useContext(InteractOutsideContext);
    return (
      // Stands in for Vaul's pointer-only backdrop; keyboard users close
      // with Escape on the panel.
      // oxlint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-static-element-interactions
      <div
        ref={ref}
        data-vaul-overlay=""
        {...props}
        onClick={() => {
          const event = new Event("pointerdown", { cancelable: true });
          interactOutside?.current?.(event);
          if (!event.defaultPrevented) onOpenChange?.(false);
        }}
      />
    );
  },
);

const Content = forwardRef<
  HTMLDivElement,
  HTMLAttributes<HTMLDivElement> & {
    onEscapeKeyDown?: (event: KeyboardEvent) => void;
    onInteractOutside?: (event: Event) => void;
  }
>(function Content({ onEscapeKeyDown, onInteractOutside, ...props }, ref) {
  const onOpenChange = useContext(CloseContext);
  const interactOutside = useContext(InteractOutsideContext);
  if (interactOutside) interactOutside.current = onInteractOutside;
  return (
    <div
      ref={ref}
      role="dialog"
      {...props}
      onKeyDownCapture={(event) => {
        props.onKeyDownCapture?.(event);
        if (event.key !== "Escape") return;
        onEscapeKeyDown?.(event.nativeEvent);
        if (!event.nativeEvent.defaultPrevented) onOpenChange?.(false);
      }}
    />
  );
});

const Close = forwardRef<
  HTMLButtonElement,
  ButtonHTMLAttributes<HTMLButtonElement>
>(function Close({ onClick, ...props }, ref) {
  const onOpenChange = useContext(CloseContext);
  return (
    <button
      ref={ref}
      {...props}
      onClick={(event) => {
        onClick?.(event);
        onOpenChange?.(false);
      }}
    />
  );
});

const Title = forwardRef<
  HTMLHeadingElement,
  HTMLAttributes<HTMLHeadingElement>
>(function Title({ children, ...props }, ref) {
  return (
    <h2 ref={ref} {...props}>
      {children ?? "Titel"}
    </h2>
  );
});

const Description = forwardRef<
  HTMLParagraphElement,
  HTMLAttributes<HTMLParagraphElement>
>(function Description(props, ref) {
  return <p ref={ref} {...props} />;
});

export const Drawer = {
  Root,
  Portal,
  Overlay,
  Content,
  Close,
  Title,
  Description,
};

import {
  createContext,
  forwardRef,
  useContext,
  type ButtonHTMLAttributes,
  type HTMLAttributes,
  type ReactNode,
} from "react";

/**
 * Stand-in for `vaul`, the drawer library behind the kit `SlideOver`. Vaul
 * renders nothing in jsdom, so a dialog built on it stays empty in a test.
 * The structure stays, only the animation layer goes; `Close` reports to
 * `onOpenChange` like the original.
 *
 * Use: `vi.mock("vaul", () => import("~/test/mocks/vaul"));`
 */
const CloseContext = createContext<((open: boolean) => void) | undefined>(
  undefined,
);

function Root({
  children,
  open,
  onOpenChange,
}: Readonly<{
  children: ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}>) {
  if (open === false) return null;
  return (
    <CloseContext.Provider value={onOpenChange}>
      <div>{children}</div>
    </CloseContext.Provider>
  );
}

function Portal({ children }: Readonly<{ children: ReactNode }>) {
  return <>{children}</>;
}

const Overlay = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  function Overlay(props, ref) {
    return <div ref={ref} {...props} />;
  },
);

const Content = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  function Content(props, ref) {
    return <div ref={ref} role="dialog" {...props} />;
  },
);

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

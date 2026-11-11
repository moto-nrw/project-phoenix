import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useEffect, useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { AnchoredPopover } from "./anchored-popover";

function NestedPopovers({
  onOuterOpenChange,
  onInnerOpenChange,
}: Readonly<{
  onOuterOpenChange: (open: boolean) => void;
  onInnerOpenChange: (open: boolean) => void;
}>) {
  const [innerOpen, setInnerOpen] = useState(false);
  useEffect(() => {
    setInnerOpen(true);
  }, []);

  return (
    <AnchoredPopover
      open
      onOpenChange={onOuterOpenChange}
      ariaLabel="Äußeres Popover"
      renderTrigger={({ ref }) => (
        <button ref={ref} type="button">
          Äußeres Popover öffnen
        </button>
      )}
    >
      {() => (
        <AnchoredPopover
          open={innerOpen}
          onOpenChange={(open) => {
            onInnerOpenChange(open);
            setInnerOpen(open);
          }}
          ariaLabel="Inneres Popover"
          renderTrigger={({ ref }) => (
            <button ref={ref} type="button">
              Inneres Popover öffnen
            </button>
          )}
        >
          {() => <p>Inhalt</p>}
        </AnchoredPopover>
      )}
    </AnchoredPopover>
  );
}

describe("AnchoredPopover im Overlay-Stapel", () => {
  it.each([
    ["vom Dokument", () => document.body],
    [
      "vom Auslöser des inneren Popovers",
      () => screen.getByRole("button", { name: "Inneres Popover öffnen" }),
    ],
  ])(
    "schließt mit Escape nur das innere Popover %s",
    async (_origin, target) => {
      const onOuterOpenChange = vi.fn();
      const onInnerOpenChange = vi.fn();
      render(
        <NestedPopovers
          onOuterOpenChange={onOuterOpenChange}
          onInnerOpenChange={onInnerOpenChange}
        />,
      );

      await waitFor(() =>
        expect(
          screen.getByRole("dialog", { name: "Inneres Popover" }),
        ).toBeInTheDocument(),
      );

      fireEvent.keyDown(target(), { key: "Escape" });

      expect(onInnerOpenChange).toHaveBeenCalledWith(false);
      expect(onOuterOpenChange).not.toHaveBeenCalled();
    },
  );
});

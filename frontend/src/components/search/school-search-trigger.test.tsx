import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SchoolSearchSlot } from "./school-search-slot";
import { SchoolSearchProvider } from "./school-search-trigger";

// The dialog's own behaviour is covered in school-search.test.tsx; here it
// only has to show where the trigger opens it.
vi.mock("./school-search", () => ({
  SchoolSearchDialog: ({ onClose }: { onClose: () => void }) => (
    <div role="dialog" aria-label="Überall suchen">
      <input aria-label="Suchbegriff" autoFocus onBlur={() => undefined} />
      <button type="button" onClick={onClose}>
        Schließen
      </button>
    </div>
  ),
}));

function renderHeader() {
  return render(
    <SchoolSearchProvider>
      <input aria-label="Notiz" />
      <SchoolSearchSlot placement="header" />
    </SchoolSearchProvider>,
  );
}

describe("SchoolSearchTrigger", () => {
  it("opens the search with Cmd/Ctrl+K and puts the focus into its field", async () => {
    renderHeader();
    fireEvent.keyDown(globalThis.window, { key: "k", metaKey: true });
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Suchbegriff" })).toHaveFocus();

    // The same shortcut closes it again, also from inside its field.
    fireEvent.keyDown(screen.getByRole("textbox", { name: "Suchbegriff" }), {
      key: "k",
      ctrlKey: true,
    });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(
      screen.getAllByRole("button", { name: /Überall suchen/ }).length,
    ).toBeGreaterThan(0);
  });

  it("leaves Cmd/Ctrl+K to a field that has the focus", async () => {
    renderHeader();
    fireEvent.keyDown(screen.getByRole("textbox", { name: "Notiz" }), {
      key: "k",
      metaKey: true,
    });
    // Let the dialog code arrive, so a wrong open could show.
    await act(async () => {
      await import("./school-search");
    });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    // Anchor: the button still opens the search on the same screen.
    fireEvent.click(
      screen.getAllByRole("button", { name: /Überall suchen/ })[0]!,
    );
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
  });

  it("opens with Cmd/Ctrl+K where only the page head carries the search (tablet)", async () => {
    render(
      <SchoolSearchProvider>
        <SchoolSearchSlot placement="pageHead" />
      </SchoolSearchProvider>,
    );
    fireEvent.keyDown(globalThis.window, { key: "k", ctrlKey: true });
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
  });

  it("opens one search for header and page head together", async () => {
    render(
      <SchoolSearchProvider>
        <SchoolSearchSlot placement="header" />
        <SchoolSearchSlot placement="pageHead" />
      </SchoolSearchProvider>,
    );
    fireEvent.keyDown(globalThis.window, { key: "k", metaKey: true });
    expect(await screen.findAllByRole("dialog")).toHaveLength(1);
  });

  it("opens from the phone's Mehr sheet and hands the focus back to its button", async () => {
    const closeSheet = vi.fn();
    function Sheet() {
      return (
        <>
          <button type="button" id="mehr">
            Mehr
          </button>
          <SchoolSearchSlot
            placement="moreSheet"
            onOpen={closeSheet}
            returnFocus={() => document.getElementById("mehr")}
          />
        </>
      );
    }
    render(
      <SchoolSearchProvider>
        <Sheet />
      </SchoolSearchProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: /Überall suchen/ }));
    expect(closeSheet).toHaveBeenCalled();
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Schließen" }));
    await vi.waitFor(() =>
      expect(screen.getByRole("button", { name: "Mehr" })).toHaveFocus(),
    );
  });
});

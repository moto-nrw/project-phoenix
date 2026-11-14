import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import {
  CommandPalette,
  CommandPaletteHighlight,
  matchRange,
  type CommandPaletteGroup,
} from "./command-palette";

function Palette({
  groups,
  placeholderContent,
}: {
  groups: CommandPaletteGroup[];
  placeholderContent?: React.ReactNode;
}) {
  const [query, setQuery] = useState("");
  return (
    <CommandPalette
      isOpen
      onClose={() => undefined}
      label="Suchen"
      query={query}
      onQueryChange={setQuery}
      placeholder="Name, Telefonnummer …"
      groups={groups}
      placeholderContent={placeholderContent}
      status="3 Treffer"
    />
  );
}

function groupsWith(select: (id: string) => void): CommandPaletteGroup[] {
  const item = (id: string, label: string) => ({
    id,
    label,
    title: label,
    onSelect: () => select(id),
  });
  return [
    {
      id: "children",
      title: "Kinder",
      items: [item("mia", "Mia"), item("ole", "Ole")],
    },
    { id: "empty", title: "Räume", items: [] },
    { id: "staff", title: "Mitarbeitende", items: [item("greta", "Greta")] },
  ];
}

function OpenedByButton() {
  const [isOpen, setIsOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setIsOpen(true)}>
        Suche öffnen
      </button>
      {isOpen ? (
        <CommandPalette
          isOpen
          onClose={() => setIsOpen(false)}
          label="Suchen"
          query=""
          onQueryChange={() => undefined}
          placeholder="Name, Telefonnummer …"
          groups={[]}
        />
      ) : null}
    </>
  );
}

describe("CommandPalette", () => {
  it("opens in its field and gives the focus back to what opened it", async () => {
    const user = userEvent.setup();
    render(<OpenedByButton />);
    const opener = screen.getByRole("button", { name: "Suche öffnen" });

    await user.click(opener);
    const field = await screen.findByRole("combobox", { name: "Suchen" });
    await vi.waitFor(() => expect(field).toHaveFocus());

    await user.keyboard("{Escape}");
    await vi.waitFor(() => expect(opener).toHaveFocus());
    expect(
      screen.queryByRole("combobox", { name: "Suchen" }),
    ).not.toBeInTheDocument();
  });

  it("keeps the focus in the field and walks every entry across the groups", async () => {
    const user = userEvent.setup();
    const select = vi.fn();
    render(<Palette groups={groupsWith(select)} />);

    const field = screen.getByRole("combobox", { name: "Suchen" });
    await user.click(field);
    expect(field).toHaveFocus();
    const options = screen.getAllByRole("option");
    expect(options.map((option) => option.getAttribute("aria-label"))).toEqual([
      "Mia",
      "Ole",
      "Greta",
    ]);
    expect(field).toHaveAttribute("aria-activedescendant", options[0]?.id);
    expect(options[0]).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByText("Räume")).not.toBeInTheDocument();

    await user.keyboard("{ArrowDown}{ArrowDown}");
    expect(field).toHaveAttribute("aria-activedescendant", options[2]?.id);
    await user.keyboard("{ArrowDown}");
    expect(field).toHaveAttribute("aria-activedescendant", options[0]?.id);
    await user.keyboard("{ArrowUp}");
    expect(field).toHaveAttribute("aria-activedescendant", options[2]?.id);
    expect(field).toHaveFocus();

    await user.keyboard("{Enter}");
    expect(select).toHaveBeenCalledWith("greta");
  });

  it("names its groups and announces the result", () => {
    render(<Palette groups={groupsWith(() => undefined)} />);
    expect(screen.getByRole("dialog", { name: "Suchen" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Kinder" })).toBeInTheDocument();
    expect(screen.getByRole("listbox", { name: "Suchen" })).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("3 Treffer");
  });

  it("opens an entry by click and shows the placeholder without entries", async () => {
    const user = userEvent.setup();
    const select = vi.fn();
    const { rerender } = render(<Palette groups={groupsWith(select)} />);
    await user.click(screen.getByRole("option", { name: "Ole" }));
    expect(select).toHaveBeenCalledWith("ole");

    rerender(
      <Palette groups={[]} placeholderContent={<p>Nichts gefunden</p>} />,
    );
    expect(screen.getByText("Nichts gefunden")).toBeInTheDocument();
    expect(screen.getByRole("combobox")).toHaveAttribute(
      "aria-expanded",
      "false",
    );
  });

  it("never opens a row of an older text while the new result is on its way", async () => {
    const user = userEvent.setup();
    const select = vi.fn();
    const props = {
      isOpen: true,
      onClose: () => undefined,
      label: "Suchen",
      query: "Mü",
      onQueryChange: () => undefined,
      placeholder: "Name …",
    } as const;
    const older = groupsWith(select);
    const { rerender } = render(
      <CommandPalette {...props} groups={older} busy />,
    );
    const field = screen.getByRole("combobox", { name: "Suchen" });
    await user.click(field);
    const stale = screen.getAllByRole("option");
    expect(stale[0]).toHaveAttribute("aria-disabled", "true");

    await user.keyboard("{ArrowDown}");
    expect(field).toHaveAttribute("aria-activedescendant", stale[0]?.id);
    await user.click(screen.getByRole("option", { name: "Ole" }));
    await user.keyboard("{Enter}");
    expect(select).not.toHaveBeenCalled();

    // The result for the typed text arrives: Enter opens its best hit.
    rerender(
      <CommandPalette
        {...props}
        groups={[
          {
            id: "children",
            title: "Kinder",
            items: [
              {
                id: "mueller",
                label: "Mia Müller",
                title: "Mia Müller",
                onSelect: () => select("mueller"),
              },
            ],
          },
        ]}
      />,
    );
    expect(select).toHaveBeenCalledTimes(1);
    expect(select).toHaveBeenCalledWith("mueller");
  });

  it("drops a waiting Enter when the person types on", async () => {
    const user = userEvent.setup();
    const select = vi.fn();
    const onQueryChange = vi.fn();
    const props = {
      isOpen: true,
      onClose: () => undefined,
      label: "Suchen",
      query: "Mü",
      onQueryChange,
      placeholder: "Name …",
      groups: groupsWith(select),
    } as const;
    const { rerender } = render(<CommandPalette {...props} busy />);
    await user.click(screen.getByRole("combobox", { name: "Suchen" }));
    await user.keyboard("{Enter}");
    await user.keyboard("l");
    expect(onQueryChange).toHaveBeenCalledWith("Mül");
    rerender(<CommandPalette {...props} query="Mül" />);
    expect(select).not.toHaveBeenCalled();
    // Anchor: without a waiting Enter the same rows open on Enter.
    await user.keyboard("{Enter}");
    expect(select).toHaveBeenCalledWith("mia");
  });

  it("keeps what a person types out of session recordings", () => {
    render(<Palette groups={[]} />);
    expect(
      screen.getByRole("combobox").closest("[data-analytics-block]"),
    ).not.toBeNull();
  });
});

describe("matchRange", () => {
  it("compares like a name search and maps back onto the original text", () => {
    expect(matchRange("Müller", "mull")).toEqual([0, 4]);
    expect(matchRange("Jens Großmann", "grossm")).toEqual([5, 10]); // "Großm"
    expect(matchRange("Mia", "xy")).toBeNull();
    expect(matchRange("Mia", "  ")).toBeNull();
  });

  it("marks the matched part", () => {
    render(<CommandPaletteHighlight text="Sabine Sonnenberg" query="sonnen" />);
    expect(
      screen.getByText("Sonnen", { selector: "mark" }),
    ).toBeInTheDocument();
  });
});

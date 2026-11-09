/**
 * DataTable with form cells (#6220): the phone form and the desktop table
 * run the same cell renderers. Only the layout for the current width may be
 * in the document, or a named control exists twice — duplicate ids break its
 * label, and the API error path focuses the hidden copy.
 */
import { AT_LEAST_MD, BELOW_MD } from "~/lib/hooks/use-media-query";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { useState } from "react";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { ToastProvider, useApiFormError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { duplicateIds } from "~/test/helpers/duplicate-ids";
import {
  DESKTOP_WIDTH,
  PHONE_WIDTH,
  setViewportWidth,
} from "~/test/helpers/viewport";

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: null, status: "unauthenticated" }),
}));

interface WageRow {
  id: string;
  label: string;
}

const wageRows: WageRow[] = [{ id: "night", label: "Nachtzuschlag" }];

const UNIT_OPTIONS = [
  { value: "hours", label: "Stunden" },
  { value: "days", label: "Tage" },
];

afterEach(() => setViewportWidth(DESKTOP_WIDTH));

function WageForm({ error }: { readonly error?: unknown }) {
  const errors = useApiFormError();
  const [amount, setAmount] = useState("12");
  const [unit, setUnit] = useState("hours");
  const columns: DataTableColumn<WageRow>[] = [
    {
      key: "label",
      header: "Kategorie",
      stacked: "title",
      render: (r) => r.label,
    },
    {
      key: "amount",
      header: "Lohnartnummer",
      render: (r) => (
        <Input
          name="amount"
          id="amount"
          aria-label={`Lohnartnummer ${r.label}`}
          value={amount}
          onChange={(event) => setAmount(event.target.value)}
          error={errors.fieldError("amount")}
        />
      ),
    },
    {
      key: "unit",
      header: "Einheit",
      render: (r) => (
        <CustomSelect
          name="unit"
          id="unit"
          ariaLabel={`Einheit ${r.label}`}
          value={unit}
          options={UNIT_OPTIONS}
          onChange={setUnit}
          error={errors.fieldError("unit")}
        />
      ),
    },
  ];
  return (
    <form>
      <FormErrorAlert message={errors.error} />
      <DataTable
        stackedOnMobile
        stackedLayout="form"
        columns={columns}
        rows={wageRows}
        getRowKey={(r) => r.id}
        rowHasInteractiveControls
      />
      <button
        type="button"
        onClick={() => void errors.show(error, { object: "die Lohnart" })}
      >
        Speichern
      </button>
    </form>
  );
}

function tree(error?: unknown) {
  return (
    <ToastProvider>
      <WageForm error={error} />
    </ToastProvider>
  );
}

const fieldError = (field: string) =>
  new ApiError("validation failed", 400, {
    code: "general.input",
    errors: [{ field, reason: "is invalid" }],
  });

const VIEWPORTS = [
  {
    name: "desktop",
    width: DESKTOP_WIDTH,
    visible: "data-table-table",
    hidden: "data-table-stacked",
  },
  {
    name: "phone",
    width: PHONE_WIDTH,
    visible: "data-table-stacked",
    hidden: "data-table-table",
  },
] as const;

describe.each(VIEWPORTS)(
  "DataTable form cells on $name (#6220)",
  (viewport) => {
    it("renders every field once, with one id and one name", () => {
      setViewportWidth(viewport.width);
      render(tree());

      expect(screen.getByTestId(viewport.visible)).toBeInTheDocument();
      expect(screen.queryByTestId(viewport.hidden)).not.toBeInTheDocument();
      expect(duplicateIds()).toEqual([]);
      expect(document.querySelectorAll('[name="amount"]')).toHaveLength(1);
      expect(document.querySelectorAll('[name="unit"]')).toHaveLength(1);
      // The field's label points at exactly one control.
      expect(
        screen.getByRole("textbox", { name: "Lohnartnummer Nachtzuschlag" }),
      ).toHaveAttribute("id", "amount");
    });

    it("focuses the visible field after an API field error", async () => {
      setViewportWidth(viewport.width);
      render(tree(fieldError("amount")));

      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      const visible = within(screen.getByTestId(viewport.visible));
      const field = visible.getByRole("textbox", {
        name: "Lohnartnummer Nachtzuschlag",
      });
      await waitFor(() => expect(field).toHaveFocus());
      expect(field).toHaveAttribute("aria-invalid", "true");
      expect(
        document.getElementById(field.getAttribute("aria-describedby")!),
      ).toBeInTheDocument();
    });

    it("focuses the visible select trigger after an API error on a CustomSelect", async () => {
      setViewportWidth(viewport.width);
      render(tree(fieldError("unit")));

      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      // The error re-renders the trigger: query it again on every check.
      const trigger = () =>
        within(screen.getByTestId(viewport.visible)).getByRole("combobox", {
          name: "Einheit Nachtzuschlag",
        });
      await waitFor(() => expect(trigger()).toHaveFocus());
      expect(trigger()).toHaveAttribute("aria-invalid", "true");
    });
  },
);

describe("DataTable form cells across a width change (#6220)", () => {
  it("keeps the entered value and swaps to the other layout", () => {
    setViewportWidth(DESKTOP_WIDTH);
    render(tree());
    // Prime Happy DOM's listener state before a matching -> non-matching resize.
    act(() => window.dispatchEvent(new Event("resize")));

    fireEvent.change(
      screen.getByRole("textbox", { name: "Lohnartnummer Nachtzuschlag" }),
      { target: { value: "4711" } },
    );
    setViewportWidth(PHONE_WIDTH);

    expect(screen.queryByTestId("data-table-table")).not.toBeInTheDocument();
    expect(
      within(screen.getByTestId("data-table-stacked")).getByRole("textbox", {
        name: "Lohnartnummer Nachtzuschlag",
      }),
    ).toHaveValue("4711");
    expect(duplicateIds()).toEqual([]);
  });

  it.each([
    { width: 767.5, desktop: false, fontSize: 16 },
    { width: 768, desktop: true, fontSize: 16 },
    { width: 800, desktop: false, fontSize: 20 },
    { width: 960, desktop: true, fontSize: 20 },
  ])(
    "matches the CSS md breakpoint at $width px ($fontSize px default font)",
    ({ width, desktop, fontSize = 16 }) => {
      // Model the browser's initial font size, which CSS rem media queries use.
      const matchMedia = vi
        .spyOn(window, "matchMedia")
        .mockImplementation((query) => ({
          media: query,
          matches:
            query === AT_LEAST_MD
              ? width >= 48 * fontSize
              : query === BELOW_MD && width < 48 * fontSize,
          onchange: null,
          addEventListener: vi.fn(),
          removeEventListener: vi.fn(),
          addListener: vi.fn(),
          removeListener: vi.fn(),
          dispatchEvent: vi.fn(() => true),
        }));
      try {
        render(tree());
        expect(
          screen.getByTestId(
            desktop ? "data-table-table" : "data-table-stacked",
          ),
        ).toBeInTheDocument();
        expect(
          screen.queryByTestId(
            desktop ? "data-table-stacked" : "data-table-table",
          ),
        ).not.toBeInTheDocument();
        expect(matchMedia).toHaveBeenCalledWith(AT_LEAST_MD);
      } finally {
        matchMedia.mockRestore();
      }
    },
  );

  it("hydrates the server markup without a mismatch, then keeps one layout", async () => {
    // The server cannot know the width: it sends both layouts and CSS picks.
    const html = renderToString(tree());
    const container = document.createElement("div");
    container.innerHTML = html;
    document.body.append(container);
    expect(
      container.querySelector('[data-testid="data-table-stacked"]'),
    ).not.toBeNull();
    expect(
      container.querySelector('[data-testid="data-table-table"]'),
    ).not.toBeNull();

    const recoverable = vi.fn();
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);
    try {
      let root: ReturnType<typeof hydrateRoot> | undefined;
      await act(async () => {
        root = hydrateRoot(container, tree(), {
          onRecoverableError: recoverable,
        });
      });

      expect(recoverable).not.toHaveBeenCalled();
      expect(consoleError.mock.calls.flat().map(String).join(" ")).not.toMatch(
        /hydrat/i,
      );
      expect(
        container.querySelector('[data-testid="data-table-stacked"]'),
      ).toBeNull();
      expect(
        container.querySelector('[data-testid="data-table-table"]'),
      ).not.toBeNull();
      expect(duplicateIds()).toEqual([]);
      act(() => root?.unmount());
    } finally {
      consoleError.mockRestore();
      container.remove();
    }
  });
});

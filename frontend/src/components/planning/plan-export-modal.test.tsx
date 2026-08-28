import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { isoDatePickerMock } from "~/test/mocks/date-picker";

import { PlanExportModal } from "./plan-export-modal";

const { mockExportPlan, mockToastError, mockToastSuccess } = vi.hoisted(() => ({
  mockExportPlan: vi.fn(),
  mockToastError: vi.fn(),
  mockToastSuccess: vi.fn(),
}));

// Vaul (SlideOver) rendert in jsdom nichts: der Export-Dialog bliebe im Test
// leer. Derselbe Ersatz wie in components/ui/slide-over.test.tsx.
vi.mock("vaul", async () => {
  const React = await import("react");

  return {
    Drawer: {
      Root: ({
        children,
        open,
      }: {
        children: React.ReactNode;
        open?: boolean;
      }) => (open === false ? null : <div>{children}</div>),
      Portal: ({ children }: { children: React.ReactNode }) => <>{children}</>,
      Overlay: React.forwardRef<
        HTMLDivElement,
        React.HTMLAttributes<HTMLDivElement>
      >((props, ref) => <div ref={ref} {...props} />),
      Content: React.forwardRef<
        HTMLDivElement,
        React.HTMLAttributes<HTMLDivElement>
      >((props, ref) => <div ref={ref} {...props} />),
      Close: React.forwardRef<
        HTMLButtonElement,
        React.ButtonHTMLAttributes<HTMLButtonElement>
      >((props, ref) => <button ref={ref} {...props} />),
      Title: React.forwardRef<
        HTMLHeadingElement,
        React.HTMLAttributes<HTMLHeadingElement>
      >(({ children, ...props }, ref) => (
        <h2 ref={ref} {...props}>
          {children ?? "Titel"}
        </h2>
      )),
      Description: React.forwardRef<
        HTMLParagraphElement,
        React.HTMLAttributes<HTMLParagraphElement>
      >((props, ref) => <p ref={ref} {...props} />),
    },
  };
});

vi.mock("~/lib/plan-export-api", async () => {
  const actual = await vi.importActual<typeof import("~/lib/plan-export-api")>(
    "~/lib/plan-export-api",
  );
  return {
    ...actual,
    exportPlan: (...args: unknown[]) => mockExportPlan(...args),
  };
});

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mockToastSuccess, error: mockToastError }),
}));

// The range rule is the subject here, not the calendar overlay, so the
// picker renders as the native input it replaced (src/test/mocks).
vi.mock("~/components/ui/date-picker", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...isoDatePickerMock(),
}));

function renderModal(
  props: Partial<React.ComponentProps<typeof PlanExportModal>> = {},
) {
  const onClose = vi.fn();
  render(
    <PlanExportModal
      isOpen
      plan="dienstplan"
      weekDay="2026-07-29"
      isWeekOnScreen
      canExportInternal={false}
      onClose={onClose}
      {...props}
    />,
  );
  return { onClose };
}

beforeEach(() => {
  mockExportPlan.mockResolvedValue(undefined);
});

afterEach(() => {
  vi.clearAllMocks();
});

describe("PlanExportModal", () => {
  it("exports the displayed week, not the day that was clicked", async () => {
    renderModal();

    fireEvent.click(screen.getByRole("button", { name: "PDF" }));

    await waitFor(() => expect(mockExportPlan).toHaveBeenCalled());
    expect(mockExportPlan).toHaveBeenCalledWith(
      "dienstplan",
      {
        from: "2026-07-29",
        to: "2026-07-29",
        template: "persons",
        variant: "aushang",
      },
      "pdf",
      "download",
      null,
    );
  });

  // The wall sheet is the safe default: it names cancellations but no reasons.
  it("defaults to the Aushang variant", () => {
    renderModal();

    expect(screen.getByRole("radio", { name: /Aushang/ })).toBeChecked();
    expect(
      screen.queryByRole("radio", { name: /Interne Fassung/ }),
    ).not.toBeInTheDocument();
  });

  it("passes the chosen row axis and variant through", async () => {
    renderModal({ canExportInternal: true });

    fireEvent.click(screen.getByRole("radio", { name: /Nach Einsatzbereich/ }));
    fireEvent.click(screen.getByRole("radio", { name: /Interne Fassung/ }));
    fireEvent.click(screen.getByRole("button", { name: "XLSX" }));

    await waitFor(() => expect(mockExportPlan).toHaveBeenCalled());
    expect(mockExportPlan).toHaveBeenCalledWith(
      "dienstplan",
      expect.objectContaining({ template: "areas", variant: "intern" }),
      "xlsx",
      "download",
      null,
    );
  });

  // One row axis means no picker: a single-option choice is noise.
  it("hides the template picker for the care plan", () => {
    renderModal({ plan: "betreuungsplan" });

    expect(screen.queryByText("Vorlage")).not.toBeInTheDocument();
  });

  it("states which days actually land on the sheet", () => {
    renderModal();

    expect(
      screen.getByText(
        /27\.07\.2026 bis 31\.07\.2026, Montag bis Freitag\. Samstag und Sonntag nur, wenn dort etwas geplant ist\./,
      ),
    ).toBeInTheDocument();
  });

  // Monats-, Serien- und Halbjahresansicht zeigen keine Woche: der Umschalter
  // darf die exportierte Woche dort nicht "angezeigt" nennen.
  it("stops calling the week displayed when no week is on screen", () => {
    renderModal({ isWeekOnScreen: false });

    expect(
      screen.getByRole("button", { name: "Einzelne Woche" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Angezeigte Woche" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(/Woche vom 27\.07\.2026 bis 31\.07\.2026/),
    ).toBeInTheDocument();
  });

  it("refuses a range beyond the eight-week cap instead of failing server-side", async () => {
    renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Mehrere Wochen" }));
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-07-27" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-10-30" },
    });

    await waitFor(() =>
      expect(screen.getByText(/höchstens 8 Wochen/)).toBeInTheDocument(),
    );
    expect(screen.getByRole("button", { name: "PDF" })).toBeDisabled();
    expect(mockExportPlan).not.toHaveBeenCalled();
  });

  it("reports a failed export instead of closing silently", async () => {
    mockExportPlan.mockRejectedValue(new Error("Backend weg"));
    const { onClose } = renderModal();

    fireEvent.click(screen.getByRole("button", { name: "PDF" }));

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith("Backend weg"),
    );
    expect(onClose).not.toHaveBeenCalled();
  });
});

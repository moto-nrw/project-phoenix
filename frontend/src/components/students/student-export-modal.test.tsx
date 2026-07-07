import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { StudentExportModal } from "./student-export-modal";

const { mockExportStudents, mockToastError, mockToastSuccess } = vi.hoisted(
  () => ({
    mockExportStudents: vi.fn(),
    mockToastError: vi.fn(),
    mockToastSuccess: vi.fn(),
  }),
);

vi.mock("~/lib/student-export-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/student-export-api")
  >("~/lib/student-export-api");
  return {
    ...actual,
    exportStudents: (...args: unknown[]) => mockExportStudents(...args),
  };
});

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
  }),
}));

function renderModal(
  props: Partial<React.ComponentProps<typeof StudentExportModal>> = {},
) {
  const onClose = vi.fn();
  render(
    <StudentExportModal
      isOpen
      filters={{ search: "mila", group_id: "5" }}
      resultCount={12}
      onClose={onClose}
      {...props}
    />,
  );
  return { onClose };
}

async function openModal(
  props: Partial<React.ComponentProps<typeof StudentExportModal>> = {},
) {
  const result = renderModal(props);
  await screen.findByRole("dialog", { name: "Kindersuche exportieren" });
  return result;
}

describe("StudentExportModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockExportStudents.mockResolvedValue(undefined);
  });

  it("does not render while closed", () => {
    renderModal({ isOpen: false });

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("renders the current count and closes from button, backdrop, and escape", async () => {
    const { onClose } = await openModal();

    expect(
      screen.getByText("12 Kinder aus der aktuellen Filterung."),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Export schließen" }));
    expect(onClose).toHaveBeenCalledTimes(1);

    const dialog = await screen.findByRole("dialog");
    fireEvent.mouseDown(dialog);
    expect(onClose).toHaveBeenCalledTimes(2);

    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(3);
  });

  it("updates title and active columns when a preset is selected", async () => {
    await openModal();

    fireEvent.click(screen.getByRole("button", { name: /Tagesliste/ }));

    await waitFor(() => {
      expect(screen.getByLabelText("Titel")).toHaveValue("Tagesliste");
    });
    expect(screen.getByText("7 aktiv")).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: /Tagesstatus/ })).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: /Tageshinweise/ }),
    ).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /Montag/ })).not.toBeChecked();
  });

  it("defaults to the class roster preset when a class filter is active", async () => {
    await openModal({
      filters: { search: "mila", school_class: "3a" },
    });

    await waitFor(() => {
      expect(screen.getByLabelText("Titel")).toHaveValue("Klassenliste");
    });
    expect(
      screen.getByRole("checkbox", {
        name: /Betreuungs-\/Anmeldestatus/,
      }),
    ).toBeChecked();
  });

  it("lets users edit title, format, and columns before exporting", async () => {
    const { onClose } = await openModal();

    fireEvent.change(screen.getByLabelText("Titel"), {
      target: { value: "Meine Liste" },
    });
    fireEvent.click(screen.getByRole("button", { name: "XLSX" }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Freitag/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Tageshinweise/ }));
    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));

    await waitFor(() => {
      expect(mockExportStudents).toHaveBeenCalledWith({
        format: "xlsx",
        preset: "ogs_weekly",
        title: "Meine Liste",
        filters: { search: "mila", group_id: "5" },
        columns: [
          "name",
          "school_class",
          "group",
          "weekly_monday",
          "weekly_tuesday",
          "weekly_wednesday",
          "weekly_thursday",
          "daily_notes",
        ],
      });
    });
    expect(mockToastSuccess).toHaveBeenCalledWith("Export wurde erstellt.");
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("shows a validation toast when all columns are disabled", async () => {
    await openModal();

    for (const checkbox of screen.getAllByRole("checkbox")) {
      if ((checkbox as HTMLInputElement).checked) {
        fireEvent.click(checkbox);
      }
    }
    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));

    expect(mockToastError).toHaveBeenCalledWith(
      "Bitte wähle mindestens eine Spalte aus.",
    );
    expect(mockExportStudents).not.toHaveBeenCalled();
  });

  it("enables grouping by class for the class roster preset without a class filter", async () => {
    await openModal();

    fireEvent.click(screen.getByRole("button", { name: /Klassenliste/ }));

    await waitFor(() => {
      expect(
        screen.getByRole("checkbox", { name: /Nach Klassen getrennt/ }),
      ).toBeChecked();
    });
    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));

    await waitFor(() => {
      expect(mockExportStudents).toHaveBeenCalledWith(
        expect.objectContaining({
          preset: "class_roster",
          filters: { search: "mila", group_id: "5", group_by_class: true },
        }),
      );
    });
  });

  it("exports without grouping when the checkbox is turned off", async () => {
    await openModal();

    fireEvent.click(screen.getByRole("button", { name: /Klassenliste/ }));
    await waitFor(() => {
      expect(
        screen.getByRole("checkbox", { name: /Nach Klassen getrennt/ }),
      ).toBeChecked();
    });
    fireEvent.click(
      screen.getByRole("checkbox", { name: /Nach Klassen getrennt/ }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));

    await waitFor(() => {
      expect(mockExportStudents).toHaveBeenCalledWith(
        expect.objectContaining({
          filters: { search: "mila", group_id: "5" },
        }),
      );
    });
  });

  it("hides the grouping option when a class filter is active", async () => {
    await openModal({
      filters: { search: "mila", school_class: "3a" },
    });

    expect(
      screen.queryByRole("checkbox", { name: /Nach Klassen getrennt/ }),
    ).not.toBeInTheDocument();
  });

  it("keeps the dialog open and reports export failures", async () => {
    mockExportStudents.mockRejectedValueOnce(new Error("PDF kaputt"));
    const { onClose } = await openModal();

    fireEvent.click(screen.getByRole("button", { name: "DOCX" }));
    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith("PDF kaputt");
    });
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });
});

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
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
  await screen.findByRole("dialog", { name: "Alle Kinder exportieren" });
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
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));

    fireEvent.click(
      screen.getByRole("button", {
        name: "Hintergrund - Klicken zum Schließen",
      }),
    );
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(2));

    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(3));
  });

  it("updates title and active columns when a preset is selected", async () => {
    await openModal();

    fireEvent.click(screen.getByRole("button", { name: /Tagesplanung/ }));

    await waitFor(() => {
      expect(screen.getByLabelText("Titel")).toHaveValue("Tagesplanung");
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

  it("treats one escaped class name as a single class (#2218)", async () => {
    // A class may legitimately be called "A,B", and it then travels escaped as
    // "A\,B". Splitting on the bare comma would read that as two classes and
    // offer per-class separation for a single-class export.
    await openModal({
      filters: { search: "mila", school_class: "A\\,B" },
    });

    expect(
      screen.queryByRole("checkbox", { name: /Nach Klassen getrennt/ }),
    ).not.toBeInTheDocument();
  });

  it("offers grouping when two classes are selected (#2218)", async () => {
    await openModal({
      filters: { search: "mila", school_class: "3a,4b" },
    });

    expect(
      screen.getByRole("checkbox", { name: /Nach Klassen getrennt/ }),
    ).toBeChecked();
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

  describe("birthday list", () => {
    beforeEach(() => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      vi.setSystemTime(new Date("2026-09-15T10:00:00Z"));
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    const selectBirthdayPreset = () =>
      fireEvent.click(
        screen.getByRole("button", { name: /^Geburtstagsliste/ }),
      );

    it("offers month selection only for the birthday preset", async () => {
      await openModal();

      expect(
        screen.queryByRole("button", { name: "September" }),
      ).not.toBeInTheDocument();

      selectBirthdayPreset();

      expect(
        await screen.findByRole("button", { name: "September" }),
      ).toBeInTheDocument();
      expect(screen.getByText("Geburtsmonate")).toBeInTheDocument();
    });

    it("preselects the current month", async () => {
      await openModal();
      selectBirthdayPreset();

      const september = await screen.findByRole("button", {
        name: "September",
      });
      expect(september).toHaveAttribute("aria-pressed", "true");
      expect(screen.getByRole("button", { name: "Januar" })).toHaveAttribute(
        "aria-pressed",
        "false",
      );
    });

    // A birthday recurs annually, so the export has to sort by calendar day —
    // the page's own sort mode would order by birth year.
    it("exports the current month sorted by birthday", async () => {
      await openModal();
      selectBirthdayPreset();
      await screen.findByRole("button", { name: "September" });

      fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));

      await waitFor(() => {
        expect(mockExportStudents).toHaveBeenCalledWith({
          format: "pdf",
          preset: "birthday_list",
          title: "Geburtstagsliste",
          filters: {
            search: "mila",
            group_id: "5",
            months: ["09"],
            sort: "birthday",
          },
          columns: ["name", "school_class", "group", "birthday", "age"],
        });
      });
    });

    it("sends the months a user picked, in calendar order", async () => {
      await openModal();
      selectBirthdayPreset();

      // Deselect September, then pick December before January to prove the
      // request is ordered rather than click-ordered.
      fireEvent.click(await screen.findByRole("button", { name: "September" }));
      fireEvent.click(screen.getByRole("button", { name: "Dezember" }));
      fireEvent.click(screen.getByRole("button", { name: "Januar" }));
      fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));

      await waitFor(() => {
        expect(mockExportStudents).toHaveBeenCalledWith(
          expect.objectContaining({
            filters: expect.objectContaining({ months: ["01", "12"] }),
          }),
        );
      });
    });

    it("exports the whole year when the month filter is cleared", async () => {
      await openModal();
      selectBirthdayPreset();

      fireEvent.click(
        await screen.findByRole("button", { name: "Ganzes Jahr" }),
      );

      expect(
        screen.getByText("Alle Geburtstage des Jahres werden aufgelistet."),
      ).toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));

      await waitFor(() => {
        expect(mockExportStudents).toHaveBeenCalledWith(
          expect.objectContaining({
            filters: expect.objectContaining({ months: [], sort: "birthday" }),
          }),
        );
      });
    });

    // Opening "Geburtstagsliste" must not present the other lists: the card
    // promised one thing, so the dialog offers exactly that.
    it("locks the dialog to the birthday list and hides the template grid", async () => {
      renderModal({
        filters: {},
        resultCount: undefined,
        heading: "Geburtstagsliste exportieren",
        lockedPreset: "birthday_list",
      });

      await screen.findByRole("dialog", {
        name: "Geburtstagsliste exportieren",
      });
      expect(screen.getByLabelText("Titel")).toHaveValue("Geburtstagsliste");
      expect(
        await screen.findByRole("button", { name: "September" }),
      ).toBeInTheDocument();

      expect(screen.queryByText("Vorlage")).not.toBeInTheDocument();
      for (const other of ["OGS Wochenliste", "Klassenliste", "Tagesplanung"]) {
        expect(
          screen.queryByRole("button", { name: new RegExp(`^${other}`) }),
        ).not.toBeInTheDocument();
      }
      expect(
        screen.queryByRole("checkbox", { name: /Nach Klassen getrennt/ }),
      ).not.toBeInTheDocument();
    });

    // A weekly matrix or a current-location snapshot has no place on a
    // birthday list.
    it("offers only the birthday list's own columns when locked", async () => {
      renderModal({
        filters: {},
        resultCount: undefined,
        lockedPreset: "birthday_list",
      });
      await screen.findByRole("dialog");

      for (const column of [
        "Geburtstag",
        "Alter",
        "Name",
        "Klasse",
        "Gruppe",
      ]) {
        expect(
          screen.getByRole("checkbox", { name: new RegExp(column) }),
        ).toBeChecked();
      }
      expect(screen.getByText("5 aktiv")).toBeInTheDocument();
      for (const column of [
        "Montag",
        "Tageshinweise",
        "Aktueller Aufenthaltsort",
      ]) {
        expect(
          screen.queryByRole("checkbox", { name: new RegExp(column) }),
        ).not.toBeInTheDocument();
      }
    });

    it("still exports the locked list correctly", async () => {
      renderModal({
        filters: {},
        resultCount: undefined,
        lockedPreset: "birthday_list",
      });
      await screen.findByRole("dialog");

      fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));

      await waitFor(() => {
        expect(mockExportStudents).toHaveBeenCalledWith({
          format: "pdf",
          preset: "birthday_list",
          title: "Geburtstagsliste",
          filters: { months: ["09"], sort: "birthday" },
          columns: ["name", "school_class", "group", "birthday", "age"],
        });
      });
    });

    // The Kindersuche has no birthday card of its own, so its export dialog
    // stays the only route there for staff without Datenverwaltung access.
    it("keeps the full template grid when not locked", async () => {
      await openModal();

      expect(screen.getByText("Vorlage")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /^Geburtstagsliste/ }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /^OGS Wochenliste/ }),
      ).toBeInTheDocument();
    });

    // Without a filtered list behind it, reporting "0 Kinder" would be a lie.
    it("describes the scope instead of a count when no count is given", async () => {
      renderModal({ filters: {}, resultCount: undefined });

      await screen.findByRole("dialog", { name: "Alle Kinder exportieren" });
      expect(screen.getByText("Alle Kinder der Schule.")).toBeInTheDocument();
    });
  });
});

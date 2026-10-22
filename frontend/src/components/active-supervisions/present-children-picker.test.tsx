import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PresentChildrenPicker } from "./present-children-picker";

const mocks = vi.hoisted(() => ({
  fetchStudents: vi.fn(),
  fetchCareOfferingOptions: vi.fn(),
}));

vi.mock("~/lib/student-api", () => ({
  fetchStudents: mocks.fetchStudents,
  fetchCareOfferingOptions: mocks.fetchCareOfferingOptions,
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "schule-am-berg",
}));

const OFFERING_STORAGE_KEY = "moto:present-children-offering:schule-am-berg";

vi.mock("~/components/ui/form-modal", () => ({
  FormModal: ({
    isOpen,
    children,
    footer,
    title,
  }: {
    isOpen: boolean;
    children: React.ReactNode;
    footer?: React.ReactNode;
    title: string;
  }) =>
    isOpen ? (
      <div data-testid="modal" data-title={title}>
        {children}
        {footer}
      </div>
    ) : null,
}));

// The shared test clock stands at 12:00 Berlin.
const presentStudents = [
  {
    id: 1,
    name: "Anna Abel",
    school_class: "2a",
    group_name: "Bären",
    current_location: "Anwesend - Turnhalle",
    pickup_time: "16:00",
  },
  {
    id: 2,
    name: "Ben Berg",
    school_class: "3b",
    current_location: "Anwesend",
    pickup_time: "11:45",
  },
  {
    id: 3,
    name: "Cem Celik",
    school_class: "1c",
    current_location: "Unterwegs",
  },
  {
    id: 4,
    name: "Dana Dorn",
    school_class: "4a",
    current_location: "Anwesend - Raum 4",
    pickup_time: "15:00",
  },
];

function renderPicker(
  overrides: Partial<React.ComponentProps<typeof PresentChildrenPicker>> = {},
) {
  const onAdd = overrides.onAdd ?? vi.fn().mockResolvedValue(true);
  const onClose = overrides.onClose ?? vi.fn();
  render(
    <PresentChildrenPicker
      isOpen
      instanceId="70"
      inBlockStudentIds={new Set(["4"])}
      isAdding={false}
      error={null}
      {...overrides}
      onAdd={onAdd}
      onClose={onClose}
    />,
  );
  return { onAdd, onClose };
}

describe("PresentChildrenPicker", () => {
  beforeEach(() => {
    globalThis.localStorage.clear();
    mocks.fetchStudents.mockReset();
    mocks.fetchStudents.mockResolvedValue({ students: presentStudents });
    mocks.fetchCareOfferingOptions.mockReset();
    mocks.fetchCareOfferingOptions.mockResolvedValue([]);
  });

  it("loads the present children with today's Gehzeit", async () => {
    renderPicker();

    await screen.findByText("Anna Abel");
    expect(mocks.fetchStudents).toHaveBeenCalledWith({
      location_state: "present",
      include_pickup_times: true,
      page: 1,
      page_size: 100,
    });
    expect(screen.getByText("geht 16:00")).toBeInTheDocument();
    expect(screen.getByText("ohne Gehzeit")).toBeInTheDocument();
    expect(screen.getByText("2a · Bären · Turnhalle")).toBeInTheDocument();
  });

  it("hides children who already left and children already in the block", async () => {
    renderPicker();

    await screen.findByText("Anna Abel");
    // Ben's Gehzeit 11:45 has passed; Dana is already in the block.
    expect(screen.queryByText("Ben Berg")).not.toBeInTheDocument();
    expect(screen.queryByText("Dana Dorn")).not.toBeInTheDocument();
    expect(screen.getByText("2 Kinder")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Alle anwesenden" }));

    expect(screen.getByText("Ben Berg")).toBeInTheDocument();
    expect(screen.queryByText("Dana Dorn")).not.toBeInTheDocument();
  });

  it("loads every page of present children", async () => {
    mocks.fetchStudents
      .mockResolvedValueOnce({
        students: [presentStudents[0]],
        pagination: {
          current_page: 1,
          page_size: 100,
          total_pages: 2,
          total_records: 2,
        },
      })
      .mockResolvedValueOnce({
        students: [presentStudents[2]],
        pagination: {
          current_page: 2,
          page_size: 100,
          total_pages: 2,
          total_records: 2,
        },
      });
    renderPicker();

    await screen.findByText("Anna Abel");
    expect(await screen.findByText("Cem Celik")).toBeInTheDocument();
    expect(mocks.fetchStudents).toHaveBeenNthCalledWith(1, {
      location_state: "present",
      include_pickup_times: true,
      page: 1,
      page_size: 100,
    });
    expect(mocks.fetchStudents).toHaveBeenNthCalledWith(2, {
      location_state: "present",
      include_pickup_times: true,
      page: 2,
      page_size: 100,
    });
  });

  it("adds the selected children together", async () => {
    const { onAdd, onClose } = renderPicker();

    await screen.findByText("Anna Abel");
    const submit = screen.getByRole("button", { name: "Kinder hinzufügen" });
    expect(submit).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Alle auswählen" }));
    fireEvent.click(
      screen.getByRole("button", { name: "2 Kinder hinzufügen" }),
    );

    await waitFor(() => expect(onAdd).toHaveBeenCalledWith(["1", "3"]));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("narrows the list by name and keeps the selection", async () => {
    const { onAdd } = renderPicker();

    await screen.findByText("Anna Abel");
    fireEvent.click(screen.getByRole("checkbox", { name: /Anna Abel/ }));
    fireEvent.change(
      screen.getByRole("searchbox", { name: "Anwesende Kinder durchsuchen" }),
      { target: { value: "cem" } },
    );

    expect(screen.queryByText("Anna Abel")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("checkbox", { name: /Cem Celik/ }));
    fireEvent.click(
      screen.getByRole("button", { name: "2 Kinder hinzufügen" }),
    );

    await waitFor(() => expect(onAdd).toHaveBeenCalledWith(["1", "3"]));
  });

  it("removes selections that do not match the current scope", async () => {
    const { onAdd } = renderPicker();

    await screen.findByText("Anna Abel");
    fireEvent.click(screen.getByRole("checkbox", { name: /Anna Abel/ }));
    fireEvent.click(screen.getByRole("button", { name: "Alle anwesenden" }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Ben Berg/ }));
    fireEvent.click(screen.getByRole("button", { name: "Noch in Betreuung" }));

    fireEvent.click(screen.getByRole("button", { name: "1 Kind hinzufügen" }));
    await waitFor(() => expect(onAdd).toHaveBeenCalledWith(["1"]));
  });

  it("keeps the dialog open when adding fails", async () => {
    const { onAdd, onClose } = renderPicker({
      onAdd: vi.fn().mockResolvedValue(false),
    });

    await screen.findByText("Anna Abel");
    fireEvent.click(screen.getByRole("checkbox", { name: /Anna Abel/ }));
    fireEvent.click(screen.getByRole("button", { name: "1 Kind hinzufügen" }));

    await waitFor(() => expect(onAdd).toHaveBeenCalled());
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("checkbox", { name: /Anna Abel/ })).toBeChecked();
  });

  it("says so when the list cannot be loaded", async () => {
    mocks.fetchStudents.mockRejectedValue(new Error("offline"));
    renderPicker();

    expect(
      await screen.findByText("Die Kinder konnten nicht geladen werden."),
    ).toBeInTheDocument();
  });

  it("points to all present children when nobody stays longer", async () => {
    mocks.fetchStudents.mockResolvedValue({
      students: [presentStudents[1]],
    });
    renderPicker();

    expect(
      await screen.findByText("Kein weiteres Kind bleibt noch länger."),
    ).toBeInTheDocument();
  });

  describe("Nach Angebot", () => {
    const offerings = [
      { id: "11", name: "OGS bis 16 Uhr" },
      { id: "12", name: "Kurzbetreuung" },
    ];
    // The backend has already narrowed the list to the offering.
    const offeringStudents = [presentStudents[1], presentStudents[3]];

    beforeEach(() => {
      mocks.fetchCareOfferingOptions.mockResolvedValue(offerings);
      mocks.fetchStudents.mockImplementation(
        ({ care_offering_id }: { care_offering_id?: string }) =>
          Promise.resolve({
            students: care_offering_id ? offeringStudents : presentStudents,
          }),
      );
    });

    it("is missing when the school has no active offering", async () => {
      mocks.fetchCareOfferingOptions.mockResolvedValue([]);
      renderPicker();

      await screen.findByText("Anna Abel");
      expect(
        screen.getByRole("button", { name: "Alle anwesenden" }),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Nach Angebot" }),
      ).not.toBeInTheDocument();
    });

    it("is missing when the offerings cannot be loaded", async () => {
      mocks.fetchCareOfferingOptions.mockRejectedValue(new Error("offline"));
      renderPicker();

      await screen.findByText("Anna Abel");
      expect(
        screen.queryByRole("button", { name: "Nach Angebot" }),
      ).not.toBeInTheDocument();
    });

    it("loads the children of the chosen offering from the backend", async () => {
      renderPicker();

      await screen.findByText("Anna Abel");
      fireEvent.click(screen.getByRole("button", { name: "Nach Angebot" }));

      // Ben's Gehzeit has passed, but the offering list keeps him; Dana is
      // already in the block.
      expect(await screen.findByText("Ben Berg")).toBeInTheDocument();
      expect(screen.queryByText("Anna Abel")).not.toBeInTheDocument();
      expect(screen.queryByText("Dana Dorn")).not.toBeInTheDocument();
      expect(mocks.fetchStudents).toHaveBeenLastCalledWith({
        location_state: "present",
        include_pickup_times: true,
        page: 1,
        page_size: 100,
        care_offering_id: "11",
      });
      expect(
        screen.getByText(
          "Ohne Kinder, die heute früher gehen oder abgemeldet sind.",
        ),
      ).toBeInTheDocument();
    });

    it("remembers the offering for the next opening", async () => {
      const { onClose } = renderPicker();

      await screen.findByText("Anna Abel");
      fireEvent.click(screen.getByRole("button", { name: "Nach Angebot" }));
      fireEvent.click(
        screen.getByRole("combobox", { name: "Betreuungsangebot" }),
      );
      fireEvent.click(screen.getByRole("option", { name: "Kurzbetreuung" }));
      await waitFor(() =>
        expect(mocks.fetchStudents).toHaveBeenLastCalledWith(
          expect.objectContaining({ care_offering_id: "12" }),
        ),
      );
      expect(globalThis.localStorage.getItem(OFFERING_STORAGE_KEY)).toBe("12");
      fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
      expect(onClose).toHaveBeenCalled();

      mocks.fetchStudents.mockClear();
      cleanup();
      renderPicker();

      expect(await screen.findByText("Ben Berg")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Nach Angebot" }),
      ).toHaveAttribute("aria-pressed", "true");
      expect(
        screen.getByRole("combobox", { name: "Betreuungsangebot" }),
      ).toHaveTextContent("Kurzbetreuung");
      expect(mocks.fetchStudents).toHaveBeenCalledTimes(1);
      expect(mocks.fetchStudents).toHaveBeenCalledWith(
        expect.objectContaining({ care_offering_id: "12" }),
      );
    });

    it("opens with the usual default when the stored offering is gone", async () => {
      globalThis.localStorage.setItem(OFFERING_STORAGE_KEY, "99");
      renderPicker();

      await screen.findByText("Anna Abel");
      expect(
        screen.getByRole("button", { name: "Noch in Betreuung" }),
      ).toHaveAttribute("aria-pressed", "true");
    });

    it("forgets the offering when another choice is made", async () => {
      globalThis.localStorage.setItem(OFFERING_STORAGE_KEY, "11");
      renderPicker();

      expect(await screen.findByText("Ben Berg")).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Alle anwesenden" }));

      expect(await screen.findByText("Anna Abel")).toBeInTheDocument();
      expect(globalThis.localStorage.getItem(OFFERING_STORAGE_KEY)).toBeNull();
    });

    it("points to all present children when nobody booked the offering", async () => {
      globalThis.localStorage.setItem(OFFERING_STORAGE_KEY, "11");
      mocks.fetchStudents.mockImplementation(
        ({ care_offering_id }: { care_offering_id?: string }) =>
          Promise.resolve({
            students: care_offering_id ? [] : presentStudents,
          }),
      );
      renderPicker();

      expect(
        await screen.findByText(
          "Kein weiteres anwesendes Kind hat dieses Angebot heute gebucht.",
        ),
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          "Unter „Alle anwesenden“ sehen Sie alle Kinder in der OGS.",
        ),
      ).toBeInTheDocument();
    });
  });
});

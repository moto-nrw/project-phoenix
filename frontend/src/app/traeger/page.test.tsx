import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const { mockFetchSchools } = vi.hoisted(() => ({
  mockFetchSchools: vi.fn(),
}));

vi.mock("~/lib/traeger-api", () => ({
  fetchTraegerSchools: () => mockFetchSchools(),
}));

import TraegerSchoolsPage from "./page";

describe("TraegerSchoolsPage", () => {
  beforeEach(() => {
    mockFetchSchools.mockReset();
  });

  it("lists every school of the Träger as a read-only table", async () => {
    mockFetchSchools.mockResolvedValue([
      {
        id: "2",
        name: "OGS Am Bach",
        slug: "am-bach",
        subdomain: "am-bach",
        city: "Bissendorf",
      },
      {
        id: "3",
        name: "OGS Lindenweg",
        slug: "lindenweg",
        subdomain: "lindenweg",
        city: "",
      },
    ]);
    render(<TraegerSchoolsPage />);

    const table = await screen.findByRole("table");
    expect(within(table).getByText("OGS Am Bach")).toBeInTheDocument();
    expect(within(table).getByText("Bissendorf")).toBeInTheDocument();
    // A school without a city shows the empty-value glyph.
    expect(within(table).getByText("–")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Alle Schulen Ihres Trägers. Neue Schulen ordnet das moto-Team zu.",
      ),
    ).toBeInTheDocument();
    // Anker: die Tabelle ist geladen; Zeilen sind reine Anzeige.
    expect(within(table).getByText("OGS Lindenweg")).toBeInTheDocument();
    expect(within(table).queryAllByRole("button", { name: /OGS/ })).toEqual([]);
  });

  it("says who adds schools when the Träger has none yet", async () => {
    mockFetchSchools.mockResolvedValue([]);
    render(<TraegerSchoolsPage />);

    // Table and phone layout share the empty state.
    const empty = await screen.findAllByText(
      "Ihrem Träger sind noch keine Schulen zugeordnet.",
    );
    expect(empty[0]).toBeInTheDocument();
    expect(
      screen.getAllByText(
        "Das moto-Team ordnet die Schulen Ihrem Träger zu.",
      )[0],
    ).toBeInTheDocument();
  });

  it("shows a failed load where the list is missing and retries", async () => {
    mockFetchSchools
      .mockRejectedValueOnce(
        new ApiError("unavailable", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce([
        {
          id: "2",
          name: "OGS Am Bach",
          slug: "am-bach",
          subdomain: "am-bach",
          city: "Bissendorf",
        },
      ]);
    render(<TraegerSchoolsPage />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Schulen"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect((await screen.findAllByText("OGS Am Bach"))[0]).toBeInTheDocument();
    expect(mockFetchSchools).toHaveBeenCalledTimes(2);
  });
});

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { KlassenTab } from "./klassen-tab";

const { mockAuthFetch } = vi.hoisted(() => ({ mockAuthFetch: vi.fn() }));

vi.mock("~/lib/api-helpers", () => ({ authFetch: mockAuthFetch }));

function mockLoad(assigned: string[], known: string[] = ["1a", "2b", "3c"]) {
  mockAuthFetch.mockImplementation(
    (url: string, init?: { method?: string }) => {
      if (url === "/api/students/school-classes") {
        return Promise.resolve({ success: true, data: known });
      }
      if (init?.method === "PUT") {
        const body = (init as { body: { school_classes: string[] } }).body;
        return Promise.resolve({
          success: true,
          data: { staff_id: 7, school_classes: body.school_classes },
        });
      }
      return Promise.resolve({
        success: true,
        data: { staff_id: 7, school_classes: assigned },
      });
    },
  );
}

const putCalls = () =>
  mockAuthFetch.mock.calls.filter(
    ([, init]) => (init as { method?: string } | undefined)?.method === "PUT",
  );

describe("KlassenTab", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });

  it("shows the assignment read-only and offers Bearbeiten only with the right", async () => {
    mockLoad(["1a"]);
    const { rerender } = render(<KlassenTab staffId="7" canEdit={false} />);

    expect(await screen.findByText("Klasse 1a")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Klasse 1a entfernen" }),
    ).not.toBeInTheDocument();

    rerender(<KlassenTab staffId="7" canEdit />);
    expect(
      await screen.findByRole("button", { name: "Bearbeiten" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Klasse 1a entfernen" }),
    ).not.toBeInTheDocument();
  });

  it("writes the whole set once on Speichern, not on every chip change", async () => {
    mockLoad(["1a"]);
    render(<KlassenTab staffId="7" canEdit />);

    fireEvent.click(await screen.findByRole("button", { name: "Bearbeiten" }));

    const input = screen.getByLabelText("Klassenname");
    fireEvent.change(input, { target: { value: "2b" } });
    fireEvent.click(screen.getByRole("button", { name: "Hinzufügen" }));
    expect(screen.getByText("Klasse 2b")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Klasse 1a entfernen" }),
    );
    expect(screen.queryByText("Klasse 1a")).not.toBeInTheDocument();

    expect(putCalls()).toHaveLength(0);

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(putCalls()).toHaveLength(1));
    expect(putCalls()[0]?.[0]).toBe("/api/staff/7/school-classes");
    expect(putCalls()[0]?.[1]).toMatchObject({
      method: "PUT",
      body: { school_classes: ["2b"] },
    });

    // Zurück im Lesezustand: kein Entfernen mehr, Bearbeiten wieder da.
    expect(
      await screen.findByRole("button", { name: "Bearbeiten" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Klasse 2b entfernen" }),
    ).not.toBeInTheDocument();
  });

  it("discards the draft on Abbrechen without writing", async () => {
    mockLoad(["1a"]);
    render(<KlassenTab staffId="7" canEdit />);

    fireEvent.click(await screen.findByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Klasse 1a entfernen" }),
    );
    expect(
      screen.getByText("Noch keine Klasse zugewiesen."),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    expect(screen.getByText("Klasse 1a")).toBeInTheDocument();
    expect(putCalls()).toHaveLength(0);
  });

  it("keeps the draft and shows an Alert when saving fails", async () => {
    mockLoad(["1a"]);
    render(<KlassenTab staffId="7" canEdit />);
    fireEvent.click(await screen.findByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Klasse 1a entfernen" }),
    );

    mockAuthFetch.mockImplementationOnce(() =>
      Promise.reject(new Error("boom")),
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Die Klassen-Zuweisung konnte nicht gespeichert werden.",
      ),
    ).toBeInTheDocument();
    // Der Entwurf bleibt: das Kind ist weiterhin entfernt, Speichern erneut möglich.
    expect(screen.queryByText("Klasse 1a")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
  });
});

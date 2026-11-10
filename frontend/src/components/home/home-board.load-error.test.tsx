import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";

import { HomeBoard, preloadHomeBoardEditor } from "./home-board";

it("behält nach fehlgeschlagenem Vorladen und Öffnen das Brett und lädt beim Wiederholen neu", async () => {
  vi.doMock("./home-board-editor", () => {
    throw new Error("Failed to fetch dynamically imported module");
  });

  await preloadHomeBoardEditor();
  render(
    <HomeBoard
      placements={[{ key: "tile.students_present", span: 1, col: 0, row: 0 }]}
      addable={[]}
      requiredKeys={new Set()}
      editing
      onMove={() => {}}
      onMoveBy={() => {}}
      onSpanChange={() => {}}
      onRemove={() => {}}
      onAdd={() => {}}
      onRestoreDefault={() => {}}
      restoring={false}
    >
      {() => <span>Lesbare Kennzahl</span>}
    </HomeBoard>,
  );

  expect(await screen.findByRole("alert")).toHaveTextContent(
    /Anpassen der Startseite/,
  );
  expect(screen.getByText("Lesbare Kennzahl")).toBeInTheDocument();

  vi.doMock("./home-board-editor", () =>
    vi.importActual("./home-board-editor"),
  );
  fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

  // Anker: Nach Wiederherstellen der Verbindung ist der echte Editor bedienbar.
  expect(
    await screen.findByRole("button", { name: /^Kinder anwesend auswählen/ }),
  ).toBeInTheDocument();
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

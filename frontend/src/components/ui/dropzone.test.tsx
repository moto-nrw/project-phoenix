import { createEvent, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Dropzone } from "./dropzone";

const baseProps = {
  accept: ".csv",
  legend: "Bereich zum Hochladen",
  label: "Datei hierher ziehen",
  dragLabel: "Datei hier ablegen",
  buttonLabel: "Datei auswählen",
};

const csv = new File(["a"], "a.csv", { type: "text/csv" });
const otherCsv = new File(["b"], "b.csv", { type: "text/csv" });

function area() {
  return screen.getByRole("group", { name: "Bereich zum Hochladen" });
}

// jsdom's drag events take no relatedTarget in their init.
function leaveTowards(next: Element) {
  const event = createEvent.dragLeave(area());
  Object.defineProperty(event, "relatedTarget", { value: next });
  fireEvent(area(), event);
}

describe("Dropzone", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("names the area and shows the limits", () => {
    render(<Dropzone {...baseProps} hint="Höchstens 10 MB" onFile={vi.fn()} />);

    expect(area()).toBeInTheDocument();
    expect(screen.getByText("Datei hierher ziehen")).toBeInTheDocument();
    expect(screen.getByText("Höchstens 10 MB")).toBeInTheDocument();
    expect(screen.getByLabelText("Datei auswählen")).toHaveAttribute(
      "accept",
      ".csv",
    );
  });

  it.each(["{Enter}", " "])("opens the file dialog with %j", async (key) => {
    const click = vi.spyOn(HTMLInputElement.prototype, "click");
    render(<Dropzone {...baseProps} onFile={vi.fn()} />);

    screen.getByRole("button", { name: "Datei auswählen" }).focus();
    await userEvent.keyboard(key);

    expect(click).toHaveBeenCalledTimes(1);
  });

  it("hands over a chosen file and clears the input for the next choice", () => {
    const onFile = vi.fn();
    render(<Dropzone {...baseProps} onFile={onFile} />);
    const input = screen.getByLabelText<HTMLInputElement>("Datei auswählen");

    fireEvent.change(input, { target: { files: [csv] } });

    expect(onFile).toHaveBeenCalledExactlyOnceWith(csv);
    expect(input.value).toBe("");
  });

  it("hands over only the first of several dropped files by default", () => {
    const onFile = vi.fn();
    render(<Dropzone {...baseProps} onFile={onFile} />);

    fireEvent.drop(area(), { dataTransfer: { files: [csv, otherCsv] } });

    expect(onFile).toHaveBeenCalledExactlyOnceWith(csv);
  });

  it("hands over every dropped file with multiple", () => {
    const onFiles = vi.fn();
    render(<Dropzone {...baseProps} multiple onFiles={onFiles} />);

    fireEvent.drop(area(), { dataTransfer: { files: [csv, otherCsv] } });

    expect(onFiles).toHaveBeenCalledExactlyOnceWith([csv, otherCsv]);
  });

  it("says where to drop while a file is held over it", () => {
    render(<Dropzone {...baseProps} onFile={vi.fn()} />);

    fireEvent.dragEnter(area());
    expect(screen.getByText("Datei hier ablegen")).toBeInTheDocument();

    // Moving onto the button inside stays over the area.
    leaveTowards(screen.getByRole("button"));
    expect(screen.getByText("Datei hier ablegen")).toBeInTheDocument();

    leaveTowards(document.body);
    expect(screen.getByText("Datei hierher ziehen")).toBeInTheDocument();
  });

  it("accepts the drag, or the browser would not fire a drop at all", () => {
    render(<Dropzone {...baseProps} onFile={vi.fn()} />);

    // fireEvent returns false when the handler called preventDefault.
    expect(fireEvent.dragOver(area())).toBe(false);
  });

  it("clears the drag message after a drop", () => {
    render(<Dropzone {...baseProps} onFile={vi.fn()} />);

    fireEvent.dragEnter(area());
    fireEvent.drop(area(), { dataTransfer: { files: [csv] } });

    expect(screen.getByText("Datei hierher ziehen")).toBeInTheDocument();
  });

  it("stops the browser from opening a file dropped while disabled", () => {
    const onFile = vi.fn();
    render(<Dropzone {...baseProps} disabled onFile={onFile} />);

    // fireEvent returns false when the handler called preventDefault.
    const notPrevented = fireEvent.drop(area(), {
      dataTransfer: { files: [csv] },
    });

    expect(notPrevented).toBe(false);
    expect(onFile).not.toHaveBeenCalled();
    expect(screen.getByRole("button")).toBeDisabled();
  });

  it("takes no file while loading and says what is happening", () => {
    const onFile = vi.fn();
    render(
      <Dropzone
        {...baseProps}
        loading
        loadingLabel="Wird hochgeladen…"
        onFile={onFile}
      />,
    );

    fireEvent.drop(area(), { dataTransfer: { files: [csv] } });

    expect(onFile).not.toHaveBeenCalled();
    expect(screen.getByText("Wird hochgeladen…")).toBeInTheDocument();
    expect(screen.queryByText("Datei hierher ziehen")).not.toBeInTheDocument();
    expect(area()).toHaveAttribute("aria-busy", "true");
    expect(screen.getByRole("button")).toBeDisabled();
  });
});

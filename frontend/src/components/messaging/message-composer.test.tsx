import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { MessageComposer } from "./message-composer";

function ParentComposer({ onSend }: Readonly<{ onSend: () => void }>) {
  const [value, setValue] = useState("");
  return (
    <MessageComposer
      value={value}
      onChange={setValue}
      onSend={onSend}
      sending={false}
      tone="parent"
      sendLabel="Senden"
      fieldLabel="Nachricht"
    />
  );
}

describe("MessageComposer in der Eltern-App", () => {
  it("behält für den Icon-Button den zugänglichen Namen und sendet", async () => {
    const user = userEvent.setup();
    const onSend = vi.fn();
    render(<ParentComposer onSend={onSend} />);

    const send = screen.getByRole("button", { name: "Senden" });
    expect(send).toBeDisabled();

    await user.type(
      screen.getByRole("textbox", { name: "Nachricht" }),
      "Hallo",
    );
    expect(send).toBeEnabled();

    await user.click(send);
    expect(onSend).toHaveBeenCalledOnce();
  });
});

import { fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import { TodoList, type TodoItem } from "./todo-list";

const items: TodoItem[] = Array.from({ length: 6 }, (_, index) => ({
  key: `item-${index + 1}`,
  concept: "parentMessages",
  title: `Offener Punkt ${index + 1}`,
  context: "Elternbrief · Demo School",
  href: `/parents/news/${index + 1}`,
}));

function renderList() {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <TodoList items={items} />
    </NextIntlClientProvider>,
  );
}

describe("TodoList", () => {
  it("zeigt zuerst fünf Punkte und lässt die restlichen ein- und ausklappen", () => {
    renderList();

    expect(screen.getByText("6 offen")).toBeInTheDocument();
    const list = document.querySelector("#parent-todo-items");
    expect(list).toHaveClass("divide-y", "divide-gray-100");
    expect(list).not.toHaveClass("space-y-1");
    expect(screen.getByText("Offener Punkt 5")).toBeInTheDocument();
    expect(screen.queryByText("Offener Punkt 6")).not.toBeInTheDocument();

    const showMore = screen.getByRole("button", {
      name: "Einen weiteren Punkt anzeigen",
    });
    expect(showMore).toHaveAttribute("aria-expanded", "false");

    fireEvent.click(showMore);

    expect(screen.getByText("Offener Punkt 6")).toBeInTheDocument();
    const showLess = screen.getByRole("button", { name: "Weniger anzeigen" });
    expect(showLess).toHaveAttribute("aria-expanded", "true");

    fireEvent.click(showLess);

    expect(screen.queryByText("Offener Punkt 6")).not.toBeInTheDocument();
  });

  it("markiert ungelesene Einträge am Icon und lässt die Zeile im Ruhezustand weiß", () => {
    render(
      <NextIntlClientProvider locale="de" messages={deMessages}>
        <TodoList items={[{ ...items[0]!, unread: true }]} />
      </NextIntlClientProvider>,
    );

    const row = screen.getByRole("link", {
      name: /Ungelesen.*Offener Punkt 1/,
    });
    const indicator = screen.getByTestId("todo-unread-indicator");

    expect(row).not.toHaveClass("bg-gray-50");
    expect(row).toHaveClass("hover:bg-gray-50", "active:bg-gray-100");
    expect(indicator).toHaveClass("bg-moto-blue", "border-2", "border-white");
  });
});

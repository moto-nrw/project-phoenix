import type { TenantPageProps } from "~/components/ui/tenant-page";

/**
 * The two registers of „Nachrichten“ (#4784): the conversations with one
 * family each, and the Gruppennachrichten to a whole group, class or AG.
 * Each register is its own page, so the tabs are links.
 */
export type MessagesTab = "conversations" | "groups";

export function messagesTabs(
  value: MessagesTab,
): NonNullable<TenantPageProps["tabs"]> {
  return {
    value,
    // Href-Reiter navigieren selbst; der Wechsel braucht keinen Zustand.
    onChange: () => undefined,
    label: "Art der Nachrichten",
    items: [
      { value: "conversations", label: "Unterhaltungen", href: "/messages" },
      {
        value: "groups",
        label: "Gruppennachrichten",
        href: "/messages/groups",
      },
    ],
  };
}

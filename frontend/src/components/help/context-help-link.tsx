"use client";

import { usePathname, useSearchParams } from "next/navigation";
import { QuestionIcon } from "@phosphor-icons/react";
import { ButtonLink } from "~/components/ui/button";
import { Tooltip } from "~/components/ui/tooltip";
import { useShellAuth } from "~/lib/shell-auth-context";
import {
  useNFCEnabled,
  useOpenCareGroupMode,
  usePresenceMode,
} from "~/lib/tenant-context";
import type { HelpTopicId } from "~/lib/help-topics";
import { cn } from "~/lib/utils";

interface ContextHelpLinkProps {
  readonly topic: HelpTopicId;
  readonly className?: string;
  readonly containerClassName?: string;
}

interface ContextHelpHrefOptions {
  readonly topic: HelpTopicId;
  readonly role: "caregiver" | "lead";
  readonly nfcEnabled: boolean;
  readonly presenceMode: "detailed" | "binary";
  readonly groupMode: "fixed_groups" | "open_care";
  readonly returnTo: string;
}

export function buildContextHelpHref({
  topic,
  role,
  nfcEnabled,
  presenceMode,
  groupMode,
  returnTo,
}: ContextHelpHrefOptions): string {
  const query = new URLSearchParams({
    role,
    nfc_enabled: String(nfcEnabled),
    presence_mode: presenceMode,
    group_mode: groupMode,
    return_to: returnTo,
  });

  return `/help/prototype/${encodeURIComponent(topic)}?${query.toString()}`;
}

export function ContextHelpLink({
  topic,
  className,
  containerClassName,
}: Readonly<ContextHelpLinkProps>) {
  const rawPathname = usePathname();
  const searchParams = useSearchParams();
  const { user } = useShellAuth();
  const nfcEnabled = useNFCEnabled();
  const presenceMode = usePresenceMode();
  const openCareGroupMode = useOpenCareGroupMode();
  const currentQuery = searchParams.toString();
  const returnTo = currentQuery
    ? `${rawPathname}?${currentQuery}`
    : rawPathname;
  const href = buildContextHelpHref({
    topic,
    role: user?.roles.includes("admin") ? "lead" : "caregiver",
    nfcEnabled,
    presenceMode,
    groupMode: openCareGroupMode ? "open_care" : "fixed_groups",
    returnTo,
  });

  return (
    <Tooltip
      content="Hilfe zu dieser Seite"
      asChild
      className={containerClassName}
      bubbleClassName="left-1/2 -translate-x-1/2"
    >
      <ButtonLink
        href={href}
        variant="ghost"
        size="icon"
        aria-label="Hilfe zu dieser Seite"
        className={cn("size-10 shrink-0 rounded-full", className)}
      >
        <QuestionIcon className="size-5" weight="bold" aria-hidden="true" />
      </ButtonLink>
    </Tooltip>
  );
}

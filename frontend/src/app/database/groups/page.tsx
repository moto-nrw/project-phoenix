"use client";

import { DatabasePage } from "@/components/ui/database/database-page";
import { groupsConfig } from "@/lib/database/configs/groups.config";

export default function GroupsPage() {
  return <DatabasePage config={groupsConfig} />;
}

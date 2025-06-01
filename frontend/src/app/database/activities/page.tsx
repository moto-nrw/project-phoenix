"use client";

import { DatabasePage } from "@/components/ui/database/database-page";
import { activitiesConfig } from "@/lib/database/configs/activities.config";

export default function ActivitiesPage() {
  return <DatabasePage config={activitiesConfig} />;
}
"use client";

import { DatabasePage } from "@/components/ui/database/database-page";
import { devicesConfig } from "@/lib/database/configs/devices.config";

export default function DevicesPage() {
  return <DatabasePage config={devicesConfig} />;
}
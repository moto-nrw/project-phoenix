import { NextResponse } from "next/server";

// Stub route required by Turbopack to register deeper /[childId]/decide
// route. There is no public collection endpoint at this level; admin
// reads come from the parent /admin/requests/{id} aggregate.
export async function GET() {
  return NextResponse.json({ error: "Method Not Allowed" }, { status: 405 });
}

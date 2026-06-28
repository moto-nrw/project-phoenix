/**
 * Single source of truth for the parent-OGS messaging wire types, shared by the
 * staff (tenant) portal and the parents portal so the two cannot drift.
 */

/** Who sent a message: the guardian or the OGS staff. */
export type MessageSenderKind = "guardian" | "staff";

/**
 * One message in a parent-OGS conversation, as it arrives over the wire (int64
 * ids already stringified by the backend). Shared verbatim by the staff client
 * (parent-messages-api `Message`) and the parent client (parent-api
 * `ParentMessage`). For staff messages on the parent side `sender_name` is the
 * "OGS [Schulname]" label.
 */
export interface ChatMessage {
  readonly id: string;
  readonly sender_kind: MessageSenderKind;
  readonly sender_name: string;
  readonly body: string;
  readonly created_at: string; // ISO timestamp
  // read_by_staff: a guardian message the OGS has read ("OGS hat gelesen",
  // shown to the parent). read_by_guardian: a staff message the guardian has
  // read ("Gelesen", shown to staff). Each side renders the receipt for its OWN
  // messages the other side has read.
  readonly read_by_staff?: boolean;
  readonly read_by_guardian?: boolean;
}

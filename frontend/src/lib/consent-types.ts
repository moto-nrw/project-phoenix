type ConsentKey = "agb" | "data_processing" | "email_contact" | "photo";

export type ConsentState = "granted" | "withdrawn" | "not_recorded";

export interface ConsentRecord {
  readonly key: ConsentKey;
  readonly state: ConsentState;
  readonly changed_at?: string;
}

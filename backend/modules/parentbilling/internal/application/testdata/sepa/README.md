# pain.008.001.08.xsd

The ISO 20022 message schema `CustomerDirectDebitInitiationV08`
(`urn:iso:std:iso:20022:tech:xsd:pain.008.001.08`), unchanged as the ISO 20022
Registration Authority generated it (Standards Editor, 2019-02-14). The test
`TestSEPADebitFileValidatesAgainstTheISOSchema` checks every PAIN.008 file of
the SEPA direct debit (#4697, ADR 0181) against it with `xmllint`.

- **Source:** ISO 20022 message catalogue, archive, "pain.008.001.08"
  (https://www.iso20022.org/catalogue-messages/iso-20022-messages-archive?search=pain).
  Downloaded on 2026-11-08 through the copy Danske Bank publishes for its
  customers; byte-identical to the copy in the open-source project
  `sebastienrousseau/pain001`.
  SHA-256 `3b2efe2239fceea22b17eb980df58c8b80db7322cf7781f8fd64ee8d17696210`.
- **Licence:** The ISO 20022 terms of use state that the material of
  iso20022.org "is intended to be used and reproduced freely by all interested
  users" under the ISO 20022 Intellectual Property Right Policy
  (https://www.iso20022.org/terms-use).
- **Not in the repository:** the technical validation subset of the Deutsche
  Kreditwirtschaft (`pain.008.001.08_GBIC_5.xsd`, Anlage 3 of the
  DFÜ-Abkommen, ebics.de). Its terms of use allow personal use only, no
  redistribution. Set `SEPA_DK_XSD` to a local copy to check the file against
  it too; the test then validates against both schemas.

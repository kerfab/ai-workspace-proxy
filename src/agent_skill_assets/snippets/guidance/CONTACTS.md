## How To Use These Examples

- Use Contacts when the user names a person but does not provide an exact email address.
- Resolve contacts before creating or sending Gmail messages to a named recipient.
- The resolver searches available Workspace directory, saved contacts, and other contacts, then standardizes and deduplicates candidates.
- If the resolver returns `needs_confirmation`, `ambiguous`, or `needs_email_choice`, ask the user before using an address.

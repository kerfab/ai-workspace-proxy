<!-- capability: contacts_read -->
### Read Contacts

Allowed: search contacts and directory entries to resolve people from partial names or email hints.

Example commands:

Use this before drafting or sending an email when the user provides a name instead of an exact email address.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" contacts resolve --query 'Frederic' --max-results 10`

Interpretation:

- `needs_confirmation`: one likely contact was found; confirm the recipient before using it.
- `ambiguous`: multiple contacts matched; ask the user which one they mean.
- `needs_email_choice`: one contact matched but has multiple usable email addresses; ask which address to use.
- `not_found`: no usable email address was found; ask the user for the address.
<!-- /capability -->

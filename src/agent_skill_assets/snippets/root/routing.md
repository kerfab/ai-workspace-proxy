## Routing System

This skill uses a routing system. `SKILL.md` is the root route.

- First infer the kind of operation requested: Gmail, Drive, Docs, Sheets, Slides, Calendar, skill update, or more than one topic. Do not stop just because the user did not name the Google product.
- Read the route file named for each needed topic in Files To Read. If the user request spans multiple topics, read every relevant route.
- If a route file points to another Markdown file, choose the next route that matches the user request and read it. Continue until the current file gives allowed capabilities and either helper examples or enough proxy guidance to use full passthrough.
- At each stage, decide whether you have enough instructions to complete the request. If not, return to this `SKILL.md`, choose the next needed route, and read it before acting.

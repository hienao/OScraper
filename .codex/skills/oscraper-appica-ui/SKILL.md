---
name: oscraper-appica-ui
description: Implement or review UI changes in the OScraper frontend using the repository's Appica component conventions. Use for React pages, forms, dialogs, navigation, responsive layouts, or frontend UI reviews in this repository.
---

# OScraper Appica UI

Apply these rules to changes under `frontend/src`.

## Component policy

- Check the installed `@appica/ui-react` exports before creating or styling a primitive. Prefer Appica primitives for buttons, inputs, selects, checkboxes, switches, fields, dialogs, drawers, tabs, tables, alerts, cards, scrolling, loaders, badges, tooltips, menus, and similar controls.
- Use icons from `@appica/icons-react`.
- Reuse project defaults in `frontend/src/components/common` before importing a primitive directly. In particular, use `AppDialog`, `AppSelect`, `CheckboxField`, `FormField`, `Message`, and `Panel` when their behavior fits.
- Custom components are appropriate for domain concepts such as directory entries, scrape jobs, media candidates, or responsive log cards. Compose them from Appica primitives; do not recreate an Appica primitive with raw HTML and Tailwind.
- A native element is acceptable when it is semantic content or an implementation detail without a reusable Appica equivalent. Examples include an invisible navigation backdrop button, a domain-specific clickable row, and mobile content that intentionally differs from a desktop table.

## Interaction and layout invariants

- Every dialog must use `AppDialog` or Appica `Dialog`. Preserve focus management, Escape/backdrop closing, background scroll locking, accessible title/description, and a visible close action unless the workflow intentionally blocks closing.
- Select and checkbox state must remain controlled by the page state. Give every control an accessible label.
- Keep domain paths and untrusted names inside `min-w-0` containers. Use `break-all` for full paths and `truncate` for one-line directory or media names.
- At widths below 640px, controls must remain at least 44px high, actions may wrap or stack, and no page or dialog may introduce horizontal scrolling.
- Let `AppDialog` body own vertical scrolling. Avoid a second `overflow-y-auto` region for directory lists inside a dialog; use `overflow-x-clip` when horizontal clipping is needed so CSS does not create another vertical scroll container.
- Keep phone-specific representations where a desktop component is inherently wide, such as log cards below `sm` and an Appica table at `sm` and above.
- Preserve the existing light/dark theme, i18n strings, reduced-motion behavior, and keyboard accessibility.

## Verification

After UI changes:

1. Run `npm test -- --run` and `npm run build` from `frontend`.
2. Search changed pages for avoidable raw primitives: `<select>`, checkbox inputs, hand-built modal overlays, hand-built tabs, and raw data tables.
3. For layout-sensitive changes, verify at 1440×900, 390×844, and 320×568. Check both Chinese and English when labels affect width.
4. For directory, path, or media lists, include long Chinese and English fixture names and at least one deep path.

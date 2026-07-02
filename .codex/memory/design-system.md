# Design System

This backend repo has no UI implementation, but future frontend work should follow the product's design rules and avoid generic dashboard UI.

## Visual direction

Build a calm, premium personal-finance companion. The dashboard should help users understand patterns in their life, not just show accounting totals. Prefer clear hierarchy, thoughtful empty states, and human explanations beside charts.

## Required tokens

Use Tailwind utility classes and these semantic tokens:

- Canvas: `bg-slate-50 dark:bg-slate-950`
- Surface: `bg-white dark:bg-slate-900`
- Primary text: `text-slate-900 dark:text-slate-50`
- Secondary text: `text-slate-500 dark:text-slate-400`
- Accent: `bg-indigo-600 dark:bg-indigo-500`
- Border: `border-slate-200 dark:border-slate-800`

Spacing:

- Icon/badge gaps: `space-x-1` or `space-x-2`
- Card padding: `p-3` or `p-4`
- Section spacing: `space-y-6` or `space-y-8`
- Layout grid: `gap-4 md:gap-6 lg:gap-8`

## Interaction states

Every interactive element should include:

- Hover: `hover:bg-slate-100 dark:hover:bg-slate-800`
- Transition: `transition-colors duration-200`
- Focus: `focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500`
- Disabled: `disabled:opacity-50 disabled:cursor-not-allowed`

## Dashboard layout guidance

- Do not create rigid, unscrollable split panes.
- Use responsive grids: `grid-cols-1`, then tablet/desktop columns.
- Let charts/cards stack on smaller laptop screens.
- Keep deterministic analytics visible even if AI insight is loading or unavailable.
- AI insight should be a separate panel with explicit generate/retry controls.

## Card rules

- Use surface background, muted border, rounded corners, and consistent padding.
- Cards need clear titles, compact supporting text, and meaningful empty states.
- Avoid dumping raw JSON-looking values. Translate metrics into human labels.

## Chart/table/list rules

- Category breakdown should combine rank, label, amount, and percentage.
- Daily spending should show income and expense clearly; avoid ambiguous color-only encoding.
- Behavior tags should use stable labels and descriptions.
- Irregular purchases should explain the reason (`impulse`, high amount, recurring) without shaming.
- Tables/lists must handle empty, loading, error, and filtered-zero states.

## Modal/form rules

- Forms must render inline errors with `text-rose-500` and `aria-live="polite"`.
- Do not only `console.log` errors.
- Correction forms should preserve existing transaction values and submit only changed non-empty fields.

## Anti-junior UI checklist

- No arbitrary hardcoded dimensions unless asset-bound.
- No inline styles except dynamic measurements like progress width.
- No array index keys.
- No fake demo data in production paths.
- No default gray boxes without hierarchy, copy, or state handling.
- No AI panel auto-loading on page entry.

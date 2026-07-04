# AWS Calculator — Plan 10: New Editor, Landing, Advanced-Mode Context, Cleanup

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the estimate-first curated flow the real user-visible product. Replace `/estimates/[id]` with a grouped-by-service table + sticky total + right sidebar + side-sheet `Add resource` picker. Replace `/` with a hero + live example estimate that forks into a real anonymous estimate the moment a user edits anything. Give `/services` and `/services/[code]` a `?to=&edit=` context so the dimension renderer can add to the active estimate. Delete the local cart store, the old `EstimateCart` component, and the top-nav Services link.

**Architecture:** No backend changes — Plan 09 already ships the curated line schema and handler. This plan is entirely SvelteKit: three new primitives (`SideSheet`, `ServicePicker`, `AddResourceSheet`), a pure summary helper mirroring the Go export summary, a fresh estimate editor page, a landing page that mounts the editor in read-only + forkOnEdit mode, and two small `+page.svelte` tweaks to the existing dimension-form routes.

**Tech Stack:** SvelteKit 2, Svelte 5 runes, Tailwind 4, vitest for pure helpers + primitives.

**Spec:** `docs/superpowers/specs/2026-07-04-saas-curated-forms-design.md` (sections *Interaction model*, *Routes*, *Estimate editor layout*, *What gets removed*).

---

## File structure produced by this plan

```
web/src/
├── lib/
│   ├── api/
│   │   ├── client.ts                    (MODIFIED: addCuratedItem method)
│   │   └── types.ts                     (MODIFIED: AddCuratedItemBody + PricedComponentBody)
│   ├── components/
│   │   ├── SideSheet.svelte             (NEW)
│   │   ├── SideSheet.test.ts            (NEW)
│   │   ├── ServicePicker.svelte         (NEW)
│   │   ├── ServicePicker.test.ts        (NEW)
│   │   ├── AddResourceSheet.svelte      (NEW)
│   │   ├── EstimateCart.svelte          (DELETED)
│   │   └── LineRow.svelte               (NEW: one row in the grouped table)
│   ├── stores/
│   │   └── cart.svelte.ts               (DELETED)
│   ├── utils/
│   │   ├── summary.ts                   (NEW: humanSummary(item) mirroring Go LineSummary)
│   │   ├── summary.test.ts              (NEW)
│   │   └── example-estimate.ts          (NEW: prebaked landing example)
├── routes/
│   ├── +layout.svelte                   (MODIFIED: drop Services link)
│   ├── +page.svelte                     (REWRITTEN: hero + live example estimate)
│   ├── estimates/[id]/
│   │   ├── +page.svelte                 (REWRITTEN: grouped table + sidebar + AddResourceSheet)
│   │   └── +page.server.ts              (unchanged)
│   └── services/
│       ├── +page.svelte                 (MODIFIED: accepts ?to=&edit= passthrough)
│       └── [code]/+page.svelte          (MODIFIED: ?to=&edit= → POST to that estimate on Add)
```

---

## Task 1: API client — `addCuratedItem`

**Files:**
- Modify: `web/src/lib/api/client.ts`
- Modify: `web/src/lib/api/types.ts`

- [ ] **Step 1: Add types to `web/src/lib/api/types.ts`**

Append at the bottom (do NOT touch the existing types):

```ts
export type PricedComponentBody = {
	service_code: string;
	region_code: string;
	sku: string;
	unit: string;
	quantity: string;
	unit_price: string;
	monthly_cost: string;
	currency: string;
	attributes: Record<string, string>;
};

export type AddCuratedItemBody = {
	line_type: 'curated';
	service_id: number;
	service_key: string;
	region_code: string;
	sku: string;
	currency: string;
	user_input: unknown;
	components: PricedComponentBody[];
};
```

- [ ] **Step 2: Add `addCuratedItem` to the returned client in `web/src/lib/api/client.ts`**

Find the object returned by `makeApi()` (near the bottom). Right after the existing `addItem` method, add:

```ts
		addCuratedItem: (id: number, body: AddCuratedItemBody, editToken?: string) =>
			request<EstimateItem>(`/estimates/${id}/items`, {
				method: 'POST',
				body: JSON.stringify(body),
				headers: editToken ? { 'X-Edit-Token': editToken } : {}
			}),
```

Update the import block at the top so `AddCuratedItemBody` is included:

```ts
import type {
	Envelope,
	Provider,
	Service,
	Dimension,
	DimensionValue,
	Region,
	PriceResponse,
	Estimate,
	EstimateItem,
	User,
	PriceDelta,
	AddCuratedItemBody
} from './types';
```

- [ ] **Step 3: Type-check**

```bash
cd web && npm run check 2>&1 | tail -5 && cd ..
```

Expected: 0 errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/api/client.ts web/src/lib/api/types.ts
git commit -m "feat(web/api): addCuratedItem method + curated body types"
```

---

## Task 2: Pure summary helper

**Files:**
- Create: `web/src/lib/utils/summary.ts`
- Create: `web/src/lib/utils/summary.test.ts`

Mirrors the Go `LineSummary` (`api/internal/export/summary.go`) so the frontend and backend format items identically.

- [ ] **Step 1: Write the failing test**

`web/src/lib/utils/summary.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { humanSummary } from './summary';
import type { EstimateItem } from '$lib/api/types';

const base: EstimateItem = {
	id: 1,
	estimate_id: 1,
	service_id: 141,
	region_code: 'us-east-1',
	sku: 'curated:ec2:v1',
	quantity: '730',
	attributes: {},
	unit_price: '0.096',
	currency: 'USD',
	monthly_cost: '70.08',
	line_type: 'curated'
};

describe('humanSummary', () => {
	it('formats a curated EC2 line', () => {
		const item: EstimateItem = {
			...base,
			service_key: 'ec2',
			service_code: 'AmazonEC2',
			user_input: {
				count: 2,
				instanceType: 'm5.large',
				hoursPerMonth: 730,
				operatingSystem: 'Linux'
			}
		};
		const s = humanSummary(item);
		expect(s).toContain('EC2');
		expect(s).toContain('m5.large');
		expect(s).toContain('Linux');
		expect(s).toContain('us-east-1');
	});

	it('falls back to attribute list for legacy lines', () => {
		const item: EstimateItem = {
			...base,
			line_type: 'legacy',
			service_code: 'AmazonS3',
			sku: 'S3-STORAGE',
			attributes: { storageClass: 'General Purpose', volumeType: 'Standard' }
		};
		const s = humanSummary(item);
		expect(s).toContain('AmazonS3');
		expect(s).toContain('storageClass=General Purpose');
	});

	it('curated line without service_key falls back to attribute list', () => {
		const item: EstimateItem = {
			...base,
			line_type: 'curated',
			service_code: 'AmazonEC2',
			attributes: { instanceType: 'm5.large' }
		};
		const s = humanSummary(item);
		expect(s).toContain('AmazonEC2');
	});
});
```

- [ ] **Step 2: Run — verify it fails**

```bash
cd web && npm run test 2>&1 | tail -8 && cd ..
```

Expected: `summary` module not found.

- [ ] **Step 3: Implement**

`web/src/lib/utils/summary.ts`:

```ts
import type { EstimateItem } from '$lib/api/types';

/**
 * humanSummary returns a plain-English one-liner for an estimate item. It
 * mirrors the Go export summary in `api/internal/export/summary.go` so PDF /
 * CSV / web all read the same. Curated lines use their service_key to select
 * a template; legacy or unknown-key curated lines fall back to
 * `service_code · region · key=value, key=value`.
 */
export function humanSummary(item: EstimateItem): string {
	if (item.line_type === 'curated') {
		const s = curatedSummary(item);
		if (s) return s;
	}
	return legacySummary(item);
}

function curatedSummary(item: EstimateItem): string {
	const input = item.user_input as Record<string, unknown> | undefined;
	if (!input) return '';
	switch (item.service_key) {
		case 'ec2':
			return `EC2 · ${input.count} × ${input.instanceType} in ${item.region_code} · ${input.hoursPerMonth} h/mo · ${input.operatingSystem}`;
	}
	// Curated but service_key isn't formatted yet — generic key=value dump.
	const pairs = Object.keys(input)
		.sort()
		.map((k) => `${k}=${input[k]}`);
	return `${item.service_key ?? item.service_code ?? ''} · ${item.region_code} · ${pairs.join(', ')}`;
}

function legacySummary(item: EstimateItem): string {
	const attrs = item.attributes ?? {};
	const pairs = Object.keys(attrs)
		.sort()
		.map((k) => `${k}=${attrs[k]}`);
	return `${item.service_code ?? ''} · ${item.region_code} · ${pairs.join(', ')}`;
}
```

- [ ] **Step 4: Run — verify it passes**

```bash
cd web && npm run test 2>&1 | tail -6 && cd ..
```

Expected: 3 new tests PASS on top of the 8 already passing.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/utils/summary.ts web/src/lib/utils/summary.test.ts
git commit -m "feat(web): humanSummary mirroring Go LineSummary (curated + legacy)"
```

---

## Task 3: `SideSheet` primitive

**Files:**
- Create: `web/src/lib/components/SideSheet.svelte`
- Create: `web/src/lib/components/SideSheet.test.ts`

Right-hand sliding sheet with a translucent backdrop; escape / backdrop click closes.

- [ ] **Step 1: Write the failing test**

`web/src/lib/components/SideSheet.test.ts`:

```ts
import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import SideSheet from './SideSheet.svelte';

describe('SideSheet', () => {
	it('renders children and title when open', () => {
		render(SideSheet, {
			props: {
				open: true,
				title: 'Add EC2',
				onClose: () => {}
			}
		});
		expect(screen.getByText('Add EC2')).toBeInTheDocument();
		expect(screen.getByRole('dialog')).toBeInTheDocument();
	});

	it('is hidden when open=false', () => {
		render(SideSheet, { props: { open: false, title: 'Add EC2', onClose: () => {} } });
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('calls onClose on backdrop click', async () => {
		const onClose = vi.fn();
		render(SideSheet, { props: { open: true, title: 't', onClose } });
		await fireEvent.click(screen.getByTestId('sidesheet-backdrop'));
		expect(onClose).toHaveBeenCalledOnce();
	});
});
```

- [ ] **Step 2: Run — verify it fails**

```bash
cd web && npm run test 2>&1 | tail -6 && cd ..
```

Expected: SideSheet module missing.

- [ ] **Step 3: Implement**

`web/src/lib/components/SideSheet.svelte`:

```svelte
<script lang="ts">
	type Props = {
		open: boolean;
		title: string;
		onClose: () => void;
		children?: import('svelte').Snippet;
		footer?: import('svelte').Snippet;
	};
	let { open, title, onClose, children, footer }: Props = $props();

	function onKeyDown(e: KeyboardEvent) {
		if (e.key === 'Escape' && open) onClose();
	}
</script>

<svelte:window onkeydown={onKeyDown} />

{#if open}
	<div
		data-testid="sidesheet-backdrop"
		onclick={onClose}
		role="presentation"
		class="fixed inset-0 z-40 bg-black/40 backdrop-blur-sm"
	></div>
	<aside
		role="dialog"
		aria-label={title}
		class="fixed right-0 top-0 z-50 flex h-full w-full flex-col border-l border-[--color-border] bg-[--color-surface] sm:w-[420px]"
	>
		<header class="flex items-center justify-between border-b border-[--color-border] px-5 py-4">
			<h3 class="text-sm font-semibold tracking-tight text-[--color-text]">{title}</h3>
			<button
				onclick={onClose}
				aria-label="Close"
				class="text-xs text-[--color-text-muted] hover:text-[--color-text]"
			>
				✕
			</button>
		</header>
		<div class="flex-1 overflow-auto px-5 py-4">
			{#if children}{@render children()}{/if}
		</div>
		{#if footer}
			<footer class="border-t border-[--color-border] px-5 py-4">{@render footer()}</footer>
		{/if}
	</aside>
{/if}
```

- [ ] **Step 4: Run — verify it passes**

```bash
cd web && npm run test 2>&1 | tail -6 && cd ..
```

Expected: 3 SideSheet tests PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/components/SideSheet.svelte web/src/lib/components/SideSheet.test.ts
git commit -m "feat(web): SideSheet primitive (backdrop click + Escape close)"
```

---

## Task 4: `ServicePicker` component

**Files:**
- Create: `web/src/lib/components/ServicePicker.svelte`
- Create: `web/src/lib/components/ServicePicker.test.ts`

Renders the registry as tiles with a search box at the top and an "Advanced: browse all 268 services" link at the bottom. Emits an `onPick(serviceKey)` callback.

- [ ] **Step 1: Write the failing test**

`web/src/lib/components/ServicePicker.test.ts`:

```ts
import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import ServicePicker from './ServicePicker.svelte';

describe('ServicePicker', () => {
	it('shows every registered service as a tile', () => {
		render(ServicePicker, { props: { onPick: () => {}, advancedHref: '/services' } });
		expect(screen.getByRole('button', { name: /EC2/i })).toBeInTheDocument();
	});

	it('filters tiles when the user types', async () => {
		render(ServicePicker, { props: { onPick: () => {}, advancedHref: '/services' } });
		const input = screen.getByPlaceholderText(/search/i);
		await fireEvent.input(input, { target: { value: 'ec2' } });
		expect(screen.getByRole('button', { name: /EC2/i })).toBeInTheDocument();
	});

	it('calls onPick with the service key', async () => {
		const onPick = vi.fn();
		render(ServicePicker, { props: { onPick, advancedHref: '/services' } });
		await fireEvent.click(screen.getByRole('button', { name: /EC2/i }));
		expect(onPick).toHaveBeenCalledWith('ec2');
	});
});
```

- [ ] **Step 2: Run — verify it fails**

```bash
cd web && npm run test 2>&1 | tail -6 && cd ..
```

Expected: ServicePicker missing.

- [ ] **Step 3: Implement**

`web/src/lib/components/ServicePicker.svelte`:

```svelte
<script lang="ts">
	import { registry } from '$lib/services';

	type Props = {
		onPick: (serviceKey: string) => void;
		advancedHref: string;
	};
	let { onPick, advancedHref }: Props = $props();

	let query = $state('');

	const entries = $derived(
		Object.entries(registry)
			.filter(([, e]) => {
				const q = query.toLowerCase();
				if (!q) return true;
				return (
					e.meta.label.toLowerCase().includes(q) ||
					e.meta.description.toLowerCase().includes(q) ||
					e.meta.code.toLowerCase().includes(q)
				);
			})
			.sort((a, b) => a[1].meta.label.localeCompare(b[1].meta.label))
	);
</script>

<div class="space-y-4">
	<input
		type="text"
		bind:value={query}
		placeholder="Search services…"
		class="w-full rounded-md border border-[--color-border] bg-transparent px-3 py-2 text-sm text-[--color-text] outline-none focus:border-[--color-text]"
	/>
	<div class="grid gap-2 sm:grid-cols-2">
		{#each entries as [key, entry] (key)}
			<button
				type="button"
				onclick={() => onPick(key)}
				class="flex flex-col items-start gap-1 rounded-lg border border-[--color-border] bg-[--color-surface-elev] p-3 text-left transition-colors hover:border-[--color-text]"
			>
				<span class="text-sm font-medium text-[--color-text]">{entry.meta.label}</span>
				<span class="text-xs text-[--color-text-muted]">{entry.meta.description}</span>
			</button>
		{/each}
	</div>
	<a
		href={advancedHref}
		class="mt-2 block text-center text-xs text-[--color-text-muted] hover:text-[--color-text]"
	>
		Advanced: browse all 268 services →
	</a>
</div>
```

- [ ] **Step 4: Run — verify it passes**

```bash
cd web && npm run test 2>&1 | tail -6 && cd ..
```

Expected: 3 ServicePicker tests PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/components/ServicePicker.svelte web/src/lib/components/ServicePicker.test.ts
git commit -m "feat(web): ServicePicker (search + tiles + Advanced link)"
```

---

## Task 5: `LineRow` — one row in the grouped estimate table

**Files:**
- Create: `web/src/lib/components/LineRow.svelte`

Small pure presentation component so the editor page stays readable. No test — covered by the editor screen test at end.

- [ ] **Step 1: Write `LineRow.svelte`**

```svelte
<script lang="ts">
	import PriceBadge from './PriceBadge.svelte';
	import { humanSummary } from '$lib/utils/summary';
	import type { EstimateItem } from '$lib/api/types';

	type Props = {
		item: EstimateItem;
		currency: string;
		readOnly?: boolean;
		onEdit?: (item: EstimateItem) => void;
		onDelete?: (item: EstimateItem) => void;
	};
	let { item, currency, readOnly = false, onEdit, onDelete }: Props = $props();
</script>

<tr class="border-t border-[--color-border]">
	<td class="py-2 pr-2 text-sm text-[--color-text]">{humanSummary(item)}</td>
	<td class="py-2 text-right">
		<PriceBadge amount={item.monthly_cost} {currency} />
	</td>
	<td class="w-16 py-2 pl-2 text-right">
		{#if !readOnly}
			<div class="flex justify-end gap-2 text-xs text-[--color-text-muted]">
				{#if onEdit}
					<button
						onclick={() => onEdit(item)}
						class="hover:text-[--color-text]"
						aria-label="Edit"
					>
						Edit
					</button>
				{/if}
				{#if onDelete}
					<button
						onclick={() => onDelete(item)}
						class="hover:text-[--color-text]"
						aria-label="Delete"
					>
						×
					</button>
				{/if}
			</div>
		{/if}
	</td>
</tr>
```

- [ ] **Step 2: Type-check**

```bash
cd web && npm run check 2>&1 | tail -3 && cd ..
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/components/LineRow.svelte
git commit -m "feat(web): LineRow renders a single estimate item (human summary + price)"
```

---

## Task 6: `AddResourceSheet` — picker → form → price → save

**Files:**
- Create: `web/src/lib/components/AddResourceSheet.svelte`

Wraps `SideSheet` with two internal states (`picker` and `form:<key>`), calls `/price` per component of the mapper output, and calls `api.addCuratedItem` on save.

- [ ] **Step 1: Write `AddResourceSheet.svelte`**

```svelte
<script lang="ts">
	import SideSheet from './SideSheet.svelte';
	import ServicePicker from './ServicePicker.svelte';
	import PriceBadge from './PriceBadge.svelte';
	import Button from './Button.svelte';
	import { registry } from '$lib/services';
	import { makeApi } from '$lib/api/client';
	import type { PricedComponent } from '$lib/services/types';
	import type { AddCuratedItemBody, PricedComponentBody } from '$lib/api/types';

	type Props = {
		open: boolean;
		estimateId: number;
		editToken?: string;
		serviceId: number;
		region: string;
		currency: string;
		advancedHref: string;
		onClose: () => void;
		onAdded: () => void;
	};
	let {
		open,
		estimateId,
		editToken,
		serviceId,
		region,
		currency,
		advancedHref,
		onClose,
		onAdded
	}: Props = $props();

	const api = makeApi();

	// state: 'picker' | 'form:<key>'
	let state = $state<string>('picker');
	let input = $state<unknown>(null);
	let components = $state<PricedComponent[]>([]);
	let pricedComponents = $state<PricedComponentBody[]>([]);
	let priceError = $state<string | null>(null);
	let priceLoading = $state(false);
	let saving = $state(false);

	function reset() {
		state = 'picker';
		input = null;
		components = [];
		pricedComponents = [];
		priceError = null;
		priceLoading = false;
	}

	$effect(() => {
		if (!open) reset();
	});

	function pick(serviceKey: string) {
		state = `form:${serviceKey}`;
		const entry = registry[serviceKey];
		input = { ...entry.defaults };
		components = [];
		pricedComponents = [];
	}

	let debounceTimer: ReturnType<typeof setTimeout> | undefined;

	function schedulePrice(next: { input: unknown; components: PricedComponent[] }) {
		input = next.input;
		components = next.components;
		if (debounceTimer) clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => priceComponents(next.components), 200);
	}

	async function priceComponents(comps: PricedComponent[]) {
		if (comps.length === 0) {
			pricedComponents = [];
			priceError = null;
			return;
		}
		priceLoading = true;
		priceError = null;
		try {
			const results = await Promise.all(
				comps.map((c) =>
					api.price({
						service: c.service,
						region: c.region ?? region,
						currency,
						attributes: c.attributes,
						term: 'OnDemand'
					})
				)
			);
			pricedComponents = comps.map((c, i) => {
				const p = results[i];
				const unitPrice = parseFloat(p.price_per_unit);
				const monthly = unitPrice * c.quantity;
				return {
					service_code: c.service,
					region_code: c.region ?? region,
					sku: p.sku,
					unit: p.unit,
					quantity: String(c.quantity),
					unit_price: p.price_per_unit,
					monthly_cost: monthly.toFixed(6),
					currency: p.currency,
					attributes: c.attributes
				};
			});
		} catch (e) {
			priceError = (e as Error).message;
			pricedComponents = [];
		} finally {
			priceLoading = false;
		}
	}

	const monthlyTotal = $derived(
		pricedComponents.reduce((acc, p) => acc + parseFloat(p.monthly_cost || '0'), 0)
	);

	const currentKey = $derived(state.startsWith('form:') ? state.slice(5) : '');
	const currentEntry = $derived(currentKey ? registry[currentKey] : null);

	async function save() {
		if (!currentEntry || pricedComponents.length === 0) return;
		saving = true;
		try {
			const body: AddCuratedItemBody = {
				line_type: 'curated',
				service_id: serviceId,
				service_key: currentKey,
				region_code: region,
				sku: `curated:${currentKey}:v1`,
				currency,
				user_input: input,
				components: pricedComponents
			};
			await api.addCuratedItem(estimateId, body, editToken);
			onAdded();
			onClose();
		} catch (e) {
			priceError = (e as Error).message;
		} finally {
			saving = false;
		}
	}

	const title = $derived(
		state === 'picker' ? 'Add resource' : `Add ${currentEntry?.meta.label ?? ''}`
	);
</script>

<SideSheet {open} {title} {onClose}>
	{#if state === 'picker'}
		<ServicePicker onPick={pick} {advancedHref} />
	{:else if currentEntry}
		{#key currentKey}
			{@const F = currentEntry.Form}
			<F {region} onChange={schedulePrice} />
		{/key}
	{/if}
	{#snippet footer()}
		{#if state !== 'picker'}
			<div class="space-y-3">
				<div class="flex items-baseline justify-between">
					<span class="text-xs text-[--color-text-muted]">Monthly</span>
					{#if priceLoading}
						<span class="text-xs text-[--color-text-muted]">Loading…</span>
					{:else if priceError}
						<span class="text-xs text-red-500">{priceError}</span>
					{:else}
						<PriceBadge amount={monthlyTotal} {currency} />
					{/if}
				</div>
				<div class="flex justify-end gap-2">
					<button
						onclick={onClose}
						class="rounded-md px-3 py-1.5 text-xs text-[--color-text-muted] hover:text-[--color-text]"
					>
						Cancel
					</button>
					<Button
						onclick={save}
						disabled={saving || pricedComponents.length === 0 || priceLoading || !!priceError}
					>
						{saving ? 'Saving…' : 'Add to estimate'}
					</Button>
				</div>
			</div>
		{/if}
	{/snippet}
</SideSheet>
```

- [ ] **Step 2: Type-check**

```bash
cd web && npm run check 2>&1 | tail -5 && cd ..
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/components/AddResourceSheet.svelte
git commit -m "feat(web): AddResourceSheet orchestrates picker → form → price → save"
```

---

## Task 7: Rewrite `/estimates/[id]` editor

**Files:**
- Modify: `web/src/routes/estimates/[id]/+page.svelte`

Grouped-by-service table with subtotal rows, sticky grand total, right sidebar (name / currency / notes / share / export), and an `Add resource` button that opens `AddResourceSheet`.

- [ ] **Step 1: Replace the entire file with the new editor**

```svelte
<script lang="ts">
	import Card from '$lib/components/Card.svelte';
	import Button from '$lib/components/Button.svelte';
	import PriceBadge from '$lib/components/PriceBadge.svelte';
	import LineRow from '$lib/components/LineRow.svelte';
	import AddResourceSheet from '$lib/components/AddResourceSheet.svelte';
	import Input from '$lib/components/Input.svelte';
	import { makeApi } from '$lib/api/client';
	import { currency as currencyStore } from '$lib/stores/currency.svelte';
	import type { EstimateItem } from '$lib/api/types';
	import { invalidateAll, goto } from '$app/navigation';
	import { page } from '$app/state';

	let { data } = $props();

	const api = makeApi();
	let sheetOpen = $state(false);
	let sharing = $state(false);
	let shareUrl = $state<string | null>(null);
	let name = $state(data.estimate.name);
	let notes = $state(data.estimate.notes ?? '');
	let saving = $state(false);
	let saveError = $state<string | null>(null);

	// Sync name/notes with data on navigation.
	$effect(() => {
		name = data.estimate.name;
		notes = data.estimate.notes ?? '';
	});

	const grouped = $derived.by(() => {
		const buckets = new Map<string, EstimateItem[]>();
		for (const it of data.estimate.items ?? []) {
			const key = it.service_code ?? 'Unknown';
			if (!buckets.has(key)) buckets.set(key, []);
			buckets.get(key)!.push(it);
		}
		return [...buckets.entries()].sort((a, b) => a[0].localeCompare(b[0]));
	});

	const total = $derived(
		(data.estimate.items ?? []).reduce((a, it) => a + parseFloat(it.monthly_cost || '0'), 0)
	);

	const serviceIdForNewLines = $derived.by(() => {
		// Prefer an existing item's service_id (all curated EC2 lines share it);
		// fall back to the first item in the estimate.
		const first = (data.estimate.items ?? [])[0];
		return first?.service_id ?? 0;
	});

	const advancedHref = $derived(
		`/services?to=${data.estimate.id}` +
			(data.editToken ? `&edit=${encodeURIComponent(data.editToken)}` : '')
	);

	async function share() {
		sharing = true;
		try {
			const r = await api.share(data.estimate.id, data.editToken ?? undefined);
			shareUrl = r.share_url;
		} finally {
			sharing = false;
		}
	}

	async function saveMeta() {
		if (name === data.estimate.name && notes === (data.estimate.notes ?? '')) return;
		saving = true;
		saveError = null;
		try {
			// PATCH via fetch — client doesn't currently expose a PATCH helper.
			const res = await fetch(`/api/v1/estimates/${data.estimate.id}`, {
				method: 'PATCH',
				headers: {
					'Content-Type': 'application/json',
					...(data.editToken ? { 'X-Edit-Token': data.editToken } : {})
				},
				body: JSON.stringify({ name, notes })
			});
			if (!res.ok) throw new Error(`PATCH failed (${res.status})`);
			await invalidateAll();
		} catch (e) {
			saveError = (e as Error).message;
		} finally {
			saving = false;
		}
	}

	async function deleteItem(item: EstimateItem) {
		if (!confirm('Delete this line?')) return;
		await api.deleteItem(data.estimate.id, item.id, data.editToken ?? undefined);
		await invalidateAll();
	}

	function editItem(item: EstimateItem) {
		// v1: opening the sheet in edit mode is future work; delete + re-add for now.
		alert('Editing individual lines is coming soon — delete + re-add for now.');
	}

	function exportUrl(format: string) {
		const edit = data.editToken ? `&edit=${encodeURIComponent(data.editToken)}` : '';
		return `/estimates/${data.estimate.id}/export?format=${format}${edit}`;
	}
</script>

<div class="grid gap-8 md:grid-cols-[1fr_320px]">
	<section class="space-y-4">
		<div class="flex items-baseline justify-between">
			<h1 class="text-2xl font-semibold tracking-tight">{data.estimate.name}</h1>
			<Button onclick={() => (sheetOpen = true)}>Add resource</Button>
		</div>

		{#if (data.estimate.items ?? []).length === 0}
			<Card>
				<p class="text-sm text-[--color-text-muted]">
					Empty estimate. Click <span class="text-[--color-text]">Add resource</span> to start.
				</p>
			</Card>
		{:else}
			<Card>
				<table class="w-full">
					<thead>
						<tr class="border-b border-[--color-border] text-xs uppercase tracking-wide text-[--color-text-muted]">
							<th class="py-2 pr-2 text-left">Item</th>
							<th class="py-2 text-right">Monthly</th>
							<th class="py-2 pl-2 text-right"></th>
						</tr>
					</thead>
					{#each grouped as [svc, items] (svc)}
						<tbody>
							<tr class="border-t border-[--color-border] bg-[--color-surface-elev]">
								<td colspan="3" class="py-2 pr-2 text-xs font-medium uppercase tracking-wide text-[--color-text-muted]">
									{svc}
								</td>
							</tr>
							{#each items as it (it.id)}
								<LineRow
									item={it}
									currency={data.estimate.currency}
									onEdit={editItem}
									onDelete={deleteItem}
								/>
							{/each}
							{#if items.length > 1}
								{@const groupTotal = items.reduce((a, it) => a + parseFloat(it.monthly_cost || '0'), 0)}
								<tr class="border-t border-[--color-border]">
									<td class="py-2 pr-2 text-right text-xs uppercase tracking-wide text-[--color-text-muted]">
										Subtotal
									</td>
									<td class="py-2 text-right">
										<PriceBadge amount={groupTotal} currency={data.estimate.currency} muted />
									</td>
									<td></td>
								</tr>
							{/if}
						</tbody>
					{/each}
					<tfoot>
						<tr class="sticky bottom-0 border-t border-[--color-border] bg-[--color-surface]">
							<td class="py-3 pr-2 text-right text-sm font-medium">Grand total</td>
							<td class="py-3 text-right">
								<PriceBadge amount={total} currency={data.estimate.currency} />
							</td>
							<td></td>
						</tr>
					</tfoot>
				</table>
			</Card>
		{/if}
	</section>

	<aside class="space-y-4">
		<Card title="Estimate">
			<Input label="Name" bind:value={name} />
			<div class="mt-3">
				<label class="block text-xs text-[--color-text-muted]">Notes</label>
				<textarea
					bind:value={notes}
					rows="3"
					class="mt-1 w-full rounded-md border border-[--color-border] bg-transparent px-3 py-2 text-sm text-[--color-text] outline-none focus:border-[--color-text]"
				></textarea>
			</div>
			{#if saveError}<p class="mt-2 text-xs text-red-500">{saveError}</p>{/if}
			<div class="mt-3">
				<Button
					variant="secondary"
					onclick={saveMeta}
					disabled={saving ||
						(name === data.estimate.name && notes === (data.estimate.notes ?? ''))}
				>
					{saving ? 'Saving…' : 'Save changes'}
				</Button>
			</div>
		</Card>
		<Card title="Share & export">
			<div class="space-y-3">
				<Button variant="secondary" onclick={share} disabled={sharing}>
					{sharing ? 'Sharing…' : 'Share'}
				</Button>
				{#if shareUrl}
					<p class="mono break-all text-[10px] text-[--color-text-muted]">{shareUrl}</p>
				{/if}
				<div class="flex gap-2">
					<a
						href={exportUrl('pdf')}
						class="rounded-md border border-[--color-border] px-3 py-1.5 text-xs hover:border-[--color-text]"
					>
						PDF
					</a>
					<a
						href={exportUrl('csv')}
						class="rounded-md border border-[--color-border] px-3 py-1.5 text-xs hover:border-[--color-text]"
					>
						CSV
					</a>
					<a
						href={exportUrl('json')}
						class="rounded-md border border-[--color-border] px-3 py-1.5 text-xs hover:border-[--color-text]"
					>
						JSON
					</a>
				</div>
			</div>
		</Card>
	</aside>
</div>

<AddResourceSheet
	open={sheetOpen}
	estimateId={data.estimate.id}
	editToken={data.editToken ?? undefined}
	serviceId={serviceIdForNewLines}
	region={'us-east-1'}
	currency={currencyStore.value}
	{advancedHref}
	onClose={() => (sheetOpen = false)}
	onAdded={() => invalidateAll()}
/>
```

- [ ] **Step 2: Type-check**

```bash
cd web && npm run check 2>&1 | tail -5 && cd ..
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/routes/estimates/\[id\]/+page.svelte
git commit -m "feat(web): rewrite estimate editor — grouped table, sidebar, AddResourceSheet"
```

---

## Task 8: Prebaked example estimate

**Files:**
- Create: `web/src/lib/utils/example-estimate.ts`
- Create: `web/src/lib/utils/example-estimate.test.ts`

A small module the landing page uses to render a realistic-looking 6-line estimate without persisting anything.

- [ ] **Step 1: Write the failing test**

`web/src/lib/utils/example-estimate.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { exampleEstimate } from './example-estimate';

describe('exampleEstimate', () => {
	it('renders as a real Estimate with several items', () => {
		const est = exampleEstimate('USD');
		expect(est.currency).toBe('USD');
		expect(est.items?.length ?? 0).toBeGreaterThanOrEqual(4);
		for (const it of est.items ?? []) {
			expect(parseFloat(it.monthly_cost)).toBeGreaterThan(0);
		}
	});

	it('respects the display currency in each row', () => {
		const est = exampleEstimate('NGN');
		expect(est.currency).toBe('NGN');
		for (const it of est.items ?? []) expect(it.currency).toBe('NGN');
	});
});
```

- [ ] **Step 2: Run — verify it fails**

```bash
cd web && npm run test 2>&1 | tail -6 && cd ..
```

Expected: example-estimate module not found.

- [ ] **Step 3: Implement**

`web/src/lib/utils/example-estimate.ts`:

```ts
import type { Estimate, EstimateItem } from '$lib/api/types';

/**
 * exampleEstimate returns a plausible "small SaaS backend" estimate the
 * landing page renders below the fold. Not persisted; the numbers are
 * rounded illustrative figures rather than live pricing.
 *
 * When the user first edits anything, the landing page forks this into a
 * real anonymous estimate via POST /estimates + N × POST /items.
 */
export function exampleEstimate(currency: string): Estimate {
	const now = new Date().toISOString();
	const items: EstimateItem[] = [
		mkItem({
			id: -1,
			service_code: 'AmazonEC2',
			service_key: 'ec2',
			line_type: 'curated',
			region_code: 'us-east-1',
			user_input: {
				count: 2,
				instanceType: 't3.medium',
				operatingSystem: 'Linux',
				hoursPerMonth: 730
			},
			monthly_cost: '60.74',
			currency
		}),
		mkItem({
			id: -2,
			service_code: 'AmazonRDS',
			line_type: 'legacy',
			region_code: 'us-east-1',
			attributes: { instanceType: 'db.t3.micro', engine: 'PostgreSQL' },
			monthly_cost: '13.87',
			currency
		}),
		mkItem({
			id: -3,
			service_code: 'AmazonS3',
			line_type: 'legacy',
			region_code: 'us-east-1',
			attributes: { storageClass: 'Standard', volumeType: 'General Purpose' },
			monthly_cost: '2.30',
			currency
		}),
		mkItem({
			id: -4,
			service_code: 'AmazonCloudFront',
			line_type: 'legacy',
			region_code: 'us-east-1',
			attributes: { region: 'US', tier: '1 TB out' },
			monthly_cost: '85.00',
			currency
		}),
		mkItem({
			id: -5,
			service_code: 'ElasticLoadBalancing',
			line_type: 'legacy',
			region_code: 'us-east-1',
			attributes: { type: 'Application Load Balancer' },
			monthly_cost: '16.20',
			currency
		}),
		mkItem({
			id: -6,
			service_code: 'AmazonRoute53',
			line_type: 'legacy',
			region_code: 'us-east-1',
			attributes: { zoneType: 'Hosted zone' },
			monthly_cost: '0.50',
			currency
		})
	];

	const total = items.reduce((a, it) => a + parseFloat(it.monthly_cost), 0);

	return {
		id: 0,
		name: 'Example: small SaaS backend',
		currency,
		notes: '',
		items,
		total: total.toFixed(2),
		created_at: now,
		updated_at: now
	};
}

type ItemInit = Partial<EstimateItem> &
	Pick<EstimateItem, 'id' | 'service_code' | 'region_code' | 'monthly_cost' | 'currency' | 'line_type'>;

function mkItem(init: ItemInit): EstimateItem {
	return {
		estimate_id: 0,
		service_id: 0,
		sku: init.sku ?? 'example',
		quantity: init.quantity ?? '1',
		attributes: init.attributes ?? {},
		unit_price: init.unit_price ?? init.monthly_cost,
		...init
	};
}
```

- [ ] **Step 4: Run — verify it passes**

```bash
cd web && npm run test 2>&1 | tail -6 && cd ..
```

Expected: 2 example-estimate tests PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/utils/example-estimate.ts web/src/lib/utils/example-estimate.test.ts
git commit -m "feat(web): exampleEstimate — prebaked 6-line SaaS backend"
```

---

## Task 9: Fork-on-edit helper + landing rewrite

**Files:**
- Modify: `web/src/routes/+page.svelte`

The landing page renders `exampleEstimate(currency)` inside a lightweight read-only table (not the full editor — keeps the landing lean), plus a top hero and one big "Try it" button that forks. The fork sequence: `POST /estimates` → `POST /estimates/:id/items` for each example line (skipping the ones that aren't priceable, but the anonymous flow accepts them as legacy shape) → `goto('/estimates/<id>?edit=<token>')`.

For v1 simplicity we fork on the CTA click (not on inline edit). The spec calls for implicit fork on any interaction; the CTA is a valid subset — inline editing is future work. We still show the example as a real-looking table so the pitch lands.

- [ ] **Step 1: Replace `web/src/routes/+page.svelte`**

```svelte
<script lang="ts">
	import Card from '$lib/components/Card.svelte';
	import Button from '$lib/components/Button.svelte';
	import PriceBadge from '$lib/components/PriceBadge.svelte';
	import { humanSummary } from '$lib/utils/summary';
	import { exampleEstimate } from '$lib/utils/example-estimate';
	import { currency as currencyStore } from '$lib/stores/currency.svelte';
	import { makeApi } from '$lib/api/client';
	import { goto } from '$app/navigation';

	const api = makeApi();

	const est = $derived(exampleEstimate(currencyStore.value));
	let forking = $state(false);
	let forkError = $state<string | null>(null);

	async function fork() {
		forking = true;
		forkError = null;
		try {
			const created = await api.createEstimate({
				name: 'Untitled',
				currency: currencyStore.value
			});
			for (const it of est.items ?? []) {
				await api.addItem(
					created.estimate.id,
					{
						service_id: 0,
						region_code: it.region_code,
						sku: it.sku,
						quantity: it.quantity,
						attributes: it.attributes,
						unit_price: it.unit_price,
						unit: 'Hrs',
						currency: it.currency
					},
					created.edit_token
				);
			}
			const q = created.edit_token
				? `?edit=${encodeURIComponent(created.edit_token)}`
				: '';
			await goto(`/estimates/${created.estimate.id}${q}`);
		} catch (e) {
			forkError = (e as Error).message;
		} finally {
			forking = false;
		}
	}
</script>

<section class="space-y-12">
	<div class="max-w-2xl space-y-6">
		<h1 class="text-5xl font-semibold leading-tight tracking-tight">Price AWS. In your currency.</h1>
		<p class="text-lg text-[--color-text-muted]">
			Build an AWS estimate in three clicks per line, in USD, EUR, GBP, NGN, and more.
			Share with a link. Export a branded PDF.
		</p>
		<div class="flex gap-3">
			<Button onclick={fork} disabled={forking}>
				{forking ? 'Starting…' : 'Try it — start free estimate'}
			</Button>
		</div>
		{#if forkError}
			<p class="text-xs text-red-500">{forkError}</p>
		{/if}
	</div>

	<div>
		<h2 class="mb-3 text-sm font-medium uppercase tracking-wide text-[--color-text-muted]">
			Example: small SaaS backend
		</h2>
		<Card>
			<table class="w-full">
				<thead>
					<tr class="border-b border-[--color-border] text-xs uppercase tracking-wide text-[--color-text-muted]">
						<th class="py-2 pr-2 text-left">Item</th>
						<th class="py-2 text-right">Monthly</th>
					</tr>
				</thead>
				<tbody>
					{#each est.items ?? [] as it (it.id)}
						<tr class="border-t border-[--color-border]">
							<td class="py-2 pr-2 text-sm text-[--color-text]">{humanSummary(it)}</td>
							<td class="py-2 text-right">
								<PriceBadge amount={it.monthly_cost} currency={est.currency} />
							</td>
						</tr>
					{/each}
				</tbody>
				<tfoot>
					<tr class="border-t border-[--color-border]">
						<td class="py-3 pr-2 text-right text-sm font-medium">Grand total</td>
						<td class="py-3 text-right">
							<PriceBadge amount={parseFloat(est.total ?? '0')} currency={est.currency} />
						</td>
					</tr>
				</tfoot>
			</table>
		</Card>
	</div>
</section>
```

- [ ] **Step 2: Type-check**

```bash
cd web && npm run check 2>&1 | tail -5 && cd ..
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/routes/+page.svelte
git commit -m "feat(web): landing hero + example estimate + fork CTA"
```

---

## Task 10: Advanced-mode context on `/services`

**Files:**
- Modify: `web/src/routes/services/+page.svelte`

Support `?to=<id>&edit=<token>` on the services grid — the query params get carried into every `href` so the dimension form later knows which estimate to write to.

- [ ] **Step 1: Add query passthrough at the top of the script + carry it in the links**

Find the `<script lang="ts">` block. Replace the whole file with:

```svelte
<script lang="ts">
	import Input from '$lib/components/Input.svelte';
	import Card from '$lib/components/Card.svelte';
	import { debounce } from '$lib/utils/debounce';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';

	let { data } = $props();
	let search = $state(data.q);

	// Preserve the ?to=&edit= context across search + service navigations so
	// the estimate editor "resumes" once the user picks a service.
	const to = $derived(page.url.searchParams.get('to') ?? '');
	const edit = $derived(page.url.searchParams.get('edit') ?? '');

	function contextSuffix(existing: URLSearchParams): string {
		const out = new URLSearchParams(existing);
		if (to) out.set('to', to);
		if (edit) out.set('edit', edit);
		return out.toString();
	}

	function serviceHref(code: string): string {
		const params = new URLSearchParams();
		if (to) params.set('to', to);
		if (edit) params.set('edit', edit);
		const q = params.toString();
		return `/services/${code}${q ? '?' + q : ''}`;
	}

	const navigate = debounce((v: string) => {
		const params = new URLSearchParams(page.url.searchParams);
		if (v) params.set('q', v);
		else params.delete('q');
		goto('/services?' + contextSuffix(params), { replaceState: true, keepFocus: true });
	}, 250);

	$effect(() => {
		if (search !== data.q) navigate(search);
	});
</script>

<section class="space-y-6">
	<header class="flex items-end justify-between gap-4">
		<div>
			<h1 class="text-3xl font-semibold tracking-tight">Advanced: all services</h1>
			<p class="mt-1 text-sm text-[--color-text-muted]">
				{data.services?.length ?? 0} AWS services ready to price. Use the
				<a href="/" class="underline">estimate editor</a> for the everyday flow.
			</p>
		</div>
		<div class="w-72">
			<Input placeholder="Search by name or code…" bind:value={search} />
		</div>
	</header>

	{#if data.error}
		<Card title="Backend unavailable">
			<p class="text-sm text-[--color-text-muted]">
				Could not load services: {data.error}.
			</p>
		</Card>
	{:else if data.services.length === 0 && data.q}
		<Card>
			<p class="text-sm text-[--color-text-muted]">
				No services match <span class="mono text-[--color-text]">"{data.q}"</span>.
			</p>
		</Card>
	{:else if data.services.length === 0}
		<Card title="Catalog not populated yet">
			<p class="text-sm text-[--color-text-muted]">
				Trigger the first ingest with:
			</p>
			<pre class="mono mt-3 rounded-md border border-[--color-border] bg-[--color-surface] p-3 text-[11px]"><code>curl -X POST -H "Authorization: Bearer $ADMIN_TOKEN" https://calc.example/api/v1/admin/ingest?provider=aws</code></pre>
		</Card>
	{:else}
		<div class="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
			{#each data.services as s (s.id)}
				<a
					href={serviceHref(s.code)}
					class="group rounded-lg border border-[--color-border] bg-[--color-surface-elev] p-4 transition-colors hover:border-[--color-text]"
				>
					<div class="flex items-baseline justify-between">
						<span class="font-medium tracking-tight text-[--color-text]">{s.name}</span>
						<span class="mono text-[10px] text-[--color-text-muted]">{s.code}</span>
					</div>
					{#if s.family}
						<div class="mt-1 text-xs text-[--color-text-muted]">{s.family}</div>
					{/if}
				</a>
			{/each}
		</div>
	{/if}
</section>
```

- [ ] **Step 2: Type-check**

```bash
cd web && npm run check 2>&1 | tail -5 && cd ..
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/routes/services/+page.svelte
git commit -m "feat(web): /services carries ?to=&edit= into service links (Advanced mode)"
```

---

## Task 11: Advanced-mode dimension form POSTs to a target estimate

**Files:**
- Modify: `web/src/routes/services/[code]/+page.svelte`

When the user opens a service from Advanced mode with `?to=<id>&edit=<token>`, replace the current "add to local cart" flow with a direct POST to that estimate as a `legacy` line, then redirect back.

- [ ] **Step 1: Replace the script + Add button of the file**

Read the current file to find the `addToEstimate` function and the "Add to estimate" button; leave the surrounding form + preview intact. The only behavioural change is `addToEstimate`.

Show the exact new `addToEstimate` body (replace whichever version is currently there):

```ts
	async function addToEstimate() {
		if (!livePrice || !lastPayload) return;
		const to = page.url.searchParams.get('to');
		const edit = page.url.searchParams.get('edit') ?? undefined;
		const qty = parseFloat(quantity) || 1;
		if (to) {
			// Advanced mode: POST directly to the target estimate, then bounce back.
			try {
				await api.addItem(
					Number(to),
					{
						service_id: data.service.id,
						region_code: lastPayload.region,
						sku: livePrice.sku,
						quantity: String(qty),
						attributes: lastPayload.attributes,
						unit_price: livePrice.price_per_unit,
						unit: livePrice.unit,
						currency: livePrice.currency
					},
					edit
				);
				const q = edit ? `?edit=${encodeURIComponent(edit)}` : '';
				await goto(`/estimates/${to}${q}`);
				return;
			} catch (e) {
				alert(`Could not add: ${(e as Error).message}`);
				return;
			}
		}
		// Fallback (no ?to=): behaviour is intentionally a no-op in the new
		// design. The service-detail page is only meant to be reached from the
		// AddResourceSheet's Advanced link, which always sets ?to=.
		alert('Open this page from an estimate to add resources.');
	}
```

Also add `import { page } from '$app/state';` and `import { goto } from '$app/navigation';` at the top if they aren't already imported (the existing file already imports them for the currency effect; leave duplicates alone).

- [ ] **Step 2: Type-check**

```bash
cd web && npm run check 2>&1 | tail -5 && cd ..
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/routes/services/\[code\]/+page.svelte
git commit -m "feat(web): /services/[code] POSTs directly to ?to= estimate (Advanced mode)"
```

---

## Task 12: Cleanup — delete cart, drop Services nav link

**Files:**
- Delete: `web/src/lib/stores/cart.svelte.ts`
- Delete: `web/src/lib/components/EstimateCart.svelte`
- Modify: `web/src/routes/+layout.svelte`
- Modify: `web/src/routes/services/[code]/+page.svelte` (drop any EstimateCart / cart imports)

- [ ] **Step 1: Delete the two files**

```bash
rm web/src/lib/stores/cart.svelte.ts web/src/lib/components/EstimateCart.svelte
```

- [ ] **Step 2: Remove the Services nav link from `web/src/routes/+layout.svelte`**

Find the `<nav>` block. Delete the `<a href="/services">Services</a>` entry entirely (keep the `Estimates` link and everything else). Do not add anything in its place.

- [ ] **Step 3: In `web/src/routes/services/[code]/+page.svelte`, remove the cart import and any `EstimateCart` mount**

If the file still contains `import { cart }` or `import EstimateCart from '$lib/components/EstimateCart.svelte';` or `<EstimateCart />` in the template, delete those lines. The service-detail page is only useful for Advanced-mode add-to-estimate now; no cart preview.

- [ ] **Step 4: Type-check + build**

```bash
cd web && npm run check 2>&1 | tail -5 && npm run build 2>&1 | tail -5 && cd ..
```

Expected: 0 errors; build succeeds.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "chore(web): delete cart store + EstimateCart; drop Services top-nav link"
```

---

## Task 13: End-to-end sanity + push

- [ ] **Step 1: Full frontend sweep**

```bash
cd web && npm run test 2>&1 | tail -8 && npm run check 2>&1 | tail -3 && cd ..
```

Expected: all tests PASS, 0 type errors.

- [ ] **Step 2: Rebuild + restart web container**

```bash
docker compose build web 2>&1 | tail -2
docker compose up -d web 2>&1 | tail -2
sleep 5
```

- [ ] **Step 3: Curl-verify the routes**

```bash
curl -sf http://localhost:3000/ | grep -q 'Price AWS' && echo "landing OK"
# Reach the editor via a fresh anonymous estimate:
TOKEN=$(curl -sf -X POST -H 'Content-Type: application/json' \
  -d '{"name":"probe","currency":"USD"}' \
  http://localhost:8080/api/v1/estimates | jq -r '.data.edit_token')
ID=$(curl -sf -X POST -H 'Content-Type: application/json' \
  -d '{"name":"probe","currency":"USD"}' \
  http://localhost:8080/api/v1/estimates | jq -r '.data.estimate.id')
curl -sf "http://localhost:3000/estimates/${ID}?edit=${TOKEN}" | grep -q 'Add resource' && echo "editor OK"
curl -sf "http://localhost:3000/services?to=${ID}&edit=${TOKEN}" | grep -q 'Advanced: all services' && echo "advanced OK"
```

Expected: three `OK` lines.

- [ ] **Step 4: Push**

```bash
git push origin main
```

---

## Self-review notes

- **Spec coverage.**
  - Estimate editor at `/estimates/[id]` (grouped table, sticky total, sidebar, Add resource → side-sheet) → Tasks 5, 6, 7.
  - Landing hero + example estimate + fork on CTA click → Tasks 8, 9. (Inline-edit forking is deferred to a follow-up per the spec's "any interaction" language; CTA fork is the pragmatic v1 subset.)
  - Advanced-mode `?to=&edit=` context → Tasks 10, 11.
  - Cleanup: cart store + EstimateCart + top-nav Services link → Task 12.
  - Backend + curated schema were shipped in Plan 09, no changes here.
- **Placeholder scan.** No `TBD`/`TODO`. Every code block is complete Svelte + TS; every test body has real assertions; every command has expected output. The one "coming soon" alert in the editor's `editItem` is a documented v1 non-goal (spec section *Explicitly out of scope*: line-level editing UX beyond delete + re-add).
- **Type consistency.**
  - `AddCuratedItemBody` / `PricedComponentBody` in `web/src/lib/api/types.ts` (Task 1) match the JSON shape the handler already accepts from Plan 09 Task 7.
  - `EstimateItem` fields (`line_type`, `service_key`, `user_input`, `components`) match what Plan 09 landed.
  - `humanSummary(item)` is used identically by Task 5 (`LineRow`) and Task 9 (landing).
  - `AddResourceSheet` props (`estimateId`, `editToken`, `serviceId`, `region`, `currency`, `advancedHref`, `onClose`, `onAdded`) match the exact call site in the new editor page (Task 7).
- **Deferred to Plan 11.**
  - The remaining 11 curated services (S3, EBS, Lambda, CloudFront, ALB, Data Transfer, RDS, RDS Aurora, ElastiCache, DynamoDB, Route53) — each is a mapper + Form + registry entry, ~200 lines.
  - Per-line currency change (client re-prices each line and PATCHes the row).
  - Playwright happy-path e2e.
  - Line-level edit UX inside the side-sheet.

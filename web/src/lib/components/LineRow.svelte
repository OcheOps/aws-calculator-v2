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

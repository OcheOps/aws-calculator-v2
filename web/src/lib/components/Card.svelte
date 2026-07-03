<script lang="ts">
	type Props = { title?: string; children: import('svelte').Snippet };
	let { title, children }: Props = $props();
</script>

<div class="rounded-lg border border-[--color-border] bg-[--color-surface-elev] p-5">
	{#if title}<h3 class="mb-3 text-sm font-medium text-[--color-text-muted]">{title}</h3>{/if}
	{@render children()}
</div>

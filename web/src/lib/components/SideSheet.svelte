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

<script lang="ts">
	type Variant = 'primary' | 'secondary' | 'ghost';
	type Props = {
		variant?: Variant;
		href?: string;
		type?: 'button' | 'submit';
		disabled?: boolean;
		onclick?: (e: MouseEvent) => void;
		children: import('svelte').Snippet;
	};
	let { variant = 'primary', href, type = 'button', disabled = false, onclick, children }: Props =
		$props();

	const styles: Record<Variant, string> = {
		primary:
			'bg-[--color-accent] text-[--color-accent-fg] hover:opacity-90',
		secondary:
			'border border-[--color-border] text-[--color-text] hover:border-[--color-text]',
		ghost: 'text-[--color-text-muted] hover:text-[--color-text]'
	};
	const cls = `inline-flex items-center justify-center rounded-md px-3 py-1.5 text-sm font-medium transition-colors disabled:opacity-50 ${styles[variant]}`;
</script>

{#if href}
	<a {href} class={cls}>
		{@render children()}
	</a>
{:else}
	<button {type} {disabled} {onclick} class={cls}>
		{@render children()}
	</button>
{/if}

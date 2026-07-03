<script lang="ts">
	import { money } from '$lib/utils/format';
	type Props = { amount: number | string; currency: string; muted?: boolean };
	let { amount, currency, muted = false }: Props = $props();
</script>

<span
	class="mono tnum whitespace-nowrap {muted
		? 'text-[--color-text-muted]'
		: 'text-[--color-text]'}"
>
	{money(amount, currency)}
</span>

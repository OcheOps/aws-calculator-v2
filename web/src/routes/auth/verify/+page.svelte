<script lang="ts">
	import Card from '$lib/components/Card.svelte';
	let { data } = $props();
</script>

<div class="mx-auto max-w-md py-12">
	<Card title="Verification failed">
		<p class="text-sm text-[--color-text-muted]">
			{data.error ?? 'Could not verify your link. It may have expired or been used already.'}
		</p>
		<p class="mt-3 text-sm">
			<a class="underline" href="/auth/request">Request a new link</a>
		</p>
	</Card>
</div>

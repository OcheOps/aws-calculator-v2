<script lang="ts">
	import Card from '$lib/components/Card.svelte';
	import Input from '$lib/components/Input.svelte';
	import Button from '$lib/components/Button.svelte';
	import { makeApi } from '$lib/api/client';

	const api = makeApi();
	let email = $state('');
	let submitted = $state(false);
	let error = $state<string | null>(null);
	let submitting = $state(false);

	async function submit(e: Event) {
		e.preventDefault();
		if (!email) return;
		submitting = true;
		try {
			await api.requestLink(email);
			submitted = true;
		} catch (err) {
			error = (err as Error).message;
		} finally {
			submitting = false;
		}
	}
</script>

<div class="mx-auto max-w-md py-12">
	<Card title="Sign in with a magic link">
		{#if submitted}
			<p class="text-sm text-[--color-text-muted]">
				If an account exists for <span class="font-medium text-[--color-text]">{email}</span>, a
				sign-in link is on its way. Check your inbox.
			</p>
		{:else}
			<form onsubmit={submit} class="space-y-4">
				<Input label="Email" type="email" bind:value={email} />
				{#if error}<p class="text-xs text-red-500">{error}</p>{/if}
				<Button type="submit" disabled={submitting}>
					{submitting ? 'Sending…' : 'Send magic link'}
				</Button>
			</form>
		{/if}
	</Card>
</div>

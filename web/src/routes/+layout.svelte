<script lang="ts">
	import '../app.css';
	import ThemeToggle from '$lib/components/ThemeToggle.svelte';
	import CurrencyPicker from '$lib/components/CurrencyPicker.svelte';
	import { session } from '$lib/stores/session.svelte';
	import { onMount } from 'svelte';

	let { data, children } = $props();

	onMount(() => {
		session.set(data.user ?? null);
	});
</script>

<div class="min-h-screen bg-[--color-surface] text-[--color-text]">
	<header class="border-b border-[--color-border]">
		<div class="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
			<a href="/" class="text-sm font-semibold tracking-tight">AWS Calculator</a>
			<nav class="flex items-center gap-6 text-sm">
				<a href="/services" class="text-[--color-text-muted] hover:text-[--color-text]">
					Services
				</a>
				{#if session.user}
					<a href="/estimates" class="text-[--color-text-muted] hover:text-[--color-text]">
						Estimates
					</a>
				{/if}
				<CurrencyPicker />
				<ThemeToggle />
				{#if session.user}
					<span class="text-xs text-[--color-text-muted]">{session.user.email}</span>
					<form method="POST" action="/auth/logout">
						<button
							class="rounded-md border border-[--color-border] px-3 py-1.5 text-xs font-medium text-[--color-text] hover:border-[--color-text]"
						>
							Sign out
						</button>
					</form>
				{:else}
					<a
						href="/auth/request"
						class="rounded-md bg-[--color-accent] px-3 py-1.5 text-xs font-medium text-[--color-accent-fg]"
					>
						Sign in
					</a>
				{/if}
			</nav>
		</div>
	</header>
	<main class="mx-auto max-w-6xl px-6 py-10">
		{@render children()}
	</main>
</div>

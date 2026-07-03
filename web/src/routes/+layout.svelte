<script lang="ts">
	import '../app.css';
	import ThemeToggle from '$lib/components/ThemeToggle.svelte';
	import CurrencyPicker from '$lib/components/CurrencyPicker.svelte';
	import { session } from '$lib/stores/session.svelte';

	let { data, children } = $props();

	// Keep the client store in sync with SSR data on every navigation so the
	// email is available for both server-rendered and client-navigated frames.
	$effect(() => {
		session.set(data.user ?? null);
	});

	// Render from `data.user` (available during SSR) rather than the client
	// store so the header never flashes a signed-out state after hydration.
	const user = $derived(data.user);
</script>

<div class="min-h-screen bg-[--color-surface] text-[--color-text]">
	<header class="border-b border-[--color-border]">
		<div class="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
			<a href="/" class="text-sm font-semibold tracking-tight">AWS Calculator</a>
			<nav class="flex items-center gap-6 text-sm">
				<a href="/services" class="text-[--color-text-muted] hover:text-[--color-text]">
					Services
				</a>
				{#if user}
					<a href="/estimates" class="text-[--color-text-muted] hover:text-[--color-text]">
						Estimates
					</a>
				{/if}
				<CurrencyPicker />
				<ThemeToggle />
				{#if user}
					<span class="text-xs text-[--color-text-muted]">{user.email}</span>
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

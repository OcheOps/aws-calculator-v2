<script lang="ts">
	import Card from '$lib/components/Card.svelte';
</script>

<section class="space-y-16">
	<div class="max-w-2xl space-y-6">
		<h1 class="text-5xl font-semibold tracking-tight leading-tight">
			Price AWS. Precisely.
		</h1>
		<p class="text-lg text-[--color-text-muted]">
			Build an estimate from live AWS pricing across every service and region. Save it, share it,
			export it.
		</p>
		<div class="flex gap-3">
			<a
				href="/services"
				class="rounded-md bg-[--color-accent] px-4 py-2 text-sm font-medium text-[--color-accent-fg]"
			>
				Browse services
			</a>
			<a
				href="/auth/request"
				class="rounded-md border border-[--color-border] px-4 py-2 text-sm font-medium text-[--color-text] hover:border-[--color-text]"
			>
				Sign in
			</a>
		</div>
	</div>

	<div class="grid gap-4 md:grid-cols-3">
		<Card title="Live pricing">
			<p class="text-sm text-[--color-text-muted]">
				Prices refreshed daily from the AWS Pricing API. Every SKU, every region, no stale
				bundles.
			</p>
		</Card>
		<Card title="Save & share">
			<p class="text-sm text-[--color-text-muted]">
				Anonymous estimates get a private link. Sign in to keep them in a workspace and share
				with a public URL.
			</p>
		</Card>
		<Card title="Your currency">
			<p class="text-sm text-[--color-text-muted]">
				Display prices in USD, EUR, GBP, or NGN. Live FX rates, applied on read so estimates
				stay comparable.
			</p>
		</Card>
	</div>
</section>

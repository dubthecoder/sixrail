<script lang="ts">
	import type { Alert } from '$lib/api';

	let {
		alerts = [],
		routeNames = []
	}: {
		alerts: Alert[];
		routeNames: string[];
	} = $props();

	let relevant = $derived(
		alerts.filter(
			(a) =>
				!a.routeNames ||
				a.routeNames.length === 0 ||
				a.routeNames.some((r) => routeNames.includes(r))
		)
	);

	let expanded = $state(false);
</script>

{#if relevant.length > 0}
	<button
		class="alert-banner w-full text-left"
		onclick={() => (expanded = !expanded)}
		aria-expanded={expanded}
	>
		<div class="banner-bar">
			<span class="icon">⚠</span>
			<span class="text text-xs font-mono uppercase tracking-wide">
				{relevant[0].headline}
				{#if relevant.length > 1}(+{relevant.length - 1} more){/if}
			</span>
			<span class="chevron text-xs">{expanded ? '▲' : '▼'}</span>
		</div>

		{#if expanded}
			<div class="banner-details">
				{#each relevant as alert}
					<div class="alert-item">
						<p class="font-mono text-xs text-amber-200">{alert.headline}</p>
						{#if alert.description}
							<p class="text-xs text-gray-400 mt-1">{alert.description}</p>
						{/if}
					</div>
				{/each}
			</div>
		{/if}
	</button>
{/if}

<style>
	.alert-banner {
		background: var(--color-alert-bg);
		border: 1px solid var(--color-alert-border);
		border-radius: 6px;
		overflow: hidden;
		margin-bottom: 12px;
	}

	.banner-bar {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 12px;
		color: var(--color-accent);
	}

	.icon {
		font-size: 0.9em;
	}
	.text {
		flex: 1;
	}

	.banner-details {
		border-top: 1px solid var(--color-alert-border-inner);
		padding: 8px 12px;
	}

	.alert-item + .alert-item {
		margin-top: 8px;
		padding-top: 8px;
		border-top: 1px solid var(--color-alert-border-subtle);
	}
</style>

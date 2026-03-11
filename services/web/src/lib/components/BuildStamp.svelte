<script lang="ts">
	import type { BuildInfo } from '$lib/build-info';

	let { buildInfo }: { buildInfo: BuildInfo } = $props();

	const title = $derived(
		buildInfo.fullSha ? `Git commit ${buildInfo.fullSha}` : 'Local development build'
	);
</script>

<div class="build-stamp" {title} aria-label={`v${buildInfo.shortSha ?? buildInfo.version}`}>
	<span class="build-value">v{buildInfo.shortSha ?? buildInfo.version}</span>
</div>

<style>
	.build-stamp {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 9px;
		border: 1px solid color-mix(in srgb, var(--color-border) 85%, transparent);
		border-radius: 999px;
		background: color-mix(in srgb, var(--color-surface-raised) 96%, transparent);
		color: var(--color-gray-500);
		font-family: monospace;
		font-size: 9px;
		letter-spacing: 0.08em;
		text-transform: lowercase;
	}

	.build-value {
		color: var(--color-amber-400, #fbbf24);
	}
</style>

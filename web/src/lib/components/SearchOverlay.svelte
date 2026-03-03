<script lang="ts">
	import type { Stop } from '$lib/api';

	let { stops = [], onstationselect }: { stops: Stop[]; onstationselect: (stop: Stop) => void } =
		$props();
	let query = $state('');
	let open = $state(false);

	let filtered = $derived(
		query.length > 0
			? stops.filter((s) => s.name.toLowerCase().includes(query.toLowerCase())).slice(0, 8)
			: []
	);

	function select(stop: Stop) {
		query = stop.name;
		open = false;
		onstationselect(stop);
	}

	function handleFocus() {
		open = true;
	}

	function handleInput() {
		open = true;
	}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
{#if open && filtered.length > 0}
	<div class="fixed inset-0 z-10" onclick={() => (open = false)} onkeydown={() => {}}></div>
{/if}

<div class="absolute top-4 left-1/2 -translate-x-1/2 z-20 w-[90vw] max-w-md">
	<input
		type="text"
		bind:value={query}
		onfocus={handleFocus}
		oninput={handleInput}
		placeholder="Search stations..."
		class="w-full px-4 py-3 rounded-lg shadow-lg border border-gray-200 bg-white text-gray-900 focus:outline-none focus:ring-2 focus:ring-green-600"
	/>
	{#if open && filtered.length > 0}
		<ul class="mt-1 bg-white border border-gray-200 rounded-lg shadow-lg max-h-72 overflow-y-auto">
			{#each filtered as stop (stop.id)}
				<li>
					<button
						onclick={() => select(stop)}
						class="w-full text-left px-4 py-2.5 hover:bg-green-50 cursor-pointer text-sm"
					>
						{stop.name}
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<script lang="ts">
	import type { Stop } from '$lib/api';

	let {
		stops,
		placeholder = 'Search stations...',
		value = $bindable(''),
		onSelect
	}: {
		stops: Stop[];
		placeholder?: string;
		value?: string;
		onSelect: (stop: Stop) => void;
	} = $props();

	const sortedStops = $derived([...stops].sort((a, b) => a.name.localeCompare(b.name)));
	let results = $state<Stop[]>([]);
	let showDropdown = $state(false);
	let activeIndex = $state(-1);

	const listboxId = `listbox-${crypto.randomUUID().slice(0, 8)}`;

	function search(q: string) {
		value = q;
		if (q.length === 0) {
			results = sortedStops;
		} else {
			const lower = q.toLowerCase();
			results = sortedStops.filter((s) => s.name.toLowerCase().includes(lower));
		}
		showDropdown = results.length > 0;
		activeIndex = -1;
	}

	function onFocus() {
		if (value.length === 0) {
			results = sortedStops;
		} else {
			const lower = value.toLowerCase();
			results = sortedStops.filter((s) => s.name.toLowerCase().includes(lower));
		}
		showDropdown = results.length > 0;
	}

	let tappingDropdown = false;

	function select(stop: Stop) {
		tappingDropdown = false;
		value = stop.name;
		results = [];
		showDropdown = false;
		activeIndex = -1;
		onSelect(stop);
	}

	function onBlur() {
		setTimeout(() => {
			if (!tappingDropdown) {
				showDropdown = false;
				activeIndex = -1;
			}
		}, 200);
	}

	function scrollActiveIntoView() {
		const el = document.getElementById(`${listboxId}-${activeIndex}`);
		el?.scrollIntoView({ block: 'nearest' });
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowDown' && !showDropdown) {
			e.preventDefault();
			onFocus();
			return;
		}
		if (!showDropdown) return;

		switch (e.key) {
			case 'ArrowDown':
				e.preventDefault();
				activeIndex = activeIndex < results.length - 1 ? activeIndex + 1 : 0;
				scrollActiveIntoView();
				break;
			case 'ArrowUp':
				e.preventDefault();
				activeIndex = activeIndex > 0 ? activeIndex - 1 : results.length - 1;
				scrollActiveIntoView();
				break;
			case 'Enter':
				e.preventDefault();
				if (activeIndex >= 0 && activeIndex < results.length) {
					select(results[activeIndex]);
				}
				break;
			case 'Escape':
				e.preventDefault();
				showDropdown = false;
				activeIndex = -1;
				break;
		}
	}
</script>

<div class="station-search-input relative">
	<input
		type="text"
		class="w-full bg-surface-input text-white font-mono px-3 py-2 rounded border border-border-input focus:border-accent focus:outline-none"
		style="font-size: 16px;"
		{placeholder}
		{value}
		role="combobox"
		aria-expanded={showDropdown}
		aria-controls={listboxId}
		aria-autocomplete="list"
		aria-activedescendant={activeIndex >= 0 ? `${listboxId}-${activeIndex}` : undefined}
		oninput={(e) => search((e.target as HTMLInputElement).value)}
		onfocus={onFocus}
		onclick={onFocus}
		onblur={onBlur}
		onkeydown={onKeydown}
		autocomplete="off"
	/>
	{#if showDropdown}
		<ul
			id={listboxId}
			class="dropdown absolute z-50 w-full mt-1 bg-surface-input border border-border-input rounded shadow-lg max-h-64 overflow-y-auto"
			role="listbox"
			onpointerdown={() => (tappingDropdown = true)}
		>
			{#each results as stop, i}
				<li id="{listboxId}-{i}" role="option" aria-selected={i === activeIndex}>
					<button
						class="w-full text-left px-3 py-2 font-mono text-white hover:bg-border focus:bg-border"
						class:bg-border={i === activeIndex}
						style="font-size: 16px;"
						onclick={() => select(stop)}
						tabindex={-1}
					>
						{stop.name}
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>

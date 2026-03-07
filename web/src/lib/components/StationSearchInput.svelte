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

  let results = $state<Stop[]>([]);
  let showDropdown = $state(false);

  function search(q: string) {
    value = q;
    if (q.length < 2) {
      results = [];
      showDropdown = false;
      return;
    }
    const lower = q.toLowerCase();
    results = stops.filter((s) => s.name.toLowerCase().includes(lower)).slice(0, 8);
    showDropdown = results.length > 0;
  }

  function select(stop: Stop) {
    value = stop.name;
    results = [];
    showDropdown = false;
    onSelect(stop);
  }

  function onBlur() {
    setTimeout(() => (showDropdown = false), 150);
  }
</script>

<div class="station-search-input relative">
  <input
    type="text"
    class="w-full bg-[#1e1e1e] text-white font-mono text-sm px-3 py-2 rounded border border-[#333] focus:border-amber-400 focus:outline-none"
    {placeholder}
    {value}
    oninput={(e) => search((e.target as HTMLInputElement).value)}
    onblur={onBlur}
    autocomplete="off"
  />
  {#if showDropdown}
    <ul class="dropdown absolute z-50 w-full mt-1 bg-[#1e1e1e] border border-[#333] rounded shadow-lg max-h-48 overflow-y-auto">
      {#each results as stop}
        <li>
          <button
            class="w-full text-left px-3 py-2 text-sm font-mono text-white hover:bg-[#2a2a2a] focus:bg-[#2a2a2a]"
            onmousedown={() => select(stop)}
          >
            {stop.name}
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>

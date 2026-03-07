<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  let { scheduledTime }: { scheduledTime: string } = $props();

  let display = $state('--:--');
  let interval: ReturnType<typeof setInterval>;

  function computeCountdown(scheduled: string): string {
    const now = new Date();
    const [h, m] = scheduled.split(':').map(Number);
    const target = new Date(now);
    target.setHours(h, m, 0, 0);
    if (target < now) target.setDate(target.getDate() + 1);
    const diffMs = target.getTime() - now.getTime();
    if (diffMs < 0) return '00:00';
    const mins = Math.floor(diffMs / 60000);
    const secs = Math.floor((diffMs % 60000) / 1000);
    return `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
  }

  onMount(() => {
    display = computeCountdown(scheduledTime);
    interval = setInterval(() => {
      display = computeCountdown(scheduledTime);
    }, 1000);
  });

  onDestroy(() => clearInterval(interval));

  $effect(() => {
    display = computeCountdown(scheduledTime);
  });
</script>

<div class="countdown" role="timer" aria-label="Time until next departure">
  <span class="label text-gray-500 text-xs uppercase tracking-widest">Next train in</span>
  <span class="time font-mono text-amber-400 tabular-nums">{display}</span>
</div>

<style>
  .countdown {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    background: #1a1a1a;
    border-radius: 8px;
    padding: 12px 24px;
    min-width: 160px;
  }

  .time {
    font-size: 2rem;
    letter-spacing: 0.1em;
  }
</style>

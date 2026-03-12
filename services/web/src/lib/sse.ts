type SSEHandler = (data: unknown) => void;
type SSEStatusHandler = (connected: boolean) => void;

const handlers = new Map<string, SSEHandler[]>();
const statusHandlers: SSEStatusHandler[] = [];
let eventSource: EventSource | null = null;
let sseUrl: string | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let stopped = false;

const RECONNECT_INTERVAL_MS = 5000;

function notifyStatus(connected: boolean) {
	for (const handler of statusHandlers) handler(connected);
}

export function onSSEStatus(handler: SSEStatusHandler): () => void {
	statusHandlers.push(handler);
	return () => {
		const idx = statusHandlers.indexOf(handler);
		if (idx >= 0) statusHandlers.splice(idx, 1);
	};
}

function createEventSource(url: string) {
	const es = new EventSource(url);

	es.onopen = () => notifyStatus(true);

	for (const event of ['alerts', 'union-departures']) {
		es.addEventListener(event, (e: MessageEvent) => {
			let data: unknown;
			try {
				data = JSON.parse(e.data);
			} catch {
				console.warn('SSE: malformed JSON for event', event);
				return;
			}
			for (const handler of handlers.get(event) || []) {
				handler(data);
			}
		});
	}

	es.onerror = () => {
		notifyStatus(false);
		// EventSource with readyState CLOSED won't auto-reconnect (e.g. server returned
		// non-200). Manually reconnect after a delay.
		if (es.readyState === EventSource.CLOSED) {
			console.warn('SSE connection closed by server, reconnecting...');
			es.close();
			eventSource = null;
			scheduleReconnect();
		}
	};

	return es;
}

function scheduleReconnect() {
	if (stopped || reconnectTimer) return;
	reconnectTimer = setTimeout(() => {
		reconnectTimer = null;
		if (stopped || !sseUrl) return;
		eventSource = createEventSource(sseUrl);
	}, RECONNECT_INTERVAL_MS);
}

export function connectSSE(url: string) {
	if (eventSource) return;
	stopped = false;
	sseUrl = url;
	eventSource = createEventSource(url);
}

export function onSSE(event: string, handler: SSEHandler): () => void {
	if (!handlers.has(event)) handlers.set(event, []);
	handlers.get(event)!.push(handler);
	return () => {
		const list = handlers.get(event);
		if (list) {
			const idx = list.indexOf(handler);
			if (idx >= 0) list.splice(idx, 1);
		}
	};
}

export function disconnectSSE() {
	stopped = true;
	if (reconnectTimer) {
		clearTimeout(reconnectTimer);
		reconnectTimer = null;
	}
	eventSource?.close();
	eventSource = null;
	sseUrl = null;
	for (const fn of statusHandlers) fn(false);
	handlers.clear();
	statusHandlers.length = 0;
}

type SSEHandler = (data: unknown) => void;
type SSEStatusHandler = (connected: boolean) => void;

const handlers = new Map<string, SSEHandler[]>();
const statusHandlers: SSEStatusHandler[] = [];
let eventSource: EventSource | null = null;

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

export function connectSSE(url: string) {
	if (eventSource) return;
	eventSource = new EventSource(url);

	eventSource.onopen = () => notifyStatus(true);

	for (const event of ['alerts', 'union-departures']) {
		eventSource.addEventListener(event, (e: MessageEvent) => {
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

	eventSource.onerror = () => {
		console.warn('SSE connection lost, auto-reconnecting...');
		notifyStatus(false);
	};
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
	eventSource?.close();
	eventSource = null;
	handlers.clear();
	statusHandlers.length = 0;
}

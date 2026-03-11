type SSEHandler = (data: unknown) => void;

const handlers = new Map<string, SSEHandler[]>();
let eventSource: EventSource | null = null;

export function connectSSE(url: string) {
	if (eventSource) return;
	eventSource = new EventSource(url);

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
}

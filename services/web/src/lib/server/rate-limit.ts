import { env } from '$env/dynamic/private';
import { createClient } from 'redis';

const RATE_WINDOW_MS = 60_000;
const SSE_TTL_SECONDS = 120;
const API_RATE_PREFIX = 'railsix:web:rate';
const SSE_RATE_PREFIX = 'railsix:web:sse';

type MemoryEntry = {
	count: number;
	resetAt: number;
};

const hits = new Map<string, MemoryEntry>();
const sseConns = new Map<string, number>();

type RedisClient = ReturnType<typeof createClient>;

let redisClientPromise: Promise<RedisClient | null> | null = null;
let redisWarningLogged = false;

setInterval(() => {
	const now = Date.now();
	for (const [ip, entry] of hits) {
		if (now > entry.resetAt) hits.delete(ip);
	}
}, 300_000);

function getRedisUrl(): string | null {
	if (env.REDIS_URL) return env.REDIS_URL;
	if (!env.REDIS_ADDR) return null;

	if (env.REDIS_PASSWORD) {
		return `redis://:${encodeURIComponent(env.REDIS_PASSWORD)}@${env.REDIS_ADDR}`;
	}

	return `redis://${env.REDIS_ADDR}`;
}

function logRedisWarning(error: unknown) {
	if (redisWarningLogged) return;
	redisWarningLogged = true;
	console.warn('[rate-limit] Redis unavailable, using in-memory fallback:', error);
}

async function getRedisClient(): Promise<RedisClient | null> {
	if (redisClientPromise) return redisClientPromise;

	const redisUrl = getRedisUrl();
	if (!redisUrl) {
		redisClientPromise = Promise.resolve(null);
		return redisClientPromise;
	}

	redisClientPromise = (async () => {
		try {
			const client = createClient({
				url: redisUrl,
				socket: {
					connectTimeout: 1000
				}
			});
			client.on('error', (error) => logRedisWarning(error));
			await client.connect();
			return client;
		} catch (error) {
			logRedisWarning(error);
			return null;
		}
	})();

	return redisClientPromise;
}

function currentRateBucket(now = Date.now()) {
	const bucket = Math.floor(now / RATE_WINDOW_MS);
	const resetAt = (bucket + 1) * RATE_WINDOW_MS;
	return { bucket, resetAt };
}

function memoryRateLimit(ip: string, maxRequests: number): boolean {
	const now = Date.now();
	const entry = hits.get(ip);

	if (!entry || now > entry.resetAt) {
		hits.set(ip, { count: 1, resetAt: now + RATE_WINDOW_MS });
		return false;
	}

	entry.count++;
	return entry.count > maxRequests;
}

function memoryOpenSSE(ip: string, maxConnections: number): boolean {
	const current = sseConns.get(ip) ?? 0;
	if (current >= maxConnections) return false;
	sseConns.set(ip, current + 1);
	return true;
}

function memoryCloseSSE(ip: string) {
	const count = (sseConns.get(ip) ?? 1) - 1;
	if (count <= 0) sseConns.delete(ip);
	else sseConns.set(ip, count);
}

export async function isRateLimited(ip: string, maxRequests: number): Promise<boolean> {
	const client = await getRedisClient();
	if (!client) {
		return memoryRateLimit(ip, maxRequests);
	}

	const { bucket, resetAt } = currentRateBucket();
	const key = `${API_RATE_PREFIX}:${bucket}:${ip}`;

	try {
		const count = await client.incr(key);
		if (count === 1) {
			await client.pExpire(key, Math.max(resetAt - Date.now(), 1));
		}
		return count > maxRequests;
	} catch (error) {
		logRedisWarning(error);
		return memoryRateLimit(ip, maxRequests);
	}
}

export async function openSSE(ip: string, maxConnections: number): Promise<boolean> {
	const client = await getRedisClient();
	if (!client) {
		return memoryOpenSSE(ip, maxConnections);
	}

	const key = `${SSE_RATE_PREFIX}:${ip}`;

	try {
		const count = await client.incr(key);
		await client.expire(key, SSE_TTL_SECONDS);
		if (count > maxConnections) {
			const remaining = await client.decr(key);
			if (remaining <= 0) {
				await client.del(key);
			}
			return false;
		}
		return true;
	} catch (error) {
		logRedisWarning(error);
		return memoryOpenSSE(ip, maxConnections);
	}
}

export async function closeSSE(ip: string): Promise<void> {
	const client = await getRedisClient();
	if (!client) {
		memoryCloseSSE(ip);
		return;
	}

	const key = `${SSE_RATE_PREFIX}:${ip}`;

	try {
		const remaining = await client.decr(key);
		if (remaining <= 0) {
			await client.del(key);
		}
	} catch (error) {
		logRedisWarning(error);
		memoryCloseSSE(ip);
	}
}

export function resetLimiterStateForTests() {
	hits.clear();
	sseConns.clear();
}

/// <reference types="node" />
import { execSync } from 'child_process';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';
import { loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, '.', '');
	const devPort = Number(env.VITE_DEV_PORT || env.PORT || 5173);
	const disableHmr = env.VITE_DISABLE_HMR === '1';
	const publicHmrUrl = env.VITE_PUBLIC_HMR_URL;

	let hmr:
		| false
		| undefined
		| {
				protocol?: 'ws' | 'wss';
				host?: string;
				clientPort?: number;
				path?: string;
		  } = undefined;

	if (disableHmr) {
		hmr = false;
	} else if (publicHmrUrl) {
		const url = new URL(publicHmrUrl);
		hmr = {
			protocol: url.protocol === 'https:' ? 'wss' : 'ws',
			host: url.hostname,
			clientPort: Number(url.port || (url.protocol === 'https:' ? 443 : 80)),
			path: env.VITE_PUBLIC_HMR_PATH || '/__vite_hmr'
		};
	}

	let appVersion = '0';
	try {
		appVersion = execSync('git rev-list --count HEAD', { encoding: 'utf-8' }).trim();
	} catch {
		// git not available (e.g. Railway build) — fall back to env or default
		appVersion = process.env.RAILWAY_GIT_COMMIT_SHA?.slice(0, 7) ?? '0';
	}

	return {
		define: {
			__APP_VERSION__: JSON.stringify(appVersion)
		},
		plugins: [tailwindcss(), sveltekit()],
		server: {
			host: true,
			port: devPort,
			strictPort: true,
			allowedHosts: true,
			hmr
		},
		test: {
			environment: 'node',
			include: ['src/**/*.test.ts']
		}
	};
});

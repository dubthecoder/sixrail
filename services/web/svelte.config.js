import adapter from '@sveltejs/adapter-node';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	kit: {
		adapter: adapter(),
		csp: {
			mode: 'auto',
			directives: {
				'default-src': ['self'],
				'script-src': ['self', 'https://umami.teclara.cloud'],
				'style-src': ['self', 'unsafe-inline'],
				'img-src': ['self', 'data:', 'https:'],
				'connect-src': ['self', 'https://umami.teclara.cloud'],
				'font-src': ['self'],
				'worker-src': ['self', 'blob:'],
				'manifest-src': ['self'],
				'frame-ancestors': ['none']
			}
		}
	}
};

export default config;

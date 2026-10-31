/// <reference types="vitest/config" />

import adapter from '@sveltejs/adapter-static';
import { svelte, vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { playwright } from '@vitest/browser-playwright';
import { defineConfig } from 'vite';
import { fileURLToPath } from 'node:url';
import { readFileSync } from 'node:fs';
import { localSingleplayer } from './local-singleplayer.ts';

const file = fileURLToPath(new URL('package.json', import.meta.url));
const json = readFileSync(file, 'utf8');
const pkg = JSON.parse(json);

export default defineConfig({
	cacheDir: process.env.STARS_TEST_KIT_DIR ? 'node_modules/.vite-content-test' : 'node_modules/.vite',
	// Screens are loaded on demand. Prebundle their shared dependencies together
	// so a cold local launch does not replace client modules during navigation.
	optimizeDeps: { noDiscovery: true, include: ['@connectrpc/connect-web', '@connectrpc/connect', '@bufbuild/protobuf', '@bufbuild/protobuf/codegenv2', '@bufbuild/protobuf/wkt', 'lodash-es', '@steeze-ui/heroicons', '@steeze-ui/svelte-icon', '@floating-ui/dom', 'ts-enum-util', 'd3-array', 'd3-format', 'd3-selection', 'd3-zoom', 'd3-scale', 'd3-quadtree', 'hotkeys-js', 'layercake'] },
	plugins: [
		localSingleplayer(),
		tailwindcss(),
		sveltekit({
			outDir: process.env.STARS_TEST_KIT_DIR ?? '.svelte-kit',
			preprocess: vitePreprocess(),
			adapter: adapter({ fallback: 'index.html' }),
			prerender: { entries: [] },
			version: {
				name: pkg.version,
				pollInterval: pkg.version === '0.0.0-develop' ? 0 : 5 * 60_000
			}
		})
	],
	test: {
		reporters: ['junit', process.env.CI ? 'github-actions' : 'default'],
		outputFile: '../tmp/test-results/vitest-report.xml',

		projects: [
			{
				// Browser component tests use Vitest's HTML harness, without SvelteKit's router.
				plugins: [tailwindcss(), svelte({ preprocess: vitePreprocess() })],
				define: { PKG: pkg },
				// Without the SvelteKit plugin, $app/* imports don't resolve, so point them at stubs.
				resolve: {
					alias: {
						'$app/env': fileURLToPath(new URL('src/test/app/env.ts', import.meta.url)),
						'$app/navigation': fileURLToPath(
							new URL('src/test/app/navigation.ts', import.meta.url)
						),
						'$app/state': fileURLToPath(new URL('src/test/app/state.ts', import.meta.url))
					}
				},
				// Prebundle deps that tests reach through dynamic imports. Otherwise Vite finds them
				// mid-run on a cold cache (like CI) and reloads the page, which breaks vi.mock.
				optimizeDeps: {
					include: [
						'@connectrpc/connect-web',
						'@connectrpc/connect',
						'lodash-es',
						'ts-enum-util',
						'd3-selection',
						'd3-zoom',
						'd3-scale',
						'd3-quadtree',
						'hotkeys-js',
						'layercake',
						'@steeze-ui/heroicons',
						'@steeze-ui/svelte-icon'
					]
				},
				test: {
					name: 'client',
					browser: {
						enabled: true,
						provider: playwright(),
						instances: [{ browser: 'chromium' }]
					},
					include: ['src/**/*.svelte.{test,spec}.{js,ts}'],
					exclude: ['src/lib/server/**'],
					setupFiles: ['./vitest-setup-client.ts']
				}
			},
			{
				extends: './vite.config.ts',
				test: {
					name: 'server',
					environment: 'node',
					include: ['src/**/*.{test,spec}.{js,ts}'],
					exclude: ['src/**/*.svelte.{test,spec}.{js,ts}']
				}
			}
		]
	},
	define: {
		PKG: pkg
	},

	server: {
		proxy: {
			'/api': {
				target: process.env.STARS_LOCAL_BACKEND ?? 'http://localhost:8080',
				changeOrigin: true
			}
		}
	},
	assetsInclude: ['**/*.wasm']
});

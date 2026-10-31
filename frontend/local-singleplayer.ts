import type { Plugin } from 'vite';

// Local single-player mode, only on Vite's loopback listener.
// Use the normal authentication API so existing local saves keep their owner.
export function localSingleplayer(): Plugin {
	return {
		name: 'stars-local-singleplayer',
		configureServer(server) {
			if (process.env.STARS_LOCAL_SINGLEPLAYER === '0' || !['127.0.0.1','localhost','::1'].includes(String(server.config.server.host ?? 'localhost'))) return;
			server.middlewares.use(async (request, response, next) => {
				const path = request.url?.split('?')[0];
				if (!['/','/launch','/games','/auth/login','/local-start','/local-ready'].includes(path ?? '')) return next();
				const remote = request.socket.remoteAddress;
				const host = request.headers.host ?? '';
				if (!['127.0.0.1', '::1', '::ffff:127.0.0.1'].includes(remote ?? '') ||
					!/^localhost(?::\d+)?$|^127\.0\.0\.1(?::\d+)?$/.test(host) ||
					(request.headers.origin && request.headers.origin !== `http://${host}`)) {
					response.writeHead(403).end(); return;
				}
				response.setHeader('Cache-Control', 'no-store');
				if (request.method !== 'GET') { response.writeHead(405).end(); return; }
				if (path === '/local-ready') { response.setHeader('Content-Type', 'text/plain; charset=utf-8'); response.end('stars-local-singleplayer'); return; }
				try {
					const backend = process.env.STARS_LOCAL_BACKEND ?? 'http://127.0.0.1:8080';
					const login = await fetch(`${backend}/api/auth/local/login?session=1`, {
						method: 'POST', headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({ user: process.env.STARS_LOCAL_USER ?? 'admin', passwd: process.env.STARS_LOCAL_PASSWORD ?? 'admin' }),
						signal: AbortSignal.timeout(10000)
					});
					if (!login.ok) throw new Error(`Local session failed (${login.status})`);
					const cookies = login.headers.getSetCookie();
					if (!cookies.length) throw new Error('Local session cookie missing');
					response.setHeader('Set-Cookie', cookies.map(cookie => cookie.replace(/;\s*Domain=[^;]+/gi, '')));
					if (path === '/games' || path === '/launch') { next(); return; }
					response.writeHead(302, { Location: '/launch' }).end();
				} catch (error) {
					server.config.logger.error(error instanceof Error ? error.message : 'Local session failed');
					response.writeHead(503, { 'Content-Type': 'text/plain; charset=utf-8' });
					response.end('Could not open the local game session. Check the server log in craig-stars/tmp/launch.');
				}
			});
		}
	};
}

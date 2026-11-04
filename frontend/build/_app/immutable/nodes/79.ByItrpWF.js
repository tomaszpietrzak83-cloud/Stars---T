import{J as e,M as t,Rt as n,X as r,gt as i,ut as a,vt as o,xt as s,zt as c}from"../chunks/Dgn-K8f2.js";import"../chunks/BD0k_xHk.js";var l=r(`<main class="min-h-screen bg-base-100 text-base-content"><section class="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-8"><header class="flex flex-col gap-2 border-b border-base-300 pb-4"><a class="link w-fit" href="/">craig-stars</a> <h1 class="text-3xl font-bold">MCP setup</h1> <p class="text-base-content/75">Connect Claude, Codex, or another MCP client to your craig-stars games with the same Discord
				sign-in you use in the browser.</p></header> <section class="flex flex-col gap-3"><h2 class="text-xl font-semibold">Server URL</h2> <div class="mockup-code text-sm"><pre><code></code></pre></div></section> <section class="flex flex-col gap-3"><h2 class="text-xl font-semibold">Codex</h2> <p class="text-base-content/75">Add an HTTP MCP server named <code>craig-stars</code> with the hosted URL. When the client asks
				to authenticate, complete the browser sign-in and return to Codex.</p> <div class="mockup-code text-sm"><pre><code></code></pre></div> <div class="mockup-code text-sm"><pre><code></code></pre></div></section> <section class="flex flex-col gap-3"><h2 class="text-xl font-semibold">Claude Code</h2> <p class="text-base-content/75">Add an HTTP MCP server named <code>craig-stars</code> with the hosted URL. When Claude asks to
				authenticate, complete the browser sign-in and return to Claude Code.</p> <div class="mockup-code text-sm"><pre><code></code></pre></div></section> <section class="flex flex-col gap-3"><h2 class="text-xl font-semibold">Authentication</h2> <p class="text-base-content/75">The MCP client opens a browser, craig-stars signs you in with Discord, then redirects back
				to the requesting client. OAuth endpoints are discovered automatically from the MCP server
				URL. The client receives a bearer token for game tools like listing games, reading universe
				data, updating orders, and submitting turns.</p></section></section></main>`);function u(r){var u=l();t(`d2eyi9`,e=>{a(()=>{i.title=`craig-stars MCP`})});var d=o(u),f=s(o(d),2),p=s(o(f),2),m=o(p),h=o(m);h.textContent=`https://craig-stars.net/api/mcp`,c(m),c(p),c(f);var g=s(f,2),_=s(o(g),4),v=o(_),y=o(v);y.textContent=`codex mcp add craig-stars --url https://craig-stars.net/api/mcp --oauth-client-id codex`,c(v),c(_);var b=s(_,2),x=o(b),S=o(x);S.textContent=`
[mcp_servers.craig-stars]
url = "https://craig-stars.net/api/mcp"
enabled = true
startup_timeout_sec = 10
tool_timeout_sec = 60
default_tools_approval_mode = "prompt"

[mcp_servers.craig-stars.oauth]
client_id = "codex"
`,c(x),c(b),c(g);var C=s(g,2),w=s(o(C),4),T=o(w),E=o(T);E.textContent=`claude mcp add --transport http --client-id claude craig-stars https://craig-stars.net/api/mcp`,c(T),c(w),c(C),n(2),c(d),c(u),e(r,u)}export{u as component};
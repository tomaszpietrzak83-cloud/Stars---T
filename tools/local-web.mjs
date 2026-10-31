// Local single-player host for the compiled UI. No npm, Vite or build compiler
// is required to play a downloaded Windows copy.
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const root=path.resolve(process.env.STARS_WEB_ROOT??path.join(path.dirname(fileURLToPath(import.meta.url)),'../frontend/build'));
const backend=new URL(process.env.STARS_LOCAL_BACKEND??'http://127.0.0.1:8080');
const port=Number(process.env.STARS_HTTP_PORT??5173);
if(!['127.0.0.1','localhost','[::1]'].includes(backend.hostname)||backend.protocol!=='http:')throw Error('The game backend must be local HTTP');
const mime={'.html':'text/html; charset=utf-8','.js':'text/javascript; charset=utf-8','.css':'text/css; charset=utf-8','.wasm':'application/wasm','.json':'application/json','.png':'image/png','.svg':'image/svg+xml','.ico':'image/x-icon','.woff2':'font/woff2','.txt':'text/plain; charset=utf-8'};
const server=http.createServer(async(req,res)=>{
 const host=req.headers.host??'';
 if(!/^localhost(?::\d+)?$|^127\.0\.0\.1(?::\d+)?$/.test(host)||(req.headers.origin&&req.headers.origin!==`http://${host}`)){res.writeHead(403).end();return;}
 let url;try{url=new URL(req.url,`http://${host}`);}catch{res.writeHead(400).end();return;}
 if(url.pathname.startsWith('/api/')){
  const proxy=http.request(new URL(req.url,backend),{method:req.method,headers:{...req.headers,host:backend.host}},upstream=>{res.writeHead(upstream.statusCode??502,upstream.headers);upstream.pipe(res);});
  proxy.on('error',()=>{if(!res.headersSent)res.writeHead(502);res.end('Local game backend is unavailable');});req.pipe(proxy);return;
 }
 if(req.method!=='GET'&&req.method!=='HEAD'){res.writeHead(405).end();return;}
 if(url.pathname==='/local-ready'){res.writeHead(200,{'Content-Type':'text/plain','Cache-Control':'no-store'}).end('stars-local-singleplayer');return;}
 if(['/','/launch','/games','/auth/login','/local-start'].includes(url.pathname)){
  try{
   const login=await fetch(new URL('/api/auth/local/login?session=1',backend),{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({user:process.env.STARS_LOCAL_USER??'admin',passwd:process.env.STARS_LOCAL_PASSWORD??'admin'}),signal:AbortSignal.timeout(10000)});
   if(!login.ok)throw Error(`Local login ${login.status}`);
   const cookies=login.headers.getSetCookie();if(!cookies.length)throw Error('No local session cookie');
   res.setHeader('Set-Cookie',cookies.map(c=>c.replace(/;\s*Domain=[^;]+/gi,'')));res.setHeader('Cache-Control','no-store');
   if(!['/launch','/games'].includes(url.pathname)){res.writeHead(302,{Location:'/launch'}).end();return;}
  }catch(e){console.error(e.message);res.writeHead(503,{'Content-Type':'text/plain; charset=utf-8'}).end('Local game session is unavailable. See tmp/launch logs.');return;}
 }
 let filename;try{filename=path.resolve(root,'.'+decodeURIComponent(url.pathname));}catch{res.writeHead(400).end();return;}
 if(filename!==root&&!filename.startsWith(root+path.sep)){res.writeHead(403).end();return;}
 if(!fs.existsSync(filename)||!fs.statSync(filename).isFile())filename=path.join(root,'index.html');
 if(!fs.existsSync(filename)){res.writeHead(503).end('Compiled game interface is missing.');return;}
 res.setHeader('Content-Type',mime[path.extname(filename)]??'application/octet-stream');
 res.setHeader('Cache-Control',filename.endsWith('index.html')?'no-store':'public,max-age=3600');
 if(req.method==='HEAD'){res.end();return;}
 fs.createReadStream(filename).on('error',()=>res.destroy()).pipe(res);
});
server.listen(port,'127.0.0.1',()=>console.log(`tomek-stars ready at http://127.0.0.1:${port}`));

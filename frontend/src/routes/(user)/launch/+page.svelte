<script lang="ts">
	const stars=Array.from({length:100},(_,i)=>({x:(i*73+19)%101,y:(i*i*31+7)%103,size:i%13===0?2:1,bright:i%5===0}));
</script>

<svelte:head><title>tomek-stars - Main menu</title></svelte:head>
<section class="classic-launch" aria-label="tomek-stars main menu">
	<div class="starfield" aria-hidden="true">{#each stars as star,i (i)}<i class:bright={star.bright} style={`left:${star.x}%;top:${star.y}%;width:${star.size}px;height:${star.size}px`}></i>{/each}</div>
	<div class="launch-window">
		<header class="titlebar"><span class="mini-star">*</span> tomek-stars <span class="titlebar-right">Local strategy game</span></header>
		<div class="window-body">
			<div class="splash">
				<div class="edition">EXPLORE  /  EXPAND  /  EXPLOIT  /  EXTERMINATE</div>
				<h1>tomek-stars</h1>
				<div class="orbit" aria-hidden="true"></div>
				<div class="ship" aria-hidden="true"><img src="/images/eee/launch-cruiser.png" alt="" width="160" height="100" /></div>
				<div class="subtitle">The galaxy awaits your orders.</div>
			</div>
			<nav class="launch-buttons" aria-label="Game mode">
				<a class="classic-button primary" href="/single-player-game"><span>N</span>ew game</a>
				<a class="classic-button" href="/games"><span>L</span>oad game</a>
				<div class="menu-divider"></div>
				<a class="classic-button" href="/races"><span>R</span>ace wizard</a>
				<a class="classic-button" href="/techs"><span>T</span>echnologies</a>
				<a class="classic-button" href="/host-game"><span>M</span>ultiplayer</a>
 <a class="classic-button" href="/credits"><span>C</span>redits</a>
			</nav>
		</div>
		<footer><span>tomek-stars / Based on Craig’s craig-stars</span><span>Single player / local saves</span></footer>
	</div>
</section>

<style>
	.classic-launch{position:fixed;inset:0;z-index:30;display:grid;place-items:center;overflow:auto;padding:28px;background:#02050b;color:#111;font-family:Tahoma,Arial,sans-serif}
	.starfield{position:absolute;inset:0;overflow:hidden;background:radial-gradient(ellipse at 32% 60%,#10172877,transparent 54%)}
	.starfield i{position:absolute;background:#868f9a}.starfield i.bright{background:#eff5ff;box-shadow:0 0 4px #8aafd0}
	.launch-window{position:relative;width:min(780px,100%);background:#bdbdbd;border:2px solid;border-color:#fff #363636 #363636 #fff;box-shadow:3px 3px 0 #000,0 18px 70px #0009;padding:3px}
	.titlebar{height:27px;display:flex;align-items:center;gap:7px;background:linear-gradient(90deg,#000060,#245483);color:#fff;font-weight:bold;font-size:13px;padding:0 7px}.mini-star{color:#ffe59a;font-size:21px}.titlebar-right{margin-left:auto;font-weight:normal;font-size:11px;color:#ccd7e1}
	.window-body{display:grid;grid-template-columns:1fr 240px;gap:26px;padding:24px 25px 26px}.splash{position:relative;min-height:300px;background:#02040a;border:2px inset #e3e3e3;overflow:hidden;color:#c7c6ad;display:flex;flex-direction:column;align-items:center}
	.splash:before{content:'';position:absolute;inset:0;background-image:radial-gradient(#c4d3df 1px,transparent 1px),radial-gradient(#6d86a0 1px,transparent 1px);background-size:51px 67px,87px 43px;background-position:11px 17px,29px 8px;opacity:.5}
	.edition{position:relative;margin-top:24px;letter-spacing:1.5px;font-size:8px;color:#a6b0ba}h1{position:relative;font:italic bold clamp(36px,6vw,62px) Georgia,serif;letter-spacing:-3px;line-height:1.25;margin:9px 0 0;color:#eacb70;text-shadow:2px 2px 0 #624311,-1px -1px #fff0b8,0 0 16px #8a672e44}
	.orbit{position:absolute;width:285px;height:80px;left:50%;top:135px;transform:translateX(-50%) rotate(-14deg);border:1px solid #93856466;border-radius:50%}.ship{position:relative;height:100px;width:160px;margin-top:4px;display:grid;place-items:center}.ship img{width:160px;height:100px;object-fit:contain;filter:drop-shadow(7px 9px 5px #000)}
	.subtitle{position:relative;font:11px Tahoma,Arial,sans-serif;letter-spacing:.3px;margin:4px 0 20px;color:#a7b1bb}.launch-buttons{display:flex;flex-direction:column;justify-content:center;gap:13px}.classic-button{display:block;text-align:center;padding:11px 12px;background:#c6c6c6;color:#111;border:2px solid;border-color:#fff #4c4c4c #4c4c4c #fff;box-shadow:inset -1px -1px #888,inset 1px 1px #dfdfdf;font-size:15px;text-decoration:none;font-weight:normal}.classic-button span{text-decoration:underline}.classic-button.primary{outline:1px solid #111;outline-offset:2px}.classic-button:hover{background:#d7d7d7}.classic-button:active{border-color:#4c4c4c #fff #fff #4c4c4c;transform:translate(1px,1px)}.classic-button:focus-visible{outline:2px dotted #111;outline-offset:-6px}.menu-divider{height:2px;background:#929292;border-bottom:1px solid #fff;margin:1px 0}
	footer{display:flex;justify-content:space-between;gap:10px;border-top:1px solid #888;box-shadow:inset 0 1px #fff;padding:9px 10px 6px;font-size:10px;color:#474747}
	@media(max-width:620px){.classic-launch{padding:16px}.window-body{grid-template-columns:1fr;padding:15px;gap:18px}.splash{min-height:255px}.edition{font-size:7px}.launch-buttons{gap:10px}.titlebar-right{display:none}.ship{height:80px}footer{font-size:9px}}
</style>

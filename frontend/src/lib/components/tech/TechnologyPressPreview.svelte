<script lang="ts">
 import {techs,tooltipComponent} from '#lib/services/Stores.js';
 import {onTechTooltip} from '#lib/components/game/tooltips/TechTooltip.js';
 let {name}:{name:string}=$props();
 function close(){tooltipComponent.set(undefined);}
 function keydown(e:KeyboardEvent){if((e.key==='Enter'||e.key===' ')&&!e.repeat){e.preventDefault();const rect=e.currentTarget instanceof HTMLElement?e.currentTarget.getBoundingClientRect():new DOMRect();onTechTooltip(new MouseEvent('pointerdown',{clientX:rect.left,clientY:rect.bottom}),$techs.getTech(name));}}
</script>
<svelte:window onpointerup={close} onpointercancel={close} onblur={close} />
<button type="button" class="link link-primary inline text-left" title="Hold to preview" aria-label={`Preview ${name}`} onpointerdown={event=>onTechTooltip(event,$techs.getTech(name))} onpointerup={close} onpointercancel={close} onkeydown={keydown} onkeyup={event=>{if(event.key==='Enter'||event.key===' ')close();}} onblur={close} oncontextmenu={event=>event.preventDefault()} onclick={event=>event.preventDefault()}>{name}</button>

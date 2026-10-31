import {expect,it} from 'vitest';
import {rememberScannerViewport,recalledScannerViewport} from './scannerViewport.js';
it('retains viewport per game and player without aliasing or invalid state',()=>{
 const view={x:1234,y:5678,scale:45};rememberScannerViewport('1:1',view);view.scale=1;
 expect(recalledScannerViewport('1:1')).toEqual({x:1234,y:5678,scale:45});
 expect(recalledScannerViewport('1:2')).toBeUndefined();
 rememberScannerViewport('1:1',{x:NaN,y:0,scale:5});
 expect(recalledScannerViewport('1:1')?.scale).toBe(45);
});

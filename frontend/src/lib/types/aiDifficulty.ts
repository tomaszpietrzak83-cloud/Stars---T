import { AiDifficulty } from './cs-proto.js';
export const difficultyLevels = [
 {value:AiDifficulty.EASY,label:'Easy'},
 {value:AiDifficulty.SKILLED,label:'Skilled'},
 {value:AiDifficulty.NORMAL,label:'Normal'},
 {value:AiDifficulty.ADVANCED,label:'Advanced'},
 {value:AiDifficulty.HARD,label:'Hard'},
 {value:AiDifficulty.ELITE,label:'Elite'},
 {value:AiDifficulty.CHEATER,label:'Cheater'}
];
// Symmetric relative weights. Out-of-range choices are excluded, not clamped.
export function variedDifficulty(base: AiDifficulty, random= Math.random): AiDifficulty {
 const center=Math.max(0,difficultyLevels.findIndex(level=>level.value===base));
 const weights=[40,18,8,3,1];
 const choices=difficultyLevels.map((level,index)=>({value:level.value,weight:weights[Math.abs(index-center)]??0}));
 let draw=random()*choices.reduce((sum,choice)=>sum+choice.weight,0);
 for(const choice of choices) {draw-=choice.weight;if(draw<0) return choice.value;}
 return base;
}

// Relative to the same race/personality on Normal; production pace is a
// planning target, not a free resource or income multiplier.
export const difficultyDetails=[
 {name:'Easy',colonizers:2,pace:'0.50',attack:'+10',research:0},
 {name:'Skilled',colonizers:3,pace:'0.75',attack:'+5',research:0},
 {name:'Normal',colonizers:4,pace:'1.00',attack:'0',research:0},
 {name:'Advanced',colonizers:6,pace:'1.25',attack:'−2',research:8},
 {name:'Hard',colonizers:8,pace:'1.50',attack:'−5',research:15},
 {name:'Elite',colonizers:8,pace:'1.50',attack:'−5',research:15},
 {name:'Cheater',colonizers:8,pace:'1.50',attack:'−5',research:15}
];

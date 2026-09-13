import fs from 'node:fs';
import postcss from 'postcss';
const root=postcss.parse(fs.readFileSync('src/app-layout.css','utf8'));
root.walkRules(rule=>{
 const selectors=rule.selectors.filter(s=>{
   if(/top-navigation|window-control|titlebar|mac-controls/.test(s)) return true;
   return !/(\bbutton\b|\binput\b|\btextarea\b|select-trigger|kit-switch|kit-dialog|kit-select|kit-menu)/.test(s);
 });
 if(selectors.length)rule.selectors=selectors; else rule.remove();
});
fs.writeFileSync('src/app-layout.css',root.toString());

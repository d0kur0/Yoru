import fs from 'node:fs';
import postcss from 'postcss';
let output='';
for(const file of ['styles.css','desktop.css','legacy-kit.css','foundation.css']) {
 const root=postcss.parse(fs.readFileSync('src/'+file,'utf8'));
 root.walkRules(rule=>{if(rule.selector.includes(':root'))rule.remove()});
 output+=root.toString()+'\n';
}
fs.writeFileSync('src/app-layout.css',output);
let css=fs.readFileSync('src/hero.css','utf8');
css=css.replace('@layer legacy, theme, base, components, utilities;','@layer theme, base, legacy, components, utilities;');
css=css.replace(/@import '\.\/(styles|desktop|legacy-kit|foundation)\.css' layer\(legacy\);\n/g,'');
css=css.replace("@import '@heroui/styles';", "@import '@heroui/styles';\n@import './app-layout.css' layer(legacy);");
fs.writeFileSync('src/hero.css',css);

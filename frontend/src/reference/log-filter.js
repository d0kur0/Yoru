export function filterLogs(content,query='',level=''){
 const accepted={debug:['debug','info','warn','warning','error','fatal','panic'],info:['info','warn','warning','error','fatal','panic'],warning:['warn','warning','error','fatal','panic'],error:['error','fatal','panic']}[level];
 return content.split('\n').filter(line=>{
  if(!line.trim()||!line.toLowerCase().includes(query.toLowerCase()))return false;
  if(!accepted)return true;
  const severity=line.match(/(?:^|\s)level=["']?(debug|info|warning|warn|error|fatal|panic)["']?(?=\s|$)/i)?.[1]?.toLowerCase();
  return accepted.includes(severity);
 }).join('\n');
}

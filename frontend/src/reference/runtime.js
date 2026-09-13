export const emptyConfig = () => ({sets:[],logging:{enabled:true,level:'info',maxSizeMB:5,files:5,days:7},tunOptions:{stack:'',mtu:0,interface:''},servers:[],selected:'',rules:[],subscriptions:[],dns:{bootstrap:['1.1.1.1','9.9.9.9'],nodeResolvers:[],directResolvers:[],respectRules:false,enabled:true,mode:'fake-ip',servers:['https://dns.quad9.net/dns-query'],fallback:[],exclusions:['+.lan','+.local','localhost'],policies:[],hijack:true,ipv6:false},settings:{tun:true,systemProxy:false,mode:'rule',autostart:false,minimized:false,autoConnect:false,tray:true,sniffer:true,ipv6:false,routeExclusions:['192.168.0.0/16','10.0.0.0/8','172.16.0.0/12']},defaultAction:'DIRECT'});

export function bytes(value) {
  const n=Number(value)||0;
  if(n>=1024**3)return (n/1024**3).toFixed(2)+' ГБ';
  if(n>=1024**2)return (n/1024**2).toFixed(1)+' МБ';
  if(n>=1024)return (n/1024).toFixed(1)+' КБ';
  return n+' Б';
}
export function sample(previous,status,now=Date.now()) {
  if(!status.running)return {time:now,started:0,download:0,upload:0,down:0,up:0};
  const dt=previous&&previous.started===status.started ? (now-previous.time)/1000 : 0;
  return {time:now,started:status.started,download:status.downloadTotal||0,upload:status.uploadTotal||0,
    down:dt>0?Math.max(0,(status.downloadTotal-previous.download)/dt):0,
    up:dt>0?Math.max(0,(status.uploadTotal-previous.upload)/dt):0};
}

export function decodeBackup(data) {
  if(!data||!['mihomo-desktop','mihomo-desktop-prototype'].includes(data.format)||data.version!==1)throw Error('Нужен файл настроек Mihomo Desktop версии 1.');
  const c=data.config;
  if(!c||!Array.isArray(c.servers)||!Array.isArray(c.rules)||!Array.isArray(c.subscriptions)||!c.dns||!c.settings)throw Error('В файле отсутствуют обязательные настройки.');
  return structuredClone(c);
}

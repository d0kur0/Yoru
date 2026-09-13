export const ruleTypes = [
 {id:'DOMAIN-SUFFIX',label:'Домен и его поддомены',description:'Совпадает сам домен и все его поддомены. Вводите без +., звёздочки и https://.',example:'example.com → example.com, api.example.com. Не other-example.com.',placeholder:'example.com'},
 {id:'DOMAIN',label:'Точный домен',description:'Совпадает только это доменное имя. Поддомены не включаются.',example:'example.com → только example.com. Не api.example.com.',placeholder:'example.com'},
 {id:'DOMAIN-KEYWORD',label:'Подстрока в доменном имени',description:'Ищет указанную последовательность символов в любой части домена, без границ слов.',example:'example → example.com, myexample.org, example-test.net.',placeholder:'example'},
 {id:'IP-CIDR',label:'IP-адрес или подсеть · IPv4',description:'Проверяет IP назначения. Для одного адреса используйте /32.',example:'192.168.1.0/24 → 192.168.1.0–192.168.1.255. Один IP: 192.168.1.10/32.',placeholder:'192.168.1.0/24'},
 {id:'IP-CIDR6',label:'IP-адрес или подсеть · IPv6',description:'Проверяет IPv6 назначения. Для одного адреса используйте /128.',example:'fd00::/8 → локальная IPv6-подсеть. Один IP: fd00::1/128.',placeholder:'fd00::/8'},
 {id:'PROCESS-NAME',label:'Точное имя процесса',description:'Имя исполняемого файла, а не название окна. У приложения могут быть отдельные вспомогательные процессы.',example:'Windows: chrome.exe. macOS: Google Chrome или Google Chrome Helper.',placeholder:'chrome.exe'},
 {id:'PROCESS-NAME-WILDCARD',label:'Имя процесса по маске',description:'* означает любое количество символов, ? — ровно один символ. Маска охватывает всё имя.',example:'*Chrome* → Google Chrome и Google Chrome Helper. app?.exe → app1.exe.',placeholder:'*Chrome*'},
 {id:'PROCESS-PATH',label:'Точный путь к процессу',description:'Полный путь к исполняемому файлу. На macOS выбирайте файл внутри .app, а не сам пакет приложения.',example:'/Applications/Telegram.app/Contents/MacOS/Telegram',placeholder:'/Applications/Telegram.app/Contents/MacOS/Telegram'},
 {id:'PROCESS-PATH-WILDCARD',label:'Путь к процессу по маске',description:'Маска полного пути: * — любое количество символов, ? — один символ.',example:'/Applications/Google Chrome.app/* → процессы внутри пакета Chrome.',placeholder:'/Applications/Google Chrome.app/*'},
];
export const ruleType = id => ruleTypes.find(t=>t.id===id) || ruleTypes[0];

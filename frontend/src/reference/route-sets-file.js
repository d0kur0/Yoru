const fields=['name','enabled','domains','keywords','processes','cidrs','action','resolvers','realIP','bypassTUN'];
function signature(set){return JSON.stringify(fields.map(key=>set[key]??(['domains','keywords','processes','cidrs','resolvers'].includes(key)?[]:null)))}
// Identical imports are skipped; changed sets are added as independent copies.
export function newRouteSets(existing,incoming){
 const known=new Set(existing.map(signature)),ids=new Set(existing.map(set=>set.id));
 const added=[];
 for(const set of incoming){
  const key=signature(set);if(known.has(key))continue;
  const copy=structuredClone(set);
  while(!copy.id||ids.has(copy.id))copy.id=crypto.randomUUID();
  ids.add(copy.id);known.add(key);added.push(copy);
 }
 return added;
}
